package expense

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Expense struct {
	ID          int
	UserID      int64
	Description string
	Amount      int64
	IsPaid      bool
	RecordedAt  time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, userID int64, description string, amount int64, isPaid bool) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO expenses (user_id, description, amount, is_paid) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, description, amount, isPaid,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create expense: %w", err)
	}
	return id, nil
}

func (r *Repository) List(ctx context.Context, userID int64, filter string, loc *time.Location) ([]Expense, error) {
	now := time.Now().In(loc)
	var query string
	var args []interface{}

	switch filter {
	case "today":
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		endOfDay := startOfDay.AddDate(0, 0, 1)
		query = `SELECT id, user_id, description, amount, is_paid, recorded_at FROM expenses
				 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3
				 ORDER BY recorded_at ASC`
		args = []interface{}{userID, startOfDay, endOfDay}
	case "this_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, loc)
		endOfWeek := startOfWeek.AddDate(0, 0, 7)
		query = `SELECT id, user_id, description, amount, is_paid, recorded_at FROM expenses
				 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3
				 ORDER BY recorded_at ASC`
		args = []interface{}{userID, startOfWeek, endOfWeek}
	case "this_month":
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		endOfMonth := startOfMonth.AddDate(0, 1, 0)
		query = `SELECT id, user_id, description, amount, is_paid, recorded_at FROM expenses
				 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3
				 ORDER BY recorded_at ASC`
		args = []interface{}{userID, startOfMonth, endOfMonth}
	default: // "all"
		query = `SELECT id, user_id, description, amount, is_paid, recorded_at FROM expenses
				 WHERE user_id = $1
				 ORDER BY recorded_at ASC`
		args = []interface{}{userID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	return scanExpenses(rows)
}

func (r *Repository) ListByMonth(ctx context.Context, userID int64, year int, month time.Month, loc *time.Location) ([]Expense, error) {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, description, amount, is_paid, recorded_at FROM expenses
		 WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3
		 ORDER BY recorded_at ASC`,
		userID, startOfMonth, endOfMonth,
	)
	if err != nil {
		return nil, fmt.Errorf("list expenses by month: %w", err)
	}
	defer rows.Close()
	return scanExpenses(rows)
}

func (r *Repository) Sum(ctx context.Context, userID int64, filter string, loc *time.Location) (int64, error) {
	now := time.Now().In(loc)
	var query string
	var args []interface{}

	switch filter {
	case "today":
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		endOfDay := startOfDay.AddDate(0, 0, 1)
		query = `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3`
		args = []interface{}{userID, startOfDay, endOfDay}
	case "this_week":
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		startOfWeek := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, loc)
		endOfWeek := startOfWeek.AddDate(0, 0, 7)
		query = `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3`
		args = []interface{}{userID, startOfWeek, endOfWeek}
	case "this_month":
		startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		endOfMonth := startOfMonth.AddDate(0, 1, 0)
		query = `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3`
		args = []interface{}{userID, startOfMonth, endOfMonth}
	default:
		query = `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1`
		args = []interface{}{userID}
	}

	var total int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum expenses: %w", err)
	}
	return total, nil
}

func (r *Repository) SumByMonth(ctx context.Context, userID int64, year int, month time.Month, loc *time.Location) (int64, error) {
	startOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	endOfMonth := startOfMonth.AddDate(0, 1, 0)
	var total int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE user_id = $1 AND recorded_at >= $2 AND recorded_at < $3`,
		userID, startOfMonth, endOfMonth,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum expenses by month: %w", err)
	}
	return total, nil
}

func (r *Repository) FindBySearch(ctx context.Context, userID int64, search string) (*Expense, error) {
	var e Expense
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, description, amount, is_paid, recorded_at FROM expenses
		 WHERE user_id = $1 AND description ILIKE '%' || $2 || '%'
		 ORDER BY recorded_at DESC LIMIT 1`,
		userID, search,
	).Scan(&e.ID, &e.UserID, &e.Description, &e.Amount, &e.IsPaid, &e.RecordedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find expense: %w", err)
	}
	return &e, nil
}

func (r *Repository) MarkPaid(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE expenses SET is_paid = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark expense paid: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM expenses WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	return nil
}

func scanExpenses(rows *sql.Rows) ([]Expense, error) {
	var expenses []Expense
	for rows.Next() {
		var e Expense
		if err := rows.Scan(&e.ID, &e.UserID, &e.Description, &e.Amount, &e.IsPaid, &e.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		expenses = append(expenses, e)
	}
	return expenses, rows.Err()
}
