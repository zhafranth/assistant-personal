package todo

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Todo struct {
	ID          int
	UserID      int64
	ProjectID   *int
	Title       string
	Description *string
	IsCompleted bool
	CompletedAt *time.Time
	DueDate     *time.Time
	DeletedAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, userID int64, title string, dueDate *time.Time) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO todos (user_id, title, due_date) VALUES ($1, $2, $3) RETURNING id`,
		userID, title, dueDate,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create todo: %w", err)
	}
	return id, nil
}

func (r *Repository) CreateWithProject(ctx context.Context, userID int64, projectID int, title string, dueDate *time.Time) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO todos (user_id, project_id, title, due_date) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, projectID, title, dueDate,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create todo with project: %w", err)
	}
	return id, nil
}

func (r *Repository) List(ctx context.Context, userID int64, filter string, loc *time.Location) ([]Todo, error) {
	var query string
	var args []interface{}

	switch filter {
	case "today":
		now := time.Now().In(loc)
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		endOfDay := startOfDay.AddDate(0, 0, 1)
		query = `SELECT id, user_id, project_id, title, description, is_completed, completed_at, due_date, deleted_at, created_at, updated_at
				 FROM todos WHERE user_id = $1 AND project_id IS NULL AND deleted_at IS NULL AND
				 ((due_date >= $2 AND due_date < $3) OR (created_at >= $2 AND created_at < $3))
				 ORDER BY is_completed ASC, created_at DESC`
		args = []interface{}{userID, startOfDay, endOfDay}
	case "pending":
		query = `SELECT id, user_id, project_id, title, description, is_completed, completed_at, due_date, deleted_at, created_at, updated_at
				 FROM todos WHERE user_id = $1 AND project_id IS NULL AND is_completed = FALSE AND deleted_at IS NULL
				 ORDER BY due_date ASC NULLS LAST, created_at DESC`
		args = []interface{}{userID}
	default: // "all"
		query = `SELECT id, user_id, project_id, title, description, is_completed, completed_at, due_date, deleted_at, created_at, updated_at
				 FROM todos WHERE user_id = $1 AND project_id IS NULL AND deleted_at IS NULL
				 ORDER BY is_completed ASC, created_at DESC`
		args = []interface{}{userID}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list todos: %w", err)
	}
	defer rows.Close()

	return scanTodos(rows)
}

func (r *Repository) FindBySearch(ctx context.Context, userID int64, search string) (*Todo, error) {
	var t Todo
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, project_id, title, description, is_completed, completed_at, due_date, deleted_at, created_at, updated_at
		 FROM todos WHERE user_id = $1 AND project_id IS NULL AND deleted_at IS NULL AND title ILIKE '%' || $2 || '%'
		 ORDER BY created_at DESC LIMIT 1`,
		userID, search,
	).Scan(&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description, &t.IsCompleted, &t.CompletedAt, &t.DueDate, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find todo: %w", err)
	}
	return &t, nil
}

func (r *Repository) Complete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE todos SET is_completed = TRUE, completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("complete todo: %w", err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, id int, title string, dueDate *time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE todos SET title = $2, due_date = $3, updated_at = NOW() WHERE id = $1`,
		id, title, dueDate,
	)
	if err != nil {
		return fmt.Errorf("update todo: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM todos WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("delete todo: %w", err)
	}
	return nil
}

func (r *Repository) CompleteAll(ctx context.Context, userID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE todos SET is_completed = TRUE, completed_at = NOW(), updated_at = NOW()
		 WHERE user_id = $1 AND project_id IS NULL AND is_completed = FALSE AND deleted_at IS NULL`,
		userID,
	)
	if err != nil {
		return 0, fmt.Errorf("complete all todos: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func (r *Repository) SoftDeleteCompletedOlderThan(ctx context.Context, before time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE todos SET deleted_at = NOW(), updated_at = NOW()
		 WHERE is_completed = TRUE AND completed_at <= $1 AND deleted_at IS NULL`,
		before,
	)
	if err != nil {
		return fmt.Errorf("soft delete completed todos: %w", err)
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id int) (*Todo, error) {
	var t Todo
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, project_id, title, description, is_completed, completed_at, due_date, deleted_at, created_at, updated_at
		 FROM todos WHERE id = $1 AND deleted_at IS NULL`,
		id,
	).Scan(&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description, &t.IsCompleted, &t.CompletedAt, &t.DueDate, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get todo: %w", err)
	}
	return &t, nil
}

func (r *Repository) ListActiveUserIDs(ctx context.Context) ([]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT user_id FROM todos WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("list active user ids: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func scanTodos(rows *sql.Rows) ([]Todo, error) {
	var todos []Todo
	for rows.Next() {
		var t Todo
		err := rows.Scan(&t.ID, &t.UserID, &t.ProjectID, &t.Title, &t.Description, &t.IsCompleted, &t.CompletedAt, &t.DueDate, &t.DeletedAt, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan todo: %w", err)
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}
