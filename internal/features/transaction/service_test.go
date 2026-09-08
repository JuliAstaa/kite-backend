package transaction

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"errors"
	"testing"
)

// --- test double ---

type FakeTransactionRepository struct {
	CreateFunc      func(ctx context.Context, param CreateTransactionParams) (TransactionDetail, error)
	GetAllFunc      func(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error)
	GetByIDFunc     func(ctx context.Context, id string) (TransactionDetail, error)
	GetRawFunc      func(ctx context.Context, id string) (Transaction, error)
	UpdateFunc      func(ctx context.Context, id string, param UpdateTransactionParams) (TransactionDetail, error)
	DeleteFunc      func(ctx context.Context, id string) (TransactionDetail, error)
	RestoreFunc     func(ctx context.Context, id string) (TransactionDetail, error)
	LastCreateParam CreateTransactionParams
	LastUpdateParam UpdateTransactionParams
}

func (f *FakeTransactionRepository) CreateTransaction(ctx context.Context, param CreateTransactionParams) (TransactionDetail, error) {
	f.LastCreateParam = param
	if f.CreateFunc != nil {
		return f.CreateFunc(ctx, param)
	}
	return TransactionDetail{ID: "created", Type: param.Type, Amount: param.Amount}, nil
}

func (f *FakeTransactionRepository) GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {
	return f.GetAllFunc(ctx, filter)
}

func (f *FakeTransactionRepository) GetTransactionByID(ctx context.Context, id string) (TransactionDetail, error) {
	return f.GetByIDFunc(ctx, id)
}

func (f *FakeTransactionRepository) GetTransactionRaw(ctx context.Context, id string) (Transaction, error) {
	return f.GetRawFunc(ctx, id)
}

func (f *FakeTransactionRepository) UpdateTransaction(ctx context.Context, id string, param UpdateTransactionParams) (TransactionDetail, error) {
	f.LastUpdateParam = param
	if f.UpdateFunc != nil {
		return f.UpdateFunc(ctx, id, param)
	}
	return TransactionDetail{ID: id, Type: param.Type, Amount: param.Amount}, nil
}

func (f *FakeTransactionRepository) DeleteTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	return f.DeleteFunc(ctx, id)
}

func (f *FakeTransactionRepository) RestoreTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	return f.RestoreFunc(ctx, id)
}

// FakeWalletReader menganggap semua wallet ada, kecuali yang didaftarkan
// sebagai sudah dihapus.
type FakeWalletReader struct {
	Missing map[string]bool
}

func (f *FakeWalletReader) IsWalletExist(ctx context.Context, id string) error {
	if f.Missing[id] {
		return apperror.NotFoundError{Resource: "wallets", ID: id}
	}
	return nil
}

type FakeCategoryReader struct {
	Types   map[string]string
	Missing map[string]bool
}

