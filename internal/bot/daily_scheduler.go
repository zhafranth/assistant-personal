package bot

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
	tele "gopkg.in/telebot.v4"
)

type DailyScheduler struct {
	bot          *tele.Bot
	todoRepo     *todo.Repository
	todoSvc      *todo.Service
	reminderRepo *reminder.Repository
	timezone     *time.Location
	hour         int
	minute       int
	stopCh       chan struct{}
	once         sync.Once
}

func NewDailyScheduler(bot *tele.Bot, todoRepo *todo.Repository, todoSvc *todo.Service, reminderRepo *reminder.Repository, timezone *time.Location) *DailyScheduler {
	return &DailyScheduler{
		bot:          bot,
		todoRepo:     todoRepo,
		todoSvc:      todoSvc,
		reminderRepo: reminderRepo,
		timezone:     timezone,
		hour:         7,
		minute:       30,
		stopCh:       make(chan struct{}),
	}
}

func (s *DailyScheduler) Start() {
	slog.Info("daily briefing scheduler started", "time", "07:30")
	for {
		now := time.Now().In(s.timezone)
		next := time.Date(now.Year(), now.Month(), now.Day(), s.hour, s.minute, 0, 0, s.timezone)
		if !next.After(now) {
			next = next.AddDate(0, 0, 1)
		}

		waitDuration := next.Sub(now)
		slog.Info("daily briefing next run", "at", next.Format("2006-01-02 15:04"), "in", waitDuration.Round(time.Second))

		select {
		case <-time.After(waitDuration):
			s.send()
		case <-s.stopCh:
			slog.Info("daily briefing scheduler stopped")
			return
		}
	}
}

func (s *DailyScheduler) Stop() {
	s.once.Do(func() { close(s.stopCh) })
}

func (s *DailyScheduler) send() {
	ctx := context.Background()

	userIDs, err := s.todoRepo.ListActiveUserIDs(ctx)
	if err != nil {
		slog.Error("daily briefing: failed to list users", "error", err)
		return
	}

	for _, userID := range userIDs {
		todos, err := s.todoSvc.List(ctx, userID, "pending")
		if err != nil {
			slog.Error("daily briefing: failed to list todos", "user_id", userID, "error", err)
			continue
		}

		reminders, err := s.reminderRepo.ListActiveByUser(ctx, userID)
		if err != nil {
			slog.Error("daily briefing: failed to list reminders", "user_id", userID, "error", err)
			reminders = nil
		}

		msg := FormatDailyBriefing(todos, s.timezone, reminders)
		user := &tele.User{ID: userID}
		if _, err := s.bot.Send(user, msg); err != nil {
			slog.Error("daily briefing: failed to send", "user_id", userID, "error", err)
			continue
		}

		slog.Info("daily briefing sent", "user_id", userID)
	}
}
