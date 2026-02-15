package expense

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

var indonesianMonths = [...]string{
	"Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
}

var indonesianMonthsFull = [...]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

type Service struct {
	repo     *Repository
	timezone *time.Location
}

func NewService(repo *Repository, timezone *time.Location) *Service {
	return &Service{repo: repo, timezone: timezone}
}

// Add records an expense and returns a formatted notification (Template 3).
func (s *Service) Add(ctx context.Context, userID int64, description string, amount int64, isPaid bool) (string, error) {
	_, err := s.repo.Create(ctx, userID, description, amount, isPaid)
	if err != nil {
		return "", err
	}

	now := time.Now().In(s.timezone)
	dateStr := fmt.Sprintf("%d %s %d", now.Day(), indonesianMonths[now.Month()-1], now.Year())

	status := "Lunas"
	if !isPaid {
		status = "Belum lunas"
	}

	// Get monthly total
	monthTotal, err := s.repo.SumByMonth(ctx, userID, now.Year(), now.Month(), s.timezone)
	if err != nil {
		monthTotal = 0
	}

	return fmt.Sprintf("✅ Pengeluaran dicatat!\n\n📝 %s\n💵 %s\n📅 %s\n📊 Status: %s\n\nTotal bulan ini: %s",
		description, FormatRupiah(amount), dateStr, status, FormatRupiah(monthTotal)), nil
}

// List returns a formatted expense list based on the filter.
func (s *Service) List(ctx context.Context, userID int64, filter string) (string, error) {
	expenses, err := s.repo.List(ctx, userID, filter, s.timezone)
	if err != nil {
		return "", err
	}

	if len(expenses) == 0 {
		return fmt.Sprintf("📭 Tidak ada pengeluaran %s.", filterLabel(filter)), nil
	}

	if filter == "all" {
		return s.formatAllExpenses(expenses), nil
	}
	return s.formatMonthlyExpenses(expenses, filter), nil
}

// PayExpense marks an expense as paid.
// amount and date are optional disambiguators when multiple expenses share the same description.
func (s *Service) PayExpense(ctx context.Context, userID int64, search string, amount int64, date *time.Time) (string, error) {
	matches, err := s.repo.FindAllBySearch(ctx, userID, search)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return fmt.Sprintf("❌ Pengeluaran \"%s\" tidak ditemukan.", search), nil
	}

	expense := s.pickExpense(matches, amount, date)
	if expense == nil {
		return s.formatDisambiguation(search, matches, "lunasi"), nil
	}

	if expense.IsPaid {
		return fmt.Sprintf("ℹ️ \"%s\" — %s sudah lunas.", expense.Description, FormatRupiah(expense.Amount)), nil
	}

	if err := s.repo.MarkPaid(ctx, expense.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("✅ Lunas: \"%s\" — %s", expense.Description, FormatRupiah(expense.Amount)), nil
}

// Delete removes an expense.
// expenseID: if > 0, look up directly by ID (bypasses search).
// amount and date are optional disambiguators when multiple expenses share the same description.
func (s *Service) Delete(ctx context.Context, userID int64, expenseID int, search string, amount int64, date *time.Time) (string, error) {
	var exp *Expense

	if expenseID > 0 {
		found, err := s.repo.FindByID(ctx, userID, expenseID)
		if err != nil {
			return "", err
		}
		if found == nil {
			return fmt.Sprintf("❌ Pengeluaran dengan ID #%d tidak ditemukan.", expenseID), nil
		}
		exp = found
	} else {
		matches, err := s.repo.FindAllBySearch(ctx, userID, search)
		if err != nil {
			return "", err
		}
		if len(matches) == 0 {
			return fmt.Sprintf("❌ Pengeluaran \"%s\" tidak ditemukan.", search), nil
		}

		exp = s.pickExpense(matches, amount, date)
		if exp == nil {
			return s.formatDisambiguation(search, matches, "hapus"), nil
		}
	}

	if err := s.repo.Delete(ctx, exp.ID); err != nil {
		return "", err
	}

	return fmt.Sprintf("🗑️ Dihapus: \"%s\" — %s", exp.Description, FormatRupiah(exp.Amount)), nil
}

