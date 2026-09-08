package analytics

import (
	"backend/internal/shared/timeutil"
	"context"
	"errors"
	"testing"
	"time"
)

type FakeAnalyticsRepository struct {
	TotalsByRange map[string]Totals
	Categories    []CategoryBreakdown
	Wallets       []WalletBreakdown
	Buckets       []bucketSum
	Err           error

	// CapturedRanges mencatat rentang yang diminta service, dipakai untuk
	// memastikan periode pembanding dihitung dengan benar.
	CapturedRanges [][2]time.Time
}

func (f *FakeAnalyticsRepository) Totals(ctx context.Context, from, to time.Time) (Totals, error) {
	if f.Err != nil {
		return Totals{}, f.Err
	}
	f.CapturedRanges = append(f.CapturedRanges, [2]time.Time{from, to})

	if f.TotalsByRange != nil {
		if got, ok := f.TotalsByRange[timeutil.FormatDate(from)]; ok {
			return got, nil
		}
	}
	return Totals{}, nil
}

func (f *FakeAnalyticsRepository) ByCategory(ctx context.Context, from, to time.Time, txType string) ([]CategoryBreakdown, error) {
	return f.Categories, f.Err
}

func (f *FakeAnalyticsRepository) ByWallet(ctx context.Context, from, to time.Time) ([]WalletBreakdown, error) {
	return f.Wallets, f.Err
}

func (f *FakeAnalyticsRepository) TrendSums(ctx context.Context, from, to time.Time, granularity string) ([]bucketSum, error) {
	return f.Buckets, f.Err
}

type FakeWalletBalanceReader struct {
	Balance int
	Err     error
}

func (f *FakeWalletBalanceReader) TotalBalance(ctx context.Context) (int, error) {
	return f.Balance, f.Err
}

func TestSummary(t *testing.T) {
	current := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	currentEnd := timeutil.EndOfDay(time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc()))

	repo := &FakeAnalyticsRepository{
		TotalsByRange: map[string]Totals{
			"2026-08-01": {Income: 8_500_000, Expense: 4_230_000, TransactionCount: 87},
			// periode pembanding: Juli, panjangnya sama dengan Agustus
			"2026-07-01": {Income: 8_157_000, Expense: 4_795_000, TransactionCount: 80},
		},
	}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{Balance: 12_750_000})

	got, err := service.Summary(context.Background(), current, currentEnd)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.TotalIncome != 8_500_000 || got.TotalExpense != 4_230_000 {
		t.Errorf("angka periode berjalan salah: %+v", got)
	}
	if got.Net != 4_270_000 {
		t.Errorf("net %d, mau 4270000", got.Net)
	}
	if got.TotalBalance != 12_750_000 {
		t.Errorf("total_balance %d, mau 12750000", got.TotalBalance)
	}
	if got.TransactionCount != 87 {
		t.Errorf("transaction_count %d, mau 87", got.TransactionCount)
	}

	// (8.500.000 - 8.157.000) / 8.157.000 * 100 = 4.2
	if got.IncomeChangePct != 4.2 {
		t.Errorf("income_change_pct %v, mau 4.2", got.IncomeChangePct)
	}
	// (4.230.000 - 4.795.000) / 4.795.000 * 100 = -11.78
	if got.ExpenseChangePct != -11.78 {
		t.Errorf("expense_change_pct %v, mau -11.78", got.ExpenseChangePct)
	}
}

// Periode pembanding harus punya panjang yang sama dan berhenti tepat sebelum
// periode berjalan mulai.
func TestSummaryMenghitungPeriodePembanding(t *testing.T) {
	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	to := timeutil.EndOfDay(time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc()))

	repo := &FakeAnalyticsRepository{}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{})

	if _, err := service.Summary(context.Background(), from, to); err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if len(repo.CapturedRanges) != 2 {
		t.Fatalf("repo dipanggil %d kali, mau 2 (periode berjalan dan pembanding)", len(repo.CapturedRanges))
	}

	prevFrom, prevTo := repo.CapturedRanges[1][0], repo.CapturedRanges[1][1]

	if !prevTo.Before(from) {
		t.Errorf("periode pembanding harus berhenti sebelum %s, dapat %s", from, prevTo)
	}
	if prevTo.Sub(prevFrom) != to.Sub(from) {
		t.Errorf("panjang periode pembanding %v, mau sama dengan %v", prevTo.Sub(prevFrom), to.Sub(from))
	}
}

// Periode sebelumnya nol berarti tidak ada pembanding yang masuk akal.
// Hasilnya 0, bukan pembagian dengan nol.
func TestPercentChange(t *testing.T) {
	tests := []struct {
		name     string
		previous int
		current  int
		want     float64
	}{
		{"naik", 100, 150, 50},
		{"turun", 100, 80, -20},
		{"tidak berubah", 100, 100, 0},
		{"periode sebelumnya nol", 0, 500, 0},
		{"dua-duanya nol", 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := percentChange(tt.previous, tt.current); got != tt.want {
				t.Errorf("percentChange(%d, %d) = %v, mau %v", tt.previous, tt.current, got, tt.want)
			}
		})
	}
}

