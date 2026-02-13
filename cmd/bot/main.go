package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/bot"
	"github.com/zhafrantharif/personal-assistant-bot/internal/config"
	"github.com/zhafrantharif/personal-assistant-bot/internal/db"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/expense"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/project"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
	"github.com/zhafrantharif/personal-assistant-bot/internal/nlp"
	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
	tele "gopkg.in/telebot.v4"
)

func main() {
	// Setup structured logging
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Load timezone
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		slog.Error("failed to load timezone", "timezone", cfg.Timezone, "error", err)
		os.Exit(1)
	}

	// Connect to database
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Run migrations
	if err := db.RunMigrations(database); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Initialize Telegram bot
	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramBotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		slog.Error("failed to create telegram bot", "error", err)
		os.Exit(1)
	}

	// Initialize repositories
	reminderRepo := reminder.NewRepository(database)
	todoRepo := todo.NewRepository(database)
	expenseRepo := expense.NewRepository(database)
	projectRepo := project.NewRepository(database)

	// Initialize services
	nlpSvc := nlp.NewService(cfg.AnthropicAPIKey, loc)
	todoSvc := todo.NewService(todoRepo, reminderRepo, loc)
	expenseSvc := expense.NewService(expenseRepo, loc)
	projectSvc := project.NewService(projectRepo, reminderRepo, loc)

	// Register bot handlers
	handler := bot.NewHandler(nlpSvc, todoSvc, expenseSvc, projectSvc, loc)
	handler.Register(b)

	// Start reminder scheduler
	schedulerInterval := time.Duration(cfg.SchedulerIntervalSec) * time.Second
	scheduler := reminder.NewScheduler(reminderRepo, b, schedulerInterval, loc)
	go scheduler.Start()

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		slog.Info("received shutdown signal", "signal", sig)

		scheduler.Stop()
		b.Stop()
		database.Close()
		slog.Info("shutdown complete")
		os.Exit(0)
	}()

	slog.Info("bot started", "timezone", cfg.Timezone, "scheduler_interval", schedulerInterval)
	b.Start()
}
