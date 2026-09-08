package backup

import (
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"context"
	"testing"
)

type fixture struct {
	repo       *BackupRepository
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
	expenseCat, err := testutil.InsertCategory(db, "Makan & Minum", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori expense: %v", err)
	}
	incomeCat, err := testutil.InsertCategory(db, "Gaji", "income")
	if err != nil {
		t.Fatalf("gagal bikin kategori income: %v", err)
	}

	return fixture{
		repo:       NewBackupRepository(db),
		walletID:   walletID,
		wallet2ID:  wallet2ID,
		expenseCat: expenseCat,
		incomeCat:  incomeCat,
	}
}

func TestExportWalletsDanCategories(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	wallets, err := f.repo.ExportWallets(ctx)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(wallets) != 2 {
		t.Errorf("dapat %d wallet, mau 2", len(wallets))
	}

	categories, err := f.repo.ExportCategories(ctx)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(categories) != 2 {
		t.Errorf("dapat %d kategori, mau 2", len(categories))
	}

	t.Run("yang sudah dihapus tidak ikut diekspor", func(t *testing.T) {
		if _, err := testDB.Exec(`UPDATE wallets SET deleted_at = now() WHERE id = $1`, f.wallet2ID); err != nil {
			t.Fatalf("gagal soft delete: %v", err)
		}

		wallets, err := f.repo.ExportWallets(ctx)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if len(wallets) != 1 {
			t.Errorf("dapat %d wallet, mau 1", len(wallets))
		}
	})
}

func TestExportTransactionsRepo(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	if _, err := testutil.InsertTransaction(testDB, "expense", 25_000, f.walletID, f.expenseCat, base); err != nil {
		t.Fatalf("gagal seed: %v", err)
	}
	if _, err := testutil.InsertTransfer(testDB, 100_000, f.walletID, f.wallet2ID, base); err != nil {
		t.Fatalf("gagal seed transfer: %v", err)
	}
	if _, err := testutil.InsertTransaction(testDB, "expense", 999_000, f.walletID, f.expenseCat, base.AddDate(0, 0, -60)); err != nil {
		t.Fatalf("gagal seed: %v", err)
	}

	got, err := f.repo.ExportTransactions(ctx, base.AddDate(0, 0, -7), timeutil.EndOfDay(base))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("dapat %d transaksi, mau 2 (yang 60 hari lalu di luar rentang)", len(got))
	}

	byType := map[string]ExportTransaction{}
	for _, tx := range got {
		byType[tx.Type] = tx
	}

	expense := byType["expense"]
	if expense.Wallet != "Cash" || expense.Category != "Makan & Minum" {
		t.Errorf("nama wallet dan kategori harus ikut: %+v", expense)
	}

	transfer := byType["transfer"]
	if transfer.ToWallet != "BCA" {
		t.Errorf("to_wallet %q, mau BCA", transfer.ToWallet)
	}
	// transfer tidak punya kategori, jadi kolomnya kosong bukan NULL
	if transfer.Category != "" {
		t.Errorf("category transfer %q, mau kosong", transfer.Category)
	}
}

func TestLookupUntukImport(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	t.Run("wallet dipetakan dengan huruf kecil", func(t *testing.T) {
		lookup, err := f.repo.WalletsByName(ctx)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if lookup["cash"] != f.walletID {
			t.Errorf("lookup[cash] = %q, mau %q", lookup["cash"], f.walletID)
		}
		if _, ok := lookup["Cash"]; ok {
			t.Error("kunci harus huruf kecil semua")
		}
	})

	// Nama kategori boleh sama antara income dan expense, misalnya "Lainnya",
	// jadi kuncinya harus ikut menyertakan tipe.
	t.Run("kategori dipetakan dengan nama dan tipe", func(t *testing.T) {
		if _, err := testutil.InsertCategory(testDB, "Lainnya", "expense"); err != nil {
			t.Fatalf("gagal bikin kategori: %v", err)
		}
		if _, err := testutil.InsertCategory(testDB, "Lainnya", "income"); err != nil {
			t.Fatalf("gagal bikin kategori: %v", err)
		}

		lookup, err := f.repo.CategoriesByNameAndType(ctx)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		expenseID, okExpense := lookup["lainnya|expense"]
		incomeID, okIncome := lookup["lainnya|income"]

		if !okExpense || !okIncome {
			t.Fatal("dua-duanya harus ada di lookup")
		}
		if expenseID == incomeID {
			t.Error("kategori dengan nama sama tapi tipe beda harus punya id berbeda")
		}
	})
}

func TestImportTransactionsAtomik(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	base := timeutil.StartOfDay(timeutil.Now())

	t.Run("semua baris masuk", func(t *testing.T) {
		rows := []ImportRow{
			{Line: 2, Type: "expense", Amount: 15_000, WalletID: f.walletID, CategoryID: f.expenseCat, Note: "Teh manis", OccurredAt: base},
			{Line: 3, Type: "income", Amount: 500_000, WalletID: f.walletID, CategoryID: f.incomeCat, Note: "Bonus", OccurredAt: base},
		}

		imported, err := f.repo.ImportTransactions(ctx, rows)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if imported != 2 {
			t.Errorf("imported %d, mau 2", imported)
		}

		var count int
		if err := testDB.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&count); err != nil {
			t.Fatalf("gagal hitung: %v", err)
		}
		if count != 2 {
			t.Errorf("ada %d transaksi di database, mau 2", count)
		}
	})

	// Import berjalan dalam satu database transaction. Kalau ada satu baris
	// yang ditolak database, tidak ada satupun yang boleh tersimpan.
	t.Run("satu baris gagal berarti tidak ada yang masuk", func(t *testing.T) {
		db := resetDB(t)

		walletID, err := testutil.InsertWallet(db, "Cash", "cash", 1_000_000)
		if err != nil {
			t.Fatalf("gagal bikin wallet: %v", err)
		}
		categoryID, err := testutil.InsertCategory(db, "Makan", "expense")
		if err != nil {
			t.Fatalf("gagal bikin kategori: %v", err)
		}

		rows := []ImportRow{
			{Line: 2, Type: "expense", Amount: 15_000, WalletID: walletID, CategoryID: categoryID, OccurredAt: base},
			// amount 0 ditolak CHECK constraint di tengah jalan
			{Line: 3, Type: "expense", Amount: 0, WalletID: walletID, CategoryID: categoryID, OccurredAt: base},
			{Line: 4, Type: "expense", Amount: 20_000, WalletID: walletID, CategoryID: categoryID, OccurredAt: base},
		}

		if _, err := f.repo.ImportTransactions(ctx, rows); err == nil {
			t.Fatal("mau error karena ada baris yang melanggar constraint")
		}

		var count int
		if err := testDB.QueryRow(`SELECT COUNT(*) FROM transactions`).Scan(&count); err != nil {
			t.Fatalf("gagal hitung: %v", err)
		}
		if count != 0 {
			t.Errorf("ada %d transaksi tersimpan, mau 0 (harus rollback semua)", count)
		}
	})

	t.Run("daftar kosong bukan error", func(t *testing.T) {
		imported, err := f.repo.ImportTransactions(ctx, nil)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if imported != 0 {
			t.Errorf("imported %d, mau 0", imported)
		}
	})
}