func TestByCategoryMenghitungPersentase(t *testing.T) {
	repo := &FakeAnalyticsRepository{
		Categories: []CategoryBreakdown{
			{CategoryID: "c1", CategoryName: "Makan", Total: 750_000},
			{CategoryID: "c2", CategoryName: "Transport", Total: 250_000},
		},
	}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{})

	got, err := service.ByCategory(context.Background(), timeutil.Now(), timeutil.Now(), "expense")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got[0].Percentage != 75 {
		t.Errorf("persentase pertama %v, mau 75", got[0].Percentage)
	}
	if got[1].Percentage != 25 {
		t.Errorf("persentase kedua %v, mau 25", got[1].Percentage)
	}
}

func TestByCategoryTanpaData(t *testing.T) {
	repo := &FakeAnalyticsRepository{Categories: []CategoryBreakdown{}}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{})

	got, err := service.ByCategory(context.Background(), timeutil.Now(), timeutil.Now(), "expense")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("dapat %d baris, mau 0", len(got))
	}
}

// Total nol tidak boleh jadi pembagian dengan nol.
func TestByCategoryTotalNol(t *testing.T) {
	repo := &FakeAnalyticsRepository{
		Categories: []CategoryBreakdown{{CategoryID: "c1", Total: 0}},
	}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{})

	got, err := service.ByCategory(context.Background(), timeutil.Now(), timeutil.Now(), "")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if got[0].Percentage != 0 {
		t.Errorf("persentase %v, mau 0", got[0].Percentage)
	}
}

// Bucket yang tidak punya transaksi tetap dikembalikan dengan nilai 0,
// supaya grafik di frontend tidak bolong.
func TestTrendMengisiBucketKosong(t *testing.T) {
	juli := time.Date(2026, time.July, 1, 0, 0, 0, 0, timeutil.Loc())

	repo := &FakeAnalyticsRepository{
		Buckets: []bucketSum{{Start: juli, Income: 500_000, Expense: 100_000}},
	}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{})

	from := juli
	to := timeutil.EndOfDay(time.Date(2026, time.September, 30, 0, 0, 0, 0, timeutil.Loc()))

	got, err := service.Trend(context.Background(), from, to, "month")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("dapat %d bucket, mau 3 (Juli, Agustus, September)", len(got))
	}

	if got[0].Bucket != "2026-07" || got[0].Income != 500_000 || got[0].Net != 400_000 {
		t.Errorf("bucket Juli salah: %+v", got[0])
	}
	if got[1].Income != 0 || got[1].Expense != 0 || got[1].Net != 0 {
		t.Errorf("bucket Agustus harusnya nol semua: %+v", got[1])
	}
	if got[2].Bucket != "2026-09" {
		t.Errorf("bucket ketiga %q, mau 2026-09", got[2].Bucket)
	}
}

func TestServiceMeneruskanErrorRepository(t *testing.T) {
	sentinel := errors.New("database mati")
	repo := &FakeAnalyticsRepository{Err: sentinel}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{})
	ctx := context.Background()
	now := timeutil.Now()

	if _, err := service.Summary(ctx, now, now); !errors.Is(err, sentinel) {
		t.Errorf("Summary: mau error asli diteruskan, dapat %v", err)
	}
	if _, err := service.ByCategory(ctx, now, now, ""); !errors.Is(err, sentinel) {
		t.Errorf("ByCategory: mau error asli diteruskan, dapat %v", err)
	}
	if _, err := service.ByWallet(ctx, now, now); !errors.Is(err, sentinel) {
		t.Errorf("ByWallet: mau error asli diteruskan, dapat %v", err)
	}
	if _, err := service.Trend(ctx, now, now, "month"); !errors.Is(err, sentinel) {
		t.Errorf("Trend: mau error asli diteruskan, dapat %v", err)
	}
}

// Saldo total diambil dari feature wallet lewat interface. Kalau di sana error,
// summary ikut gagal, bukan diam-diam melaporkan saldo 0.
func TestSummaryGagalSaatSaldoWalletError(t *testing.T) {
	sentinel := errors.New("wallet error")
	repo := &FakeAnalyticsRepository{}
	service := NewAnalyticsService(repo, &FakeWalletBalanceReader{Err: sentinel})

	_, err := service.Summary(context.Background(), timeutil.Now(), timeutil.Now())
	if !errors.Is(err, sentinel) {
		t.Errorf("mau error dari wallet diteruskan, dapat %v", err)
	}
}
