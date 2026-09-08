package saving

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"math"
	"time"
)

type SavingServicer interface {
	Summary(ctx context.Context, period string, from, to time.Time, explicitRange bool) (Summary, error)
	Breakdown(ctx context.Context, period string, from, to time.Time) ([]BreakdownBucket, error)
	AverageMonthlySavable(ctx context.Context, months int) (avg int, sampleMonths int, err error)

	CreateTarget(ctx context.Context, req CreateTargetRequest) (Target, error)
	GetAllTargets(ctx context.Context, includeDeleted bool) ([]Target, int, error)
	PatchTarget(ctx context.Context, id string, req PatchTargetRequest) (Target, error)
	DeleteTarget(ctx context.Context, id string) (Target, error)
}

type SavingService struct {
	repo SavingRepositorer
}

func NewSavingService(repo SavingRepositorer) *SavingService {
	return &SavingService{repo: repo}
}

// Summary menjawab pertanyaan "berapa uang yang bisa aku tabung".
//
// explicitRange menandai apakah client mengirim from/to sendiri. Kalau iya,
// proyeksi tidak dihitung dari sisa hari, karena periodenya bukan periode
// berjalan lagi.
func (s *SavingService) Summary(ctx context.Context, period string, from, to time.Time, explicitRange bool) (Summary, error) {
	income, expense, err := s.repo.Totals(ctx, from, to)
	if err != nil {
		return Summary{}, err
	}

	// savable boleh negatif dan ditampilkan apa adanya (business rule 10)
	savable := income - expense

	daysTotal := timeutil.DaysBetween(from, to)
	daysElapsed := daysTotal

	now := timeutil.Now()
	isOngoing := !explicitRange && !now.After(to) && !now.Before(from)
	if isOngoing {
		daysElapsed = timeutil.DaysBetween(from, now)
	}
	if daysElapsed < 1 {
		daysElapsed = 1
	}

	dailyAverageExpense := expense / daysElapsed

	// Untuk periode yang sudah lewat, proyeksi sama dengan angka aktualnya.
	projectedSavable := savable
	if isOngoing {
		projectedSavable = income - dailyAverageExpense*daysTotal
	}

	summary := Summary{
		Period:              period,
		From:                from,
		To:                  to,
		Income:              income,
		Expense:             expense,
		Savable:             savable,
		SavingsRate:         savingsRate(income, savable),
		DailyAverageExpense: dailyAverageExpense,
		ProjectedSavable:    projectedSavable,
		DaysElapsed:         daysElapsed,
		DaysTotal:           daysTotal,
	}

	target, found, err := s.repo.ActiveTarget(ctx, normalizePeriod(period), from)
	if err != nil {
		return Summary{}, err
	}
	if found {
		summary.Target = buildTargetProgress(target, income, savable)
	}

	return summary, nil
}

func (s *SavingService) Breakdown(ctx context.Context, period string, from, to time.Time) ([]BreakdownBucket, error) {
	granularity := "month"
	if period == "week" || period == "weekly" {
		granularity = "week"
	}

	sums, err := s.repo.BucketSums(ctx, from, to, granularity)
	if err != nil {
		return nil, err
	}

	byLabel := map[string]bucketSum{}
	for _, sum := range sums {
		byLabel[timeutil.BucketLabel(sum.Start, granularity)] = sum
	}

	// Bucket kosong tetap dikembalikan dengan nilai 0 supaya grafik tidak bolong.
	buckets := timeutil.Buckets(from, to, granularity)
	result := make([]BreakdownBucket, 0, len(buckets))
	for _, b := range buckets {
		sum := byLabel[b.Label]
		savable := sum.Income - sum.Expense
		result = append(result, BreakdownBucket{
			Bucket:      b.Label,
			Start:       b.Start,
			End:         b.End,
			Income:      sum.Income,
			Expense:     sum.Expense,
			Savable:     savable,
			SavingsRate: savingsRate(sum.Income, savable),
		})
	}

	return result, nil
}

// AverageMonthlySavable memberi rata-rata savable dari beberapa bulan terakhir
// yang sudah lengkap. Bulan berjalan sengaja tidak ikut, karena datanya belum
// penuh dan akan menarik rata-rata ke bawah.
func (s *SavingService) AverageMonthlySavable(ctx context.Context, months int) (int, int, error) {
	if months < 1 {
		months = 3
	}

	now := timeutil.Now()
	lastCompleteMonth := timeutil.StartOfMonth(now).AddDate(0, -1, 0)
	from := lastCompleteMonth.AddDate(0, -(months - 1), 0)
	to := timeutil.EndOfDay(timeutil.EndOfMonth(lastCompleteMonth))

	if to.Before(from) {
		return 0, 0, nil
	}

	sums, err := s.repo.BucketSums(ctx, from, to, "month")
	if err != nil {
		return 0, 0, err
	}

	byLabel := map[string]bucketSum{}
	for _, sum := range sums {
		byLabel[timeutil.MonthLabel(sum.Start)] = sum
	}

	total := 0
	sample := 0
	for cursor := from; !cursor.After(lastCompleteMonth); cursor = cursor.AddDate(0, 1, 0) {
		sum := byLabel[timeutil.MonthLabel(cursor)]
		total += sum.Income - sum.Expense
		sample++
	}

	if sample == 0 {
		return 0, 0, nil
	}
	return total / sample, sample, nil
}