// Edit updates description and/or paid status of an expense.
// expenseID: if > 0, look up directly by ID (bypasses search).
// amount and date are optional disambiguators.
func (s *Service) Edit(ctx context.Context, userID int64, expenseID int, search string, amount int64, date *time.Time, newTitle string, newIsPaid *bool) (string, error) {
	if newTitle == "" && newIsPaid == nil {
		return "ℹ️ Tidak ada perubahan yang diminta.", nil
	}

	var expense *Expense

	if expenseID > 0 {
		found, err := s.repo.FindByID(ctx, userID, expenseID)
		if err != nil {
			return "", err
		}
		if found == nil {
			return fmt.Sprintf("❌ Pengeluaran dengan ID #%d tidak ditemukan.", expenseID), nil
		}
		expense = found
	} else {
		matches, err := s.repo.FindAllBySearch(ctx, userID, search)
		if err != nil {
			return "", err
		}
		if len(matches) == 0 {
			return fmt.Sprintf("❌ Pengeluaran \"%s\" tidak ditemukan.", search), nil
		}

		expense = s.pickExpense(matches, amount, date)
		if expense == nil {
			return s.formatDisambiguation(search, matches, "edit"), nil
		}
	}

	var descPtr *string
	if newTitle != "" {
		descPtr = &newTitle
	}
	if err := s.repo.UpdateExpense(ctx, expense.ID, descPtr, newIsPaid); err != nil {
		return "", fmt.Errorf("update expense: %w", err)
	}

	displayDesc := expense.Description
	if newTitle != "" {
		displayDesc = newTitle
	}
	statusStr := ""
	if newIsPaid != nil {
		if *newIsPaid {
			statusStr = " · ✅ Lunas"
		} else {
			statusStr = " · 🔴 Belum lunas"
		}
	}
	return fmt.Sprintf("✏️ Pengeluaran diperbarui: \"%s\" — %s%s", displayDesc, FormatRupiah(expense.Amount), statusStr), nil
}

// ClearByMonth deletes all expenses for a specific year/month.
// If year is 0 and the month exists across multiple years, returns a disambiguation prompt.
func (s *Service) ClearByMonth(ctx context.Context, userID int64, month, year int) (string, error) {
	if month < 1 || month > 12 {
		return "❌ Bulan tidak valid.", nil
	}

	if year == 0 {
		years, err := s.repo.ListYearsForMonth(ctx, userID, month, s.timezone)
		if err != nil {
			return "", err
		}
		if len(years) == 0 {
			return fmt.Sprintf("📭 Tidak ada pengeluaran di %s.", indonesianMonthsFull[month-1]), nil
		}
		if len(years) > 1 {
			lines := []string{fmt.Sprintf("🔍 Pengeluaran \"%s\" ada di beberapa tahun:\n", indonesianMonthsFull[month-1])}
			for _, y := range years {
				lines = append(lines, fmt.Sprintf("• %s %d", indonesianMonthsFull[month-1], y))
			}
			lines = append(lines, "\nSebutkan tahunnya, contoh:")
			lines = append(lines, fmt.Sprintf("• \"kosongkan %s %d\"", indonesianMonthsFull[month-1], years[len(years)-1]))
			return strings.Join(lines, "\n"), nil
		}
		year = years[0]
	}

	count, err := s.repo.ClearByMonth(ctx, userID, year, time.Month(month), s.timezone)
	if err != nil {
		return "", err
	}
	if count == 0 {
		return fmt.Sprintf("📭 Tidak ada pengeluaran di %s %d.", indonesianMonthsFull[month-1], year), nil
	}
	return fmt.Sprintf("🗑️ %d pengeluaran di %s %d dihapus.", count, indonesianMonthsFull[month-1], year), nil
}

// pickExpense returns the single matching expense.
// Filters by amount (if > 0) and by recorded date (if non-nil). Returns nil when ambiguous.
func (s *Service) pickExpense(matches []Expense, amount int64, date *time.Time) *Expense {
	if len(matches) == 1 {
		return &matches[0]
	}

	filtered := matches
	if date != nil {
		d := date.In(s.timezone)
		dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, s.timezone)
		dayEnd := dayStart.AddDate(0, 0, 1)
		var byDate []Expense
		for _, e := range filtered {
			t := e.RecordedAt.In(s.timezone)
			if !t.Before(dayStart) && t.Before(dayEnd) {
				byDate = append(byDate, e)
			}
		}
		filtered = byDate
	}
	if amount > 0 {
		var byAmount []Expense
		for _, e := range filtered {
			if e.Amount == amount {
				byAmount = append(byAmount, e)
			}
		}
		filtered = byAmount
	}

	if len(filtered) == 1 {
		return &filtered[0]
	}
	return nil
}

