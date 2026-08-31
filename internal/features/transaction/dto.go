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

type TransactionFilter struct {
	From       time.Time
	To         time.Time
	Type       string
	CategoryID string
	WalletID   string
	MinAmount  int
	MaxAmount  int
	Query      string
	Sort       string
	Limit      int
	Offset     int
}
