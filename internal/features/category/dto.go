package category

import (
	"time"
)

type CategoryResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Color     string    `json:"color"`
	Icon      string    `json:"icon"`
	IsDefault bool      `json:"is_default"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCategoryRequest struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Color string `json:"color"`
	Icon  string `json:"icon"`
}

type PatchCategoryRequest struct {
	Name  *string `json:"name"`
	Type  *string `json:"type"`
	Color *string `json:"color"`
	Icon  *string `json:"icon"`
}
