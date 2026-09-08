package saving

import (
	"backend/internal/shared/timeutil"
	"encoding/json"
	"time"
)

type RangeResponse struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type TargetProgressResponse struct {
	Amount      int     `json:"amount"`
	Achieved    bool    `json:"achieved"`
	ProgressPct float64 `json:"progress_pct"`
	Difference  int     `json:"difference"`
}

type SummaryResponse struct {
	Period              string                  `json:"period"`
	Range               RangeResponse           `json:"range"`
	Income              int                     `json:"income"`
	Expense             int                     `json:"expense"`
	Savable             int                     `json:"savable"`
	SavingsRate         float64                 `json:"savings_rate"`
	Target              *TargetProgressResponse `json:"target"`
	DailyAverageExpense int                     `json:"daily_average_expense"`
	ProjectedSavable    int                     `json:"projected_savable"`
	DaysElapsed         int                     `json:"days_elapsed"`
	DaysTotal           int                     `json:"days_total"`
}

func NewSummaryResponse(s Summary) SummaryResponse {
	resp := SummaryResponse{
		Period:              s.Period,
		Range:               RangeResponse{From: timeutil.FormatDate(s.From), To: timeutil.FormatDate(s.To)},
		Income:              s.Income,
		Expense:             s.Expense,
		Savable:             s.Savable,
		SavingsRate:         s.SavingsRate,
		DailyAverageExpense: s.DailyAverageExpense,
		ProjectedSavable:    s.ProjectedSavable,
		DaysElapsed:         s.DaysElapsed,
		DaysTotal:           s.DaysTotal,
	}
	if s.Target != nil {
		resp.Target = &TargetProgressResponse{
			Amount:      s.Target.Amount,
			Achieved:    s.Target.Achieved,
			ProgressPct: s.Target.ProgressPct,
			Difference:  s.Target.Difference,
		}
	}
	return resp
}

type BreakdownBucketResponse struct {
	Bucket      string  `json:"bucket"`
	Start       string  `json:"start"`
	End         string  `json:"end"`
	Income      int     `json:"income"`
	Expense     int     `json:"expense"`
	Savable     int     `json:"savable"`
	SavingsRate float64 `json:"savings_rate"`
}

func NewBreakdownResponse(buckets []BreakdownBucket) []BreakdownBucketResponse {
	resp := make([]BreakdownBucketResponse, 0, len(buckets))
	for _, b := range buckets {
		resp = append(resp, BreakdownBucketResponse{
			Bucket:      b.Bucket,
			Start:       timeutil.FormatDate(b.Start),
			End:         timeutil.FormatDate(b.End),
			Income:      b.Income,
			Expense:     b.Expense,
			Savable:     b.Savable,
			SavingsRate: b.SavingsRate,
		})
	}
	return resp
}

type TargetResponse struct {
	ID         string     `json:"id"`
	Period     string     `json:"period"`
	Amount     *int       `json:"amount"`
	TargetRate *float64   `json:"target_rate"`
	StartDate  string     `json:"start_date"`
	EndDate    *string    `json:"end_date"`
	IsActive   bool       `json:"is_active"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
}

func NewTargetResponse(t Target) TargetResponse {
	resp := TargetResponse{
		ID:        t.ID,
		Period:    t.Period,
		StartDate: timeutil.FormatDate(t.StartDate),
		IsActive:  t.IsActive,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.Amount.Valid {
		amount := int(t.Amount.Int64)
		resp.Amount = &amount
	}
	if t.TargetRate.Valid {
		rate := t.TargetRate.Float64
		resp.TargetRate = &rate
	}
	if t.EndDate.Valid {
		endDate := timeutil.FormatDate(t.EndDate.Time)
		resp.EndDate = &endDate
	}
	if t.DeletedAt.Valid {
		deletedAt := t.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

// CreateTargetRequest dan PatchTargetRequest memakai string untuk tanggal
// supaya client cukup mengirim "2026-08-01", bukan timestamp lengkap.
type CreateTargetRequest struct {
	Period     string
	Amount     *int
	TargetRate *float64
	StartDate  *time.Time
	EndDate    *time.Time
	IsActive   *bool
}

type PatchTargetRequest struct {
	Period       *string
	Amount       *int
	TargetRate   *float64
	StartDate    *time.Time
	EndDate      *time.Time
	IsActive     *bool
	ClearEndDate bool
}

// createTargetBody adalah bentuk JSON mentah dari client.
type createTargetBody struct {
	Period     string   `json:"period"`
	Amount     *int     `json:"amount"`
	TargetRate *float64 `json:"target_rate"`
	StartDate  string   `json:"start_date"`
	EndDate    *string  `json:"end_date"`
	IsActive   *bool    `json:"is_active"`
}

// patchTargetBody memakai json.RawMessage untuk end_date supaya bisa dibedakan
// antara field yang tidak dikirim (nil) dan yang sengaja dikirim null
// (artinya: hapus tanggal akhir, target berlaku terus).
type patchTargetBody struct {
	Period     *string         `json:"period"`
	Amount     *int            `json:"amount"`
	TargetRate *float64        `json:"target_rate"`
	StartDate  *string         `json:"start_date"`
	EndDate    json.RawMessage `json:"end_date"`
	IsActive   *bool           `json:"is_active"`
}
