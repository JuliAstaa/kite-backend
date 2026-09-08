package wallet

import (
	"database/sql"
	"time"
)

type Wallet struct {
	ID                  string
	Name                string
	Type                string
	InitialBalance      int
	Color               string
	Icon                string
	IsExcludedFromTotal bool
	SortOrder           int

	// CurrentBalance dan TransactionCount tidak disimpan sebagai kolom, tapi
	// dihitung dari transaksi setiap kali dibaca. Kolom saldo yang di-update
	// manual adalah sumber bug nomor satu di aplikasi keuangan.
	CurrentBalance   int
	TransactionCount int

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt sql.NullTime
}
