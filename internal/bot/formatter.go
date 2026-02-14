package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
)

var indonesianMonths = [...]string{
	"Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
}

var indonesianMonthsFull = [...]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

var indonesianDays = [...]string{
	"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu",
}

func formatDateShort(t time.Time) string {
	return fmt.Sprintf("%d %s", t.Day(), indonesianMonths[t.Month()-1])
}

func formatMonthYear(t time.Time) string {
	return fmt.Sprintf("%s %d", indonesianMonthsFull[t.Month()-1], t.Year())
}

func formatDayFull(t time.Time) string {
	return fmt.Sprintf("%s, %d %s %d", indonesianDays[t.Weekday()], t.Day(), indonesianMonths[t.Month()-1], t.Year())
}

func hasTimeComponent(t time.Time) bool {
	return t.Hour() != 0 || t.Minute() != 0
}

func formatTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

func recurringLabel(rule *string) string {
	if rule == nil {
		return ""
	}
	r := *rule
	switch {
	case r == "daily":
		return "Harian"
	case strings.HasPrefix(r, "weekly:"):
		return "Mingguan"
	case strings.HasPrefix(r, "monthly:"):
		return "Bulanan"
	case strings.HasPrefix(r, "yearly:"):
		return "Tahunan"
	default:
		return ""
	}
}

// buildReminderMap creates a lookup map from todoID to its active reminder info.
func buildReminderMap(reminders []reminder.TodoReminder) map[int]reminder.TodoReminder {
	m := make(map[int]reminder.TodoReminder, len(reminders))
	for _, r := range reminders {
		m[r.TodoID] = r
	}
	return m
}

// FormatTodoList formats the todo list (Template 1).
//
// 📋 Todo List
//
// 🔘 Cukur rambut
// ⏳ Develop landing page
//
//	📅 20 Feb · 🏷 Laundry App
//
// ✅ Setup database
// ─────────────
// ⏳ 2  🔘 3  ✅ 3
func FormatTodoList(todos []todo.Todo, filter string, loc *time.Location, reminders []reminder.TodoReminder) string {
	if len(todos) == 0 {
		return fmt.Sprintf("📭 Tidak ada todo %s.", filterTodoLabel(filter))
	}

	reminderMap := buildReminderMap(reminders)
	now := time.Now().In(loc)

	var lines []string
	lines = append(lines, "📋 Todo List\n")

	var countPending, countProgress, countDone int

	for _, t := range todos {
		if t.IsCompleted {
			countDone++
			lines = append(lines, fmt.Sprintf("✅ %s", t.Title))
			continue
		}

		// Determine status icon: ⏳ if has due date (active task), 🔘 if no due date
		icon := "🔘"
		if t.DueDate != nil {
			icon = "⏳"
			countProgress++
		} else {
			countPending++
		}

		lines = append(lines, fmt.Sprintf("%s %s", icon, t.Title))

		// Build detail line
		var details []string
		if t.DueDate != nil {
			d := t.DueDate.In(loc)
			dateStr := formatDateShort(d)
			if hasTimeComponent(d) {
				dateStr += " · " + formatTime(d)
			}

			// Overdue indicator
			if d.Before(now) {
				dateStr += " ⚠️"
			}

			details = append(details, "📅 "+dateStr)
		}

		// Reminder time + recurring indicator
		if rm, ok := reminderMap[t.ID]; ok {
			rmStr := "⏰ " + formatTime(rm.RemindAt.In(loc))
			if rm.IsRecurring {
				if label := recurringLabel(rm.RecurrenceRule); label != "" {
					rmStr += " 🔁 " + label
				}
			}
			details = append(details, rmStr)
		}

		if len(details) > 0 {
			lines = append(lines, "   "+strings.Join(details, " · "))
		}
	}

	// Summary footer
	lines = append(lines, "\n─────────────")
	var summary []string
	if countProgress > 0 {
		summary = append(summary, fmt.Sprintf("⏳ %d", countProgress))
	}
	if countPending > 0 {
		summary = append(summary, fmt.Sprintf("🔘 %d", countPending))
	}
	if countDone > 0 {
		summary = append(summary, fmt.Sprintf("✅ %d", countDone))
	}
	lines = append(lines, strings.Join(summary, "  "))

	return strings.Join(lines, "\n")
}

