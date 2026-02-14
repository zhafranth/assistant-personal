package bot

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/module/expense"
	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
	tele "gopkg.in/telebot.v4"
)

type scheduledTask struct {
	hour   int
	minute int
	name   string
	fn     func()
}

type DailyScheduler struct {
	bot          *tele.Bot
	todoRepo     *todo.Repository
	todoSvc      *todo.Service
	expenseSvc   *expense.Service
	reminderRepo *reminder.Repository
	timezone     *time.Location
	stopCh       chan struct{}
	once         sync.Once
}

func NewDailyScheduler(bot *tele.Bot, todoRepo *todo.Repository, todoSvc *todo.Service, expenseSvc *expense.Service, reminderRepo *reminder.Repository, timezone *time.Location) *DailyScheduler {
	return &DailyScheduler{
		bot:          bot,
		todoRepo:     todoRepo,
		todoSvc:      todoSvc,
		expenseSvc:   expenseSvc,
		reminderRepo: reminderRepo,
		timezone:     timezone,
		stopCh:       make(chan struct{}),
	}
}

func (s *DailyScheduler) Start() {
	slog.Info("daily scheduler started", "briefing", "07:30", "overdue", "19:00", "monthly_report", "1st 08:00")

	tasks := []scheduledTask{
		{hour: 7, minute: 30, name: "daily_briefing", fn: s.sendBriefing},
		{hour: 8, minute: 0, name: "monthly_report", fn: s.sendMonthlyReport},
		{hour: 19, minute: 0, name: "overdue_followup", fn: s.sendOverdueFollowups},
	}

	for {
		now := time.Now().In(s.timezone)
		nextTask, waitDuration := s.findNextTask(now, tasks)

		slog.Info("daily scheduler next run",
			"task", nextTask.name,
			"at", now.Add(waitDuration).Format("2006-01-02 15:04"),
			"in", waitDuration.Round(time.Second),
		)

		select {
		case <-time.After(waitDuration):
			nextTask.fn()
		case <-s.stopCh:
			slog.Info("daily scheduler stopped")
			return
		}
	}
}

func (s *DailyScheduler) findNextTask(now time.Time, tasks []scheduledTask) (scheduledTask, time.Duration) {
	var best scheduledTask
	var bestDuration time.Duration
	first := true

	for _, t := range tasks {
		// Monthly report only runs on the 1st
		if t.name == "monthly_report" {
			target := s.nextFirstOfMonth(now, t.hour, t.minute)
			d := target.Sub(now)
			if first || d < bestDuration {
				best = t
				bestDuration = d
				first = false
			}
			continue
		}

		target := time.Date(now.Year(), now.Month(), now.Day(), t.hour, t.minute, 0, 0, s.timezone)
		if !target.After(now) {
			target = target.AddDate(0, 0, 1)
		}
		d := target.Sub(now)
		if first || d < bestDuration {
			best = t
			bestDuration = d
			first = false
		}
	}

	return best, bestDuration
}

func (s *DailyScheduler) nextFirstOfMonth(now time.Time, hour, minute int) time.Time {
	// If today is the 1st and the time hasn't passed yet, use today
	if now.Day() == 1 {
		target := time.Date(now.Year(), now.Month(), 1, hour, minute, 0, 0, s.timezone)
		if target.After(now) {
			return target
		}
	}
	// Otherwise, the 1st of next month
	return time.Date(now.Year(), now.Month()+1, 1, hour, minute, 0, 0, s.timezone)
}

func (s *DailyScheduler) Stop() {
	s.once.Do(func() { close(s.stopCh) })
}

func (s *DailyScheduler) sendBriefing() {
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

func (s *DailyScheduler) sendOverdueFollowups() {
	ctx := context.Background()

	userIDs, err := s.todoRepo.ListActiveUserIDs(ctx)
	if err != nil {
		slog.Error("overdue followup: failed to list users", "error", err)
		return
	}

	for _, userID := range userIDs {
		overdueTodos, err := s.todoRepo.ListOverdueByUser(ctx, userID, s.timezone)
		if err != nil {
			slog.Error("overdue followup: failed to list overdue", "user_id", userID, "error", err)
			continue
		}

		if len(overdueTodos) == 0 {
			continue
		}

		user := &tele.User{ID: userID}
		for _, t := range overdueTodos {
			msg := FormatOverdueNotification(t, s.timezone)
			if _, err := s.bot.Send(user, msg); err != nil {
				slog.Error("overdue followup: failed to send", "user_id", userID, "todo_id", t.ID, "error", err)
				continue
			}
		}

		slog.Info("overdue followup sent", "user_id", userID, "count", len(overdueTodos))
	}
}

func (s *DailyScheduler) sendMonthlyReport() {
	ctx := context.Background()
	now := time.Now().In(s.timezone)

	// Report for previous month
	prevMonth := now.AddDate(0, -1, 0)
	year := prevMonth.Year()
	month := prevMonth.Month()

	slog.Info("monthly expense report triggered", "year", year, "month", month)

	userIDs, err := s.todoRepo.ListActiveUserIDs(ctx)
	if err != nil {
		slog.Error("monthly report: failed to list users", "error", err)
		return
	}

	for _, userID := range userIDs {
		report, err := s.expenseSvc.MonthlyReport(ctx, userID, year, month)
		if err != nil {
			slog.Error("monthly report: failed to generate", "user_id", userID, "error", err)
			continue
		}

		user := &tele.User{ID: userID}
		if _, err := s.bot.Send(user, report); err != nil {
			slog.Error("monthly report: failed to send", "user_id", userID, "error", err)
			continue
		}

		slog.Info("monthly report sent", "user_id", userID, "month", month)
	}
}
