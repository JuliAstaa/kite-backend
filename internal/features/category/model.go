package category

import (
	"database/sql"
	"time"
)

type Category struct {
	ID        string
	Name      string
	Type      string
	Color     string
	Icon      string
	IsDefault bool
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt sql.NullTime
}
