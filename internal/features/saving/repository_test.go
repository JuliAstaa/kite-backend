package saving

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"context"
	"errors"
	"testing"
	"time"
)

type fixture struct {
	repo       *SavingRepository
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
		repo:       NewSavingRepository(db),
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
	mustInsert(t, "expense", 4_230_000, f.walletID, f.expenseCat, base)

	// Transfer diabaikan: memindahkan uang antar dompet bukan menabung.
	if _, err := testutil.InsertTransfer(testDB, 1_000_000, f.walletID, f.wallet2ID, base); err != nil {
		t.Fatalf("gagal bikin transfer: %v", err)
	}

	income, expense, err := f.repo.Totals(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if income != 8_500_000 {
		t.Errorf("income %d, mau 8500000", income)
	}
	if expense != 4_230_000 {
		t.Errorf("expense %d, mau 4230000 (transfer tidak boleh ikut)", expense)
	}
}

func TestBucketSumsMingguanMulaiSenin(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// 2026-08-12 adalah hari Rabu, minggunya mulai Senin 2026-08-10
	rabu := time.Date(2026, time.August, 12, 9, 0, 0, 0, timeutil.Loc())
	mustInsert(t, "expense", 620_000, f.walletID, f.expenseCat, rabu)

	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	to := timeutil.EndOfDay(time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc()))

	got, err := f.repo.BucketSums(ctx, from, to, "week")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("dapat %d bucket, mau 1", len(got))
	}

	if timeutil.FormatDate(got[0].Start) != "2026-08-10" {
		t.Errorf("awal minggu %s, mau 2026-08-10 (Senin)", timeutil.FormatDate(got[0].Start))
	}
	if got[0].Expense != 620_000 {
		t.Errorf("expense %d, mau 620000", got[0].Expense)
	}
}

func TestBucketSumsBulanan(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	juni := time.Date(2026, time.June, 10, 12, 0, 0, 0, timeutil.Loc())
	juli := time.Date(2026, time.July, 20, 12, 0, 0, 0, timeutil.Loc())

	mustInsert(t, "income", 8_500_000, f.walletID, f.incomeCat, juni)
	mustInsert(t, "expense", 1_240_000, f.walletID, f.expenseCat, juli)

	from := time.Date(2026, time.June, 1, 0, 0, 0, 0, timeutil.Loc())
	to := timeutil.EndOfDay(time.Date(2026, time.July, 31, 0, 0, 0, 0, timeutil.Loc()))

	got, err := f.repo.BucketSums(ctx, from, to, "month")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("dapat %d bucket, mau 2", len(got))
	}
	if got[0].Income != 8_500_000 || got[1].Expense != 1_240_000 {
		t.Errorf("isi bucket salah: %+v", got)
	}
}

func TestTargetCRUDRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	start := timeutil.StartOfMonth(timeutil.Now())

	amount := 3_000_000
	created, err := f.repo.CreateTarget(ctx, CreateTargetParams{
		Period: "monthly", Amount: &amount, StartDate: start, IsActive: true,
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	if !created.Amount.Valid || created.Amount.Int64 != 3_000_000 {
		t.Errorf("amount tidak tersimpan: %+v", created.Amount)
	}
	if created.TargetRate.Valid {
		t.Error("target_rate harus kosong kalau yang diisi amount")
	}

	t.Run("get by id", func(t *testing.T) {
		got, err := f.repo.GetTargetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("id %q, mau %q", got.ID, created.ID)
		}
	})

	t.Run("id tidak ada memberi NotFoundError", func(t *testing.T) {
		_, err := f.repo.GetTargetByID(ctx, "00000000-0000-0000-0000-000000000000")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("mau NotFoundError, dapat %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		targets, total, err := f.repo.GetAllTargets(ctx, false)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if total != 1 || len(targets) != 1 {
			t.Errorf("dapat %d target total %d, mau 1", len(targets), total)
		}
	})

	t.Run("patch ganti dari nominal ke persen", func(t *testing.T) {
		rate := 20.0
		patched, err := f.repo.PatchTarget(ctx, created.ID, PatchTargetParams{
			Amount: nil, TargetRate: &rate,
		})
		if err != nil {
			t.Fatalf("patch error: %v", err)
		}
		if patched.Amount.Valid {
			t.Error("amount harus benar-benar dikosongkan, bukan tertinggal karena COALESCE")
		}
		if !patched.TargetRate.Valid || patched.TargetRate.Float64 != 20 {
			t.Errorf("target_rate %+v, mau 20", patched.TargetRate)
		}
	})

	t.Run("delete lalu tidak muncul lagi", func(t *testing.T) {
		if _, err := f.repo.DeleteTarget(ctx, created.ID); err != nil {
			t.Fatalf("delete error: %v", err)
		}

		if _, err := f.repo.GetTargetByID(ctx, created.ID); err == nil {
			t.Error("target terhapus tidak boleh ketemu")
		}

		_, total, err := f.repo.GetAllTargets(ctx, false)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if total != 0 {
			t.Errorf("total %d, mau 0", total)
		}

		_, totalWithDeleted, err := f.repo.GetAllTargets(ctx, true)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if totalWithDeleted != 1 {
			t.Errorf("include_deleted total %d, mau 1", totalWithDeleted)
		}
	})
}

