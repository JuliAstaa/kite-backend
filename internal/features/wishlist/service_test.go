package wishlist

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"errors"
	"testing"
)

// --- test double ---

type FakeWishlistRepository struct {
	Item          Item
	AllocateErr   error
	LastAllocated int
	PurchaseCalls int
}

func (f *FakeWishlistRepository) CreateItem(ctx context.Context, param CreateItemParams) (Item, error) {
	return Item{Name: param.Name, EstimatedPrice: param.EstimatedPrice, Priority: param.Priority, Status: param.Status}, nil
}

func (f *FakeWishlistRepository) GetAllItems(ctx context.Context, filter ListFilter) ([]Item, int, error) {
	return []Item{f.Item}, 1, nil
}

func (f *FakeWishlistRepository) GetItemByID(ctx context.Context, id string) (Item, error) {
	return f.Item, nil
}

func (f *FakeWishlistRepository) PatchItem(ctx context.Context, id string, param PatchItemParams) (Item, error) {
	item := f.Item
	if param.EstimatedPrice != nil {
		item.EstimatedPrice = *param.EstimatedPrice
	}
	return item, nil
}

func (f *FakeWishlistRepository) DeleteItem(ctx context.Context, id string) (Item, error) {
	return f.Item, nil
}

func (f *FakeWishlistRepository) RestoreItem(ctx context.Context, id string) (Item, error) {
	return f.Item, nil
}

func (f *FakeWishlistRepository) Allocate(ctx context.Context, id string, amount int) (Item, error) {
	if f.AllocateErr != nil {
		return Item{}, f.AllocateErr
	}
	f.LastAllocated = amount
	item := f.Item
	item.SavedAmount += amount
	return item, nil
}

func (f *FakeWishlistRepository) Purchase(ctx context.Context, id string, param PurchaseParams) (Item, error) {
	f.PurchaseCalls++
	item := f.Item
	item.Status = "purchased"
	return item, nil
}

func (f *FakeWishlistRepository) Summary(ctx context.Context) (SummaryTotals, error) {
	return SummaryTotals{}, nil
}

type FakeSavingsReader struct {
	Avg    int
	Sample int
}

func (f *FakeSavingsReader) AverageMonthlySavable(ctx context.Context, months int) (int, int, error) {
	return f.Avg, f.Sample, nil
}

type FakeWalletReader struct{ Missing bool }

func (f *FakeWalletReader) IsWalletExist(ctx context.Context, id string) error {
	if f.Missing {
		return apperror.NotFoundError{Resource: "wallets", ID: id}
	}
	return nil
}

type FakeCategoryReader struct{ Type string }

func (f *FakeCategoryReader) GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error) {
	catType := f.Type
	if catType == "" {
		catType = "expense"
	}
	return CategoryInfo{ID: id, Type: catType}, nil
}

func newTestService(repo *FakeWishlistRepository, savings *FakeSavingsReader) *WishlistService {
	if savings == nil {
		savings = &FakeSavingsReader{Avg: 1_000_000, Sample: 3}
	}
	return NewWishlistService(repo, savings, &FakeWalletReader{}, &FakeCategoryReader{})
}

// --- business rule 11 ---

func TestAllocateTidakBolehMelebihiHarga(t *testing.T) {
	t.Run("amount nol ditolak", func(t *testing.T) {
		repo := &FakeWishlistRepository{Item: Item{ID: "i1", EstimatedPrice: 1_000_000, Status: "planned"}}
		service := newTestService(repo, nil)

		_, err := service.Allocate(context.Background(), "i1", 0)

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "amount" {
			t.Fatalf("mau ValidationError amount, dapat %v", err)
		}
	})

	t.Run("kelebihan harga ditolak repository", func(t *testing.T) {
		repo := &FakeWishlistRepository{
			Item:        Item{ID: "i1", EstimatedPrice: 1_000_000, SavedAmount: 900_000, Status: "saving"},
			AllocateErr: apperror.ValidationError{Field: "amount", Message: "total saved_amount tidak boleh melebihi estimated_price"},
		}
		service := newTestService(repo, nil)

		_, err := service.Allocate(context.Background(), "i1", 200_000)

		var ve apperror.ValidationError
		if !errors.As(err, &ve) {
			t.Fatalf("mau ValidationError, dapat %v", err)
		}
	})

	t.Run("harga baru di bawah saved_amount ditolak", func(t *testing.T) {
		repo := &FakeWishlistRepository{Item: Item{ID: "i1", EstimatedPrice: 1_000_000, SavedAmount: 800_000, Status: "saving"}}
		service := newTestService(repo, nil)

		lower := 500_000
		_, err := service.PatchItem(context.Background(), "i1", PatchItemRequest{EstimatedPrice: &lower})

		var ve apperror.ValidationError
		if !errors.As(err, &ve) || ve.Field != "estimated_price" {
			t.Fatalf("mau ValidationError estimated_price, dapat %v", err)
		}
	})
}

// --- business rule 12 ---

