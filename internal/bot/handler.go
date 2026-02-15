package bot

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/module/expense"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/project"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
	"github.com/zhafrantharif/personal-assistant-bot/internal/nlp"
	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
	tele "gopkg.in/telebot.v4"
)

type Handler struct {
	nlpSvc       *nlp.Service
	todoSvc      *todo.Service
	expenseSvc   *expense.Service
	projectSvc   *project.Service
	reminderRepo *reminder.Repository
	timezone     *time.Location
}

func NewHandler(nlpSvc *nlp.Service, todoSvc *todo.Service, expenseSvc *expense.Service, projectSvc *project.Service, reminderRepo *reminder.Repository, timezone *time.Location) *Handler {
	return &Handler{
		nlpSvc:       nlpSvc,
		todoSvc:      todoSvc,
		expenseSvc:   expenseSvc,
		projectSvc:   projectSvc,
		reminderRepo: reminderRepo,
		timezone:     timezone,
	}
}

func (h *Handler) Register(b *tele.Bot) {
	b.Handle(tele.OnText, h.handleText)
	b.Handle("/help", h.handleHelp)
	b.Handle("/start", h.handleHelp)
	b.Handle("/todos", h.handleTodos)
	b.Handle("/daily", h.handleDaily)
	b.Handle("/expenses", h.handleExpenses)
	b.Handle("/projects", h.handleProjects)
}

func (h *Handler) handleText(c tele.Context) error {
	ctx := context.Background()
	userID := c.Sender().ID
	text := c.Text()

	slog.Info("received message", "user_id", userID, "text", text)

	intents, err := h.nlpSvc.Parse(ctx, text)
	if err != nil {
		slog.Error("nlp parse failed", "error", err)
		return c.Send("⚠️ Maaf, terjadi kesalahan. Coba lagi nanti.")
	}

	slog.Info("parsed intents", "count", len(intents), "user_id", userID)

	var responses []string
	for _, intent := range intents {
		resp, err := h.route(ctx, userID, &intent)
		if err != nil {
			slog.Error("handler error", "intent", intent.Intent, "error", err)
			responses = append(responses, "⚠️ Maaf, terjadi kesalahan saat memproses permintaan kamu.")
			continue
		}
		responses = append(responses, resp)
	}

	return c.Send(strings.Join(responses, "\n\n"))
}

func (h *Handler) route(ctx context.Context, userID int64, intent *nlp.ParsedIntent) (string, error) {
	switch intent.Intent {
	// === Todo ===
	case "add_todo":
		remindAt, _ := intent.ParseRemindAt(h.timezone)
		dueDate, _ := intent.ParseDueDate(h.timezone)
		return h.todoSvc.Add(ctx, userID, intent.Title, dueDate, intent.Reminder, remindAt, intent.Recurring)

	case "list_todo":
		filter := intent.Filter
		if filter == "" {
			filter = "all"
		}
		todos, err := h.todoSvc.List(ctx, userID, filter)
		if err != nil {
			return "", err
		}
		reminders, err := h.reminderRepo.ListActiveByUser(ctx, userID)
		if err != nil {
			slog.Error("list reminders for todo list failed", "error", err)
			reminders = nil
		}
		return FormatTodoList(todos, filter, h.timezone, reminders), nil

	case "daily_briefing":
		return h.dailyBriefing(ctx, userID)

	case "complete_todo":
		return h.todoSvc.Complete(ctx, userID, intent.Search)

	case "edit_todo":
		dueDate, _ := intent.ParseDueDate(h.timezone)
		remindAt, _ := intent.ParseRemindAt(h.timezone)
		return h.todoSvc.Edit(ctx, userID, intent.Search, intent.Title, dueDate, remindAt)

	case "clear_todo":
		return h.todoSvc.ClearAll(ctx, userID)

	case "delete_todo":
		return h.todoSvc.Delete(ctx, userID, intent.Search)

	// === Expense ===
	case "add_expense":
		isPaid := true
		if intent.IsPaid != nil {
			isPaid = *intent.IsPaid
		}
		return h.expenseSvc.Add(ctx, userID, intent.Description, intent.Amount, isPaid)

	case "pay_expense":
		date, _ := intent.ParseDate(h.timezone)
		return h.expenseSvc.PayExpense(ctx, userID, intent.Search, intent.Amount, date)

	case "list_expense":
		filter := intent.Filter
		if filter == "" {
			filter = "this_month"
		}
		return h.expenseSvc.List(ctx, userID, filter)

	case "delete_expense":
		date, _ := intent.ParseDate(h.timezone)
		return h.expenseSvc.Delete(ctx, userID, intent.ExpenseID, intent.Search, intent.Amount, date)

	case "edit_expense":
		date, _ := intent.ParseDate(h.timezone)
		return h.expenseSvc.Edit(ctx, userID, intent.ExpenseID, intent.Search, intent.Amount, date, intent.NewTitle, intent.NewIsPaid)

	case "clear_expense":
		return h.expenseSvc.ClearByMonth(ctx, userID, intent.Month, intent.Year)

	// === Project ===
	case "add_project":
		dueDate, _ := intent.ParseDueDate(h.timezone)
		var desc *string
		if intent.Description != "" {
			desc = &intent.Description
		}
		return h.projectSvc.Add(ctx, userID, intent.Name, desc, dueDate)

	case "add_goal":
		remindAt, _ := intent.ParseRemindAt(h.timezone)
		dueDate, _ := intent.ParseDueDate(h.timezone)
		return h.projectSvc.AddGoal(ctx, userID, intent.Project, intent.Title, dueDate, intent.Reminder, remindAt, intent.Recurring)

	case "complete_goal":
		return h.projectSvc.CompleteGoal(ctx, userID, intent.Project, intent.Search)

	case "list_project":
		return h.projectSvc.List(ctx, userID)

	case "show_project":
		return h.projectSvc.Show(ctx, userID, intent.Project)

	case "delete_project":
		return h.projectSvc.Delete(ctx, userID, intent.Project)

	case "delete_goal":
		return h.projectSvc.DeleteGoal(ctx, userID, intent.Project, intent.Search)

	// === Help ===
	case "help":
		return helpText(), nil

	// === Unknown ===
	default:
		return "🤔 Maaf, saya tidak mengerti. Ketik /help untuk bantuan.", nil
	}
}

