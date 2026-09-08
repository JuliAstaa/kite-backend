package recurring

import (
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"context"
	"testing"
	"time"
)

func seedRule(t *testing.T) Rule {
	t.Helper()
	db := resetDB(t)
	ctx := context.Background()

	walletID, err := testutil.InsertWallet(db, "BCA Test", "bank", 5_000_000)
	if err != nil {
		t.Fatalf("gagal bikin wallet: %v", err)
	}
	categoryID, err := testutil.InsertCategory(db, "Tagihan Test", "expense")
	if err != nil {
		t.Fatalf("gagal bikin kategori: %v", err)
	}

	// next_run_at sengaja mundur 3 hari, seolah server sempat mati.
	startDate := timeutil.StartOfDay(timeutil.Now()).AddDate(0, 0, -3)

	rule, err := NewRecurringRepository(db).CreateRule(ctx, CreateRuleParams{
		Name:       "Langganan streaming",
		Type:       "expense",
		Amount:     50000,
		WalletID:   walletID,
		CategoryID: categoryID,
		Note:       "auto",
		Frequency:  "daily",
		Interval:   1,
		StartDate:  startDate,
		NextRunAt:  startDate,
		IsActive:   true,
	})
	if err != nil {
		t.Fatalf("gagal bikin rule: %v", err)
	}

	return rule
}

func countTransactions(t *testing.T, ruleID string) int {
	t.Helper()

	var count int
	err := testDB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE recurring_rule_id = $1 AND deleted_at IS NULL`, ruleID).Scan(&count)
	if err != nil {
		t.Fatalf("gagal hitung transaksi: %v", err)
	}
	return count
}

// Acceptance criteria: scheduler idempoten, dibuktikan dengan menjalankan
// siklus dua kali di database sungguhan. Yang menjaganya adalah unique index
// (recurring_rule_id, occurred_at) plus ON CONFLICT DO NOTHING.
func TestSchedulerIdempotenDiDatabase(t *testing.T) {
	db := requireDB(t)
	rule := seedRule(t)

	repo := NewRecurringRepository(db)
	service := NewRecurringService(repo, nil, nil, nil)
	ctx := context.Background()

	first, err := service.RunDue(ctx)
	if err != nil {
		t.Fatalf("putaran pertama error: %v", err)
	}
	if first.Created != 4 {
		t.Fatalf("putaran pertama membuat %d transaksi, mau 4 (3 hari terlewat + hari ini)", first.Created)
	}

	afterFirst := countTransactions(t, rule.ID)

	second, err := service.RunDue(ctx)
	if err != nil {
		t.Fatalf("putaran kedua error: %v", err)
	}
	if second.Created != 0 {
		t.Errorf("putaran kedua membuat %d transaksi, mau 0", second.Created)
	}

	afterSecond := countTransactions(t, rule.ID)
	if afterFirst != afterSecond {
		t.Errorf("jumlah transaksi berubah dari %d jadi %d, scheduler tidak idempoten", afterFirst, afterSecond)
	}
}

// Memaksa next_run_at mundur lagi lalu menjalankan ulang: tanggal yang sama
// harus dihitung skip, bukan error, dan tidak menambah transaksi.
func TestGenerateOccurrencesMenganggapDuplikatSebagaiSkip(t *testing.T) {
	db := requireDB(t)
	rule := seedRule(t)

	repo := NewRecurringRepository(db)
	ctx := context.Background()

	dates := []time.Time{
		timeutil.StartOfDay(timeutil.Now()).AddDate(0, 0, -2),
		timeutil.StartOfDay(timeutil.Now()).AddDate(0, 0, -1),
	}
	nextRun := timeutil.StartOfDay(timeutil.Now())

	created, skipped, err := repo.GenerateOccurrences(ctx, rule, dates, nextRun)
	if err != nil {
		t.Fatalf("panggilan pertama error: %v", err)
	}
	if created != 2 || skipped != 0 {
		t.Fatalf("dapat created=%d skipped=%d, mau created=2 skipped=0", created, skipped)
	}

	created, skipped, err = repo.GenerateOccurrences(ctx, rule, dates, nextRun)
	if err != nil {
		t.Fatalf("panggilan kedua error: %v", err)
	}
	if created != 0 || skipped != 2 {
		t.Errorf("dapat created=%d skipped=%d, mau created=0 skipped=2", created, skipped)
	}

	if got := countTransactions(t, rule.ID); got != 2 {
		t.Errorf("jumlah transaksi %d, mau tetap 2", got)
	}
}

// Soft delete rule tidak boleh menghapus transaksi yang sudah digenerate
// (business rule 9).
func TestDeleteRuleTidakMenghapusTransaksi(t *testing.T) {
	db := requireDB(t)
	rule := seedRule(t)

	repo := NewRecurringRepository(db)
	service := NewRecurringService(repo, nil, nil, nil)
	ctx := context.Background()

	if _, err := service.RunDue(ctx); err != nil {
		t.Fatalf("RunDue error: %v", err)
	}

	before := countTransactions(t, rule.ID)
	if before == 0 {
		t.Fatal("harusnya ada transaksi yang tergenerate")
	}

	if _, err := repo.DeleteRule(ctx, rule.ID); err != nil {
		t.Fatalf("gagal hapus rule: %v", err)
	}

	if after := countTransactions(t, rule.ID); after != before {
		t.Errorf("transaksi jadi %d dari %d, riwayat tidak boleh ikut hilang", after, before)
	}
}
