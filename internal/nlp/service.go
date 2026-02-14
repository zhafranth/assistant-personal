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
- "edit todo beli susu jadi beli madu" → 1 elemen edit_todo dengan search="beli susu", title="beli madu"

INTENTS:
- add_todo: {title, reminder?, remind_at?, recurring?, due_date?}
- complete_todo: {search}
- list_todo: {filter: "all"|"today"|"pending"}
- delete_todo: {search}
- edit_todo: {search, title?, due_date?, remind_at?}
- add_expense: {description, amount}
- list_expense: {filter: "today"|"this_week"|"this_month"|"all"}
- delete_expense: {search}
- add_project: {name, due_date?, description?}
- add_goal: {project, title, due_date?, reminder?, remind_at?, recurring?}
- complete_goal: {project, search}
- list_project: {}
- show_project: {project}
- delete_project: {project}
- delete_goal: {project, search}
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