func (h *Handler) handleHelp(c tele.Context) error {
	return c.Send(helpText())
}

func (h *Handler) handleTodos(c tele.Context) error {
	ctx := context.Background()
	userID := c.Sender().ID
	todos, err := h.todoSvc.List(ctx, userID, "all")
	if err != nil {
		slog.Error("list todos failed", "error", err)
		return c.Send("⚠️ Gagal mengambil daftar todo.")
	}
	reminders, err := h.reminderRepo.ListActiveByUser(ctx, userID)
	if err != nil {
		slog.Error("list reminders failed", "error", err)
		reminders = nil
	}
	return c.Send(FormatTodoList(todos, "all", h.timezone, reminders))
}

func (h *Handler) handleDaily(c tele.Context) error {
	ctx := context.Background()
	userID := c.Sender().ID
	resp, err := h.dailyBriefing(ctx, userID)
	if err != nil {
		slog.Error("daily briefing failed", "error", err)
		return c.Send("⚠️ Gagal membuat daily briefing.")
	}
	return c.Send(resp)
}

func (h *Handler) dailyBriefing(ctx context.Context, userID int64) (string, error) {
	todos, err := h.todoSvc.List(ctx, userID, "pending")
	if err != nil {
		return "", err
	}
	reminders, err := h.reminderRepo.ListActiveByUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return FormatDailyBriefing(todos, h.timezone, reminders), nil
}

func (h *Handler) handleExpenses(c tele.Context) error {
	ctx := context.Background()
	userID := c.Sender().ID
	resp, err := h.expenseSvc.List(ctx, userID, "this_month")
	if err != nil {
		slog.Error("list expenses failed", "error", err)
		return c.Send("⚠️ Gagal mengambil daftar pengeluaran.")
	}
	return c.Send(resp)
}

func (h *Handler) handleProjects(c tele.Context) error {
	ctx := context.Background()
	userID := c.Sender().ID
	resp, err := h.projectSvc.List(ctx, userID)
	if err != nil {
		slog.Error("list projects failed", "error", err)
		return c.Send("⚠️ Gagal mengambil daftar project.")
	}
	return c.Send(resp)
}

func helpText() string {
	return `🤖 Personal Assistant Bot

📋 Todo & Reminder:
• "tambah todo beli susu"
• "tambah todo beli susu, beli roti, beli kopi" (bulk)
• "edit todo beli susu jadi beli madu"
• "ingetin bayar listrik besok"
• "ingetin bayar wifi tiap tanggal 5"
• "list todo"
• "selesaiin todo beli susu"
• "hapus todo beli susu"
• "hapus todo A, selesaikan todo B" (bulk)

💰 Pengeluaran:
• "catat makan siang 35rb"
• "catat makan siang 35rb, bensin 50rb" (bulk)
• "catat hutang sewa kos 1.5jt" (belum lunas)
• "lunasi sewa kos"
• "lunasi beli kecap 20rb" (jika nama sama, sebut harga)
• "hapus beli kecap 14 feb" (filter by tanggal)
• "ganti nama bensin jadi bensin motor"
• "tandai beli kecap 20rb sudah lunas"
• "kosongkan februari 2026"
• "pengeluaran hari ini"
• "pengeluaran bulan ini"
• "semua pengeluaran"
• "hapus pengeluaran parkir"

📁 Project:
• "buat project Laundry App deadline April"
• "tambah goal di Laundry App: bikin wireframe"
• "list project"
• "progress Laundry App"
• "hapus project Laundry App"

⌨️ Shortcut Commands:
/todos — List semua todo
/daily — Daily briefing + reminder bulanan
/expenses — Pengeluaran bulan ini
/projects — List semua project
/help — Tampilkan bantuan ini`
}
