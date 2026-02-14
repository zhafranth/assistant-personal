package project

import (
	"context"
	"fmt"
	"time"

	"github.com/zhafrantharif/personal-assistant-bot/internal/reminder"
)

type Service struct {
	repo        *Repository
	reminderRepo *reminder.Repository
	timezone    *time.Location
}

func NewService(repo *Repository, reminderRepo *reminder.Repository, timezone *time.Location) *Service {
	return &Service{
		repo:        repo,
		reminderRepo: reminderRepo,
		timezone:    timezone,
	}
}

func (s *Service) Add(ctx context.Context, userID int64, name string, description *string, dueDate *time.Time) (string, error) {
	_, err := s.repo.Create(ctx, userID, name, description, dueDate)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("📁 Project dibuat: \"%s\"", name)
	if dueDate != nil {
		resp += fmt.Sprintf("\n📅 Deadline: %s", dueDate.In(s.timezone).Format("2 Jan 2006"))
	}
	return resp, nil
}

func (s *Service) List(ctx context.Context, userID int64) (string, error) {
	projects, err := s.repo.List(ctx, userID)
	if err != nil {
		return "", err
	}

	if len(projects) == 0 {
		return "📭 Belum ada project.", nil
	}

	resp := "📁 Project Kamu:\n"
	for i, p := range projects {
		progress := fmt.Sprintf("%d/%d goals ✓", p.CompletedGoals, p.TotalGoals)
		deadline := ""
		if p.DueDate != nil {
			deadline = fmt.Sprintf(" — deadline %s", p.DueDate.In(s.timezone).Format("2 Jan 2006"))
		}
		resp += fmt.Sprintf("%d. %s (%s)%s\n", i+1, p.Name, progress, deadline)
	}
	return resp, nil
}

func (s *Service) Show(ctx context.Context, userID int64, projectName string) (string, error) {
	proj, err := s.repo.FindByName(ctx, userID, projectName)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return fmt.Sprintf("❌ Project \"%s\" tidak ditemukan.", projectName), nil
	}

	goals, err := s.repo.GetGoals(ctx, proj.ID)
	if err != nil {
		return "", err
	}

	total := len(goals)
	completed := 0
	for _, g := range goals {
		if g.IsCompleted {
			completed++
		}
	}

	// Build progress bar (10 blocks)
	progressBar := ""
	if total > 0 {
		filled := (completed * 10) / total
		for i := 0; i < 10; i++ {
			if i < filled {
				progressBar += "█"
			} else {
				progressBar += "░"
			}
		}
		pct := (completed * 100) / total
		progressBar = fmt.Sprintf("[%s] %d%%", progressBar, pct)
	} else {
		progressBar = "[░░░░░░░░░░] 0%"
	}

	resp := fmt.Sprintf("📁 %s\n", proj.Name)
	if proj.Description != nil {
		resp += fmt.Sprintf("📝 %s\n", *proj.Description)
	}
	if proj.DueDate != nil {
		resp += fmt.Sprintf("📅 Deadline: %s\n", proj.DueDate.In(s.timezone).Format("2 Jan 2006"))
	}
	resp += fmt.Sprintf("📊 Progress: %d/%d goals %s\n", completed, total, progressBar)

	if total == 0 {
		resp += "\n_Belum ada goals. Tambahkan dengan:_\n\"tambah goal di " + proj.Name + ": nama goal\""
		return resp, nil
	}

	now := time.Now().In(s.timezone)
	resp += "\nGoals:\n"
	for i, g := range goals {
		if g.IsCompleted {
			resp += fmt.Sprintf("%d. ✅ %s\n", i+1, g.Title)
		} else {
			line := fmt.Sprintf("%d. ☐ %s", i+1, g.Title)
			if g.DueDate != nil {
				d := g.DueDate.In(s.timezone)
				dateStr := d.Format("2 Jan 2006")
				if d.Before(now) {
					dateStr += " ⚠️"
				}
				line += fmt.Sprintf(" — %s", dateStr)
			}
			resp += line + "\n"
		}
	}

	return resp, nil
}

func (s *Service) AddGoal(ctx context.Context, userID int64, projectName, title string, dueDate *time.Time, hasReminder bool, remindAt *time.Time, recurring string) (string, error) {
	proj, err := s.repo.FindByName(ctx, userID, projectName)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return fmt.Sprintf("❌ Project \"%s\" tidak ditemukan.", projectName), nil
	}

	goalID, err := s.repo.AddGoal(ctx, userID, proj.ID, title, dueDate)
	if err != nil {
		return "", err
	}

	resp := fmt.Sprintf("✅ Goal ditambahkan ke %s: \"%s\"", proj.Name, title)

	if dueDate != nil {
		resp += fmt.Sprintf("\n📅 Deadline: %s", dueDate.In(s.timezone).Format("2 Jan 2006"))
	}

	if hasReminder && remindAt != nil {
		err := s.reminderRepo.Create(ctx, goalID, *remindAt, recurring != "", recurring)
		if err != nil {
			return "", fmt.Errorf("create goal reminder: %w", err)
		}
		resp += fmt.Sprintf("\n⏰ Reminder: %s", remindAt.In(s.timezone).Format("2 Jan 2006 15:04 WIB"))
		if recurring != "" {
			resp += fmt.Sprintf(" (recurring: %s)", recurring)
		}
	}

	return resp, nil
}

