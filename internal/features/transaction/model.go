package transaction

import (
	"database/sql"
	"time"
)

type Transaction struct {
	ID              string
	Type            string
	Amount          int
	WalletID        string
	ToWalletID      string
	CategoryID      string
	Note            string
	OccurredAt      time.Time
	RecurringRuleID string
	WishlistItemID  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       sql.NullTime
}

type CreateTransactionParams struct {
	Type            string
	Amount          int
	WalletID        string
	ToWalletID      *string
	CategoryID      *string
	Note            string
	OccurredAt      time.Time
	RecurringRuleID *string
	WishlistItemID  *string
}

type WalletRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDeleted bool   `json:"is_deleted"`
}

type CategoryRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	IsDeleted bool   `json:"is_deleted"`
}

type TransactionDetail struct {
	ID         string
	Type       string
	Amount     int
	Note       string
	OccurredAt time.Time
	Wallet     WalletRef
	Category   *CategoryRef
	ToWallet   *WalletRef
}

type CategoryInfo struct {
	ID   string
	Type string
}
