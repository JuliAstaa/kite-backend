package wallet

import "time"

type WalletResponse struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Type                string     `json:"type"`
	InitialBalance      int        `json:"initial_balance"`
	CurrentBalance      int        `json:"current_balance"`
	TransactionCount    int        `json:"transaction_count"`
	Color               string     `json:"color"`
	Icon                string     `json:"icon"`
	IsExcludedFromTotal bool       `json:"is_excluded_from_total"`
	SortOrder           int        `json:"sort_order"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at"`
}

// NewWalletResponse mengubah bentuk domain jadi bentuk JSON.
func NewWalletResponse(w Wallet) WalletResponse {
	resp := WalletResponse{
		ID:                  w.ID,
		Name:                w.Name,
		Type:                w.Type,
		InitialBalance:      w.InitialBalance,
		CurrentBalance:      w.CurrentBalance,
		TransactionCount:    w.TransactionCount,
		Color:               w.Color,
		Icon:                w.Icon,
		IsExcludedFromTotal: w.IsExcludedFromTotal,
		SortOrder:           w.SortOrder,
		CreatedAt:           w.CreatedAt,
		UpdatedAt:           w.UpdatedAt,
	}
	if w.DeletedAt.Valid {
		deletedAt := w.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

// DeleteWalletResponse memberi tahu frontend berapa transaksi yang ikut
// terpengaruh, supaya bisa konfirmasi dulu ke user sebelum menghapus.
type DeleteWalletResponse struct {
	ID                   string     `json:"id"`
	Name                 string     `json:"name"`
	DeletedAt            *time.Time `json:"deleted_at"`
	AffectedTransactions int        `json:"affected_transactions"`
}

func NewDeleteWalletResponse(w Wallet) DeleteWalletResponse {
	resp := DeleteWalletResponse{
		ID:                   w.ID,
		Name:                 w.Name,
		AffectedTransactions: w.TransactionCount,
	}
	if w.DeletedAt.Valid {
		deletedAt := w.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
}

type CreateWalletRequest struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	InitialBalance      int    `json:"initial_balance"`
	Color               string `json:"color"`
	Icon                string `json:"icon"`
	IsExcludedFromTotal bool   `json:"is_excluded_from_total"`
}

type PatchWalletRequest struct {
	Name                *string `json:"name"`
	Type                *string `json:"type"`
	InitialBalance      *int    `json:"initial_balance"`
	Color               *string `json:"color"`
	Icon                *string `json:"icon"`
	IsExcludedFromTotal *bool   `json:"is_excluded_from_total"`
}
