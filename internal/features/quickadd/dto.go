package quickadd

import (
	"encoding/json"
	"time"
)

type QuickAddResponse struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	Type       string     `json:"type"`
	Amount     *int       `json:"amount"`
	WalletID   string     `json:"wallet_id"`
	CategoryID string     `json:"category_id"`
	Note       string     `json:"note"`
	SortOrder  int        `json:"sort_order"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
}

func NewQuickAddResponse(q QuickAdd) QuickAddResponse {
	resp := QuickAddResponse{
		ID:         q.ID,
		Label:      q.Label,
		Type:       q.Type,
		WalletID:   q.WalletID,
		CategoryID: q.CategoryID,
		Note:       q.Note,
		SortOrder:  q.SortOrder,
		CreatedAt:  q.CreatedAt,
		UpdatedAt:  q.UpdatedAt,
	}
	if q.Amount.Valid {
		amount := int(q.Amount.Int64)
		resp.Amount = &amount
	}
	if q.DeletedAt.Valid {
		deletedAt := q.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

type ExecuteResponse struct {
	TransactionID string    `json:"transaction_id"`
	Type          string    `json:"type"`
	Amount        int       `json:"amount"`
	Note          string    `json:"note"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type CreateQuickAddRequest struct {
	Label      string
	Type       string
	Amount     *int
	WalletID   string
	CategoryID string
	Note       string
}

type PatchQuickAddRequest struct {
	Label       *string
	Type        *string
	Amount      *int
	ClearAmount bool
	WalletID    *string
	CategoryID  *string
	Note        *string
	SortOrder   *int
}

type ExecuteRequest struct {
	Amount     *int
	Note       string
	OccurredAt time.Time
}

type createQuickAddBody struct {
	Label      string `json:"label"`
	Type       string `json:"type"`
	Amount     *int   `json:"amount"`
	WalletID   string `json:"wallet_id"`
	CategoryID string `json:"category_id"`
	Note       string `json:"note"`
}

// amount memakai json.RawMessage supaya null (nominal diisi manual saat dipakai)
// bisa dibedakan dari field yang tidak dikirim.
type patchQuickAddBody struct {
	Label      *string         `json:"label"`
	Type       *string         `json:"type"`
	Amount     json.RawMessage `json:"amount"`
	WalletID   *string         `json:"wallet_id"`
	CategoryID *string         `json:"category_id"`
	Note       *string         `json:"note"`
	SortOrder  *int            `json:"sort_order"`
}

type executeBody struct {
	Amount     *int       `json:"amount"`
	Note       string     `json:"note"`
	OccurredAt *time.Time `json:"occurred_at"`
}
