package analytics

import (
	"backend/internal/shared/timeutil"
	"context"
	"math"
	"time"
)

// WalletBalanceReader dideklarasikan di sisi yang memakai. Feature wallet tidak
// perlu tahu bahwa analytics memakainya.
type WalletBalanceReader interface {
	TotalBalance(ctx context.Context) (int, error)
}

type AnalyticsServicer interface {
	Summary(ctx context.Context, from, to time.Time) (Summary, error)
	ByCategory(ctx context.Context, from, to time.Time, txType string) ([]CategoryBreakdown, error)
	ByWallet(ctx context.Context, from, to time.Time) ([]WalletBreakdown, error)
	Trend(ctx context.Context, from, to time.Time, granularity string) ([]TrendBucket, error)
}

type AnalyticsService struct {
	repo   AnalyticsRepositorer
	wallet WalletBalanceReader
}

func NewAnalyticsService(repo AnalyticsRepositorer, wallet WalletBalanceReader) *AnalyticsService {
	return &AnalyticsService{repo: repo, wallet: wallet}
}

func (s *AnalyticsService) Summary(ctx context.Context, from, to time.Time) (Summary, error) {
	current, err := s.repo.Totals(ctx, from, to)
	if err != nil {
		return Summary{}, err
	}

	// periode pembanding: rentang sebelumnya dengan panjang yang sama
	length := to.Sub(from)
	prevTo := from.Add(-time.Nanosecond)
	prevFrom := prevTo.Add(-length)

	previous, err := s.repo.Totals(ctx, prevFrom, prevTo)
	if err != nil {
		return Summary{}, err
	}

	// total_balance tidak dibatasi periode, ini saldo semua wallet aktif
	totalBalance, err := s.wallet.TotalBalance(ctx)
	if err != nil {
		return Summary{}, err
	}

	return Summary{
		From:             from,
		To:               to,
		TotalIncome:      current.Income,
		TotalExpense:     current.Expense,
		Net:              current.Net(),
		TotalBalance:     totalBalance,
		TransactionCount: current.TransactionCount,
		IncomeChangePct:  percentChange(previous.Income, current.Income),
		ExpenseChangePct: percentChange(previous.Expense, current.Expense),
	}, nil
}

func (s *AnalyticsService) ByCategory(ctx context.Context, from, to time.Time, txType string) ([]CategoryBreakdown, error) {
	rows, err := s.repo.ByCategory(ctx, from, to, txType)
	if err != nil {
		return nil, err
	}

	total := 0
	for _, r := range rows {
		total += r.Total
	}

	for i := range rows {
		if total > 0 {
			rows[i].Percentage = round2(float64(rows[i].Total) / float64(total) * 100)
		}
	}

	return rows, nil
}

func (s *AnalyticsService) ByWallet(ctx context.Context, from, to time.Time) ([]WalletBreakdown, error) {
	return s.repo.ByWallet(ctx, from, to)
}

// Trend menggabungkan hasil query dengan daftar bucket lengkap, supaya bucket
// yang tidak punya transaksi tetap muncul dengan nilai 0.
func (s *AnalyticsService) Trend(ctx context.Context, from, to time.Time, granularity string) ([]TrendBucket, error) {
	sums, err := s.repo.TrendSums(ctx, from, to, granularity)
	if err != nil {
		return nil, err
	}

	byLabel := map[string]bucketSum{}
	for _, s := range sums {
		byLabel[timeutil.BucketLabel(s.Start, granularity)] = s
	}

	buckets := timeutil.Buckets(from, to, granularity)
	result := make([]TrendBucket, 0, len(buckets))
	for _, b := range buckets {
		sum := byLabel[b.Label]
		result = append(result, TrendBucket{
			Bucket:  b.Label,
			Start:   b.Start,
			End:     b.End,
			Income:  sum.Income,
			Expense: sum.Expense,
			Net:     sum.Income - sum.Expense,
		})
	}

	return result, nil
}

// percentChange menghitung perubahan terhadap periode sebelumnya.
// Kalau periode sebelumnya nol, tidak ada pembanding yang masuk akal, jadi
// hasilnya 0 dan bukan pembagian dengan nol.
func percentChange(previous, current int) float64 {
	if previous == 0 {
		return 0
	}
	return round2(float64(current-previous) / math.Abs(float64(previous)) * 100)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
