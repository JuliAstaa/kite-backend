package wishlist

import (
	"backend/internal/shared/timeutil"
	"encoding/json"
	"math"
	"time"
)

type AffordabilityResponse struct {
	AvgMonthlySavable   int     `json:"avg_monthly_savable"`
	MonthsNeeded        *int    `json:"months_needed"`
	EstimatedReadyDate  *string `json:"estimated_ready_date"`
	OnTrackForTargetDay bool    `json:"on_track_for_target_date"`
}

type ItemResponse struct {
	ID                    string                 `json:"id"`
	Name                  string                 `json:"name"`
	EstimatedPrice        int                    `json:"estimated_price"`
	Priority              string                 `json:"priority"`
	TargetDate            *string                `json:"target_date"`
	ProductURL            *string                `json:"product_url"`
	Note                  string                 `json:"note"`
	Status                string                 `json:"status"`
	SavedAmount           int                    `json:"saved_amount"`
	Remaining             int                    `json:"remaining"`
	ProgressPct           float64                `json:"progress_pct"`
	PurchasedAt           *time.Time             `json:"purchased_at"`
	PurchaseTransactionID *string                `json:"purchase_transaction_id"`
	SortOrder             int                    `json:"sort_order"`
	Affordability         *AffordabilityResponse `json:"affordability"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	DeletedAt             *time.Time             `json:"deleted_at"`
}

func NewItemResponse(view ItemView) ItemResponse {
	item := view.Item

	resp := ItemResponse{
		ID:             item.ID,
		Name:           item.Name,
		EstimatedPrice: item.EstimatedPrice,
		Priority:       item.Priority,
		Note:           item.Note,
		Status:         item.Status,
		SavedAmount:    item.SavedAmount,
		Remaining:      item.Remaining(),
		ProgressPct:    math.Round(item.ProgressPct()*100) / 100,
		SortOrder:      item.SortOrder,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}

	if item.TargetDate.Valid {
		targetDate := timeutil.FormatDate(item.TargetDate.Time)
		resp.TargetDate = &targetDate
	}
	if item.ProductURL.Valid {
		productURL := item.ProductURL.String
		resp.ProductURL = &productURL
	}
	if item.PurchasedAt.Valid {
		purchasedAt := item.PurchasedAt.Time
		resp.PurchasedAt = &purchasedAt
	}
	if item.PurchaseTransactionID.Valid {
		transactionID := item.PurchaseTransactionID.String
		resp.PurchaseTransactionID = &transactionID
	}
	if item.DeletedAt.Valid {
		deletedAt := item.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}

	if view.Affordability != nil {
		affordability := &AffordabilityResponse{
			AvgMonthlySavable:   view.Affordability.AvgMonthlySavable,
			MonthsNeeded:        view.Affordability.MonthsNeeded,
			OnTrackForTargetDay: view.Affordability.OnTrackForTargetDay,
		}
		if view.Affordability.EstimatedReadyDate != nil {
			readyDate := timeutil.FormatDate(*view.Affordability.EstimatedReadyDate)
			affordability.EstimatedReadyDate = &readyDate
		}
		resp.Affordability = affordability
	}

	return resp
}

// NewItemOnlyResponse dipakai delete dan restore, yang tidak perlu menghitung
// keterjangkauan.
func NewItemOnlyResponse(item Item) ItemResponse {
	return NewItemResponse(ItemView{Item: item})
}

type SummaryResponse struct {
	TotalItems     int            `json:"total_items"`
	TotalEstimated int            `json:"total_estimated"`
	TotalSaved     int            `json:"total_saved"`
	ByPriority     map[string]int `json:"by_priority"`
}

type CreateItemRequest struct {
	Name           string
	EstimatedPrice int
	Priority       string
	TargetDate     *time.Time
	ProductURL     *string
	Note           string
	Status         string
}

type PatchItemRequest struct {
	Name            *string
	EstimatedPrice  *int
	Priority        *string
	TargetDate      *time.Time
	ClearTargetDate bool
	ProductURL      *string
	ClearProductURL bool
	Note            *string
	Status          *string
	SortOrder       *int
}

type PurchaseRequest struct {
	WalletID    string
	CategoryID  string
	ActualPrice int
	OccurredAt  time.Time
	Note        string
}

// createItemBody, patchItemBody, allocateBody, dan purchaseBody adalah bentuk
// JSON mentah dari client. target_date dan product_url memakai json.RawMessage
// supaya "tidak dikirim" bisa dibedakan dari "dikirim null".
type createItemBody struct {
	Name           string  `json:"name"`
	EstimatedPrice int     `json:"estimated_price"`
	Priority       string  `json:"priority"`
	TargetDate     *string `json:"target_date"`
	ProductURL     *string `json:"product_url"`
	Note           string  `json:"note"`
	Status         string  `json:"status"`
}

type patchItemBody struct {
	Name           *string         `json:"name"`
	EstimatedPrice *int            `json:"estimated_price"`
	Priority       *string         `json:"priority"`
	TargetDate     json.RawMessage `json:"target_date"`
	ProductURL     json.RawMessage `json:"product_url"`
	Note           *string         `json:"note"`
	Status         *string         `json:"status"`
	SortOrder      *int            `json:"sort_order"`
}

type allocateBody struct {
	Amount int `json:"amount"`
}

type purchaseBody struct {
	WalletID    string     `json:"wallet_id"`
	CategoryID  string     `json:"category_id"`
	ActualPrice int        `json:"actual_price"`
	OccurredAt  *time.Time `json:"occurred_at"`
	Note        string     `json:"note"`
}
