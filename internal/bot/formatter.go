package bot

import (
	"fmt"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/module/todo"
)

func FormatTodoList(todos []todo.Todo, filter string, loc *time.Location) string {
	if len(todos) == 0 {
		return fmt.Sprintf("📭 Tidak ada todo %s.", filterTodoLabel(filter))
	}

	pending := 0
	for _, t := range todos {
		if !t.IsCompleted {
			pending++
		}
	}

	resp := fmt.Sprintf("📋 Todo Kamu (%d pending):\n", pending)
	for i, t := range todos {
		check := "☐"
		suffix := ""
		if t.IsCompleted {
			check = "☑"
			suffix = " ✓"
		}
		if t.DueDate != nil && !t.IsCompleted {
			suffix += fmt.Sprintf(" 📅 %s", t.DueDate.In(loc).Format("2 Jan 15:04"))
		}
		resp += fmt.Sprintf("%d. %s %s%s\n", i+1, check, t.Title, suffix)
	}
	return resp
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
