package budget

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
	repo         *BudgetRepository
	walletID     string
	expenseCat   string
	transportCat string
}

func setup(t *testing.T) fixture {
	t.Helper()
	db := resetDB(t)

	walletID, err := testutil.InsertWallet(db, "Cash", "cash", 1_000_000)
	if err != nil {
		t.Fatalf("gagal bikin wallet: %v", err)
	}
	expenseCat, err := testutil.InsertCategory(db, "Makan", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori: %v", err)
	}
	transportCat, err := testutil.InsertCategory(db, "Transport", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori kedua: %v", err)
	}

	return fixture{
		repo:         NewBudgetRepository(db),
		walletID:     walletID,
		expenseCat:   expenseCat,
		transportCat: transportCat,
	}
}

func TestCreateBudgetRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	start := timeutil.StartOfMonth(timeutil.Now())

	created, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
		CategoryID: f.expenseCat, Amount: 2_000_000, Period: "monthly", StartMonth: start,
	})
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	// Nama kategori ikut diambil supaya frontend tidak perlu request kedua.
	if created.CategoryName != "Makan" {
		t.Errorf("category_name %q, mau Makan", created.CategoryName)
	}
	if created.Amount != 2_000_000 {
		t.Errorf("amount %d, mau 2000000", created.Amount)
	}
	if created.EndMonth.Valid {
		t.Error("end_month harus NULL kalau tidak diisi")
	}
}

// Partial unique index menjaga satu kategori cuma punya satu budget berjalan.
func TestBudgetBerjalanKeduaDitolak(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	start := timeutil.StartOfMonth(timeutil.Now())

	if _, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
		CategoryID: f.expenseCat, Amount: 2_000_000, Period: "monthly", StartMonth: start,
	}); err != nil {
		t.Fatalf("budget pertama gagal: %v", err)
	}

	_, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
		CategoryID: f.expenseCat, Amount: 3_000_000, Period: "monthly", StartMonth: start,
	})

	var ce apperror.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("mau ConflictError, dapat %v", err)
	}

	t.Run("kategori lain masih boleh", func(t *testing.T) {
		if _, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
			CategoryID: f.transportCat, Amount: 500_000, Period: "monthly", StartMonth: start,
		}); err != nil {
			t.Errorf("kategori berbeda harusnya boleh: %v", err)
		}
	})

	t.Run("budget yang sudah punya end_month tidak menghalangi", func(t *testing.T) {
		db := resetDB(t)

		categoryID, err := testutil.InsertCategory(db, "Hiburan", "expense")
		if err != nil {
			t.Fatalf("gagal bikin kategori: %v", err)
		}

		end := start.AddDate(0, 1, 0)
		if _, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
			CategoryID: categoryID, Amount: 1_000_000, Period: "monthly",
			StartMonth: start, EndMonth: &end,
		}); err != nil {
			t.Fatalf("budget berbatas waktu gagal: %v", err)
		}

		if _, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
			CategoryID: categoryID, Amount: 1_500_000, Period: "monthly",
			StartMonth: start.AddDate(0, 2, 0),
		}); err != nil {
			t.Errorf("budget lanjutan harusnya boleh: %v", err)
		}
	})
}

func TestBudgetCRUDRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	start := timeutil.StartOfMonth(timeutil.Now())

	created, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
		CategoryID: f.expenseCat, Amount: 2_000_000, Period: "monthly", StartMonth: start,
	})
	if err != nil {
		t.Fatalf("gagal bikin budget: %v", err)
	}

	t.Run("get by id", func(t *testing.T) {
		got, err := f.repo.GetBudgetByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("id %q, mau %q", got.ID, created.ID)
		}
	})

	t.Run("id tidak ada memberi NotFoundError", func(t *testing.T) {
		_, err := f.repo.GetBudgetByID(ctx, "00000000-0000-0000-0000-000000000000")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("mau NotFoundError, dapat %v", err)
		}
	})

	t.Run("patch amount", func(t *testing.T) {
		amount := 3_500_000
		patched, err := f.repo.PatchBudget(ctx, created.ID, PatchBudgetParams{Amount: &amount})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if patched.Amount != 3_500_000 {
			t.Errorf("amount %d, mau 3500000", patched.Amount)
		}
	})

	t.Run("patch mengisi lalu mengosongkan end_month", func(t *testing.T) {
		end := start.AddDate(0, 3, 0)
		withEnd, err := f.repo.PatchBudget(ctx, created.ID, PatchBudgetParams{EndMonth: &end})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if !withEnd.EndMonth.Valid {
			t.Fatal("end_month harus terisi")
		}

		cleared, err := f.repo.PatchBudget(ctx, created.ID, PatchBudgetParams{ClearEndMonth: true})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if cleared.EndMonth.Valid {
			t.Error("end_month harus kembali NULL")
		}
	})

	t.Run("delete lalu tidak muncul di list", func(t *testing.T) {
		deleted, err := f.repo.DeleteBudget(ctx, created.ID)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if !deleted.DeletedAt.Valid {
			t.Error("deleted_at harus terisi")
		}

		_, total, err := f.repo.GetAllBudgets(ctx, false)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if total != 0 {
			t.Errorf("total %d, mau 0", total)
		}

		_, totalWithDeleted, err := f.repo.GetAllBudgets(ctx, true)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if totalWithDeleted != 1 {
			t.Errorf("include_deleted total %d, mau 1", totalWithDeleted)
		}
	})
}

func TestActiveBudgetsRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	agustus := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	oktober := time.Date(2026, time.October, 1, 0, 0, 0, 0, timeutil.Loc())

	// budget yang sudah berakhir September
	september := time.Date(2026, time.September, 1, 0, 0, 0, 0, timeutil.Loc())
	if _, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
		CategoryID: f.expenseCat, Amount: 2_000_000, Period: "monthly",
		StartMonth: agustus, EndMonth: &september,
	}); err != nil {
		t.Fatalf("gagal bikin budget: %v", err)
	}

	tests := []struct {
		name      string
		month     time.Time
		wantCount int
	}{
		{"bulan mulai", agustus, 1},
		{"bulan terakhir berlaku", september, 1},
		{"setelah berakhir", oktober, 0},
		{"sebelum mulai", time.Date(2026, time.July, 1, 0, 0, 0, 0, timeutil.Loc()), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := f.repo.ActiveBudgets(ctx, tt.month)
			if err != nil {
				t.Fatalf("tidak mau error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("dapat %d budget, mau %d", len(got), tt.wantCount)
			}
		})
	}

	t.Run("budget tanpa end_month berlaku terus", func(t *testing.T) {
		if _, err := f.repo.CreateBudget(ctx, CreateBudgetParams{
			CategoryID: f.transportCat, Amount: 500_000, Period: "monthly", StartMonth: agustus,
		}); err != nil {
			t.Fatalf("gagal bikin budget: %v", err)
		}

		got, err := f.repo.ActiveBudgets(ctx, time.Date(2027, time.March, 1, 0, 0, 0, 0, timeutil.Loc()))
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(got) != 1 {
			t.Errorf("dapat %d budget, mau 1", len(got))
		}
	})
}

func TestSpentOnCategoryRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	seed := func(txType string, amount int, categoryID string, occurredAt time.Time) string {
		t.Helper()
		id, err := testutil.InsertTransaction(testDB, txType, amount, f.walletID, categoryID, occurredAt)
		if err != nil {
			t.Fatalf("gagal seed: %v", err)
		}
		return id
	}

	incomeCat, err := testutil.InsertCategory(testDB, "Gaji", "income")
	if err != nil {
		t.Fatalf("gagal bikin kategori income: %v", err)
	}

	seed("expense", 500_000, f.expenseCat, base)
	seed("expense", 350_000, f.expenseCat, base.AddDate(0, 0, -3))
	seed("expense", 100_000, f.transportCat, base)                  // kategori lain
	seed("income", 9_000_000, incomeCat, base)                      // bukan expense
	seed("expense", 999_000, f.expenseCat, base.AddDate(0, 0, -60)) // di luar rentang

	got, err := f.repo.SpentOnCategory(ctx, f.expenseCat, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if got != 850_000 {
		t.Errorf("spent %d, mau 850000", got)
	}

	t.Run("transaksi terhapus tidak dihitung", func(t *testing.T) {
		id := seed("expense", 200_000, f.expenseCat, base)
		if _, err := testDB.Exec(`UPDATE transactions SET deleted_at = now() WHERE id = $1`, id); err != nil {
			t.Fatalf("gagal soft delete: %v", err)
		}

		got, err := f.repo.SpentOnCategory(ctx, f.expenseCat, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got != 850_000 {
			t.Errorf("spent %d, mau tetap 850000", got)
		}
	})

	t.Run("kategori tanpa pengeluaran memberi nol", func(t *testing.T) {
		got, err := f.repo.SpentOnCategory(ctx, incomeCat, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got != 0 {
			t.Errorf("spent %d, mau 0", got)
		}
	})
}
