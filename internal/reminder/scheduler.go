package reminder

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
)

type Scheduler struct {
	repo     *Repository
	bot      *tele.Bot
	interval time.Duration
	timezone *time.Location
	stopCh   chan struct{}
	once     sync.Once
}

func NewScheduler(repo *Repository, bot *tele.Bot, interval time.Duration, timezone *time.Location) *Scheduler {
	return &Scheduler{
		repo:     repo,
		bot:      bot,
		interval: interval,
		timezone: timezone,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	slog.Info("reminder scheduler started", "interval", s.interval)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.tick()
		case <-s.stopCh:
			slog.Info("reminder scheduler stopped")
			return
		}
	}
}

func (s *Scheduler) Stop() {
	s.once.Do(func() { close(s.stopCh) })
}

func (s *Scheduler) tick() {
	ctx := context.Background()
	reminders, err := s.repo.GetDueReminders(ctx)
	if err != nil {
		slog.Error("failed to get due reminders", "error", err)
		return
	}

	for _, r := range reminders {
		user := &tele.User{ID: r.TodoUserID}
		msg := formatReminderNotification(r, s.timezone)

		if _, err := s.bot.Send(user, msg); err != nil {
			slog.Error("failed to send reminder", "todo_id", r.TodoID, "user_id", r.TodoUserID, "error", err)
			continue
		}

		slog.Info("reminder sent", "todo_id", r.TodoID, "user_id", r.TodoUserID)

		if r.IsRecurring && r.RecurrenceRule != nil {
			nextTime := calculateNext(r.RemindAt, *r.RecurrenceRule, s.timezone)
			if err := s.repo.UpdateRemindAt(ctx, r.ID, nextTime); err != nil {
				slog.Error("failed to update recurring reminder", "id", r.ID, "error", err)
			}
		} else {
			if err := s.repo.Deactivate(ctx, r.ID); err != nil {
				slog.Error("failed to deactivate reminder", "id", r.ID, "error", err)
			}
		}
	}
}

var indonesianDays = [...]string{
	"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu",
}

var indonesianMonths = [...]string{
	"Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
}

func formatReminderNotification(r ReminderWithTodo, loc *time.Location) string {
	t := r.RemindAt.In(loc)
	dateStr := fmt.Sprintf("%s, %d %s %d · %02d:%02d",
		indonesianDays[t.Weekday()], t.Day(), indonesianMonths[t.Month()-1], t.Year(),
		t.Hour(), t.Minute(),
	)

	if r.IsRecurring && r.RecurrenceRule != nil {
		header := recurringHeader(*r.RecurrenceRule)
		detail := recurringDetail(*r.RecurrenceRule, t)
		msg := fmt.Sprintf("🔔 %s\n\n📌 %s\n📅 %s\n🔁 %s", header, r.TodoTitle, dateStr, detail)
		msg += fmt.Sprintf("\n\nKetik \"done %s\" untuk selesaikan", r.TodoTitle)
		return msg
	}

	msg := fmt.Sprintf("🔔 Reminder\n\n📌 %s\n📅 %s", r.TodoTitle, dateStr)
	msg += fmt.Sprintf("\n\nKetik \"done %s\" untuk selesaikan", r.TodoTitle)
	return msg
}

func recurringHeader(rule string) string {
	switch {
	case rule == "daily":
		return "Reminder Harian"
	case strings.HasPrefix(rule, "weekly:"):
		return "Reminder Mingguan"
	case strings.HasPrefix(rule, "monthly:"):
		return "Reminder Bulanan"
	case strings.HasPrefix(rule, "yearly:"):
		return "Reminder Tahunan"
	default:
		return "Reminder"
	}
}

func recurringDetail(rule string, t time.Time) string {
	switch {
	case rule == "daily":
		return fmt.Sprintf("Setiap hari jam %02d:%02d", t.Hour(), t.Minute())
	case strings.HasPrefix(rule, "weekly:"):
		dayStr := strings.TrimPrefix(rule, "weekly:")
		dayName := indonesianDayName(dayStr)
		return fmt.Sprintf("Setiap hari %s", dayName)
	case strings.HasPrefix(rule, "monthly:"):
		dateStr := strings.TrimPrefix(rule, "monthly:")
		return fmt.Sprintf("Setiap tanggal %s", dateStr)
	case strings.HasPrefix(rule, "yearly:"):
		dateStr := strings.TrimPrefix(rule, "yearly:")
		parts := strings.Split(dateStr, "-")
		if len(parts) == 2 {
			month, _ := strconv.Atoi(parts[0])
			if month >= 1 && month <= 12 {
				return fmt.Sprintf("Setiap %s %s", parts[1], indonesianMonths[month-1])
			}
		}
		return "Setiap tahun"
	default:
		return "Recurring"
	}
}

