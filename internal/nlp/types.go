package nlp

import (
	"fmt"
	"time"
)

type ParsedIntent struct {
	Intent      string  `json:"intent"`
	Title       string  `json:"title,omitempty"`
	Search      string  `json:"search,omitempty"`
	Filter      string  `json:"filter,omitempty"`
	Amount      int64   `json:"amount,omitempty"`
	Description string  `json:"description,omitempty"`
	Project     string  `json:"project,omitempty"`
	Name        string  `json:"name,omitempty"`
	Reminder    bool    `json:"reminder,omitempty"`
	RemindAt    string  `json:"remind_at,omitempty"`
	Recurring   string  `json:"recurring,omitempty"`
	DueDate     string  `json:"due_date,omitempty"`
	IsPaid      *bool   `json:"is_paid,omitempty"`
	Raw         string  `json:"raw,omitempty"`
	// Expense-specific fields
	Date        string  `json:"date,omitempty"`       // filter by recorded date (YYYY-MM-DD)
	Month       int     `json:"month,omitempty"`      // 1-12, for clear_expense
	Year        int     `json:"year,omitempty"`       // e.g. 2026, for clear_expense
	NewTitle    string  `json:"new_title,omitempty"`  // edit_expense: new description
	NewIsPaid   *bool   `json:"new_is_paid,omitempty"` // edit_expense: new paid status
	ExpenseID   int     `json:"expense_id,omitempty"` // direct ID reference for delete/edit
}

func (p *ParsedIntent) ParseDate(loc *time.Location) (*time.Time, error) {
	if p.Date == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", p.Date, loc)
	if err != nil {
		return nil, fmt.Errorf("unsupported date format: %s", p.Date)
	}
	return &t, nil
}

func (p *ParsedIntent) ParseRemindAt(loc *time.Location) (*time.Time, error) {
	if p.RemindAt == "" {
		return nil, nil
	}
	// Try RFC3339 first (e.g. 2026-02-13T23:18:00+07:00)
	if t, err := time.Parse(time.RFC3339, p.RemindAt); err == nil {
		t = t.In(loc)
		return &t, nil
	}
	// Try without timezone (e.g. 2026-02-13T23:18:00)
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", p.RemindAt, loc); err == nil {
		return &t, nil
	}
	// Try date + time without seconds (e.g. 2026-02-13T23:18)
	if t, err := time.ParseInLocation("2006-01-02T15:04", p.RemindAt, loc); err == nil {
		return &t, nil
	}
	return nil, fmt.Errorf("unsupported remind_at format: %s", p.RemindAt)
}

func (p *ParsedIntent) ParseDueDate(loc *time.Location) (*time.Time, error) {
	if p.DueDate == "" {
		return nil, nil
	}
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, p.DueDate); err == nil {
		t = t.In(loc)
		return &t, nil
	}
	// Try date-only format
	t, err := time.ParseInLocation("2006-01-02", p.DueDate, loc)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