// Partial unique index menjaga maksimal satu target aktif per periode.
func TestTargetAktifKeduaDitolak(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	start := timeutil.StartOfMonth(timeutil.Now())

	amount := 3_000_000
	if _, err := f.repo.CreateTarget(ctx, CreateTargetParams{
		Period: "monthly", Amount: &amount, StartDate: start, IsActive: true,
	}); err != nil {
		t.Fatalf("target pertama gagal: %v", err)
	}

	_, err := f.repo.CreateTarget(ctx, CreateTargetParams{
		Period: "monthly", Amount: &amount, StartDate: start, IsActive: true,
	})

	var ce apperror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("mau ConflictError, dapat %v", err)
	}

	// Periode berbeda masih boleh
	if _, err := f.repo.CreateTarget(ctx, CreateTargetParams{
		Period: "weekly", Amount: &amount, StartDate: start, IsActive: true,
	}); err != nil {
		t.Errorf("target mingguan harusnya boleh: %v", err)
	}
}

func TestActiveTarget(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	start := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	end := time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc())

	amount := 3_000_000
	if _, err := f.repo.CreateTarget(ctx, CreateTargetParams{
		Period: "monthly", Amount: &amount, StartDate: start, EndDate: &end, IsActive: true,
	}); err != nil {
		t.Fatalf("gagal bikin target: %v", err)
	}

	tests := []struct {
		name      string
		at        time.Time
		wantFound bool
	}{
		{"di dalam rentang", time.Date(2026, time.August, 15, 0, 0, 0, 0, timeutil.Loc()), true},
		{"tepat di awal", start, true},
		{"tepat di akhir", end, true},
		{"sebelum mulai", time.Date(2026, time.July, 31, 0, 0, 0, 0, timeutil.Loc()), false},
		{"setelah berakhir", time.Date(2026, time.September, 1, 0, 0, 0, 0, timeutil.Loc()), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, found, err := f.repo.ActiveTarget(ctx, "monthly", tt.at)
			if err != nil {
				t.Fatalf("tidak mau error: %v", err)
			}
			if found != tt.wantFound {
				t.Errorf("found %v, mau %v", found, tt.wantFound)
			}
		})
	}

	t.Run("periode lain tidak ketemu", func(t *testing.T) {
		_, found, err := f.repo.ActiveTarget(ctx, "weekly", start)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if found {
			t.Error("target monthly tidak boleh ketemu saat mencari weekly")
		}
	})

	t.Run("tidak ada target sama sekali bukan error", func(t *testing.T) {
		if err := testutil.TruncateAll(testDB); err != nil {
			t.Fatalf("gagal bersihkan: %v", err)
		}

		_, found, err := f.repo.ActiveTarget(ctx, "monthly", start)
		if err != nil {
			t.Fatalf("tidak ada target harusnya bukan error: %v", err)
		}
		if found {
			t.Error("found harus false")
		}
	})
}

func mustInsert(t *testing.T, txType string, amount int, walletID, categoryID string, occurredAt time.Time) string {
	t.Helper()

	id, err := testutil.InsertTransaction(testDB, txType, amount, walletID, categoryID, occurredAt)
	if err != nil {
		t.Fatalf("gagal seed transaksi: %v", err)
	}
	return id
}
