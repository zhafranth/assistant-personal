package nlp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type Service struct {
	client   anthropic.Client
	timezone *time.Location
}

func NewService(apiKey string, timezone *time.Location) *Service {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &Service{
		client:   client,
		timezone: timezone,
	}
}

func (s *Service) Parse(ctx context.Context, userMessage string) ([]ParsedIntent, error) {
	now := time.Now().In(s.timezone)
	tomorrow := now.AddDate(0, 0, 1)
	dayAfterTomorrow := now.AddDate(0, 0, 2)

	systemPrompt := fmt.Sprintf(`Kamu adalah parser untuk personal assistant bot. Tugas kamu HANYA mengubah pesan user menjadi JSON array.

Hari ini: %s
Timezone: %s

RULES:
- Output HANYA JSON array (selalu dalam bentuk array [...]), tanpa markdown, tanpa penjelasan
- Jika user melakukan 1 aksi, return array dengan 1 elemen. Jika multiple aksi, return array dengan banyak elemen
- Format tanggal: due_date = "YYYY-MM-DD", remind_at = "YYYY-MM-DDTHH:MM:SS+07:00" (RFC3339 dengan timezone Asia/Jakarta)
- Jika user sebut tanggal tanpa jam, default jam 07:00 WIB
- Jika user menyebut jam/waktu, SELALU set reminder=true dan remind_at dengan waktu tersebut
- "besok" = %s
- "lusa" = %s
- Nominal uang: "35rb" = 35000, "1.5jt" = 1500000, "1juta" = 1000000
- "minggu depan" = 7 hari dari sekarang
- "bulan depan" = 1 bulan dari sekarang, gunakan hari terakhir bulan tersebut untuk due_date jika tidak spesifik
- Jika tidak bisa parsing, return: [{"intent": "unknown", "raw": "<pesan asli>"}]

CONTOH BULK:
- "tambah todo beli susu, beli roti, dan beli kopi" → 3 elemen add_todo
- "hapus todo beli susu dan selesaikan todo beli roti" → 1 delete_todo + 1 complete_todo
- "done makan mie dan cuci piring" → 2 elemen complete_todo (search="makan mie", search="cuci piring")
- "edit todo beli susu jadi beli madu" → 1 elemen edit_todo dengan search="beli susu", title="beli madu"
- "kosongkan todo" → 1 elemen clear_todo (tanpa nama spesifik = hapus semua)
- "buat done semua todo" → 1 elemen clear_todo HANYA jika tidak ada nama spesifik yang disebutkan
- "lihat goals Laundry App" → show_project dengan project="Laundry App"
- "progress Laundry App" → show_project dengan project="Laundry App"
- "list project" → list_project (tanpa project field)
- "tambah goal di Laundry App: wireframe, database, deploy" → 3 elemen add_goal dengan project="Laundry App"
- "hapus goal wireframe dan database dari Laundry App" → 2 elemen delete_goal dengan project="Laundry App"
- "done goal wireframe" → complete_goal dengan project="" (kosong, jika user tidak sebut project)
- "selesaikan goal wireframe di Laundry App" → complete_goal dengan project="Laundry App", search="wireframe"
- "catat makan siang 35rb, bensin 50rb, parkir 5rb" → 3 elemen add_expense
- "catat makan siang 35rb dan bensin 50rb" → 2 elemen add_expense
- "hapus pengeluaran parkir dan bensin" → 2 elemen delete_expense (search="parkir", search="bensin")
- "lunasi beli kecap" → 1 elemen pay_expense (BUKAN add_expense)
- "lunasi beli kecap 20rb" → 1 elemen pay_expense dengan search="beli kecap", amount=20000
- "hapus beli kecap 14 feb" → 1 elemen delete_expense dengan search="beli kecap", date="2026-02-14"
- "ganti nama bensin jadi bensin motor" → 1 elemen edit_expense dengan search="bensin", new_title="bensin motor"
- "tandai beli kecap 20rb sudah lunas" → 1 elemen edit_expense dengan search="beli kecap", amount=20000, new_is_paid=true
- "kosongkan februari 2026" → 1 elemen clear_expense dengan month=2, year=2026

INTENTS:
- add_todo: {title, reminder?, remind_at?, recurring?, due_date?}
- complete_todo: {search}
- list_todo: {filter: "all"|"today"|"pending"}
- delete_todo: {search}
- edit_todo: {search, title?, due_date?, remind_at?}
- clear_todo: {} (HANYA jika user ingin menghapus/mengosongkan semua todo sekaligus tanpa menyebut nama spesifik: "kosongkan todo", "hapus semua todo", "clear todo list". JANGAN gunakan ini jika user menyebut nama todo tertentu — gunakan complete_todo atau delete_todo per item)
- add_expense: {description, amount, is_paid?} (default is_paid=true. Set is_paid=false jika user bilang "hutang", "belum bayar", "belum lunas", "cicilan". Contoh: "catat hutang sewa kos 1.5jt" → is_paid=false. JANGAN gunakan ini untuk pesan seperti "lunasi X" atau "bayar hutang X" — itu adalah pay_expense)
- pay_expense: {search, amount?, date?} (tandai pengeluaran lunas. Contoh: "lunasi sewa kos", "lunasi beli kecap 20rb" → search="beli kecap", amount=20000. "lunasi beli kecap 14 feb" → search="beli kecap", date="2026-02-14". Jika ada nominal → isi amount. Jika ada tanggal → isi date=YYYY-MM-DD)
- list_expense: {filter: "today"|"this_week"|"this_month"|"all"}
- delete_expense: {search, amount?, date?} (hapus pengeluaran. "hapus beli kecap 100rb" → search="beli kecap", amount=100000. "hapus beli kecap 14 feb" → search="beli kecap", date="2026-02-14")
- edit_expense: {search, amount?, date?, new_title?, new_is_paid?} (edit judul atau status pengeluaran. "ganti nama bensin jadi bensin motor" → search="bensin", new_title="bensin motor". "tandai beli kecap 20rb sudah lunas" → search="beli kecap", amount=20000, new_is_paid=true. "ubah beli kecap jadi belum lunas" → search="beli kecap", new_is_paid=false)
- clear_expense: {month, year?} (hapus semua pengeluaran di bulan tertentu. month=1-12. "kosongkan februari 2026" → month=2, year=2026. "hapus semua pengeluaran februari" → month=2, year tidak diisi)
- add_project: {name, due_date?, description?}
- add_goal: {project, title, due_date?, reminder?, remind_at?, recurring?} (project WAJIB diisi. Jika bulk: tiap goal = 1 elemen dengan project yang sama)
- complete_goal: {project?, search} (project boleh kosong jika user tidak menyebutkan project)
- list_project: {} (tampilkan semua project, "list project", "project apa saja", "daftar project")
- show_project: {project} (tampilkan detail + goals satu project. "lihat project X", "detail project X", "goals X", "progress X", "tampilkan X", "apa saja goals X" → project="X")
- delete_project: {project}
- delete_goal: {project?, search} (project boleh kosong jika user tidak menyebutkan project)
- daily_briefing: {} (user minta rangkuman harian, daily briefing, "apa yang harus dikerjakan hari ini", "briefing", "rangkuman")
- help: {}
- unknown: {raw}`,
		now.Format("2006-01-02 (Monday)"),
		s.timezone.String(),
		tomorrow.Format("2006-01-02"),
		dayAfterTomorrow.Format("2006-01-02"),
	)

	intents, err := s.callAPI(ctx, systemPrompt, userMessage)
	if err != nil {
		// Retry once
		slog.Warn("NLP first attempt failed, retrying", "error", err)
		intents, err = s.callAPI(ctx, systemPrompt, userMessage)
		if err != nil {
			return nil, fmt.Errorf("nlp parse failed: %w", err)
		}
	}

	return intents, nil
}

func (s *Service) callAPI(ctx context.Context, systemPrompt, userMessage string) ([]ParsedIntent, error) {
	message, err := s.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					{OfRequestTextBlock: &anthropic.TextBlockParam{Text: userMessage}},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic api call: %w", err)
	}

	if len(message.Content) == 0 {
		return nil, fmt.Errorf("empty response from api")
	}

	text := ""
	for _, block := range message.Content {
		if block.Type == "text" {
			text = block.Text
			break
		}
	}

	// Clean potential markdown wrapping
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var intents []ParsedIntent
	if err := json.Unmarshal([]byte(text), &intents); err != nil {
		return nil, fmt.Errorf("parse json response: %w (raw: %s)", err, text)
	}

	return intents, nil
}