func (s *SavingService) CreateTarget(ctx context.Context, req CreateTargetRequest) (Target, error) {
	period := normalizePeriod(req.Period)
	if period != "weekly" && period != "monthly" {
		return Target{}, apperror.ValidationError{Field: "period", Message: "period harus weekly atau monthly"}
	}

	if err := validateAmountOrRate(req.Amount, req.TargetRate); err != nil {
		return Target{}, err
	}

	if req.StartDate == nil {
		return Target{}, apperror.ValidationError{Field: "start_date", Message: "start_date wajib diisi"}
	}
	if req.EndDate != nil && req.EndDate.Before(*req.StartDate) {
		return Target{}, apperror.ValidationError{Field: "end_date", Message: "end_date tidak boleh lebih awal dari start_date"}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	return s.repo.CreateTarget(ctx, CreateTargetParams{
		Period:     period,
		Amount:     req.Amount,
		TargetRate: req.TargetRate,
		StartDate:  *req.StartDate,
		EndDate:    req.EndDate,
		IsActive:   isActive,
	})
}

func (s *SavingService) GetAllTargets(ctx context.Context, includeDeleted bool) ([]Target, int, error) {
	return s.repo.GetAllTargets(ctx, includeDeleted)
}

// PatchTarget menggabungkan dulu dengan data lama, karena aturan
// "isi salah satu antara amount atau target_rate" harus dicek pada keadaan
// akhir, bukan pada field yang dikirim saja.
func (s *SavingService) PatchTarget(ctx context.Context, id string, req PatchTargetRequest) (Target, error) {
	existing, err := s.repo.GetTargetByID(ctx, id)
	if err != nil {
		return Target{}, err
	}

	amount := req.Amount
	rate := req.TargetRate

	switch {
	case req.Amount != nil:
		// ganti ke target nominal, persennya dibuang
		rate = nil
	case req.TargetRate != nil:
		// ganti ke target persen, nominalnya dibuang
		amount = nil
	default:
		if existing.Amount.Valid {
			value := int(existing.Amount.Int64)
			amount = &value
		}
		if existing.TargetRate.Valid {
			value := existing.TargetRate.Float64
			rate = &value
		}
	}

	if err := validateAmountOrRate(amount, rate); err != nil {
		return Target{}, err
	}

	if req.Period != nil {
		period := normalizePeriod(*req.Period)
		if period != "weekly" && period != "monthly" {
			return Target{}, apperror.ValidationError{Field: "period", Message: "period harus weekly atau monthly"}
		}
		req.Period = &period
	}

	startDate := existing.StartDate
	if req.StartDate != nil {
		startDate = *req.StartDate
	}
	if req.EndDate != nil && req.EndDate.Before(startDate) {
		return Target{}, apperror.ValidationError{Field: "end_date", Message: "end_date tidak boleh lebih awal dari start_date"}
	}

	return s.repo.PatchTarget(ctx, id, PatchTargetParams{
		Period:       req.Period,
		Amount:       amount,
		TargetRate:   rate,
		StartDate:    req.StartDate,
		EndDate:      req.EndDate,
		IsActive:     req.IsActive,
		ClearEndDate: req.ClearEndDate,
	})
}

func (s *SavingService) DeleteTarget(ctx context.Context, id string) (Target, error) {
	return s.repo.DeleteTarget(ctx, id)
}

// validateAmountOrRate menegakkan business rule 13.
func validateAmountOrRate(amount *int, rate *float64) error {
	if amount == nil && rate == nil {
		return apperror.ValidationError{Field: "amount", Message: "isi salah satu antara amount atau target_rate"}
	}
	if amount != nil && rate != nil {
		return apperror.ValidationError{Field: "amount", Message: "amount dan target_rate tidak boleh diisi dua-duanya"}
	}
	if amount != nil && *amount <= 0 {
		return apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
	}
	if rate != nil && (*rate <= 0 || *rate > 100) {
		return apperror.ValidationError{Field: "target_rate", Message: "target_rate harus di antara 0 dan 100"}
	}
	return nil
}

// buildTargetProgress mengubah target jadi angka nominal, lalu membandingkannya
// dengan tabungan nyata. Target berbasis persen dihitung dari pemasukan periode
// tersebut.
func buildTargetProgress(target Target, income, savable int) *TargetProgress {
	amount := 0
	switch {
	case target.Amount.Valid:
		amount = int(target.Amount.Int64)
	case target.TargetRate.Valid:
		amount = int(math.Round(float64(income) * target.TargetRate.Float64 / 100))
	}

	progress := TargetProgress{
		Amount:     amount,
		Achieved:   savable >= amount,
		Difference: savable - amount,
	}
	if amount > 0 {
		progress.ProgressPct = round2(float64(savable) / float64(amount) * 100)
	}

	return &progress
}

// savingsRate dibiarkan 0 kalau tidak ada pemasukan, karena persentase dari
// nol tidak punya arti.
func savingsRate(income, savable int) float64 {
	if income <= 0 {
		return 0
	}
	return round2(float64(savable) / float64(income) * 100)
}

func normalizePeriod(period string) string {
	switch period {
	case "week", "weekly":
		return "weekly"
	default:
		return "monthly"
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
