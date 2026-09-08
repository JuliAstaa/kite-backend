package quickadd

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"errors"
	"testing"
)

type FakeQuickAddRepository struct {
	Item          QuickAdd
	Items         []QuickAdd
	Err           error
	CapturedParam CreateQuickAddParams
	CapturedPatch PatchQuickAddParams
}

func (f *FakeQuickAddRepository) CreateQuickAdd(ctx context.Context, param CreateQuickAddParams) (QuickAdd, error) {
	f.CapturedParam = param
	if f.Err != nil {
		return QuickAdd{}, f.Err
	}
	return QuickAdd{Label: param.Label, Type: param.Type, WalletID: param.WalletID, CategoryID: param.CategoryID}, nil
}

func (f *FakeQuickAddRepository) GetAllQuickAdds(ctx context.Context, includeDeleted bool) ([]QuickAdd, int, error) {
	return f.Items, len(f.Items), f.Err
}

func (f *FakeQuickAddRepository) GetQuickAddByID(ctx context.Context, id string) (QuickAdd, error) {
	if f.Err != nil {
		return QuickAdd{}, f.Err
	}
	return f.Item, nil
}

func (f *FakeQuickAddRepository) PatchQuickAdd(ctx context.Context, id string, param PatchQuickAddParams) (QuickAdd, error) {
	f.CapturedPatch = param
	return f.Item, f.Err
}

func (f *FakeQuickAddRepository) DeleteQuickAdd(ctx context.Context, id string) (QuickAdd, error) {
	return f.Item, f.Err
}

type FakeWalletReader struct{ Missing bool }

func (f *FakeWalletReader) IsWalletExist(ctx context.Context, id string) error {
	if f.Missing {
		return apperror.NotFoundError{Resource: "wallets", ID: id}
	}
	return nil
}

type FakeCategoryReader struct {
	Type    string
	Missing bool
}

