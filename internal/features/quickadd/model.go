package quickadd

import (
	"database/sql"
	"time"
)

// QuickAdd adalah tombol pintasan untuk transaksi yang sering diulang,
// misalnya "Kopi 25rb" atau "Ongkos angkot".
type QuickAdd struct {
	ID         string
	Label      string
	Type       string
	Amount     sql.NullInt64
	WalletID   string
	CategoryID string
	Note       string
	SortOrder  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  sql.NullTime
}

type CreateQuickAddParams struct {
	Label      string
	Type       string
	Amount     *int
	WalletID   string
	CategoryID string
	Note       string
}

type PatchQuickAddParams struct {
	Label       *string
	Type        *string
	Amount      *int
	ClearAmount bool
	WalletID    *string
	CategoryID  *string
	Note        *string
	SortOrder   *int
}

type CategoryInfo struct {
	ID   string
	Type string
}

// CreateTransactionInput dan CreatedTransaction dipakai untuk bicara dengan
// feature transaction lewat interface, tanpa mengimpor package-nya.
type CreateTransactionInput struct {
	Type       string
	Amount     int
	WalletID   string
	CategoryID *string
	Note       string
	OccurredAt time.Time
}

type CreatedTransaction struct {
	ID         string
	Type       string
	Amount     int
	Note       string
	OccurredAt time.Time
}
