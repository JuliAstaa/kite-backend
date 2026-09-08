package analytics

import (
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"context"
	"testing"
	"time"
)

type fixture struct {
	repo       *AnalyticsRepository
	walletID   string
	wallet2ID  string
	expenseCat string
	incomeCat  string
}

func setup(t *testing.T) fixture {
	t.Helper()
	db := resetDB(t)

	walletID, err := testutil.InsertWallet(db, "Cash", "cash", 1_000_000)
	if err != nil {
		t.Fatalf("gagal bikin wallet: %v", err)
	}
	wallet2ID, err := testutil.InsertWallet(db, "BCA", "bank", 5_000_000)
	if err != nil {
		t.Fatalf("gagal bikin wallet kedua: %v", err)
	}
	expenseCat, err := testutil.InsertCategory(db, "Makan", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori expense: %v", err)
	}
	incomeCat, err := testutil.InsertCategory(db, "Gaji", "income")
	if err != nil {
		t.Fatalf("gagal bikin kategori income: %v", err)
	}

	return fixture{
		repo:       NewAnalyticsRepository(db),
		walletID:   walletID,
		wallet2ID:  wallet2ID,
		expenseCat: expenseCat,
		incomeCat:  incomeCat,
	}
}

func TestTotalsRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	mustInsert(t, "income", 8_500_000, f.walletID, f.incomeCat, base)
	mustInsert(t, "expense", 25_000, f.walletID, f.expenseCat, base)
	mustInsert(t, "expense", 150_000, f.walletID, f.expenseCat, base.AddDate(0, 0, -2))

	// Transfer tidak boleh ikut dihitung sebagai pemasukan maupun pengeluaran.
	if _, err := testutil.InsertTransfer(testDB, 500_000, f.walletID, f.wallet2ID, base); err != nil {
		t.Fatalf("gagal bikin transfer: %v", err)
	}

	got, err := f.repo.Totals(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.Income != 8_500_000 {
		t.Errorf("income %d, mau 8500000", got.Income)
	}
	if got.Expense != 175_000 {
		t.Errorf("expense %d, mau 175000", got.Expense)
	}
	if got.Net() != 8_325_000 {
		t.Errorf("net %d, mau 8325000", got.Net())
	}
	// transaction_count menghitung semua baris termasuk transfer
	if got.TransactionCount != 4 {
		t.Errorf("transaction_count %d, mau 4", got.TransactionCount)
	}
}

func TestTotalsMenghormatiRentang(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	mustInsert(t, "expense", 100_000, f.walletID, f.expenseCat, base)
	mustInsert(t, "expense", 999_000, f.walletID, f.expenseCat, base.AddDate(0, 0, -40))

	got, err := f.repo.Totals(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if got.Expense != 100_000 {
		t.Errorf("expense %d, mau 100000 (yang 40 hari lalu tidak boleh ikut)", got.Expense)
	}
}

func TestTotalsMengabaikanTransaksiTerhapus(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	id := mustInsert(t, "expense", 100_000, f.walletID, f.expenseCat, base)
	if _, err := testDB.Exec(`UPDATE transactions SET deleted_at = now() WHERE id = $1`, id); err != nil {
		t.Fatalf("gagal soft delete: %v", err)
	}

	got, err := f.repo.Totals(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if got.Expense != 0 {
		t.Errorf("expense %d, mau 0", got.Expense)
	}
}

func TestByCategoryRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	transportCat, err := testutil.InsertCategory(testDB, "Transport", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori: %v", err)
	}

	mustInsert(t, "expense", 300_000, f.walletID, f.expenseCat, base)
	mustInsert(t, "expense", 100_000, f.walletID, transportCat, base)
	mustInsert(t, "income", 5_000_000, f.walletID, f.incomeCat, base)

	from, to := base.AddDate(0, 0, -7), timeutil.EndOfDay(base)

	t.Run("filter expense", func(t *testing.T) {
		got, err := f.repo.ByCategory(ctx, from, to, "expense")
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("dapat %d kategori, mau 2", len(got))
		}
		// diurutkan dari yang terbesar
		if got[0].CategoryName != "Makan" || got[0].Total != 300_000 {
			t.Errorf("baris pertama salah: %+v", got[0])
		}
	})

	t.Run("type kosong berarti income dan expense", func(t *testing.T) {
		got, err := f.repo.ByCategory(ctx, from, to, "")
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("dapat %d kategori, mau 3", len(got))
		}
	})

	t.Run("kategori terhapus tetap muncul dengan penanda", func(t *testing.T) {
		if _, err := testDB.Exec(`UPDATE categories SET deleted_at = now() WHERE id = $1`, transportCat); err != nil {
			t.Fatalf("gagal soft delete kategori: %v", err)
		}

		got, err := f.repo.ByCategory(ctx, from, to, "expense")
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		var found bool
		for _, c := range got {
			if c.CategoryName == "Transport" {
				found = true
				if !c.IsDeleted {
					t.Error("kategori terhapus harus ditandai is_deleted")
				}
			}
		}
		if !found {
			t.Error("kategori terhapus harus tetap muncul di laporan lama")
		}
	})
}

func TestByWalletRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	mustInsert(t, "income", 5_000_000, f.walletID, f.incomeCat, base)
	mustInsert(t, "expense", 200_000, f.walletID, f.expenseCat, base)

	got, err := f.repo.ByWallet(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	// Kedua wallet ikut muncul walaupun yang satu belum punya transaksi,
	// karena JOIN-nya LEFT.
	if len(got) != 2 {
		t.Fatalf("dapat %d wallet, mau 2", len(got))
	}

	byName := map[string]WalletBreakdown{}
	for _, w := range got {
		byName[w.WalletName] = w
	}

	cash := byName["Cash"]
	if cash.Income != 5_000_000 || cash.Expense != 200_000 || cash.Net != 4_800_000 {
		t.Errorf("Cash salah: %+v", cash)
	}
	if cash.Count != 2 {
		t.Errorf("count Cash %d, mau 2", cash.Count)
	}

	bca := byName["BCA"]
	if bca.Income != 0 || bca.Expense != 0 || bca.Count != 0 {
		t.Errorf("BCA yang belum punya transaksi harus nol semua: %+v", bca)
	}
}

// occurred_at dikonversi ke Asia/Makassar sebelum date_trunc. Transaksi jam 7
// pagi WITA harus masuk hari itu, bukan hari sebelumnya seperti kalau dihitung
// dalam UTC.
func TestTrendSumsMemakaiZonaWaktuAplikasi(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	pagi := time.Date(2026, time.August, 11, 7, 0, 0, 0, timeutil.Loc())
	mustInsert(t, "expense", 25_000, f.walletID, f.expenseCat, pagi)

	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	to := timeutil.EndOfDay(time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc()))

	got, err := f.repo.TrendSums(ctx, from, to, "day")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("dapat %d bucket, mau 1", len(got))
	}

	if timeutil.FormatDate(got[0].Start) != "2026-08-11" {
		t.Errorf("bucket %s, mau 2026-08-11. Kemungkinan AT TIME ZONE kelewat",
			timeutil.FormatDate(got[0].Start))
	}
	if got[0].Expense != 25_000 {
		t.Errorf("expense %d, mau 25000", got[0].Expense)
	}
}

func TestTrendSumsGranularity(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	juli := time.Date(2026, time.July, 15, 12, 0, 0, 0, timeutil.Loc())
	agustus := time.Date(2026, time.August, 15, 12, 0, 0, 0, timeutil.Loc())

	mustInsert(t, "expense", 100_000, f.walletID, f.expenseCat, juli)
	mustInsert(t, "income", 500_000, f.walletID, f.incomeCat, agustus)

	from := time.Date(2026, time.July, 1, 0, 0, 0, 0, timeutil.Loc())
	to := timeutil.EndOfDay(time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc()))

	got, err := f.repo.TrendSums(ctx, from, to, "month")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("dapat %d bucket bulanan, mau 2", len(got))
	}
	if got[0].Expense != 100_000 || got[1].Income != 500_000 {
		t.Errorf("isi bucket salah: %+v", got)
	}
}

// Transfer tidak pernah muncul di tren, karena bukan pemasukan atau pengeluaran.
func TestTrendSumsMengabaikanTransfer(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	if _, err := testutil.InsertTransfer(testDB, 500_000, f.walletID, f.wallet2ID, base); err != nil {
		t.Fatalf("gagal bikin transfer: %v", err)
	}

	got, err := f.repo.TrendSums(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base), "day")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("dapat %d bucket, mau 0", len(got))
	}
}

func mustInsert(t *testing.T, txType string, amount int, walletID, categoryID string, occurredAt time.Time) string {
	t.Helper()

	id, err := testutil.InsertTransaction(testDB, txType, amount, walletID, categoryID, occurredAt)
	if err != nil {
		t.Fatalf("gagal seed transaksi: %v", err)
	}
	return id
}
