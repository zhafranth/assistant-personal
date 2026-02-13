package bot

import (
	"context"
	"log/slog"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/module/expense"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/project"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
	"github.com/zhafrantharif/personal-assistant-bot/internal/nlp"
	tele "gopkg.in/telebot.v4"
)

type Handler struct {
	nlpSvc     *nlp.Service
	todoSvc    *todo.Service
	expenseSvc *expense.Service
	projectSvc *project.Service
	timezone   *time.Location
}

func NewHandler(nlpSvc *nlp.Service, todoSvc *todo.Service, expenseSvc *expense.Service, projectSvc *project.Service, timezone *time.Location) *Handler {
	return &Handler{
		nlpSvc:     nlpSvc,
		todoSvc:    todoSvc,
		expenseSvc: expenseSvc,
		projectSvc: projectSvc,
		timezone:   timezone,
	}
}

func (h *Handler) Register(b *tele.Bot) {
	b.Handle(tele.OnText, h.handleText)
	b.Handle("/help", h.handleHelp)
	b.Handle("/start", h.handleHelp)
	b.Handle("/todos", h.handleTodos)
	b.Handle("/expenses", h.handleExpenses)
	b.Handle("/projects", h.handleProjects)
}

func (h *Handler) handleText(c tele.Context) error {
	ctx := context.Background()
	userID := c.Sender().ID
	text := c.Text()

	slog.Info("received message", "user_id", userID, "text", text)

	intent, err := h.nlpSvc.Parse(ctx, text)
	if err != nil {
		slog.Error("nlp parse failed", "error", err)
		return c.Send("⚠️ Maaf, terjadi kesalahan. Coba lagi nanti.")
	}

	slog.Info("parsed intent", "intent", intent.Intent, "user_id", userID)

	resp, err := h.route(ctx, userID, intent)
	if err != nil {
		slog.Error("handler error", "intent", intent.Intent, "error", err)
		return c.Send("⚠️ Maaf, terjadi kesalahan saat memproses permintaan kamu.")
	}

	return c.Send(resp)
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
		return FormatTodoList(todos, filter, h.timezone), nil

	case "complete_todo":
		return h.todoSvc.Complete(ctx, userID, intent.Search)

	case "delete_todo":
		return h.todoSvc.Delete(ctx, userID, intent.Search)

	// === Expense ===
	case "add_expense":
		return h.expenseSvc.Add(ctx, userID, intent.Description, intent.Amount)

	case "list_expense":
		filter := intent.Filter
		if filter == "" {
			filter = "this_month"
		}
		return h.expenseSvc.List(ctx, userID, filter)

	case "delete_expense":
		return h.expenseSvc.Delete(ctx, userID, intent.Search)

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
	todos, err := h.todoSvc.List(ctx, userID, "pending")
	if err != nil {
		slog.Error("list todos failed", "error", err)
		return c.Send("⚠️ Gagal mengambil daftar todo.")
	}
	return c.Send(FormatTodoList(todos, "pending", h.timezone))
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
• "ingetin bayar listrik besok"
• "ingetin bayar wifi tiap tanggal 5"
• "list todo"
• "selesaiin todo beli susu"
• "hapus todo beli susu"

💰 Pengeluaran:
• "catat makan siang 35rb"
• "bayar parkir 5000"
• "pengeluaran hari ini"
• "pengeluaran bulan ini"
• "hapus pengeluaran parkir"

📁 Project:
• "buat project Laundry App deadline April"
• "tambah goal di Laundry App: bikin wireframe"
• "list project"
• "progress Laundry App"
• "hapus project Laundry App"

⌨️ Shortcut Commands:
/todos — List todo pending
/expenses — Pengeluaran bulan ini
/projects — List semua project
/help — Tampilkan bantuan ini`
}
