package expense

import (
	"context"
	"fmt"
	"time"
)

type Service struct {
	repo     *Repository
	timezone *time.Location
}

func NewService(repo *Repository, timezone *time.Location) *Service {
	return &Service{repo: repo, timezone: timezone}
}

func (s *Service) Add(ctx context.Context, userID int64, description string, amount int64) (string, error) {
	_, err := s.repo.Create(ctx, userID, description, amount)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("💰 Pengeluaran dicatat: %s — %s", description, FormatRupiah(amount)), nil
}

func (s *Service) List(ctx context.Context, userID int64, filter string) (string, error) {
	expenses, err := s.repo.List(ctx, userID, filter, s.timezone)
	if err != nil {
		return "", err
	}
	total, err := s.repo.Sum(ctx, userID, filter, s.timezone)
	if err != nil {
		return "", err
	}

	if len(expenses) == 0 {
		return fmt.Sprintf("📭 Tidak ada pengeluaran %s.", filterLabel(filter)), nil
	}

	resp := fmt.Sprintf("💰 Pengeluaran %s:\n", filterLabel(filter))
	for _, e := range expenses {
		resp += fmt.Sprintf("• %s — %s\n", e.Description, FormatRupiah(e.Amount))
	}
	resp += fmt.Sprintf("\nTotal: %s", FormatRupiah(total))
	return resp, nil
}

func (s *Service) Delete(ctx context.Context, userID int64, search string) (string, error) {
	expense, err := s.repo.FindBySearch(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if expense == nil {
		return fmt.Sprintf("❌ Pengeluaran \"%s\" tidak ditemukan.", search), nil
	}

	if err := s.repo.Delete(ctx, expense.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("🗑️ Pengeluaran dihapus: %s — %s", expense.Description, FormatRupiah(expense.Amount)), nil
}

func FormatRupiah(amount int64) string {
	s := fmt.Sprintf("%d", amount)
	n := len(s)
	if n <= 3 {
		return "Rp " + s
	}

	var result []byte
	for i, c := range s {
		if i > 0 && (n-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}
	return "Rp " + string(result)
}

func filterLabel(filter string) string {
	switch filter {
	case "today":
		return "Hari Ini"
	case "this_week":
		return "Minggu Ini"
	case "this_month":
		return "Bulan Ini"
	default:
		return "Semua"
	}
}
