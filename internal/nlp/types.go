package nlp

import "time"

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
	Raw         string  `json:"raw,omitempty"`
}

func (p *ParsedIntent) ParseRemindAt(loc *time.Location) (*time.Time, error) {
	if p.RemindAt == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, p.RemindAt)
	if err != nil {
		return nil, err
	}
	t = t.In(loc)
	return &t, nil
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
