package analytics

import "time"

// Totals adalah angka mentah satu periode. Transfer tidak pernah masuk sini,
// karena memindahkan uang antar dompet bukan pemasukan maupun pengeluaran.
type Totals struct {
	Income           int
	Expense          int
	TransactionCount int
}

func (t Totals) Net() int { return t.Income - t.Expense }

type CategoryBreakdown struct {
	CategoryID   string
	CategoryName string
	CategoryType string
	IsDeleted    bool
	Total        int
	Count        int
	Percentage   float64
}

type WalletBreakdown struct {
	WalletID   string
	WalletName string
	IsDeleted  bool
	Income     int
	Expense    int
	Net        int
	Count      int
}

type TrendBucket struct {
	Bucket  string
	Start   time.Time
	End     time.Time
	Income  int
	Expense int
	Net     int
}

// bucketSum adalah hasil mentah dari query trend, sebelum digabung dengan
// daftar bucket kosong.
type bucketSum struct {
	Start   time.Time
	Income  int
	Expense int
}

type Summary struct {
	From             time.Time
	To               time.Time
	TotalIncome      int
	TotalExpense     int
	Net              int
	TotalBalance     int
	TransactionCount int
	IncomeChangePct  float64
	ExpenseChangePct float64
}