func (f *FakeCategoryReader) GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error) {
	if f.Missing {
		return CategoryInfo{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}
	catType := f.Type
	if catType == "" {
		catType = "expense"
	}
	return CategoryInfo{ID: id, Type: catType}, nil
}

type FakeTransactionCreator struct {
	Captured CreateTransactionInput
	Calls    int
	Err      error
}

func (f *FakeTransactionCreator) CreateTransaction(ctx context.Context, input CreateTransactionInput) (CreatedTransaction, error) {
	f.Calls++
	f.Captured = input
	if f.Err != nil {
		return CreatedTransaction{}, f.Err
	}
	return CreatedTransaction{
		ID: "tx-1", Type: input.Type, Amount: input.Amount,
		Note: input.Note, OccurredAt: input.OccurredAt,
	}, nil
}

func newService(repo *FakeQuickAddRepository, wallet *FakeWalletReader, category *FakeCategoryReader, tx *FakeTransactionCreator) *QuickAddService {
	if repo == nil {
		repo = &FakeQuickAddRepository{}
	}
	if wallet == nil {
		wallet = &FakeWalletReader{}
	}
	if category == nil {
		category = &FakeCategoryReader{}
	}
	if tx == nil {
		tx = &FakeTransactionCreator{}
	}
	return NewQuickAddService(repo, wallet, category, tx)
}

func TestCreateQuickAddService(t *testing.T) {
	amount := 25_000
	zero := 0

	tests := []struct {
		name      string
		request   CreateQuickAddRequest
		category  *FakeCategoryReader
		wallet    *FakeWalletReader
		wantErr   bool
		wantField string
	}{
		{
			name:    "valid",
			request: CreateQuickAddRequest{Label: "Kopi", Type: "expense", Amount: &amount, WalletID: "w1", CategoryID: "c1"},
		},
		{
			name:    "amount boleh kosong",
			request: CreateQuickAddRequest{Label: "Bensin", Type: "expense", WalletID: "w1", CategoryID: "c1"},
		},
		{
			name:      "label kosong ditolak",
			request:   CreateQuickAddRequest{Label: "   ", Type: "expense", Amount: &amount, WalletID: "w1", CategoryID: "c1"},
			wantErr:   true,
			wantField: "label",
		},
		{
			name:      "type ngawur ditolak",
			request:   CreateQuickAddRequest{Label: "Kopi", Type: "transfer", Amount: &amount, WalletID: "w1", CategoryID: "c1"},
			wantErr:   true,
			wantField: "type",
		},
		{
			name:      "amount nol ditolak",
			request:   CreateQuickAddRequest{Label: "Kopi", Type: "expense", Amount: &zero, WalletID: "w1", CategoryID: "c1"},
			wantErr:   true,
			wantField: "amount",
		},
		{
			name:      "kategori income untuk quick add expense ditolak",
			request:   CreateQuickAddRequest{Label: "Kopi", Type: "expense", Amount: &amount, WalletID: "w1", CategoryID: "c1"},
			category:  &FakeCategoryReader{Type: "income"},
			wantErr:   true,
			wantField: "category_id",
		},
		{
			name:    "wallet terhapus ditolak",
			request: CreateQuickAddRequest{Label: "Kopi", Type: "expense", Amount: &amount, WalletID: "w1", CategoryID: "c1"},
			wallet:  &FakeWalletReader{Missing: true},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newService(nil, tt.wallet, tt.category, nil)

			_, err := service.CreateQuickAdd(context.Background(), tt.request)

			if tt.wantErr && err == nil {
				t.Fatal("mau error, dapat nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("mau sukses, dapat %v", err)
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

// Ganti tipe berarti kategorinya harus dicek ulang, walaupun category_id
// tidak ikut dikirim.
func TestPatchQuickAddMengecekUlangKategoriSaatTipeBerubah(t *testing.T) {
	repo := &FakeQuickAddRepository{Item: QuickAdd{ID: "q1", Type: "expense", CategoryID: "c1"}}
	service := newService(repo, nil, &FakeCategoryReader{Type: "expense"}, nil)

	newType := "income"
	_, err := service.PatchQuickAdd(context.Background(), "q1", PatchQuickAddRequest{Type: &newType})

	var ve apperror.ValidationError
	if !errors.As(err, &ve) || ve.Field != "category_id" {
		t.Fatalf("mau ValidationError category_id, dapat %v", err)
	}
}

func TestPatchQuickAddValidasi(t *testing.T) {
	repo := &FakeQuickAddRepository{Item: QuickAdd{ID: "q1", Type: "expense", CategoryID: "c1"}}

	t.Run("label kosong ditolak", func(t *testing.T) {
		service := newService(repo, nil, nil, nil)
		label := "  "

		_, err := service.PatchQuickAdd(context.Background(), "q1", PatchQuickAddRequest{Label: &label})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "label" {
			t.Fatalf("mau ValidationError label, dapat %v", err)
		}
	})

	t.Run("amount nol ditolak", func(t *testing.T) {
		service := newService(repo, nil, nil, nil)
		zero := 0

		_, err := service.PatchQuickAdd(context.Background(), "q1", PatchQuickAddRequest{Amount: &zero})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "amount" {
			t.Fatalf("mau ValidationError amount, dapat %v", err)
		}
	})

	t.Run("ClearAmount diteruskan ke repository", func(t *testing.T) {
		service := newService(repo, nil, nil, nil)

		if _, err := service.PatchQuickAdd(context.Background(), "q1", PatchQuickAddRequest{ClearAmount: true}); err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if !repo.CapturedPatch.ClearAmount {
			t.Error("ClearAmount harus diteruskan")
		}
	})
}

func TestExecuteQuickAdd(t *testing.T) {
	t.Run("memakai amount dari quick add", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Item: QuickAdd{
			ID: "q1", Label: "Kopi", Type: "expense",
			Amount:   sql.NullInt64{Int64: 25_000, Valid: true},
			WalletID: "w1", CategoryID: "c1", Note: "kopi susu",
		}}
		creator := &FakeTransactionCreator{}
		service := newService(repo, nil, nil, creator)

		got, err := service.Execute(context.Background(), "q1", ExecuteRequest{})
		if err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}

		if creator.Captured.Amount != 25_000 {
			t.Errorf("amount %d, mau 25000", creator.Captured.Amount)
		}
		if creator.Captured.Note != "kopi susu" {
			t.Errorf("note %q, mau dari quick add", creator.Captured.Note)
		}
		if creator.Captured.CategoryID == nil || *creator.Captured.CategoryID != "c1" {
			t.Errorf("category_id salah: %+v", creator.Captured.CategoryID)
		}
		if got.ID != "tx-1" {
			t.Errorf("transaction id %q, mau tx-1", got.ID)
		}
	})

	t.Run("amount dari request menang", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Item: QuickAdd{
			ID: "q1", Type: "expense", Amount: sql.NullInt64{Int64: 25_000, Valid: true},
			WalletID: "w1", CategoryID: "c1",
		}}
		creator := &FakeTransactionCreator{}
		service := newService(repo, nil, nil, creator)

		override := 50_000
		if _, err := service.Execute(context.Background(), "q1", ExecuteRequest{Amount: &override}); err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if creator.Captured.Amount != 50_000 {
			t.Errorf("amount %d, mau 50000", creator.Captured.Amount)
		}
	})

	t.Run("quick add tanpa amount butuh amount di request", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Item: QuickAdd{
			ID: "q1", Type: "expense", Amount: sql.NullInt64{Valid: false},
			WalletID: "w1", CategoryID: "c1",
		}}
		creator := &FakeTransactionCreator{}
		service := newService(repo, nil, nil, creator)

		_, err := service.Execute(context.Background(), "q1", ExecuteRequest{})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "amount" {
			t.Fatalf("mau ValidationError amount, dapat %v", err)
		}
		if creator.Calls != 0 {
			t.Error("transaksi tidak boleh dibuat")
		}
	})

	t.Run("occurred_at kosong diisi waktu sekarang", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Item: QuickAdd{
			ID: "q1", Type: "expense", Amount: sql.NullInt64{Int64: 25_000, Valid: true},
			WalletID: "w1", CategoryID: "c1",
		}}
		creator := &FakeTransactionCreator{}
		service := newService(repo, nil, nil, creator)

		if _, err := service.Execute(context.Background(), "q1", ExecuteRequest{}); err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if creator.Captured.OccurredAt.IsZero() {
			t.Error("occurred_at harus diisi otomatis")
		}
	})

	t.Run("note dari request menang", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Item: QuickAdd{
			ID: "q1", Type: "expense", Amount: sql.NullInt64{Int64: 25_000, Valid: true},
			WalletID: "w1", CategoryID: "c1", Note: "bawaan",
		}}
		creator := &FakeTransactionCreator{}
		service := newService(repo, nil, nil, creator)

		if _, err := service.Execute(context.Background(), "q1", ExecuteRequest{Note: "catatan khusus"}); err != nil {
			t.Fatalf("tidak mau error: %v", err)
		}
		if creator.Captured.Note != "catatan khusus" {
			t.Errorf("note %q, mau catatan khusus", creator.Captured.Note)
		}
	})

	// Transaksi dibuat lewat service transaction, jadi aturan bisnisnya tetap
	// berlaku. Kalau di sana ditolak, quick add ikut gagal.
	t.Run("error dari service transaction diteruskan", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Item: QuickAdd{
			ID: "q1", Type: "expense", Amount: sql.NullInt64{Int64: 25_000, Valid: true},
			WalletID: "w1", CategoryID: "c1",
		}}
		creator := &FakeTransactionCreator{
			Err: apperror.ValidationError{Field: "occurred_at", Message: "terlalu jauh di masa depan"},
		}
		service := newService(repo, nil, nil, creator)

		_, err := service.Execute(context.Background(), "q1", ExecuteRequest{OccurredAt: timeutil.Now().AddDate(0, 0, 30)})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("mau ValidationError diteruskan, dapat %v", err)
		}
	})

	t.Run("quick add tidak ketemu", func(t *testing.T) {
		repo := &FakeQuickAddRepository{Err: apperror.NotFoundError{Resource: "quick_adds", ID: "q1"}}
		creator := &FakeTransactionCreator{}
		service := newService(repo, nil, nil, creator)

		_, err := service.Execute(context.Background(), "q1", ExecuteRequest{})

		var nf apperror.NotFoundError
		if !errors.As(err, &nf) {
			t.Fatalf("mau NotFoundError, dapat %v", err)
		}
		if creator.Calls != 0 {
			t.Error("transaksi tidak boleh dibuat")
		}
	})
}
