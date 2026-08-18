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
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           sql.NullTime
}
