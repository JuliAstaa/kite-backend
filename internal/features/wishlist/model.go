package wishlist

import (
	"database/sql"
	"time"
)

type Item struct {
	ID                    string
	Name                  string
	EstimatedPrice        int
	Priority              string
	TargetDate            sql.NullTime
	ProductURL            sql.NullString
	Note                  string
	Status                string
	SavedAmount           int
	PurchasedAt           sql.NullTime
	PurchaseTransactionID sql.NullString
	SortOrder             int
	CreatedAt             time.Time
	UpdatedAt             time.Time
	DeletedAt             sql.NullTime
}

// Remaining adalah sisa uang yang masih perlu dikumpulkan.
func (i Item) Remaining() int {
	remaining := i.EstimatedPrice - i.SavedAmount
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (i Item) ProgressPct() float64 {
	if i.EstimatedPrice <= 0 {
		return 0
	}
	return float64(i.SavedAmount) / float64(i.EstimatedPrice) * 100
}

type CreateItemParams struct {
	Name           string
	EstimatedPrice int
	Priority       string
	TargetDate     *time.Time
	ProductURL     *string
	Note           string
	Status         string
}

type PatchItemParams struct {
	Name            *string
	EstimatedPrice  *int
	Priority        *string
	TargetDate      *time.Time
	ClearTargetDate bool
	ProductURL      *string
	ClearProductURL bool
	Note            *string
	Status          *string
	SortOrder       *int
}

type PurchaseParams struct {
	WalletID    string
	CategoryID  string
	ActualPrice int
	OccurredAt  time.Time
	Note        string
}

type ListFilter struct {
	Status         string
	Sort           string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

// Affordability adalah perkiraan kapan barang ini terbeli, dihitung dari
// kemampuan menabung beberapa bulan terakhir.
type Affordability struct {
	AvgMonthlySavable   int
	MonthsNeeded        *int
	EstimatedReadyDate  *time.Time
	OnTrackForTargetDay bool
}

// ItemView adalah item beserta hitungan turunannya.
type ItemView struct {
	Item          Item
	Affordability *Affordability
}

type SummaryTotals struct {
	TotalItems     int
	TotalEstimated int
	TotalSaved     int
	ByPriority     map[string]int
}
