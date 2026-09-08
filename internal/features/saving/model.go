package saving

import (
	"database/sql"
	"time"
)

// Target adalah target menabung per periode. Isi salah satu antara Amount
// (nominal) atau TargetRate (persen dari pemasukan), tidak boleh dua-duanya.
type Target struct {
	ID         string
	Period     string
	Amount     sql.NullInt64
	TargetRate sql.NullFloat64
	StartDate  time.Time
	EndDate    sql.NullTime
	IsActive   bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  sql.NullTime
}

type CreateTargetParams struct {
	Period     string
	Amount     *int
	TargetRate *float64
	StartDate  time.Time
	EndDate    *time.Time
	IsActive   bool
}

type PatchTargetParams struct {
	Period     *string
	Amount     *int
	TargetRate *float64
	StartDate  *time.Time
	EndDate    *time.Time
	IsActive   *bool
	// ClearEndDate dipakai saat client mengirim end_date: null secara sengaja.
	ClearEndDate bool
}

// TargetProgress adalah hasil membandingkan tabungan nyata dengan targetnya.
type TargetProgress struct {
	Amount      int
	Achieved    bool
	ProgressPct float64
	Difference  int
}

type Summary struct {
	Period              string
	From                time.Time
	To                  time.Time
	Income              int
	Expense             int
	Savable             int
	SavingsRate         float64
	Target              *TargetProgress
	DailyAverageExpense int
	ProjectedSavable    int
	DaysElapsed         int
	DaysTotal           int
}

type BreakdownBucket struct {
	Bucket      string
	Start       time.Time
	End         time.Time
	Income      int
	Expense     int
	Savable     int
	SavingsRate float64
}

// bucketSum adalah hasil mentah query per potongan waktu.
type bucketSum struct {
	Start   time.Time
	Income  int
	Expense int
}
