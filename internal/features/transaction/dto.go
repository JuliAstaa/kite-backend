package transaction

import (
	"time"
)

type TransactionResponse struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Amount          int       `json:"amount"`
	WalletID        string    `json:"wallet_id"`
	ToWalletID      string    `json:"to_wallet_id"`
	CategoryID      string    `json:"category_id"`
	Note            string    `json:"note"`
	OccurredAt      time.Time `json:"occurred_at"`
	RecurringRuleID string    `json:"recurring_rule_id"`
	WishlistItemID  string    `json:"wishlist_item_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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
