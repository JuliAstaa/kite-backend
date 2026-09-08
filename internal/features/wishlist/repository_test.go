package wishlist

import (
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"context"
	"testing"
)

// seedWalletAndCategory menyiapkan satu wallet dan satu kategori expense.
func seedWalletAndCategory(t *testing.T) (walletID string, categoryID string) {
	t.Helper()
	db := resetDB(t)

	walletID, err := testutil.InsertWallet(db, "Cash Test", "cash", 1_000_000)
	if err != nil {
		t.Fatalf("gagal bikin wallet: %v", err)
	}

	categoryID, err = testutil.InsertCategory(db, "Belanja Test", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori: %v", err)
	}

	return walletID, categoryID
}

func TestPurchaseMembuatTransaksiDanMenandaiItem(t *testing.T) {
	db := requireDB(t)
	walletID, categoryID := seedWalletAndCategory(t)

	repo := NewWishlistRepository(db)
	ctx := context.Background()

	item, err := repo.CreateItem(ctx, CreateItemParams{
		Name: "Keyboard mekanik", EstimatedPrice: 1_200_000, Priority: "high", Status: "planned",
	})
	if err != nil {
		t.Fatalf("gagal bikin item: %v", err)
	}

	purchased, err := repo.Purchase(ctx, item.ID, PurchaseParams{
		WalletID:    walletID,
		CategoryID:  categoryID,
		ActualPrice: 1_150_000,
		OccurredAt:  timeutil.Now(),
		Note:        "Pembelian wishlist",
	})
	if err != nil {
		t.Fatalf("purchase gagal: %v", err)
	}

	if purchased.Status != "purchased" {
		t.Errorf("status %q, mau purchased", purchased.Status)
	}
	if !purchased.PurchaseTransactionID.Valid {
		t.Errorf("purchase_transaction_id harus terisi")
	}

	var amount int
	err = db.QueryRowContext(ctx,
		`SELECT amount FROM transactions WHERE wishlist_item_id = $1 AND deleted_at IS NULL`, item.ID).Scan(&amount)
	if err != nil {
		t.Fatalf("transaksi tidak ketemu: %v", err)
	}
	if amount != 1_150_000 {
		t.Errorf("amount transaksi %d, mau 1150000", amount)
	}
}

// Acceptance criteria: endpoint purchase benar-benar atomik, dibuktikan dengan
// test yang memaksa gagal di langkah kedua.
//
// Cara memaksanya: item di-soft-delete setelah dibuat. Insert transaksi tetap
// bisa jalan karena barisnya masih ada, tapi UPDATE wishlist_items yang
// memfilter deleted_at IS NULL tidak kena baris apapun, jadi langkah kedua
// gagal. Kalau transaksinya tetap tersimpan, berarti tidak atomik.
func TestPurchaseAtomikSaatLangkahKeduaGagal(t *testing.T) {
	db := requireDB(t)
	walletID, categoryID := seedWalletAndCategory(t)

	repo := NewWishlistRepository(db)
	ctx := context.Background()

	item, err := repo.CreateItem(ctx, CreateItemParams{
		Name: "Monitor", EstimatedPrice: 3_000_000, Priority: "medium", Status: "planned",
	})
	if err != nil {
		t.Fatalf("gagal bikin item: %v", err)
	}

	if _, err := repo.DeleteItem(ctx, item.ID); err != nil {
		t.Fatalf("gagal soft delete item: %v", err)
	}

	_, err = repo.Purchase(ctx, item.ID, PurchaseParams{
		WalletID:    walletID,
		CategoryID:  categoryID,
		ActualPrice: 2_900_000,
		OccurredAt:  timeutil.Now(),
		Note:        "harusnya gagal",
	})
	if err == nil {
		t.Fatal("purchase harusnya gagal karena item sudah dihapus")
	}

	// Inilah inti tesnya: transaksi dari langkah pertama harus ikut batal.
	var count int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transactions WHERE wishlist_item_id = $1`, item.ID).Scan(&count); err != nil {
		t.Fatalf("gagal hitung transaksi: %v", err)
	}
	if count != 0 {
		t.Errorf("ada %d transaksi nyangkut, mau 0 (transaksi harus ikut di-rollback)", count)
	}
}

// Business rule 11 ditegakkan juga di level SQL, bukan cuma di service.
func TestAllocateTidakBolehLebihDariHargaDiLevelSQL(t *testing.T) {
	db := requireDB(t)
	seedWalletAndCategory(t)

	repo := NewWishlistRepository(db)
	ctx := context.Background()

	item, err := repo.CreateItem(ctx, CreateItemParams{
		Name: "Headset", EstimatedPrice: 500_000, Priority: "low", Status: "planned",
	})
	if err != nil {
		t.Fatalf("gagal bikin item: %v", err)
	}

	if _, err := repo.Allocate(ctx, item.ID, 400_000); err != nil {
		t.Fatalf("alokasi pertama harusnya sukses: %v", err)
	}

	if _, err := repo.Allocate(ctx, item.ID, 200_000); err == nil {
		t.Fatal("alokasi yang membuat total melebihi harga harusnya ditolak")
	}

	var saved int
	if err := db.QueryRowContext(ctx, `SELECT saved_amount FROM wishlist_items WHERE id = $1`, item.ID).Scan(&saved); err != nil {
		t.Fatalf("gagal baca saved_amount: %v", err)
	}
	if saved != 400_000 {
		t.Errorf("saved_amount %d, mau tetap 400000", saved)
	}
}