func indonesianDayName(day string) string {
	switch strings.ToLower(day) {
	case "mon", "senin":
		return "Senin"
	case "tue", "selasa":
		return "Selasa"
	case "wed", "rabu":
		return "Rabu"
	case "thu", "kamis":
		return "Kamis"
	case "fri", "jumat":
		return "Jumat"
	case "sat", "sabtu":
		return "Sabtu"
	case "sun", "minggu":
		return "Minggu"
	default:
		return day
	}
}

func calculateNext(current time.Time, rule string, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	// Convert current to local timezone so hour/minute are in the user's timezone,
	// not UTC (postgres returns TIMESTAMPTZ as UTC).
	cur := current.In(loc)

	switch {
	case rule == "daily":
		next := current.AddDate(0, 0, 1)
		if next.Before(now) {
			next = time.Date(now.Year(), now.Month(), now.Day()+1, cur.Hour(), cur.Minute(), 0, 0, loc)
		}
		return next

	case strings.HasPrefix(rule, "weekly:"):
		dayStr := strings.TrimPrefix(rule, "weekly:")
		targetDay := parseDayOfWeek(dayStr)
		next := current.AddDate(0, 0, 7)
		// Adjust to the correct weekday
		for next.Weekday() != targetDay {
			next = next.AddDate(0, 0, 1)
		}
		if next.Before(now) {
			next = time.Date(now.Year(), now.Month(), now.Day(), cur.Hour(), cur.Minute(), 0, 0, loc)
			for next.Weekday() != targetDay || !next.After(now) {
				next = next.AddDate(0, 0, 1)
			}
		}
		return next

	case strings.HasPrefix(rule, "monthly:"):
		dateStr := strings.TrimPrefix(rule, "monthly:")
		day, err := strconv.Atoi(dateStr)
		if err != nil || day < 1 || day > 31 {
			slog.Warn("invalid monthly recurrence rule", "rule", rule)
			return current.AddDate(0, 0, 1)
		}
		next := time.Date(cur.Year(), cur.Month()+1, day, cur.Hour(), cur.Minute(), 0, 0, loc)
		if next.Before(now) {
			next = time.Date(now.Year(), now.Month()+1, day, cur.Hour(), cur.Minute(), 0, 0, loc)
			if next.Before(now) {
				next = time.Date(now.Year(), now.Month()+2, day, cur.Hour(), cur.Minute(), 0, 0, loc)
			}
		}
		return next

	case strings.HasPrefix(rule, "yearly:"):
		dateStr := strings.TrimPrefix(rule, "yearly:")
		parts := strings.Split(dateStr, "-")
		if len(parts) == 2 {
			month, err1 := strconv.Atoi(parts[0])
			day, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || month < 1 || month > 12 || day < 1 || day > 31 {
				slog.Warn("invalid yearly recurrence rule", "rule", rule)
				return current.AddDate(0, 0, 1)
			}
			next := time.Date(cur.Year()+1, time.Month(month), day, cur.Hour(), cur.Minute(), 0, 0, loc)
			if next.Before(now) {
				next = time.Date(now.Year()+1, time.Month(month), day, cur.Hour(), cur.Minute(), 0, 0, loc)
			}
			return next
		}
	}

	// Fallback: next day
	return current.AddDate(0, 0, 1)
}

func parseDayOfWeek(day string) time.Weekday {
	switch strings.ToLower(day) {
	case "mon", "senin":
		return time.Monday
	case "tue", "selasa":
		return time.Tuesday
	case "wed", "rabu":
		return time.Wednesday
	case "thu", "kamis":
		return time.Thursday
	case "fri", "jumat":
		return time.Friday
	case "sat", "sabtu":
		return time.Saturday
	case "sun", "minggu":
		return time.Sunday
	default:
		return time.Monday
	}
}
