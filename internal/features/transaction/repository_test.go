package transaction

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"context"
	"errors"
	"testing"
)

type fixture struct {
	repo       *TransactionRepository
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
		repo:       NewTransactionRepository(db),
		walletID:   walletID,
		wallet2ID:  wallet2ID,
		expenseCat: expenseCat,
		incomeCat:  incomeCat,
	}
}

func TestCreateTransactionRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	now := timeutil.Now()

	t.Run("expense mengembalikan detail lengkap", func(t *testing.T) {
		detail, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
			Type: "expense", Amount: 25000, WalletID: f.walletID,
			CategoryID: &f.expenseCat, Note: "Kopi susu", OccurredAt: now,
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		if detail.ID == "" {
			t.Error("id harus terisi")
		}
		if detail.Wallet.Name != "Cash" {
			t.Errorf("nama wallet %q, mau Cash", detail.Wallet.Name)
		}
		if detail.Category == nil || detail.Category.Name != "Makan" {
			t.Errorf("kategori tidak ikut ter-expand: %+v", detail.Category)
		}
		if detail.ToWallet != nil {
			t.Errorf("to_wallet harus nil untuk expense")
		}
	})

	t.Run("transfer mengisi to_wallet dan mengosongkan kategori", func(t *testing.T) {
		detail, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
			Type: "transfer", Amount: 100000, WalletID: f.walletID,
			ToWalletID: &f.wallet2ID, Note: "pindah dana", OccurredAt: now,
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		if detail.ToWallet == nil || detail.ToWallet.Name != "BCA" {
			t.Errorf("to_wallet tidak ter-expand: %+v", detail.ToWallet)
		}
		if detail.Category != nil {
			t.Errorf("kategori harus nil untuk transfer")
		}
	})

	t.Run("amount nol ditolak CHECK constraint", func(t *testing.T) {
		_, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
			Type: "expense", Amount: 0, WalletID: f.walletID,
			CategoryID: &f.expenseCat, OccurredAt: now,
		})
		if err == nil {
			t.Error("database harusnya menolak amount 0")
		}
	})
}

func TestGetTransactionByIDRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	created, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
		Type: "income", Amount: 8_500_000, WalletID: f.walletID,
		CategoryID: &f.incomeCat, Note: "gajian", OccurredAt: timeutil.Now(),
	})
	if err != nil {
		t.Fatalf("gagal bikin transaksi: %v", err)
	}

	t.Run("ketemu", func(t *testing.T) {
		got, err := f.repo.GetTransactionByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.Amount != 8_500_000 {
			t.Errorf("amount %d, mau 8500000", got.Amount)
		}
	})

	t.Run("id tidak ada memberi NotFoundError", func(t *testing.T) {
		_, err := f.repo.GetTransactionByID(ctx, "00000000-0000-0000-0000-000000000000")

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("mau NotFoundError, dapat %v", err)
		}
	})
}

func TestUpdateTransactionRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	created, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
		Type: "expense", Amount: 25000, WalletID: f.walletID,
		CategoryID: &f.expenseCat, Note: "awal", OccurredAt: timeutil.Now(),
	})
	if err != nil {
		t.Fatalf("gagal bikin transaksi: %v", err)
	}

	t.Run("menulis keadaan akhir apa adanya", func(t *testing.T) {
		updated, err := f.repo.UpdateTransaction(ctx, created.ID, UpdateTransactionParams{
			Type: "transfer", Amount: 50000, WalletID: f.walletID,
			ToWalletID: &f.wallet2ID, CategoryID: nil,
			Note: "jadi transfer", OccurredAt: timeutil.Now(),
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		if updated.Type != "transfer" {
			t.Errorf("type %q, mau transfer", updated.Type)
		}
		// Kategori harus benar-benar hilang, bukan tertinggal karena COALESCE.
		if updated.Category != nil {
			t.Errorf("kategori harus kosong setelah jadi transfer, dapat %+v", updated.Category)
		}
		if updated.ToWallet == nil {
			t.Error("to_wallet harus terisi")
		}
	})

	t.Run("id tidak ada memberi NotFoundError", func(t *testing.T) {
		_, err := f.repo.UpdateTransaction(ctx, "00000000-0000-0000-0000-000000000000", UpdateTransactionParams{
			Type: "expense", Amount: 1000, WalletID: f.walletID,
			CategoryID: &f.expenseCat, OccurredAt: timeutil.Now(),
		})

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("mau NotFoundError, dapat %v", err)
		}
	})
}

func TestDeleteDanRestoreTransactionRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	created, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
		Type: "expense", Amount: 25000, WalletID: f.walletID,
		CategoryID: &f.expenseCat, OccurredAt: timeutil.Now(),
	})
	if err != nil {
		t.Fatalf("gagal bikin transaksi: %v", err)
	}

	deleted, err := f.repo.DeleteTransaction(ctx, created.ID)
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if !deleted.DeletedAt.Valid {
		t.Error("deleted_at harus terisi setelah soft delete")
	}

	// Setelah dihapus, transaksi tidak boleh muncul lagi di query biasa.
	if _, err := f.repo.GetTransactionByID(ctx, created.ID); err == nil {
		t.Error("transaksi terhapus tidak boleh ketemu lewat GetTransactionByID")
	}

	// Hapus dua kali harus NotFound, bukan sukses diam-diam.
	if _, err := f.repo.DeleteTransaction(ctx, created.ID); err == nil {
		t.Error("delete kedua harusnya NotFound")
	}

	restored, err := f.repo.RestoreTransaction(ctx, created.ID)
	if err != nil {
		t.Fatalf("restore error: %v", err)
	}
	if restored.DeletedAt.Valid {
		t.Error("deleted_at harus kosong setelah restore")
	}

	if _, err := f.repo.GetTransactionByID(ctx, created.ID); err != nil {
		t.Errorf("transaksi harusnya ketemu lagi setelah restore: %v", err)
	}
}

// Transaksi lama yang menunjuk wallet terhapus tetap muncul, ditandai is_deleted.
func TestTransaksiLamaTetapMunculSaatWalletDihapus(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	created, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
		Type: "expense", Amount: 25000, WalletID: f.walletID,
		CategoryID: &f.expenseCat, OccurredAt: timeutil.Now(),
	})
	if err != nil {
		t.Fatalf("gagal bikin transaksi: %v", err)
	}

	if _, err := testDB.ExecContext(ctx, `UPDATE wallets SET deleted_at = now() WHERE id = $1`, f.walletID); err != nil {
		t.Fatalf("gagal soft delete wallet: %v", err)
	}
	if _, err := testDB.ExecContext(ctx, `UPDATE categories SET deleted_at = now() WHERE id = $1`, f.expenseCat); err != nil {
		t.Fatalf("gagal soft delete kategori: %v", err)
	}

	got, err := f.repo.GetTransactionByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("transaksi lama harus tetap ketemu: %v", err)
	}

	if got.Wallet.Name != "Cash" {
		t.Errorf("nama wallet hilang: %q", got.Wallet.Name)
	}
	if !got.Wallet.IsDeleted {
		t.Error("wallet harus ditandai is_deleted true")
	}
	if got.Category == nil || !got.Category.IsDeleted {
		t.Error("kategori harus ditandai is_deleted true")
	}
}

