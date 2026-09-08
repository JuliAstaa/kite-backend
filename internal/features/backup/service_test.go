package backup

import (
	"backend/internal/shared/timeutil"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type FakeBackupRepository struct {
	Transactions []ExportTransaction
	Wallets      []ExportWallet
	Categories   []ExportCategory
	WalletLookup map[string]string
	CatLookup    map[string]string
	Err          error

	CapturedRows []ImportRow
	ImportCalls  int
}

func (f *FakeBackupRepository) ExportTransactions(ctx context.Context, from, to time.Time) ([]ExportTransaction, error) {
	return f.Transactions, f.Err
}

func (f *FakeBackupRepository) ExportWallets(ctx context.Context) ([]ExportWallet, error) {
	return f.Wallets, f.Err
}

func (f *FakeBackupRepository) ExportCategories(ctx context.Context) ([]ExportCategory, error) {
	return f.Categories, f.Err
}

func (f *FakeBackupRepository) WalletsByName(ctx context.Context) (map[string]string, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	if f.WalletLookup == nil {
		return map[string]string{"cash": "w1", "bca": "w2"}, nil
	}
	return f.WalletLookup, nil
}

func (f *FakeBackupRepository) CategoriesByNameAndType(ctx context.Context) (map[string]string, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	if f.CatLookup == nil {
		return map[string]string{"makan & minum|expense": "c1", "gaji|income": "c2"}, nil
	}
	return f.CatLookup, nil
}

func (f *FakeBackupRepository) ImportTransactions(ctx context.Context, rows []ImportRow) (int, error) {
	f.ImportCalls++
	f.CapturedRows = rows
	if f.Err != nil {
		return 0, f.Err
	}
	return len(rows), nil
}

func TestExportService(t *testing.T) {
	repo := &FakeBackupRepository{
		Wallets:      []ExportWallet{{ID: "w1", Name: "Cash", Type: "cash", InitialBalance: 1_000_000}},
		Categories:   []ExportCategory{{ID: "c1", Name: "Makan & Minum", Type: "expense"}},
		Transactions: []ExportTransaction{{ID: "t1", Type: "expense", Amount: 25_000, Wallet: "Cash"}},
	}
	service := NewBackupService(repo)

	from := time.Date(2026, time.August, 1, 0, 0, 0, 0, timeutil.Loc())
	to := timeutil.EndOfDay(time.Date(2026, time.August, 31, 0, 0, 0, 0, timeutil.Loc()))

	got, err := service.Export(context.Background(), from, to)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.From != "2026-08-01" || got.To != "2026-08-31" {
		t.Errorf("rentang salah: %s sampai %s", got.From, got.To)
	}
	if len(got.Wallets) != 1 || len(got.Categories) != 1 || len(got.Transactions) != 1 {
		t.Errorf("isi bundle kurang: %+v", got)
	}
	if got.ExportedAt.IsZero() {
		t.Error("exported_at harus terisi")
	}
}

func TestExportMeneruskanError(t *testing.T) {
	sentinel := errors.New("database mati")
	service := NewBackupService(&FakeBackupRepository{Err: sentinel})

	_, err := service.Export(context.Background(), timeutil.Now(), timeutil.Now())
	if !errors.Is(err, sentinel) {
		t.Errorf("mau error asli diteruskan, dapat %v", err)
	}
}

const header = "date,type,amount,wallet,category,note\n"

func TestImportDryRun(t *testing.T) {
	repo := &FakeBackupRepository{}
	service := NewBackupService(repo)

	csv := header +
		"2026-08-11,expense,25000,Cash,Makan & Minum,Kopi susu\n" +
		"2026-08-12,income,500000,Cash,Gaji,Bonus\n"

	got, err := service.Import(context.Background(), strings.NewReader(csv), true)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.TotalRows != 2 || got.ValidRows != 2 || got.InvalidRows != 0 {
		t.Errorf("hitungan salah: %+v", got)
	}
	if got.Imported != 0 {
		t.Errorf("dry run tidak boleh menulis, imported %d", got.Imported)
	}
	if repo.ImportCalls != 0 {
		t.Error("dry run tidak boleh memanggil repository")
	}
}

func TestImportSungguhan(t *testing.T) {
	repo := &FakeBackupRepository{}
	service := NewBackupService(repo)

	csv := header + "2026-08-11,expense,25000,Cash,Makan & Minum,Kopi susu\n"

	got, err := service.Import(context.Background(), strings.NewReader(csv), false)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.Imported != 1 {
		t.Errorf("imported %d, mau 1", got.Imported)
	}
	if repo.ImportCalls != 1 {
		t.Errorf("repository dipanggil %d kali, mau 1", repo.ImportCalls)
	}

	row := repo.CapturedRows[0]
	if row.WalletID != "w1" || row.CategoryID != "c1" {
		t.Errorf("nama tidak diterjemahkan jadi id: %+v", row)
	}
	if timeutil.FormatDate(row.OccurredAt) != "2026-08-11" {
		t.Errorf("occurred_at %s, mau 2026-08-11", timeutil.FormatDate(row.OccurredAt))
	}
}

func TestImportBarisSalah(t *testing.T) {
	tests := []struct {
		name        string
		row         string
		wantMessage string
	}{
		{"tanggal ngawur", "kemarin,expense,25000,Cash,Makan & Minum,x", "date harus format YYYY-MM-DD"},
		{"type transfer tidak didukung", "2026-08-11,transfer,25000,Cash,,x", "transfer tidak didukung"},
		{"type ngawur", "2026-08-11,kredit,25000,Cash,Makan & Minum,x", "type harus income atau expense"},
		{"amount bukan angka", "2026-08-11,expense,abc,Cash,Makan & Minum,x", "amount harus angka"},
		{"amount nol", "2026-08-11,expense,0,Cash,Makan & Minum,x", "amount harus lebih besar dari 0"},
		{"wallet tidak ada", "2026-08-11,expense,25000,Dompet Hantu,Makan & Minum,x", "tidak ditemukan"},
		{"kategori kosong", "2026-08-11,expense,25000,Cash,,x", "category wajib diisi"},
		{"kategori tidak ada", "2026-08-11,expense,25000,Cash,Kategori Hantu,x", "tidak ditemukan"},
		{"kategori salah tipe", "2026-08-11,income,25000,Cash,Makan & Minum,x", "tidak ditemukan"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewBackupService(&FakeBackupRepository{})

			got, err := service.Import(context.Background(), strings.NewReader(header+tt.row+"\n"), true)
			if err != nil {
				t.Fatalf("tidak mau error: %v", err)
			}

			if got.InvalidRows != 1 {
				t.Fatalf("invalid_rows %d, mau 1", got.InvalidRows)
			}
			if len(got.Errors) != 1 {
				t.Fatalf("dapat %d error, mau 1", len(got.Errors))
			}
			if !strings.Contains(got.Errors[0].Message, tt.wantMessage) {
				t.Errorf("pesan %q, mau memuat %q", got.Errors[0].Message, tt.wantMessage)
			}
			// nomor barisnya menunjuk baris di file, bukan indeks slice
			if got.Errors[0].Line != 2 {
				t.Errorf("line %d, mau 2", got.Errors[0].Line)
			}
		})
	}
}