// FormatDailyBriefing formats the daily briefing (Template 2).
//
// ☀️ Daily Briefing — Jumat, 14 Feb 2026
//
// 📌 Todo
// 🔘 Beli domain baru — 18 Feb
// ⏳ Riset kompetitor — 20 Feb
//
// ⚡ Overdue
// 🔘 Bayar pajak — 10 Feb ⚠️
//
// ─────────────
//
// 🗓 Reminder Bulan Ini — Februari 2026
//
//	25 Feb · Bayar listrik 🔁
//	25 Feb · Bayar internet 🔁
//
// ─────────────
// 📊 Hari ini: 2 todo
// 📊 Bulan ini: 3 reminder tersisa
func FormatDailyBriefing(todos []todo.Todo, loc *time.Location, reminders []reminder.TodoReminder) string {
	now := time.Now().In(loc)

	var lines []string
	lines = append(lines, fmt.Sprintf("☀️ Daily Briefing — %s\n", formatDayFull(now)))

	reminderMap := buildReminderMap(reminders)

	// Separate pending todos into upcoming and overdue
	var upcoming, overdue []todo.Todo
	for _, t := range todos {
		if t.IsCompleted {
			continue
		}
		if t.DueDate != nil && t.DueDate.In(loc).Before(now) {
			// Due date is in the past (before today start)
			todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
			if t.DueDate.In(loc).Before(todayStart) {
				overdue = append(overdue, t)
				continue
			}
		}
		upcoming = append(upcoming, t)
	}

	// 📌 Todo section
	if len(upcoming) > 0 {
		lines = append(lines, "📌 Todo")
		for _, t := range upcoming {
			icon := "🔘"
			if t.DueDate != nil {
				icon = "⏳"
			}

			line := fmt.Sprintf("%s %s", icon, t.Title)
			if t.DueDate != nil {
				d := t.DueDate.In(loc)
				dateStr := formatDateShort(d)
				if hasTimeComponent(d) {
					dateStr += " · " + formatTime(d)
				}
				line += " — " + dateStr
			}

			// Reminder time + recurring indicator
			if rm, ok := reminderMap[t.ID]; ok {
				line += " ⏰ " + formatTime(rm.RemindAt.In(loc))
				if rm.IsRecurring {
					if label := recurringLabel(rm.RecurrenceRule); label != "" {
						line += " 🔁"
					}
				}
			}

			lines = append(lines, line)
		}
	} else {
		lines = append(lines, "📌 Todo")
		lines = append(lines, "   Tidak ada todo pending 🎉")
	}

	// ⚡ Overdue section
	if len(overdue) > 0 {
		lines = append(lines, "")
		lines = append(lines, "⚡ Overdue")
		for _, t := range overdue {
			line := fmt.Sprintf("🔘 %s", t.Title)
			if t.DueDate != nil {
				line += " — " + formatDateShort(t.DueDate.In(loc)) + " ⚠️"
			}
			lines = append(lines, line)
		}
	}

	// 🗓 Monthly reminders section
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)

	var monthlyReminders []reminder.TodoReminder
	for _, r := range reminders {
		rt := r.RemindAt.In(loc)
		if r.IsRecurring && !rt.Before(startOfMonth) && rt.Before(endOfMonth) {
			monthlyReminders = append(monthlyReminders, r)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "─────────────")
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("🗓 Reminder Bulan Ini — %s\n", formatMonthYear(now)))

	if len(monthlyReminders) > 0 {
		for _, r := range monthlyReminders {
			rt := r.RemindAt.In(loc)
			dateStr := formatDateShort(rt)
			line := fmt.Sprintf(" %s · %s 🔁", dateStr, r.TodoTitle)
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, " Tidak ada reminder bulan ini")
	}

	// Summary footer
	lines = append(lines, "")
	lines = append(lines, "─────────────")

	pendingCount := len(upcoming) + len(overdue)
	lines = append(lines, fmt.Sprintf("📊 Pending: %d todo", pendingCount))
	if len(overdue) > 0 {
		lines = append(lines, fmt.Sprintf("📊 Overdue: %d todo", len(overdue)))
	}
	lines = append(lines, fmt.Sprintf("📊 Bulan ini: %d reminder tersisa", len(monthlyReminders)))

	return strings.Join(lines, "\n")
}

func filterTodoLabel(filter string) string {
	switch filter {
	case "today":
		return "hari ini"
	case "pending":
		return "yang pending"
	default:
		return ""
	}
}
