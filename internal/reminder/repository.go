package reminder

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Reminder struct {
	ID             int
	TodoID         int
	RemindAt       time.Time
	IsRecurring    bool
	RecurrenceRule *string
	LastFiredAt    *time.Time
	IsActive       bool
	CreatedAt      time.Time
}

type ReminderWithTodo struct {
	Reminder
	TodoTitle  string
	TodoUserID int64
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, todoID int, remindAt time.Time, isRecurring bool, recurrenceRule string) error {
	var rule *string
	if recurrenceRule != "" {
		rule = &recurrenceRule
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO reminders (todo_id, remind_at, is_recurring, recurrence_rule) VALUES ($1, $2, $3, $4)`,
		todoID, remindAt, isRecurring, rule,
	)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

func (r *Repository) GetDueReminders(ctx context.Context) ([]ReminderWithTodo, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT r.id, r.todo_id, r.remind_at, r.is_recurring, r.recurrence_rule, r.last_fired_at, r.is_active, r.created_at,
		        t.title, t.user_id
		 FROM reminders r
		 JOIN todos t ON t.id = r.todo_id
		 WHERE r.remind_at <= NOW() AND r.is_active = TRUE
		 ORDER BY r.remind_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get due reminders: %w", err)
	}
	defer rows.Close()

	var reminders []ReminderWithTodo
	for rows.Next() {
		var rt ReminderWithTodo
		err := rows.Scan(
			&rt.ID, &rt.TodoID, &rt.RemindAt, &rt.IsRecurring, &rt.RecurrenceRule,
			&rt.LastFiredAt, &rt.IsActive, &rt.CreatedAt,
			&rt.TodoTitle, &rt.TodoUserID,
		)
		if err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		reminders = append(reminders, rt)
	}
	return reminders, rows.Err()
}

func (r *Repository) UpdateRemindAt(ctx context.Context, id int, nextTime time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE reminders SET remind_at = $1, last_fired_at = NOW() WHERE id = $2`,
		nextTime, id,
	)
	if err != nil {
		return fmt.Errorf("update remind_at: %w", err)
	}
	return nil
}

func (r *Repository) Deactivate(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE reminders SET is_active = FALSE, last_fired_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("deactivate reminder: %w", err)
	}
	return nil
}