// Semua baris dicek sampai habis, jadi laporannya lengkap, bukan berhenti di
// baris salah pertama.
func TestImportMelaporkanSemuaBarisSalah(t *testing.T) {
	service := NewBackupService(&FakeBackupRepository{})

	csv := header +
		"2026-08-11,expense,25000,Cash,Makan & Minum,valid\n" +
		"2026-08-12,expense,abc,Cash,Makan & Minum,amount rusak\n" +
		"2026-08-13,expense,20000,Dompet Hantu,Makan & Minum,wallet hilang\n"

	got, err := service.Import(context.Background(), strings.NewReader(csv), true)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.TotalRows != 3 || got.ValidRows != 1 || got.InvalidRows != 2 {
		t.Errorf("hitungan salah: %+v", got)
	}
	if len(got.Errors) != 2 {
		t.Fatalf("dapat %d error, mau 2", len(got.Errors))
	}
	if got.Errors[0].Line != 3 || got.Errors[1].Line != 4 {
		t.Errorf("nomor baris salah: %+v", got.Errors)
	}
}

// Import sungguhan menolak file yang masih ada barisnya salah, supaya tidak ada
// data setengah masuk.
func TestImportSungguhanMenolakFileBermasalah(t *testing.T) {
	repo := &FakeBackupRepository{}
	service := NewBackupService(repo)

	csv := header +
		"2026-08-11,expense,25000,Cash,Makan & Minum,valid\n" +
		"2026-08-12,expense,abc,Cash,Makan & Minum,rusak\n"

	got, err := service.Import(context.Background(), strings.NewReader(csv), false)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if got.Imported != 0 {
		t.Errorf("imported %d, mau 0", got.Imported)
	}
	if repo.ImportCalls != 0 {
		t.Error("repository tidak boleh dipanggil kalau masih ada baris salah")
	}
}

