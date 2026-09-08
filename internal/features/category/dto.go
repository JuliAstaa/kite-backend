package category

import (
	"time"
)

type CategoryResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Color     string     `json:"color"`
	Icon      string     `json:"icon"`
	IsDefault bool       `json:"is_default"`
	SortOrder int        `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

// NewCategoryResponse mengubah bentuk domain jadi bentuk JSON.
func NewCategoryResponse(c Category) CategoryResponse {
	resp := CategoryResponse{
		ID:        c.ID,
		Name:      c.Name,
		Type:      c.Type,
		Color:     c.Color,
		Icon:      c.Icon,
		IsDefault: c.IsDefault,
		SortOrder: c.SortOrder,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
	if c.DeletedAt.Valid {
		deletedAt := c.DeletedAt.Time
		resp.DeletedAt = &deletedAt
	}
	return resp
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
