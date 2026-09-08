package budget

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"errors"
	"testing"
	"time"
)

type FakeBudgetRepository struct {
	Budgets []Budget
	Spent   int
	Created CreateBudgetParams
}

func (f *FakeBudgetRepository) CreateBudget(ctx context.Context, param CreateBudgetParams) (Budget, error) {
	f.Created = param
	return Budget{CategoryID: param.CategoryID, Amount: param.Amount, Period: param.Period, StartMonth: param.StartMonth}, nil
}
func (f *FakeBudgetRepository) GetAllBudgets(ctx context.Context, includeDeleted bool) ([]Budget, int, error) {
	return f.Budgets, len(f.Budgets), nil
}
func (f *FakeBudgetRepository) GetBudgetByID(ctx context.Context, id string) (Budget, error) {
	return f.Budgets[0], nil
}
func (f *FakeBudgetRepository) PatchBudget(ctx context.Context, id string, param PatchBudgetParams) (Budget, error) {
	return f.Budgets[0], nil
}
func (f *FakeBudgetRepository) DeleteBudget(ctx context.Context, id string) (Budget, error) {
	return f.Budgets[0], nil
}
func (f *FakeBudgetRepository) ActiveBudgets(ctx context.Context, monthStart time.Time) ([]Budget, error) {
	return f.Budgets, nil
}
func (f *FakeBudgetRepository) SpentOnCategory(ctx context.Context, categoryID string, from, to time.Time) (int, error) {
	return f.Spent, nil
}

type FakeCategoryReader struct{ Type string }

func (f *FakeCategoryReader) GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error) {
	catType := f.Type
	if catType == "" {
		catType = "expense"
	}
	return CategoryInfo{ID: id, Type: catType}, nil
}

// Budget hanya masuk akal untuk kategori pengeluaran.
func TestCreateBudgetMenolakKategoriIncome(t *testing.T) {
	repo := &FakeBudgetRepository{}
	service := NewBudgetService(repo, &FakeCategoryReader{Type: "income"})

	start := timeutil.Now()
	_, err := service.CreateBudget(context.Background(), CreateBudgetRequest{
		CategoryID: "c1", Amount: 2_000_000, Period: "monthly", StartMonth: &start,
	})

	var ve apperror.ValidationError
	if !errors.As(err, &ve) || ve.Field != "category_id" {
		t.Fatalf("mau ValidationError category_id, dapat %v", err)
	}
}

func TestCreateBudgetMenormalkanStartMonthKeTanggalSatu(t *testing.T) {
	repo := &FakeBudgetRepository{}
	service := NewBudgetService(repo, &FakeCategoryReader{})

	start := time.Date(2026, time.August, 17, 15, 0, 0, 0, timeutil.Loc())
	_, err := service.CreateBudget(context.Background(), CreateBudgetRequest{
		CategoryID: "c1", Amount: 2_000_000, StartMonth: &start,
	})
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if repo.Created.StartMonth.Day() != 1 {
		t.Errorf("start_month tanggal %d, mau 1", repo.Created.StartMonth.Day())
	}
}

func TestStateFor(t *testing.T) {
	tests := []struct {
		percentage float64
		want       string
	}{
		{0, "safe"},
		{74.9, "safe"},
		{75, "warning"},
		{92.5, "warning"},
		{100, "warning"},
		{100.1, "exceeded"},
		{250, "exceeded"},
	}

	for _, tt := range tests {
		if got := stateFor(tt.percentage); got != tt.want {
			t.Errorf("stateFor(%v) = %q, mau %q", tt.percentage, got, tt.want)
		}
	}
}

func TestStatusForMonth(t *testing.T) {
	monthStart := timeutil.StartOfMonth(timeutil.Now())

	repo := &FakeBudgetRepository{
		Budgets: []Budget{{
			ID: "b1", CategoryID: "c1", CategoryName: "Makan & Minum",
			Amount: 2_000_000, Period: "monthly", StartMonth: monthStart,
		}},
		Spent: 1_850_000,
	}
	service := NewBudgetService(repo, &FakeCategoryReader{})

	statuses, err := service.StatusForMonth(context.Background(), timeutil.Now())
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("dapat %d status, mau 1", len(statuses))
	}

	got := statuses[0]
	if got.Remaining != 150_000 {
		t.Errorf("remaining %d, mau 150000", got.Remaining)
	}
	if got.Percentage != 92.5 {
		t.Errorf("percentage %v, mau 92.5", got.Percentage)
	}
	if got.State != "warning" {
		t.Errorf("status %q, mau warning", got.State)
	}
}

// Budget yang kelewat batas memberi remaining negatif, bukan nol.
func TestStatusMelebihiBatas(t *testing.T) {
	monthStart := timeutil.StartOfMonth(timeutil.Now())

	repo := &FakeBudgetRepository{
		Budgets: []Budget{{ID: "b1", CategoryID: "c1", Amount: 1_000_000, Period: "monthly", StartMonth: monthStart}},
		Spent:   1_400_000,
	}
	service := NewBudgetService(repo, &FakeCategoryReader{})

	statuses, err := service.StatusForMonth(context.Background(), timeutil.Now())
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if statuses[0].Remaining != -400_000 {
		t.Errorf("remaining %d, mau -400000", statuses[0].Remaining)
	}
	if statuses[0].State != "exceeded" {
		t.Errorf("status %q, mau exceeded", statuses[0].State)
	}
}