// formatDisambiguation builds a disambiguation message listing all matching expenses with their IDs.
func (s *Service) formatDisambiguation(search string, matches []Expense, action string) string {
	lines := []string{
		fmt.Sprintf("🔍 Ada %d pengeluaran \"%s\":\n", len(matches), search),
	}
	for _, e := range matches {
		t := e.RecordedAt.In(s.timezone)
		statusIcon := "✅"
		statusLabel := "Lunas"
		if !e.IsPaid {
			statusIcon = "🔴"
			statusLabel = "Belum lunas"
		}
		lines = append(lines, fmt.Sprintf("#%d · 📅 %d %s %d · %s · %s %s",
			e.ID,
			t.Day(), indonesianMonths[t.Month()-1], t.Year(),
			FormatRupiah(e.Amount),
			statusIcon, statusLabel,
		))
	}

	// Build example commands using ID
	lines = append(lines, fmt.Sprintf("\nSebutkan ID-nya untuk diproses, contoh:"))
	for _, e := range matches {
		lines = append(lines, fmt.Sprintf("• \"%s id %d\"", action, e.ID))
	}

	return strings.Join(lines, "\n")
}

// formatRupiahShort converts amount to shorthand: 35000 → "35rb", 1500000 → "1.5jt".
func formatRupiahShort(amount int64) string {
	switch {
	case amount >= 1_000_000 && amount%1_000_000 == 0:
		return fmt.Sprintf("%djt", amount/1_000_000)
	case amount >= 1_000_000:
		f := float64(amount) / 1_000_000
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2fjt", f), "0"), ".")
	case amount >= 1_000 && amount%1_000 == 0:
		return fmt.Sprintf("%drb", amount/1_000)
	default:
		return fmt.Sprintf("%d", amount)
	}
}

// MonthlyReport generates a full monthly report (Template 4).
func (s *Service) MonthlyReport(ctx context.Context, userID int64, year int, month time.Month) (string, error) {
	expenses, err := s.repo.ListByMonth(ctx, userID, year, month, s.timezone)
	if err != nil {
		return "", err
	}
	if len(expenses) == 0 {
		monthName := fmt.Sprintf("%s %d", indonesianMonthsFull[month-1], year)
		return fmt.Sprintf("📭 Tidak ada pengeluaran di %s.", monthName), nil
	}

	return s.formatMonthlyReport(expenses, year, month), nil
}

// formatAllExpenses formats all expenses grouped by month (Template 1).
func (s *Service) formatAllExpenses(expenses []Expense) string {
	now := time.Now().In(s.timezone)

	// Group by year-month
	type monthKey struct {
		year  int
		month time.Month
	}
	grouped := make(map[monthKey][]Expense)
	var keys []monthKey
	seen := make(map[monthKey]bool)

	for _, e := range expenses {
		t := e.RecordedAt.In(s.timezone)
		k := monthKey{t.Year(), t.Month()}
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
		grouped[k] = append(grouped[k], e)
	}

	// Sort keys descending (newest first)
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].year != keys[j].year {
			return keys[i].year > keys[j].year
		}
		return keys[i].month > keys[j].month
	})

	var lines []string
	lines = append(lines, fmt.Sprintf("💰 Pengeluaran %d\n", now.Year()))

	var grandTotal int64

	for _, k := range keys {
		monthExpenses := grouped[k]
		lines = append(lines, fmt.Sprintf("📅 %s %d", indonesianMonthsFull[k.month-1], k.year))

		var monthTotal int64
		var unpaidCount int
		for _, e := range monthExpenses {
			t := e.RecordedAt.In(s.timezone)
			icon := "✅"
			if !e.IsPaid {
				icon = "🔴"
				unpaidCount++
			}
			lines = append(lines, fmt.Sprintf("%s %d %s · %s · %s",
				icon, t.Day(), indonesianMonths[t.Month()-1], e.Description, FormatRupiah(e.Amount)))
			monthTotal += e.Amount
		}

		monthShort := indonesianMonths[k.month-1]
		suffix := ""
		if unpaidCount > 0 {
			suffix = fmt.Sprintf(" (%d belum lunas)", unpaidCount)
		}
		lines = append(lines, fmt.Sprintf("── %s: %s%s ──\n", monthShort, FormatRupiah(monthTotal), suffix))

		grandTotal += monthTotal
	}

	lines = append(lines, "─────────────")
	lines = append(lines, fmt.Sprintf("💵 Total: %s", FormatRupiah(grandTotal)))

	return strings.Join(lines, "\n")
}

