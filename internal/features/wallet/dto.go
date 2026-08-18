package wallet

import "time"

type WalletResponse struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Type                string    `json:"type"`
	InitialBalance      int       `json:"initial_balance"`
	Color               string    `json:"color"`
	Icon                string    `json:"icon"`
	IsExcludedFromTotal bool      `json:"is_excluded_from_total"`
	SortOrder           int       `json:"sort_order"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateWalletRequest struct {
	Name                string `json:"name"`
	Type                string `json:"type"`
	InitialBalance      *int   `json:"initial_balance"`
	Color               string `json:"color"`
	Icon                string `json:"icon"`
	IsExcludedFromTotal *bool  `json:"is_excluded_from_total"`
}

type PatchWalletRequest struct {
	Name                *string `json:"name"`
	Type                *string `json:"type"`
	InitialBalance      *int    `json:"initial_balance"`
	Color               *string `json:"color"`
	Icon                *string `json:"icon"`
	IsExcludedFromTotal *bool   `json:"is_excluded_from_total"`
}
