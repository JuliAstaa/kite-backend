package transaction

import (
	"time"
)

type TransactionResponse struct {
	ID         string       `json:"id"`
	Type       string       `json:"type"`
	Amount     int          `json:"amount"`
	Note       string       `json:"note"`
	OccurredAt time.Time    `json:"occurred_at"`
	Wallet     WalletRef    `json:"wallet"`
	Category   *CategoryRef `json:"category"`
	ToWallet   *WalletRef   `json:"to_wallet"`
	DeletedAt  *time.Time   `json:"deleted_at"`
}

// NewTransactionResponse mengubah bentuk domain jadi bentuk JSON.
func NewTransactionResponse(d TransactionDetail) TransactionResponse {
	resp := TransactionResponse{
		ID:         d.ID,
		Type:       d.Type,
		Amount:     d.Amount,
		Note:       d.Note,
		OccurredAt: d.OccurredAt,
		Wallet:     d.Wallet,
		Category:   d.Category,
		ToWallet:   d.ToWallet,
	}
	if d.DeletedAt.Valid {
		deletedAt := d.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

type CreateTransactionRequest struct {
	Type       string    `json:"type"`
	Amount     int       `json:"amount"`
	WalletID   string    `json:"wallet_id"`
	ToWalletID *string   `json:"to_wallet_id"`
	CategoryID *string   `json:"category_id"`
	Note       string    `json:"note"`
	OccurredAt time.Time `json:"occurred_at"`
}

// PatchTransactionRequest memakai pointer supaya bisa dibedakan antara
// "field tidak dikirim" dan "field dikirim bernilai kosong".
type PatchTransactionRequest struct {
	Type       *string    `json:"type"`
	Amount     *int       `json:"amount"`
	WalletID   *string    `json:"wallet_id"`
	ToWalletID *string    `json:"to_wallet_id"`
	CategoryID *string    `json:"category_id"`
	Note       *string    `json:"note"`
	OccurredAt *time.Time `json:"occurred_at"`
}

type TransactionFilter struct {
	From        time.Time
	To          time.Time
	Type        string
	CategoryIDs []string
	WalletIDs   []string
	MinAmount   int
	MaxAmount   int
	Query       string
	Sort        string
	Limit       int
	Offset      int
}