// formatMonthlyExpenses formats expenses for a single month/period (Template 2).
func (s *Service) formatMonthlyExpenses(expenses []Expense, filter string) string {
	now := time.Now().In(s.timezone)

	var lines []string
	lines = append(lines, fmt.Sprintf("💰 %s %d\n", indonesianMonthsFull[now.Month()-1], now.Year()))

	var total, paidTotal, unpaidTotal int64
	var paidCount, unpaidCount int

	for _, e := range expenses {
		t := e.RecordedAt.In(s.timezone)
		icon := "✅"
		if !e.IsPaid {
			icon = "🔴"
			unpaidTotal += e.Amount
			unpaidCount++
		} else {
			paidTotal += e.Amount
			paidCount++
		}
		lines = append(lines, fmt.Sprintf("%s %d %s · %s · %s",
			icon, t.Day(), indonesianMonths[t.Month()-1], e.Description, FormatRupiah(e.Amount)))
		total += e.Amount
	}

	lines = append(lines, "\n─────────────")
	lines = append(lines, fmt.Sprintf("💵 Total: %s", FormatRupiah(total)))
	if paidCount > 0 {
		lines = append(lines, fmt.Sprintf("✅ Lunas: %s (%d)", FormatRupiah(paidTotal), paidCount))
	}
	if unpaidCount > 0 {
		lines = append(lines, fmt.Sprintf("🔴 Belum: %s (%d)", FormatRupiah(unpaidTotal), unpaidCount))
	}

	return strings.Join(lines, "\n")
}

// formatMonthlyReport generates a detailed monthly report (Template 4).
func (s *Service) formatMonthlyReport(expenses []Expense, year int, month time.Month) string {
	monthName := fmt.Sprintf("%s %d", indonesianMonthsFull[month-1], year)

	var lines []string
	lines = append(lines, fmt.Sprintf("💰 Laporan Pengeluaran — %s\n", monthName))
	lines = append(lines, "━━━━━━━━━━━━━━━━━━━━\n")

	// Separate paid and unpaid
	var paid, unpaid []Expense
	var paidTotal, unpaidTotal int64
	for _, e := range expenses {
		if e.IsPaid {
			paid = append(paid, e)
			paidTotal += e.Amount
		} else {
			unpaid = append(unpaid, e)
			unpaidTotal += e.Amount
		}
	}

	// Paid section
	lines = append(lines, fmt.Sprintf("✅ Lunas (%d item)", len(paid)))
	maxShow := 8
	for i, e := range paid {
		if i >= maxShow {
			lines = append(lines, fmt.Sprintf("  ... dan %d lainnya", len(paid)-maxShow))
			break
		}
		t := e.RecordedAt.In(s.timezone)
		lines = append(lines, fmt.Sprintf("  %d %s · %s · %s",
			t.Day(), indonesianMonths[t.Month()-1], e.Description, FormatRupiah(e.Amount)))
	}

	// Unpaid section
	if len(unpaid) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("🔴 Belum Lunas (%d item)", len(unpaid)))
		for _, e := range unpaid {
			t := e.RecordedAt.In(s.timezone)
			lines = append(lines, fmt.Sprintf("  %d %s · %s · %s",
				t.Day(), indonesianMonths[t.Month()-1], e.Description, FormatRupiah(e.Amount)))
		}
	}

	lines = append(lines, "\n━━━━━━━━━━━━━━━━━━━━\n")
	lines = append(lines, "📊 Ringkasan\n")

	grandTotal := paidTotal + unpaidTotal
	lines = append(lines, fmt.Sprintf("  Total         : %s", FormatRupiah(grandTotal)))
	lines = append(lines, fmt.Sprintf("  ✅ Lunas      : %s", FormatRupiah(paidTotal)))
	if unpaidTotal > 0 {
		lines = append(lines, fmt.Sprintf("  🔴 Belum      : %s", FormatRupiah(unpaidTotal)))
	}

	// Top 3 biggest expenses
	sorted := make([]Expense, len(expenses))
	copy(sorted, expenses)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Amount > sorted[j].Amount
	})

	lines = append(lines, "")
	lines = append(lines, "  Item terbesar :")
	topN := 3
	if len(sorted) < topN {
		topN = len(sorted)
	}
	for i := 0; i < topN; i++ {
		lines = append(lines, fmt.Sprintf("  %d. %s — %s", i+1, sorted[i].Description, FormatRupiah(sorted[i].Amount)))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  Jumlah transaksi : %d", len(expenses)))

	// Next month recurring reminders section
	// This will be populated by the caller if needed
	lines = append(lines, "\n━━━━━━━━━━━━━━━━━━━━")

	return strings.Join(lines, "\n")
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