func TestImportHeader(t *testing.T) {
	t.Run("kolom wajib tidak ada", func(t *testing.T) {
		service := NewBackupService(&FakeBackupRepository{})

		csv := "date,type,amount\n2026-08-11,expense,25000\n"
		_, err := service.Import(context.Background(), strings.NewReader(csv), true)

		if err == nil || !strings.Contains(err.Error(), "wallet") {
			t.Fatalf("mau error tentang kolom wallet, dapat %v", err)
		}
	})

	t.Run("urutan kolom bebas", func(t *testing.T) {
		repo := &FakeBackupRepository{}
		service := NewBackupService(repo)

		csv := "note,wallet,amount,type,date,category\n" +
			"Kopi,Cash,25000,expense,2026-08-11,Makan & Minum\n"

		got, err := service.Import(context.Background(), strings.NewReader(csv), false)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.ValidRows != 1 {
			t.Fatalf("valid_rows %d, mau 1. errors: %+v", got.ValidRows, got.Errors)
		}
		if repo.CapturedRows[0].Note != "Kopi" {
			t.Errorf("note %q, mau Kopi", repo.CapturedRows[0].Note)
		}
	})

	// File dari Excel sering diawali BOM.
	t.Run("BOM di awal file dibuang", func(t *testing.T) {
		service := NewBackupService(&FakeBackupRepository{})

		csv := "\ufeff" + header + "2026-08-11,expense,25000,Cash,Makan & Minum,Kopi\n"

		got, err := service.Import(context.Background(), strings.NewReader(csv), true)
		if err != nil {
			t.Fatalf("BOM harusnya tidak bikin error: %v", err)
		}
		if got.ValidRows != 1 {
			t.Errorf("valid_rows %d, mau 1. errors: %+v", got.ValidRows, got.Errors)
		}
	})

	t.Run("file kosong bukan error", func(t *testing.T) {
		service := NewBackupService(&FakeBackupRepository{})

		got, err := service.Import(context.Background(), strings.NewReader(""), true)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.TotalRows != 0 {
			t.Errorf("total_rows %d, mau 0", got.TotalRows)
		}
	})

	t.Run("baris kosong dilewati", func(t *testing.T) {
		service := NewBackupService(&FakeBackupRepository{})

		csv := header + "2026-08-11,expense,25000,Cash,Makan & Minum,Kopi\n" + ",,,,,\n"

		got, err := service.Import(context.Background(), strings.NewReader(csv), true)
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if got.TotalRows != 1 {
			t.Errorf("total_rows %d, mau 1 (baris kosong tidak dihitung)", got.TotalRows)
		}
	})
}

// Nominal dengan pemisah ribuan gaya Indonesia tetap kebaca.
func TestImportAmountDenganTitik(t *testing.T) {
	repo := &FakeBackupRepository{}
	service := NewBackupService(repo)

	csv := header + "2026-08-11,expense,1.250.000,Cash,Makan & Minum,Belanja\n"

	got, err := service.Import(context.Background(), strings.NewReader(csv), false)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if got.ValidRows != 1 {
		t.Fatalf("valid_rows %d, mau 1. errors: %+v", got.ValidRows, got.Errors)
	}
	if repo.CapturedRows[0].Amount != 1_250_000 {
		t.Errorf("amount %d, mau 1250000", repo.CapturedRows[0].Amount)
	}
}