func (s *Service) CompleteGoal(ctx context.Context, userID int64, projectName, search string) (string, error) {
	// If project not specified, search across all projects
	if projectName == "" {
		return s.completeGoalAcrossProjects(ctx, userID, search)
	}

	proj, err := s.repo.FindByName(ctx, userID, projectName)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return fmt.Sprintf("❌ Project \"%s\" tidak ditemukan.", projectName), nil
	}

	goal, err := s.repo.FindGoalBySearch(ctx, proj.ID, search)
	if err != nil {
		return "", err
	}
	if goal == nil {
		return fmt.Sprintf("❌ Goal \"%s\" tidak ditemukan di project %s.", search, proj.Name), nil
	}
	if goal.IsCompleted {
		return fmt.Sprintf("ℹ️ Goal \"%s\" sudah selesai sebelumnya.", goal.Title), nil
	}

	if err := s.repo.CompleteGoal(ctx, goal.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Goal selesai di %s: \"%s\"", proj.Name, goal.Title), nil
}

func (s *Service) completeGoalAcrossProjects(ctx context.Context, userID int64, search string) (string, error) {
	matches, err := s.repo.FindGoalAcrossProjects(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return fmt.Sprintf("❌ Goal \"%s\" tidak ditemukan di project manapun.", search), nil
	}
	// Check if all matches are in the same project
	if allSameProject(matches) {
		g := matches[0]
		if g.IsCompleted {
			return fmt.Sprintf("ℹ️ Goal \"%s\" sudah selesai sebelumnya.", g.Title), nil
		}
		if err := s.repo.CompleteGoal(ctx, g.ID); err != nil {
			return "", err
		}
		return fmt.Sprintf("✅ Goal selesai di %s: \"%s\"", g.ProjectName, g.Title), nil
	}
	return formatGoalDisambiguation("selesaikan", search, matches), nil
}

func (s *Service) Delete(ctx context.Context, userID int64, projectName string) (string, error) {
	proj, err := s.repo.FindByName(ctx, userID, projectName)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return fmt.Sprintf("❌ Project \"%s\" tidak ditemukan.", projectName), nil
	}

	if err := s.repo.Delete(ctx, proj.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("🗑️ Project dihapus: \"%s\" (beserta semua goals)", proj.Name), nil
}

func (s *Service) DeleteGoal(ctx context.Context, userID int64, projectName, search string) (string, error) {
	// If project not specified, search across all projects
	if projectName == "" {
		return s.deleteGoalAcrossProjects(ctx, userID, search)
	}

	proj, err := s.repo.FindByName(ctx, userID, projectName)
	if err != nil {
		return "", err
	}
	if proj == nil {
		return fmt.Sprintf("❌ Project \"%s\" tidak ditemukan.", projectName), nil
	}

	goal, err := s.repo.FindGoalBySearch(ctx, proj.ID, search)
	if err != nil {
		return "", err
	}
	if goal == nil {
		return fmt.Sprintf("❌ Goal \"%s\" tidak ditemukan di project %s.", search, proj.Name), nil
	}

	if err := s.repo.DeleteGoal(ctx, goal.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("🗑️ Goal dihapus dari %s: \"%s\"", proj.Name, goal.Title), nil
}

func (s *Service) deleteGoalAcrossProjects(ctx context.Context, userID int64, search string) (string, error) {
	matches, err := s.repo.FindGoalAcrossProjects(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return fmt.Sprintf("❌ Goal \"%s\" tidak ditemukan di project manapun.", search), nil
	}
	if allSameProject(matches) {
		g := matches[0]
		if err := s.repo.DeleteGoal(ctx, g.ID); err != nil {
			return "", err
		}
		return fmt.Sprintf("🗑️ Goal dihapus dari %s: \"%s\"", g.ProjectName, g.Title), nil
	}
	return formatGoalDisambiguation("hapus", search, matches), nil
}

// allSameProject returns true if all GoalWithProject entries belong to the same project.
func allSameProject(goals []GoalWithProject) bool {
	if len(goals) == 0 {
		return true
	}
	first := goals[0].ProjectID
	for _, g := range goals[1:] {
		if g.ProjectID != first {
			return false
		}
	}
	return true
}

// formatGoalDisambiguation builds a message asking the user to specify the project.
func formatGoalDisambiguation(action, search string, matches []GoalWithProject) string {
	// Collect unique project names
	seen := make(map[string]bool)
	var projectNames []string
	for _, g := range matches {
		if !seen[g.ProjectName] {
			seen[g.ProjectName] = true
			projectNames = append(projectNames, g.ProjectName)
		}
	}

	msg := fmt.Sprintf("🔍 Goal \"%s\" ditemukan di %d project:\n", search, len(projectNames))
	for i, name := range projectNames {
		msg += fmt.Sprintf("%d. %s\n", i+1, name)
	}
	msg += fmt.Sprintf("\nSebutkan projectnya, contoh:\n\"")
	msg += fmt.Sprintf("%s goal %s di %s\"", action, search, projectNames[0])
	return msg
}