func (f *FakeCategoryReader) GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error) {
	if f.Missing[id] {
		return CategoryInfo{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}
	catType, ok := f.Types[id]
	if !ok {
		catType = "expense"
	}
	return CategoryInfo{ID: id, Type: catType}, nil
}

func newTestService(repo TransactionRepositorer, wallet *FakeWalletReader, category *FakeCategoryReader) *TransactionService {
	if wallet == nil {
		wallet = &FakeWalletReader{}
	}
	if category == nil {
		category = &FakeCategoryReader{}
	}
	return NewTransactionService(repo, wallet, category)
}

func strptr(v string) *string { return &v }

// --- business rules 1 sampai 7 ---

func TestCreateTransactionBusinessRules(t *testing.T) {
	now := timeutil.Now()
	expenseCategory := "cat-expense"
	incomeCategory := "cat-income"

	categories := &FakeCategoryReader{Types: map[string]string{
		expenseCategory: "expense",
		incomeCategory:  "income",
	}}

	tests := []struct {
		name      string
		request   CreateTransactionRequest
		wallets   *FakeWalletReader
		wantErr   bool
		wantField string
	}{
		{
			name: "expense valid diterima",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "w1",
				CategoryID: &expenseCategory, OccurredAt: now,
			},
		},
		{
			name: "aturan 1 - amount nol ditolak",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 0, WalletID: "w1",
				CategoryID: &expenseCategory, OccurredAt: now,
			},
			wantErr: true, wantField: "amount",
		},
		{
			name: "aturan 1 - amount minus ditolak",
			request: CreateTransactionRequest{
				Type: "expense", Amount: -5000, WalletID: "w1",
				CategoryID: &expenseCategory, OccurredAt: now,
			},
			wantErr: true, wantField: "amount",
		},
		{
			name: "aturan 2 - kategori income dipakai transaksi expense ditolak",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "w1",
				CategoryID: &incomeCategory, OccurredAt: now,
			},
			wantErr: true, wantField: "category_id",
		},
		{
			name: "aturan 3 - transfer tanpa to_wallet_id ditolak",
			request: CreateTransactionRequest{
				Type: "transfer", Amount: 25000, WalletID: "w1", OccurredAt: now,
			},
			wantErr: true, wantField: "to_wallet_id",
		},
		{
			name: "aturan 3 - transfer ke wallet yang sama ditolak",
			request: CreateTransactionRequest{
				Type: "transfer", Amount: 25000, WalletID: "w1",
				ToWalletID: strptr("w1"), OccurredAt: now,
			},
			wantErr: true, wantField: "to_wallet_id",
		},
		{
			name: "aturan 3 - transfer valid diterima",
			request: CreateTransactionRequest{
				Type: "transfer", Amount: 25000, WalletID: "w1",
				ToWalletID: strptr("w2"), OccurredAt: now,
			},
		},
		{
			name: "aturan 4 - expense tanpa category_id ditolak",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "w1", OccurredAt: now,
			},
			wantErr: true, wantField: "category_id",
		},
		{
			name: "aturan 5 - backdate diterima",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "w1",
				CategoryID: &expenseCategory, OccurredAt: now.AddDate(-2, 0, 0),
			},
		},
		{
			name: "aturan 5 - besok masih diterima",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "w1",
				CategoryID: &expenseCategory, OccurredAt: now.AddDate(0, 0, 1),
			},
		},
		{
			name: "aturan 5 - lebih dari 1 hari ke depan ditolak",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "w1",
				CategoryID: &expenseCategory, OccurredAt: now.AddDate(0, 0, 3),
			},
			wantErr: true, wantField: "occurred_at",
		},
		{
			name: "aturan 6 - wallet yang sudah dihapus ditolak",
			request: CreateTransactionRequest{
				Type: "expense", Amount: 25000, WalletID: "deleted-wallet",
				CategoryID: &expenseCategory, OccurredAt: now,
			},
			wallets: &FakeWalletReader{Missing: map[string]bool{"deleted-wallet": true}},
			wantErr: true,
		},
		{
			name: "type tidak dikenal ditolak",
			request: CreateTransactionRequest{
				Type: "kredit", Amount: 25000, WalletID: "w1", OccurredAt: now,
			},
			wantErr: true, wantField: "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &FakeTransactionRepository{}
			service := newTestService(repo, tt.wallets, categories)

			_, err := service.CreateTransaction(context.Background(), tt.request)

			if tt.wantErr && err == nil {
				t.Fatalf("mau error, dapat nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("mau sukses, dapat error: %v", err)
			}

			if tt.wantField != "" {
				var ve apperror.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("mau ValidationError, dapat %T (%v)", err, err)
				}
				if ve.Field != tt.wantField {
					t.Errorf("field %q, mau %q", ve.Field, tt.wantField)
				}
			}
		})
	}
}

