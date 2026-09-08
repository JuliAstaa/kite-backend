package budget

import (
	"database/sql"
	"time"
)

type Budget struct {
	ID           string
	CategoryID   string
	CategoryName string
	CategoryType string
	Amount       int
	Period       string
	StartMonth   time.Time
	EndMonth     sql.NullTime
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    sql.NullTime
}

type CreateBudgetParams struct {
	CategoryID string
	Amount     int
	Period     string
	StartMonth time.Time
	EndMonth   *time.Time
}

type PatchBudgetParams struct {
	Amount        *int
	Period        *string
	StartMonth    *time.Time
	EndMonth      *time.Time
	ClearEndMonth bool
}

// Status adalah progres satu budget pada rentang yang sedang dilihat.
type Status struct {
	BudgetID     string
	CategoryID   string
	CategoryName string
	Limit        int
	Spent        int
	Remaining    int
	Percentage   float64
	State        string
	DaysLeft     int
	From         time.Time
	To           time.Time
	Period       string
}

type CategoryInfo struct {
	ID   string
	Type string
}
