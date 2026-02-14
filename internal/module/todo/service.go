package todo

import (
	"context"
	"fmt"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
)

type Service struct {
	repo         *Repository
	reminderRepo *reminder.Repository
	timezone     *time.Location
}

func NewService(repo *Repository, reminderRepo *reminder.Repository, timezone *time.Location) *Service {
	return &Service{
		repo:         repo,
		reminderRepo: reminderRepo,
		timezone:     timezone,
	}
}

func (s *Service) Add(ctx context.Context, userID int64, title string, dueDate *time.Time, hasReminder bool, remindAt *time.Time, recurring string) (string, error) {
	todoID, err := s.repo.Create(ctx, userID, title, dueDate)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("✅ Todo ditambahkan: \"%s\"", title)

	if dueDate != nil {
		resp += fmt.Sprintf("\n📅 Deadline: %s", dueDate.In(s.timezone).Format("2 Jan 2006"))
	}

	if hasReminder && remindAt != nil {
		err := s.reminderRepo.Create(ctx, todoID, *remindAt, recurring != "", recurring)
		if err != nil {
			return "", fmt.Errorf("create reminder: %w", err)
		}
		resp += fmt.Sprintf("\n⏰ Reminder: %s", remindAt.In(s.timezone).Format("2 Jan 2006 15:04 WIB"))
		if recurring != "" {
			resp += fmt.Sprintf(" (recurring: %s)", recurring)
		}
	}

	return resp, nil
}

func (s *Service) List(ctx context.Context, userID int64, filter string) ([]Todo, error) {
	return s.repo.List(ctx, userID, filter, s.timezone)
}

func (s *Service) Complete(ctx context.Context, userID int64, search string) (string, error) {
	todo, err := s.repo.FindBySearch(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if todo == nil {
		return fmt.Sprintf("❌ Todo \"%s\" tidak ditemukan.", search), nil
	}
	if todo.IsCompleted {
		return fmt.Sprintf("ℹ️ Todo \"%s\" sudah selesai sebelumnya.", todo.Title), nil
	}

	if err := s.repo.Complete(ctx, todo.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Todo selesai: \"%s\"", todo.Title), nil
}

func (s *Service) Edit(ctx context.Context, userID int64, search string, newTitle string, newDueDate *time.Time, newRemindAt *time.Time) (string, error) {
	todo, err := s.repo.FindBySearch(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if todo == nil {
		return fmt.Sprintf("❌ Todo \"%s\" tidak ditemukan.", search), nil
	}

	title := todo.Title
	if newTitle != "" {
		title = newTitle
	}

	dueDate := todo.DueDate
	if newDueDate != nil {
		dueDate = newDueDate
	}

	if err := s.repo.Update(ctx, todo.ID, title, dueDate); err != nil {
		return "", err
	}

	resp := fmt.Sprintf("✏️ Todo diupdate: \"%s\"", title)
	if dueDate != nil {
		resp += fmt.Sprintf("\n📅 Deadline: %s", dueDate.In(s.timezone).Format("2 Jan 2006"))
	}

	if newRemindAt != nil {
		if err := s.reminderRepo.UpsertByTodoID(ctx, todo.ID, *newRemindAt); err != nil {
			return "", fmt.Errorf("upsert reminder: %w", err)
		}
		resp += fmt.Sprintf("\n⏰ Reminder diupdate: %s", newRemindAt.In(s.timezone).Format("2 Jan 2006 15:04 WIB"))
	}

	return resp, nil
}

func (s *Service) Delete(ctx context.Context, userID int64, search string) (string, error) {
	todo, err := s.repo.FindBySearch(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if todo == nil {
		return fmt.Sprintf("❌ Todo \"%s\" tidak ditemukan.", search), nil
	}

	if err := s.repo.Delete(ctx, todo.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("🗑️ Todo dihapus: \"%s\"", todo.Title), nil
}

func (s *Service) ClearAll(ctx context.Context, userID int64) (string, error) {
	n, err := s.repo.DeleteAll(ctx, userID)
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "ℹ️ Tidak ada todo yang perlu dihapus.", nil
	}
	return fmt.Sprintf("🗑️ %d todo dihapus dari daftar.", n), nil
}

func (s *Service) CleanupCompletedTodos(ctx context.Context) error {
	before := time.Now().Add(-24 * time.Hour)
	return s.repo.SoftDeleteCompletedOlderThan(ctx, before)
}
