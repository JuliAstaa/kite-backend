package budget

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"math"
	"strings"
	"time"
)

// CategoriesReader dideklarasikan di sini, di sisi yang memakai.
type CategoriesReader interface {
	GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error)
}

type BudgetServicer interface {
	CreateBudget(ctx context.Context, req CreateBudgetRequest) (Budget, error)
	GetAllBudgets(ctx context.Context, includeDeleted bool) ([]Budget, int, error)
	PatchBudget(ctx context.Context, id string, req PatchBudgetRequest) (Budget, error)
	DeleteBudget(ctx context.Context, id string) (Budget, error)
	StatusForMonth(ctx context.Context, month time.Time) ([]Status, error)
}

type BudgetService struct {
	repo     BudgetRepositorer
	category CategoriesReader
}

func NewBudgetService(repo BudgetRepositorer, category CategoriesReader) *BudgetService {
	return &BudgetService{repo: repo, category: category}
}

func (s *BudgetService) CreateBudget(ctx context.Context, req CreateBudgetRequest) (Budget, error) {
	if req.Amount <= 0 {
		return Budget{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
	}

	period := strings.ToLower(strings.TrimSpace(req.Period))
	if period == "" {
		period = "monthly"
	}
	if period != "weekly" && period != "monthly" {
		return Budget{}, apperror.ValidationError{Field: "period", Message: "period harus weekly atau monthly"}
	}

	// Budget cuma masuk akal untuk kategori pengeluaran.
	cat, err := s.category.GetCategoryInfo(ctx, req.CategoryID)
	if err != nil {
		return Budget{}, err
	}
	if cat.Type != "expense" {
		return Budget{}, apperror.ValidationError{Field: "category_id", Message: "budget hanya bisa dibuat untuk kategori bertipe expense"}
	}

	if req.StartMonth == nil {
		return Budget{}, apperror.ValidationError{Field: "start_month", Message: "start_month wajib diisi"}
	}

	// start_month dan end_month selalu disimpan sebagai tanggal 1.
	startMonth := timeutil.StartOfMonth(*req.StartMonth)
	var endMonth *time.Time
	if req.EndMonth != nil {
		normalized := timeutil.StartOfMonth(*req.EndMonth)
		if normalized.Before(startMonth) {
			return Budget{}, apperror.ValidationError{Field: "end_month", Message: "end_month tidak boleh lebih awal dari start_month"}
		}
		endMonth = &normalized
	}

	return s.repo.CreateBudget(ctx, CreateBudgetParams{
		CategoryID: req.CategoryID,
		Amount:     req.Amount,
		Period:     period,
		StartMonth: startMonth,
		EndMonth:   endMonth,
	})
}

func (s *BudgetService) GetAllBudgets(ctx context.Context, includeDeleted bool) ([]Budget, int, error) {
	return s.repo.GetAllBudgets(ctx, includeDeleted)
}

func (s *BudgetService) PatchBudget(ctx context.Context, id string, req PatchBudgetRequest) (Budget, error) {
	existing, err := s.repo.GetBudgetByID(ctx, id)
	if err != nil {
		return Budget{}, err
	}

	param := PatchBudgetParams{ClearEndMonth: req.ClearEndMonth}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return Budget{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
		}
		param.Amount = req.Amount
	}

	if req.Period != nil {
		period := strings.ToLower(strings.TrimSpace(*req.Period))
		if period != "weekly" && period != "monthly" {
			return Budget{}, apperror.ValidationError{Field: "period", Message: "period harus weekly atau monthly"}
		}
		param.Period = &period
	}

	startMonth := existing.StartMonth
	if req.StartMonth != nil {
		startMonth = timeutil.StartOfMonth(*req.StartMonth)
		param.StartMonth = &startMonth
	}

	if req.EndMonth != nil {
		endMonth := timeutil.StartOfMonth(*req.EndMonth)
		if endMonth.Before(startMonth) {
			return Budget{}, apperror.ValidationError{Field: "end_month", Message: "end_month tidak boleh lebih awal dari start_month"}
		}
		param.EndMonth = &endMonth
	}

	return s.repo.PatchBudget(ctx, id, param)
}

func (s *BudgetService) DeleteBudget(ctx context.Context, id string) (Budget, error) {
	return s.repo.DeleteBudget(ctx, id)
}

// StatusForMonth menghitung progres tiap budget yang berlaku pada bulan itu.
//
// Budget bulanan dinilai untuk satu bulan penuh. Budget mingguan dinilai untuk
// satu minggu: minggu berjalan kalau bulan yang diminta adalah bulan sekarang,
// kalau bulan lampau dipakai minggu terakhir bulan tersebut.
func (s *BudgetService) StatusForMonth(ctx context.Context, month time.Time) ([]Status, error) {
	monthStart := timeutil.StartOfMonth(month)
	monthEnd := timeutil.EndOfMonth(monthStart)

	budgets, err := s.repo.ActiveBudgets(ctx, monthStart)
	if err != nil {
		return nil, err
	}

	now := timeutil.Now()
	isCurrentMonth := timeutil.StartOfMonth(now).Equal(monthStart)

	result := make([]Status, 0, len(budgets))
	for _, b := range budgets {
		from, to := monthStart, monthEnd

		if b.Period == "weekly" {
			reference := monthEnd
			if isCurrentMonth {
				reference = now
			}
			from = timeutil.StartOfWeek(reference)
			to = timeutil.EndOfWeek(reference)
		}

		spent, err := s.repo.SpentOnCategory(ctx, b.CategoryID, from, timeutil.EndOfDay(to))
		if err != nil {
			return nil, err
		}

		status := Status{
			BudgetID:     b.ID,
			CategoryID:   b.CategoryID,
			CategoryName: b.CategoryName,
			Limit:        b.Amount,
			Spent:        spent,
			Remaining:    b.Amount - spent,
			DaysLeft:     daysLeft(now, to),
			From:         from,
			To:           to,
			Period:       b.Period,
		}

		if b.Amount > 0 {
			status.Percentage = math.Round(float64(spent)/float64(b.Amount)*10000) / 100
		}
		status.State = stateFor(status.Percentage)

		result = append(result, status)
	}

	return result, nil
}

// stateFor: safe di bawah 75%, warning 75 sampai 100%, exceeded di atas 100%.
func stateFor(percentage float64) string {
	switch {
	case percentage > 100:
		return "exceeded"
	case percentage >= 75:
		return "warning"
	default:
		return "safe"
	}
}

// daysLeft menghitung sisa hari sampai akhir periode. Periode yang sudah lewat
// memberi 0, bukan angka negatif.
func daysLeft(now, to time.Time) int {
	if now.After(to) {
		return 0
	}
	return timeutil.DaysBetween(now, to)
}
