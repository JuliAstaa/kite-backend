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
	ToWalletID      sql.NullString
	CategoryID      sql.NullString
	Note            string
	OccurredAt      time.Time
	RecurringRuleID sql.NullString
	WishlistItemID  sql.NullString
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

// UpdateTransactionParams berisi keadaan akhir transaksi setelah patch
// digabung dengan data lama. Service yang menggabungkan, repository tinggal
// menulis apa adanya, jadi tidak ada COALESCE yang bikin aturan transfer bocor.
type UpdateTransactionParams struct {
	Type       string
	Amount     int
	WalletID   string
	ToWalletID *string
	CategoryID *string
	Note       string
	OccurredAt time.Time
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
	DeletedAt  sql.NullTime
}

type CategoryInfo struct {
	ID   string
	Type string
}