// Aturan 3 dan 4: field lawannya harus dibersihkan, bukan sekadar diabaikan.
func TestCreateTransactionClearsOppositeFields(t *testing.T) {
	now := timeutil.Now()
	category := "cat-expense"

	t.Run("expense membuang to_wallet_id", func(t *testing.T) {
		repo := &FakeTransactionRepository{}
		service := newTestService(repo, nil, nil)

		_, err := service.CreateTransaction(context.Background(), CreateTransactionRequest{
			Type: "expense", Amount: 1000, WalletID: "w1",
			CategoryID: &category, ToWalletID: strptr("w2"), OccurredAt: now,
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if repo.LastCreateParam.ToWalletID != nil {
			t.Errorf("to_wallet_id harus nil untuk expense, dapat %v", *repo.LastCreateParam.ToWalletID)
		}
	})

	t.Run("transfer membuang category_id", func(t *testing.T) {
		repo := &FakeTransactionRepository{}
		service := newTestService(repo, nil, nil)

		_, err := service.CreateTransaction(context.Background(), CreateTransactionRequest{
			Type: "transfer", Amount: 1000, WalletID: "w1",
			ToWalletID: strptr("w2"), CategoryID: &category, OccurredAt: now,
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if repo.LastCreateParam.CategoryID != nil {
			t.Errorf("category_id harus nil untuk transfer, dapat %v", *repo.LastCreateParam.CategoryID)
		}
	})
}

// Aturan 8: patch cuma mengubah transaksinya, dan tetap tunduk pada semua aturan.
func TestPatchTransactionRevalidates(t *testing.T) {
	now := timeutil.Now()

	existing := Transaction{
		ID: "t1", Type: "expense", Amount: 25000, WalletID: "w1",
		CategoryID:      sql.NullString{String: "cat-expense", Valid: true},
		OccurredAt:      now,
		RecurringRuleID: sql.NullString{String: "rule-1", Valid: true},
	}

	newRepo := func() *FakeTransactionRepository {
		return &FakeTransactionRepository{
			GetRawFunc: func(ctx context.Context, id string) (Transaction, error) { return existing, nil },
		}
	}

	t.Run("amount nol ditolak", func(t *testing.T) {
		service := newTestService(newRepo(), nil, nil)
		zero := 0

		_, err := service.PatchTransaction(context.Background(), "t1", PatchTransactionRequest{Amount: &zero})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "amount" {
			t.Fatalf("mau ValidationError amount, dapat %v", err)
		}
	})

	t.Run("ganti tipe ke transfer membuang category_id", func(t *testing.T) {
		repo := newRepo()
		service := newTestService(repo, nil, nil)

		_, err := service.PatchTransaction(context.Background(), "t1", PatchTransactionRequest{
			Type:       strptr("transfer"),
			ToWalletID: strptr("w2"),
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if repo.LastUpdateParam.CategoryID != nil {
			t.Errorf("category_id harus nil setelah jadi transfer")
		}
		if repo.LastUpdateParam.ToWalletID == nil || *repo.LastUpdateParam.ToWalletID != "w2" {
			t.Errorf("to_wallet_id harus terisi w2")
		}
	})

	t.Run("field yang tidak dikirim tetap memakai nilai lama", func(t *testing.T) {
		repo := newRepo()
		service := newTestService(repo, nil, nil)

		_, err := service.PatchTransaction(context.Background(), "t1", PatchTransactionRequest{
			Note: strptr("catatan baru"),
		})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if repo.LastUpdateParam.Amount != 25000 {
			t.Errorf("amount %d, mau tetap 25000", repo.LastUpdateParam.Amount)
		}
		if repo.LastUpdateParam.Note != "catatan baru" {
			t.Errorf("note %q, mau %q", repo.LastUpdateParam.Note, "catatan baru")
		}
	})

	t.Run("occurred_at terlalu jauh di masa depan ditolak", func(t *testing.T) {
		service := newTestService(newRepo(), nil, nil)
		future := now.AddDate(0, 0, 10)

		_, err := service.PatchTransaction(context.Background(), "t1", PatchTransactionRequest{OccurredAt: &future})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "occurred_at" {
			t.Fatalf("mau ValidationError occurred_at, dapat %v", err)
		}
	})
}

// Aturan 7: saldo minus tidak pernah diblokir. Service tidak punya jalur untuk
// menolak berdasarkan saldo, jadi transaksi besar pun tetap lolos.
func TestSaldoMinusTidakDiblokir(t *testing.T) {
	repo := &FakeTransactionRepository{}
	service := newTestService(repo, nil, nil)
	category := "cat-expense"

	_, err := service.CreateTransaction(context.Background(), CreateTransactionRequest{
		Type: "expense", Amount: 999_000_000, WalletID: "w1",
		CategoryID: &category, OccurredAt: timeutil.Now(),
	})
	if err != nil {
		t.Fatalf("pengeluaran besar tidak boleh diblokir: %v", err)
	}
}
