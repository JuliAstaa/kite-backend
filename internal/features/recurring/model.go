package recurring

import (
	"database/sql"
	"time"
)

type Rule struct {
	ID         string
	Name       string
	Type       string
	Amount     int
	WalletID   string
	CategoryID string
	Note       string
	Frequency  string
	Interval   int
	DayOfMonth sql.NullInt64
	DayOfWeek  sql.NullInt64
	StartDate  time.Time
	EndDate    sql.NullTime
	NextRunAt  time.Time
	LastRunAt  sql.NullTime
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  sql.NullTime
}

type CreateRuleParams struct {
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
	StartDate  time.Time
	EndDate    *time.Time
	NextRunAt  time.Time
	IsActive   bool
}

type PatchRuleParams struct {
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

// RunResult adalah ringkasan satu putaran scheduler.
type RunResult struct {
	RulesChecked  int
	Created       int
	Skipped       int
	RulesFailed   int
	FailedRuleIDs []string
}

type CategoryInfo struct {
	ID   string
	Type string
}
