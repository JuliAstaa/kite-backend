package recurring

import (
	"backend/internal/shared/timeutil"
	"encoding/json"
	"time"
)

type RuleResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       string     `json:"type"`
	Amount     int        `json:"amount"`
	WalletID   string     `json:"wallet_id"`
	CategoryID string     `json:"category_id"`
	Note       string     `json:"note"`
	Frequency  string     `json:"frequency"`
	Interval   int        `json:"interval"`
	DayOfMonth *int       `json:"day_of_month"`
	DayOfWeek  *int       `json:"day_of_week"`
	StartDate  string     `json:"start_date"`
	EndDate    *string    `json:"end_date"`
	NextRunAt  string     `json:"next_run_at"`
	LastRunAt  *string    `json:"last_run_at"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
}

func NewRuleResponse(r Rule) RuleResponse {
	resp := RuleResponse{
		ID:         r.ID,
		Name:       r.Name,
		Type:       r.Type,
		Amount:     r.Amount,
		WalletID:   r.WalletID,
		CategoryID: r.CategoryID,
		Note:       r.Note,
		Frequency:  r.Frequency,
		Interval:   r.Interval,
		StartDate:  timeutil.FormatDate(r.StartDate),
		NextRunAt:  timeutil.FormatDate(r.NextRunAt),
		IsActive:   r.IsActive,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if r.DayOfMonth.Valid {
		day := int(r.DayOfMonth.Int64)
		resp.DayOfMonth = &day
	}
	if r.DayOfWeek.Valid {
		day := int(r.DayOfWeek.Int64)
		resp.DayOfWeek = &day
	}
	if r.EndDate.Valid {
		endDate := timeutil.FormatDate(r.EndDate.Time)
		resp.EndDate = &endDate
	}
	if r.LastRunAt.Valid {
		lastRunAt := timeutil.FormatDate(r.LastRunAt.Time)
		resp.LastRunAt = &lastRunAt
	}
	if r.DeletedAt.Valid {
		deletedAt := r.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

type RunResponse struct {
	RulesChecked  int      `json:"rules_checked"`
	Created       int      `json:"created"`
	Skipped       int      `json:"skipped"`
	RulesFailed   int      `json:"rules_failed"`
	FailedRuleIDs []string `json:"failed_rule_ids"`
}

type CreateRuleRequest struct {
	Name       string
	Type       string
	Amount     int
	WalletID   string
	CategoryID string
	Note       string
	Frequency  string
	Interval   int
	DayOfMonth *int
	DayOfWeek  *int
	StartDate  *time.Time
	EndDate    *time.Time
	IsActive   *bool
}

type PatchRuleRequest struct {
	Name         *string
	Amount       *int
	WalletID     *string
	CategoryID   *string
	Note         *string
	Frequency    *string
	Interval     *int
	DayOfMonth   *int
	DayOfWeek    *int
	StartDate    *time.Time
	EndDate      *time.Time
	ClearEndDate bool
	NextRunAt    *time.Time
	IsActive     *bool
}

type createRuleBody struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Amount     int     `json:"amount"`
	WalletID   string  `json:"wallet_id"`
	CategoryID string  `json:"category_id"`
	Note       string  `json:"note"`
	Frequency  string  `json:"frequency"`
	Interval   int     `json:"interval"`
	DayOfMonth *int    `json:"day_of_month"`
	DayOfWeek  *int    `json:"day_of_week"`
	StartDate  string  `json:"start_date"`
	EndDate    *string `json:"end_date"`
	IsActive   *bool   `json:"is_active"`
}

type patchRuleBody struct {
	Name       *string         `json:"name"`
	Amount     *int            `json:"amount"`
	WalletID   *string         `json:"wallet_id"`
	CategoryID *string         `json:"category_id"`
	Note       *string         `json:"note"`
	Frequency  *string         `json:"frequency"`
	Interval   *int            `json:"interval"`
	DayOfMonth *int            `json:"day_of_month"`
	DayOfWeek  *int            `json:"day_of_week"`
	StartDate  *string         `json:"start_date"`
	EndDate    json.RawMessage `json:"end_date"`
	NextRunAt  *string         `json:"next_run_at"`
	IsActive   *bool           `json:"is_active"`
}
