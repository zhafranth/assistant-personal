package project

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Project struct {
	ID          int
	UserID      int64
	Name        string
	Description *string
	DueDate     *time.Time
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectWithProgress struct {
	Project
	TotalGoals     int
	CompletedGoals int
}

type Goal struct {
	ID          int
	ProjectID   int
	Title       string
	IsCompleted bool
	CompletedAt *time.Time
	DueDate     *time.Time
	CreatedAt   time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, userID int64, name string, description *string, dueDate *time.Time) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO projects (user_id, name, description, due_date) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, name, description, dueDate,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create project: %w", err)
	}
	return id, nil
}

func (r *Repository) List(ctx context.Context, userID int64) ([]ProjectWithProgress, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.user_id, p.name, p.description, p.due_date, p.is_active, p.created_at, p.updated_at,
		        COUNT(t.id) AS total_goals,
		        COUNT(CASE WHEN t.is_completed THEN 1 END) AS completed_goals
		 FROM projects p
		 LEFT JOIN todos t ON t.project_id = p.id
		 WHERE p.user_id = $1 AND p.is_active = TRUE
		 GROUP BY p.id
		 ORDER BY p.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []ProjectWithProgress
	for rows.Next() {
		var p ProjectWithProgress
		err := rows.Scan(
			&p.ID, &p.UserID, &p.Name, &p.Description, &p.DueDate,
			&p.IsActive, &p.CreatedAt, &p.UpdatedAt,
			&p.TotalGoals, &p.CompletedGoals,
		)
		if err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (r *Repository) FindByName(ctx context.Context, userID int64, name string) (*Project, error) {
	var p Project
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, description, due_date, is_active, created_at, updated_at
		 FROM projects WHERE user_id = $1 AND is_active = TRUE AND name ILIKE '%' || $2 || '%'
		 ORDER BY created_at DESC LIMIT 1`,
		userID, name,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.Description, &p.DueDate, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find project: %w", err)
	}
	return &p, nil
}

func (r *Repository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

func (r *Repository) GetGoals(ctx context.Context, projectID int) ([]Goal, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, title, is_completed, completed_at, due_date, created_at
		 FROM todos WHERE project_id = $1
		 ORDER BY is_completed ASC, created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("get goals: %w", err)
	}
	defer rows.Close()

	var goals []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.ProjectID, &g.Title, &g.IsCompleted, &g.CompletedAt, &g.DueDate, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan goal: %w", err)
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

func (r *Repository) AddGoal(ctx context.Context, userID int64, projectID int, title string, dueDate *time.Time) (int, error) {
	var id int
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO todos (user_id, project_id, title, due_date) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, projectID, title, dueDate,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("add goal: %w", err)
	}
	return id, nil
}

func (r *Repository) FindGoalBySearch(ctx context.Context, projectID int, search string) (*Goal, error) {
	var g Goal
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, title, is_completed, completed_at, due_date, created_at
		 FROM todos WHERE project_id = $1 AND title ILIKE '%' || $2 || '%'
		 ORDER BY created_at DESC LIMIT 1`,
		projectID, search,
	).Scan(&g.ID, &g.ProjectID, &g.Title, &g.IsCompleted, &g.CompletedAt, &g.DueDate, &g.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find goal: %w", err)
	}
	return &g, nil
}

func (r *Repository) CompleteGoal(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE todos SET is_completed = TRUE, completed_at = NOW(), updated_at = NOW() WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("complete goal: %w", err)
	}
	return nil
}

func (r *Repository) DeleteGoal(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM todos WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete goal: %w", err)
	}
	return nil
}
