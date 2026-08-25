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
	ID        string
	Name      string
	IsDeleted bool
}

type CategoryRef struct {
	ID        string
	Name      string
	Type      string
	IsDeleted bool
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