func TestItemSudahDibeliTidakBisaDiubah(t *testing.T) {
	purchased := Item{ID: "i1", EstimatedPrice: 1_000_000, SavedAmount: 1_000_000, Status: "purchased"}

	t.Run("tidak bisa dialokasi tambahan", func(t *testing.T) {
		service := newTestService(&FakeWishlistRepository{Item: purchased}, nil)

		_, err := service.Allocate(context.Background(), "i1", 50_000)

		var ue apperror.UnprocessableError
		if !errors.As(err, &ue) {
			t.Fatalf("mau UnprocessableError, dapat %v", err)
		}
	})

	t.Run("tidak bisa diubah harganya", func(t *testing.T) {
		service := newTestService(&FakeWishlistRepository{Item: purchased}, nil)

		newPrice := 2_000_000
		_, err := service.PatchItem(context.Background(), "i1", PatchItemRequest{EstimatedPrice: &newPrice})

		var ue apperror.UnprocessableError
		if !errors.As(err, &ue) {
			t.Fatalf("mau UnprocessableError, dapat %v", err)
		}
	})

	t.Run("tidak bisa dibeli dua kali", func(t *testing.T) {
		repo := &FakeWishlistRepository{Item: purchased}
		service := newTestService(repo, nil)

		_, err := service.Purchase(context.Background(), "i1", PurchaseRequest{
			WalletID: "w1", CategoryID: "c1", ActualPrice: 1_000_000,
		})

		var ue apperror.UnprocessableError
		if !errors.As(err, &ue) {
			t.Fatalf("mau UnprocessableError, dapat %v", err)
		}
		if repo.PurchaseCalls != 0 {
			t.Errorf("repository.Purchase tidak boleh dipanggil")
		}
	})
}

func TestPurchaseMenolakKategoriIncome(t *testing.T) {
	repo := &FakeWishlistRepository{Item: Item{ID: "i1", EstimatedPrice: 1_000_000, Status: "planned"}}
	service := NewWishlistService(repo, &FakeSavingsReader{Avg: 100, Sample: 3},
		&FakeWalletReader{}, &FakeCategoryReader{Type: "income"})

	_, err := service.Purchase(context.Background(), "i1", PurchaseRequest{
		WalletID: "w1", CategoryID: "c1", ActualPrice: 500_000,
	})

	var ve apperror.ValidationError
	if !errors.As(err, &ve) || ve.Field != "category_id" {
		t.Fatalf("mau ValidationError category_id, dapat %v", err)
	}
}

// --- affordability ---

func TestAffordability(t *testing.T) {
	item := Item{ID: "i1", EstimatedPrice: 1_200_000, SavedAmount: 400_000, Status: "saving"}

	t.Run("data historis kurang dari satu bulan penuh memberi nil", func(t *testing.T) {
		got := affordabilityFor(item, 0, 0)
		if got != nil {
			t.Fatalf("mau nil, dapat %+v", got)
		}
	})

	t.Run("kemampuan menabung nol memberi months_needed nil", func(t *testing.T) {
		got := affordabilityFor(item, 0, 3)
		if got == nil {
			t.Fatal("mau ada isinya")
		}
		if got.MonthsNeeded != nil {
			t.Errorf("months_needed harus nil, dapat %d", *got.MonthsNeeded)
		}
		if got.OnTrackForTargetDay {
			t.Errorf("on_track harus false kalau tidak bisa menabung")
		}
	})

	t.Run("kemampuan menabung minus juga memberi months_needed nil", func(t *testing.T) {
		got := affordabilityFor(item, -500_000, 3)
		if got.MonthsNeeded != nil {
			t.Errorf("months_needed harus nil, dapat %d", *got.MonthsNeeded)
		}
	})

	t.Run("months_needed dibulatkan ke atas", func(t *testing.T) {
		// sisa 800.000 dengan kemampuan 300.000 per bulan butuh 3 bulan
		got := affordabilityFor(item, 300_000, 3)
		if got.MonthsNeeded == nil || *got.MonthsNeeded != 3 {
			t.Fatalf("months_needed %v, mau 3", got.MonthsNeeded)
		}
	})

	t.Run("target_date terlalu cepat berarti tidak on track", func(t *testing.T) {
		withTarget := item
		withTarget.TargetDate = sql.NullTime{Time: timeutil.Now().AddDate(0, 1, 0), Valid: true}

		got := affordabilityFor(withTarget, 100_000, 3)
		if got.OnTrackForTargetDay {
			t.Errorf("butuh 8 bulan tapi targetnya 1 bulan lagi, harusnya tidak on track")
		}
	})

	t.Run("sisa nol berarti sudah siap sekarang", func(t *testing.T) {
		lunas := Item{EstimatedPrice: 1_000_000, SavedAmount: 1_000_000}

		got := affordabilityFor(lunas, 500_000, 3)
		if got.MonthsNeeded == nil || *got.MonthsNeeded != 0 {
			t.Fatalf("months_needed %v, mau 0", got.MonthsNeeded)
		}
	})
}
