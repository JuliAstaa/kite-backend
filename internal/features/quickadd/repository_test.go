package quickadd

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/testutil"
	"context"
	"errors"
	"testing"
)

type fixture struct {
	repo       *QuickAddRepository
	walletID   string
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
	expenseCat, err := testutil.InsertCategory(db, "Makan", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori expense: %v", err)
	}
	incomeCat, err := testutil.InsertCategory(db, "Gaji", "income")
	if err != nil {
		t.Fatalf("gagal bikin kategori income: %v", err)
	}

	return fixture{
		repo:       NewQuickAddRepository(db),
		walletID:   walletID,
		expenseCat: expenseCat,
		incomeCat:  incomeCat,
	}
}

func TestCreateQuickAddRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	t.Run("dengan amount tetap", func(t *testing.T) {
		amount := 25_000
		created, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
			Label: "Kopi", Type: "expense", Amount: &amount,
			WalletID: f.walletID, CategoryID: f.expenseCat, Note: "kopi susu",
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if !created.Amount.Valid || created.Amount.Int64 != 25_000 {
			t.Errorf("amount salah: %+v", created.Amount)
		}
	})

	t.Run("tanpa amount berarti diisi manual saat dipakai", func(t *testing.T) {
		created, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
			Label: "Bensin", Type: "expense", Amount: nil,
			WalletID: f.walletID, CategoryID: f.expenseCat,
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if created.Amount.Valid {
			t.Error("amount harus NULL")
		}
	})

	t.Run("label yang sama ditolak", func(t *testing.T) {
		amount := 10_000
		params := CreateQuickAddParams{
			Label: "Duplikat", Type: "expense", Amount: &amount,
			WalletID: f.walletID, CategoryID: f.expenseCat,
		}

		if _, err := f.repo.CreateQuickAdd(ctx, params); err != nil {
			t.Fatalf("yang pertama harusnya sukses: %v", err)
		}

		_, err := f.repo.CreateQuickAdd(ctx, params)

		var ae apperror.AlreadyExistsErr
		if !errors.As(err, &ae) {
			t.Fatalf("mau AlreadyExistsErr, dapat %v", err)
		}
	})

	t.Run("label beda huruf besar kecil tetap dianggap sama", func(t *testing.T) {
		amount := 10_000
		if _, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
			Label: "Nasi Padang", Type: "expense", Amount: &amount,
			WalletID: f.walletID, CategoryID: f.expenseCat,
		}); err != nil {
			t.Fatalf("yang pertama harusnya sukses: %v", err)
		}

		_, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
			Label: "NASI PADANG", Type: "expense", Amount: &amount,
			WalletID: f.walletID, CategoryID: f.expenseCat,
		})
		if err == nil {
			t.Error("unique index-nya pakai lower(label), harusnya ditolak")
		}
	})

	t.Run("amount nol ditolak CHECK constraint", func(t *testing.T) {
		zero := 0
		_, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
			Label: "Gratis", Type: "expense", Amount: &zero,
			WalletID: f.walletID, CategoryID: f.expenseCat,
		})
		if err == nil {
			t.Error("database harusnya menolak amount 0")
		}
	})
}

func TestQuickAddCRUDRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	amount := 25_000
	created, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
		Label: "Kopi", Type: "expense", Amount: &amount,
		WalletID: f.walletID, CategoryID: f.expenseCat, Note: "kopi susu",
	})
	if err != nil {
		t.Fatalf("gagal bikin quick add: %v", err)
	}

	t.Run("get by id", func(t *testing.T) {
		got, err := f.repo.GetQuickAddByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.Label != "Kopi" {
			t.Errorf("label %q, mau Kopi", got.Label)
		}
	})

	t.Run("id tidak ada memberi NotFoundError", func(t *testing.T) {
		_, err := f.repo.GetQuickAddByID(ctx, "00000000-0000-0000-0000-000000000000")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("mau NotFoundError, dapat %v", err)
		}
	})

	t.Run("patch label dan amount", func(t *testing.T) {
		label := "Kopi Susu"
		newAmount := 30_000

		patched, err := f.repo.PatchQuickAdd(ctx, created.ID, PatchQuickAddParams{
			Label: &label, Amount: &newAmount,
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if patched.Label != "Kopi Susu" || patched.Amount.Int64 != 30_000 {
			t.Errorf("hasil patch salah: %+v", patched)
		}
	})

	t.Run("patch mengosongkan amount", func(t *testing.T) {
		patched, err := f.repo.PatchQuickAdd(ctx, created.ID, PatchQuickAddParams{ClearAmount: true})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if patched.Amount.Valid {
			t.Error("amount harus NULL setelah dikosongkan")
		}
	})

	t.Run("delete lalu hilang dari list", func(t *testing.T) {
		if _, err := f.repo.DeleteQuickAdd(ctx, created.ID); err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		_, total, err := f.repo.GetAllQuickAdds(ctx, false)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if total != 0 {
			t.Errorf("total %d, mau 0", total)
		}

		_, totalWithDeleted, err := f.repo.GetAllQuickAdds(ctx, true)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if totalWithDeleted != 1 {
			t.Errorf("include_deleted total %d, mau 1", totalWithDeleted)
		}
	})

	// Partial unique index-nya memfilter deleted_at IS NULL, jadi label yang
	// sudah dihapus boleh dipakai lagi.
	t.Run("label bisa dipakai lagi setelah dihapus", func(t *testing.T) {
		if _, err := f.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
			Label: "Kopi Susu", Type: "expense", Amount: &amount,
			WalletID: f.walletID, CategoryID: f.expenseCat,
		}); err != nil {
			t.Errorf("label bekas hapus harusnya boleh dipakai lagi: %v", err)
		}
	})
}
