package budget

import (
	"backend/internal/shared/timeutil"
	"encoding/json"
	"time"
)

type BudgetResponse struct {
	ID           string     `json:"id"`
	CategoryID   string     `json:"category_id"`
	CategoryName string     `json:"category_name"`
	Amount       int        `json:"amount"`
	Period       string     `json:"period"`
	StartMonth   string     `json:"start_month"`
	EndMonth     *string    `json:"end_month"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at"`
}

func NewBudgetResponse(b Budget) BudgetResponse {
	resp := BudgetResponse{
		ID:           b.ID,
		CategoryID:   b.CategoryID,
		CategoryName: b.CategoryName,
		Amount:       b.Amount,
		Period:       b.Period,
		StartMonth:   timeutil.FormatDate(b.StartMonth),
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
	}
	if b.EndMonth.Valid {
		endMonth := timeutil.FormatDate(b.EndMonth.Time)
		resp.EndMonth = &endMonth
	}
	if b.DeletedAt.Valid {
		deletedAt := b.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

type StatusResponse struct {
	BudgetID     string  `json:"budget_id"`
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Limit        int     `json:"limit"`
	Spent        int     `json:"spent"`
	Remaining    int     `json:"remaining"`
	Percentage   float64 `json:"percentage"`
	Status       string  `json:"status"`
	DaysLeft     int     `json:"days_left"`
	Period       string  `json:"period"`
	From         string  `json:"from"`
	To           string  `json:"to"`
}

func NewStatusResponse(s Status) StatusResponse {
	return StatusResponse{
		BudgetID:     s.BudgetID,
		CategoryID:   s.CategoryID,
		CategoryName: s.CategoryName,
		Limit:        s.Limit,
		Spent:        s.Spent,
		Remaining:    s.Remaining,
		Percentage:   s.Percentage,
		Status:       s.State,
		DaysLeft:     s.DaysLeft,
		Period:       s.Period,
		From:         timeutil.FormatDate(s.From),
		To:           timeutil.FormatDate(s.To),
	}
}

type CreateBudgetRequest struct {
	CategoryID string
	Amount     int
	Period     string
	StartMonth *time.Time
	EndMonth   *time.Time
}

type PatchBudgetRequest struct {
	Amount        *int
	Period        *string
	StartMonth    *time.Time
	EndMonth      *time.Time
	ClearEndMonth bool
}

type createBudgetBody struct {
	CategoryID string  `json:"category_id"`
	Amount     int     `json:"amount"`
	Period     string  `json:"period"`
	StartMonth string  `json:"start_month"`
	EndMonth   *string `json:"end_month"`
}

// end_month memakai json.RawMessage supaya null (berlaku terus) bisa dibedakan
// dari field yang memang tidak dikirim.
type patchBudgetBody struct {
	Amount     *int            `json:"amount"`
	Period     *string         `json:"period"`
	StartMonth *string         `json:"start_month"`
	EndMonth   json.RawMessage `json:"end_month"`
}