func TestGetAllTransactionsFilter(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	base := timeutil.StartOfDay(timeutil.Now())
	seed := []struct {
		txType   string
		amount   int
		category *string
		note     string
		daysAgo  int
	}{
		{"expense", 25_000, &f.expenseCat, "Kopi susu", 0},
		{"expense", 150_000, &f.expenseCat, "Belanja bulanan", 3},
		{"income", 8_500_000, &f.incomeCat, "Gaji Agustus", 5},
		{"expense", 75_000, &f.expenseCat, "Bensin motor", 20},
	}

	for _, s := range seed {
		_, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
			Type: s.txType, Amount: s.amount, WalletID: f.walletID,
			CategoryID: s.category, Note: s.note,
			OccurredAt: base.AddDate(0, 0, -s.daysAgo),
		})
		if err != nil {
			t.Fatalf("gagal seed transaksi: %v", err)
		}
	}

	// transfer ikut disiapkan untuk menguji filter wallet dua arah
	if _, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
		Type: "transfer", Amount: 200_000, WalletID: f.walletID,
		ToWalletID: &f.wallet2ID, Note: "pindah", OccurredAt: base,
	}); err != nil {
		t.Fatalf("gagal seed transfer: %v", err)
	}

	baseFilter := func() TransactionFilter {
		return TransactionFilter{
			From:  base.AddDate(0, 0, -30),
			To:    timeutil.EndOfDay(base),
			Limit: 50,
		}
	}

	tests := []struct {
		name      string
		mutate    func(*TransactionFilter)
		wantCount int
	}{
		{"tanpa filter tambahan", func(f *TransactionFilter) {}, 5},
		{"type expense", func(f *TransactionFilter) { f.Type = "expense" }, 3},
		{"type income", func(f *TransactionFilter) { f.Type = "income" }, 1},
		{"type transfer", func(f *TransactionFilter) { f.Type = "transfer" }, 1},
		{"rentang 7 hari terakhir", func(fl *TransactionFilter) { fl.From = base.AddDate(0, 0, -7) }, 4},
		{"min_amount", func(f *TransactionFilter) { f.MinAmount = 100_000 }, 3},
		{"max_amount", func(f *TransactionFilter) { f.MaxAmount = 100_000 }, 2},
		{"cari di note", func(f *TransactionFilter) { f.Query = "kopi" }, 1},
		{"cari case-insensitive", func(f *TransactionFilter) { f.Query = "BENSIN" }, 1},
		{"limit membatasi hasil", func(f *TransactionFilter) { f.Limit = 2 }, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := baseFilter()
			tt.mutate(&filter)

			got, _, err := f.repo.GetAllTransactions(ctx, filter)
			if err != nil {
				t.Fatalf("tidak mau error: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Errorf("dapat %d transaksi, mau %d", len(got), tt.wantCount)
			}
		})
	}

	t.Run("filter kategori CSV", func(t *testing.T) {
		filter := baseFilter()
		filter.CategoryIDs = []string{f.expenseCat, f.incomeCat}

		got, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		// transfer tidak punya kategori, jadi ikut tersaring keluar
		if len(got) != 4 {
			t.Errorf("dapat %d transaksi, mau 4", len(got))
		}
	})

	t.Run("filter wallet ikut menangkap transfer masuk", func(t *testing.T) {
		filter := baseFilter()
		filter.WalletIDs = []string{f.wallet2ID}

		got, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("dapat %d transaksi, mau 1 (transfer masuk)", len(got))
		}
		if got[0].Type != "transfer" {
			t.Errorf("type %q, mau transfer", got[0].Type)
		}
	})

	t.Run("total tidak ikut dibatasi limit", func(t *testing.T) {
		filter := baseFilter()
		filter.Limit = 2

		got, total, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("dapat %d baris, mau 2", len(got))
		}
		if total != 5 {
			t.Errorf("total %d, mau 5", total)
		}
	})

	t.Run("offset melewati baris pertama", func(t *testing.T) {
		filter := baseFilter()
		filter.Sort = "occurred_at:desc"

		semua, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		filter.Offset = 1
		digeser, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		if digeser[0].ID != semua[1].ID {
			t.Error("offset 1 harusnya mulai dari baris kedua")
		}
	})

	t.Run("transaksi terhapus tidak ikut terhitung", func(t *testing.T) {
		filter := baseFilter()
		semua, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		if _, err := f.repo.DeleteTransaction(ctx, semua[0].ID); err != nil {
			t.Fatalf("gagal hapus: %v", err)
		}

		sisa, total, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(sisa) != len(semua)-1 || total != len(semua)-1 {
			t.Errorf("dapat %d baris total %d, mau %d", len(sisa), total, len(semua)-1)
		}
	})
}

func TestSortTransaksi(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	amounts := []int{50_000, 10_000, 90_000}
	for i, amount := range amounts {
		if _, err := f.repo.CreateTransaction(ctx, CreateTransactionParams{
			Type: "expense", Amount: amount, WalletID: f.walletID,
			CategoryID: &f.expenseCat, OccurredAt: base.AddDate(0, 0, -i),
		}); err != nil {
			t.Fatalf("gagal seed: %v", err)
		}
	}

	filter := TransactionFilter{From: base.AddDate(0, 0, -10), To: timeutil.EndOfDay(base), Limit: 50}

	t.Run("amount asc", func(t *testing.T) {
		filter.Sort = "amount:asc"
		got, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got[0].Amount != 10_000 || got[2].Amount != 90_000 {
			t.Errorf("urutan salah: %d, %d, %d", got[0].Amount, got[1].Amount, got[2].Amount)
		}
	})

	t.Run("kolom tidak dikenal jatuh ke occurred_at desc", func(t *testing.T) {
		filter.Sort = "drop table; --"
		got, _, err := f.repo.GetAllTransactions(ctx, filter)
		if err != nil {
			t.Fatalf("sort ngawur tidak boleh bikin error: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("dapat %d baris, mau 3", len(got))
		}
		if !got[0].OccurredAt.After(got[1].OccurredAt) {
			t.Error("default harus occurred_at menurun")
		}
	})
}
