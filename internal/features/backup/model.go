package backup

import "time"

// ExportWallet, ExportCategory, dan ExportTransaction adalah bentuk data untuk
// file backup. Sengaja dibuat datar dan memakai nama, bukan cuma UUID, supaya
// file-nya masih kebaca manusia dan bisa diimpor ke database yang masih kosong.
type ExportWallet struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Type                string `json:"type"`
	InitialBalance      int    `json:"initial_balance"`
	IsExcludedFromTotal bool   `json:"is_excluded_from_total"`
}

type ExportCategory struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type ExportTransaction struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Amount     int       `json:"amount"`
	Wallet     string    `json:"wallet"`
	ToWallet   string    `json:"to_wallet"`
	Category   string    `json:"category"`
	Note       string    `json:"note"`
	OccurredAt time.Time `json:"occurred_at"`
}

type ExportBundle struct {
	ExportedAt   time.Time           `json:"exported_at"`
	From         string              `json:"from"`
	To           string              `json:"to"`
	Wallets      []ExportWallet      `json:"wallets"`
	Categories   []ExportCategory    `json:"categories"`
	Transactions []ExportTransaction `json:"transactions"`
}

// ImportRow adalah satu baris CSV yang sudah diterjemahkan ke id.
type ImportRow struct {
	Line       int
	Type       string
	Amount     int
	WalletID   string
	CategoryID string
	Note       string
	OccurredAt time.Time
}

type RowError struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type ImportResult struct {
	DryRun      bool
	TotalRows   int
	ValidRows   int
	InvalidRows int
	Imported    int
	Errors      []RowError
}
