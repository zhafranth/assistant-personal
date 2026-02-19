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

	// 🔁 Recurring reminders section — show ALL active recurring reminders
	var recurringReminders []reminder.TodoReminder
	for _, r := range reminders {
		if r.IsRecurring {
			recurringReminders = append(recurringReminders, r)
		}
	}

	lines = append(lines, "")
	lines = append(lines, "─────────────")
	lines = append(lines, "")
	lines = append(lines, "🔁 Reminder Rutin\n")

	if len(recurringReminders) > 0 {
		for _, r := range recurringReminders {
			rt := r.RemindAt.In(loc)
			dateStr := formatDateShort(rt)
			label := recurringLabel(r.RecurrenceRule)
			line := fmt.Sprintf(" %s · %s", dateStr, r.TodoTitle)
			if label != "" {
				line += fmt.Sprintf(" (%s)", label)
			}
			lines = append(lines, line)
		}
	} else {
		lines = append(lines, " Tidak ada reminder rutin aktif")
	}

	// Summary footer
	lines = append(lines, "")
	lines = append(lines, "─────────────")

	pendingCount := len(upcoming) + len(overdue)
	lines = append(lines, fmt.Sprintf("📊 Pending: %d todo", pendingCount))
	if len(overdue) > 0 {
		lines = append(lines, fmt.Sprintf("📊 Overdue: %d todo", len(overdue)))
	}
	lines = append(lines, fmt.Sprintf("📊 Reminder rutin: %d aktif", len(recurringReminders)))

	return strings.Join(lines, "\n")
}

// FormatReminderList formats all active reminders.
//
// 🔔 Daftar Reminder Aktif
//
// 🔁 Bayar wifi
//
//	Bulanan · tanggal 5
//	Berikutnya: 5 Mar 2026 07:00
//
// 🔁 Bayar listrik
//
//	Bulanan · tanggal 17
//	Berikutnya: 17 Mar 2026 07:00
//
// ─────────────
// 🔔 2  🔁 2
func FormatReminderList(reminders []reminder.TodoReminder, loc *time.Location) string {
	if len(reminders) == 0 {
		return "🔔 Tidak ada reminder aktif."
	}

	var lines []string
	lines = append(lines, "🔔 Daftar Reminder Aktif\n")

	var countRecurring, countOnce int
	for _, r := range reminders {
		rt := r.RemindAt.In(loc)
		nextStr := fmt.Sprintf("%d %s %d %02d:%02d",
			rt.Day(), indonesianMonths[rt.Month()-1], rt.Year(), rt.Hour(), rt.Minute())

		if r.IsRecurring {
			countRecurring++
			label := recurringLabel(r.RecurrenceRule)
			detail := recurringRuleDetail(r.RecurrenceRule)
			line := fmt.Sprintf("🔁 %s", r.TodoTitle)
			lines = append(lines, line)
			if detail != "" {
				lines = append(lines, fmt.Sprintf("   %s · %s", label, detail))
			}
			lines = append(lines, fmt.Sprintf("   Berikutnya: %s", nextStr))
		} else {
			countOnce++
			lines = append(lines, fmt.Sprintf("🔔 %s", r.TodoTitle))
			lines = append(lines, fmt.Sprintf("   %s", nextStr))
		}
	}

	lines = append(lines, "\n─────────────")
	var summary []string
	if countOnce > 0 {
		summary = append(summary, fmt.Sprintf("🔔 %d", countOnce))
	}
	if countRecurring > 0 {
		summary = append(summary, fmt.Sprintf("🔁 %d", countRecurring))
	}
	lines = append(lines, strings.Join(summary, "  "))

	return strings.Join(lines, "\n")
}

// recurringRuleDetail returns a human-readable detail of the recurrence rule.
func recurringRuleDetail(rule *string) string {
	if rule == nil {
		return ""
	}
	r := *rule
	switch {
	case r == "daily":
		return "setiap hari"
	case strings.HasPrefix(r, "weekly:"):
		day := strings.TrimPrefix(r, "weekly:")
		switch strings.ToUpper(day) {
		case "MON", "SENIN":
			return "setiap Senin"
		case "TUE", "SELASA":
			return "setiap Selasa"
		case "WED", "RABU":
			return "setiap Rabu"
		case "THU", "KAMIS":
			return "setiap Kamis"
		case "FRI", "JUMAT":
			return "setiap Jumat"
		case "SAT", "SABTU":
			return "setiap Sabtu"
		case "SUN", "MINGGU":
			return "setiap Minggu"
		default:
			return fmt.Sprintf("setiap %s", day)
		}
	case strings.HasPrefix(r, "monthly:"):
		d := strings.TrimPrefix(r, "monthly:")
		return fmt.Sprintf("tanggal %s", d)
	case strings.HasPrefix(r, "yearly:"):
		parts := strings.Split(strings.TrimPrefix(r, "yearly:"), "-")
		if len(parts) == 2 {
			m := 0
			fmt.Sscanf(parts[0], "%d", &m)
			if m >= 1 && m <= 12 {
				return fmt.Sprintf("%s %s", parts[1], indonesianMonthsFull[m-1])
			}
		}
		return "setiap tahun"
	default:
		return ""
	}
}

// FormatOverdueNotification formats a single overdue todo follow-up.
//
// ⚠️ Masih belum selesai
//
// 📌 Bayar listrik
// 📅 Jatuh tempo: 25 Feb (2 hari lalu)
//
// Ketik "done bayar listrik" jika sudah selesai
func FormatOverdueNotification(t todo.Todo, loc *time.Location) string {
	now := time.Now().In(loc)
	d := t.DueDate.In(loc)

	dateStr := formatDateShort(d)
	agoStr := relativeTimeAgo(now, d)

	return fmt.Sprintf("⚠️ Masih belum selesai\n\n📌 %s\n📅 Jatuh tempo: %s (%s)\n\nKetik \"done %s\" jika sudah selesai",
		t.Title, dateStr, agoStr, t.Title)
}

func relativeTimeAgo(now, target time.Time) string {
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	targetDate := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, target.Location())
	days := int(nowDate.Sub(targetDate).Hours() / 24)

	switch {
	case days == 1:
		return "kemarin"
	case days < 7:
		return fmt.Sprintf("%d hari lalu", days)
	case days < 30:
		weeks := days / 7
		if weeks == 1 {
			return "1 minggu lalu"
		}
		return fmt.Sprintf("%d minggu lalu", weeks)
	default:
		months := days / 30
		if months == 1 {
			return "1 bulan lalu"
		}
		return fmt.Sprintf("%d bulan lalu", months)
	}
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
