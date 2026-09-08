package recurring

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"log/slog"
	"strings"
	"time"
)

// maxOccurrencesPerRun membatasi berapa transaksi yang boleh dibuat satu rule
// dalam satu putaran. Ini pengaman kalau start_date-nya kejauhan di masa lalu,
// bukan aturan bisnis.
const maxOccurrencesPerRun = 1000

type WalletsReader interface {
	IsWalletExist(ctx context.Context, id string) error
}

type CategoriesReader interface {
	GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error)
}

type RecurringServicer interface {
	CreateRule(ctx context.Context, req CreateRuleRequest) (Rule, error)
	GetAllRules(ctx context.Context, includeDeleted bool) ([]Rule, int, error)
	GetRuleByID(ctx context.Context, id string) (Rule, error)
	PatchRule(ctx context.Context, id string, req PatchRuleRequest) (Rule, error)
	DeleteRule(ctx context.Context, id string) (Rule, error)
	ToggleRule(ctx context.Context, id string) (Rule, error)
	RunDue(ctx context.Context) (RunResult, error)
}

type RecurringService struct {
	repo     RecurringRepositorer
	wallet   WalletsReader
	category CategoriesReader
	logger   *slog.Logger
}

func NewRecurringService(repo RecurringRepositorer, wallet WalletsReader, category CategoriesReader, logger *slog.Logger) *RecurringService {
	if logger == nil {
		logger = slog.Default()
	}
	return &RecurringService{repo: repo, wallet: wallet, category: category, logger: logger}
}

func (s *RecurringService) CreateRule(ctx context.Context, req CreateRuleRequest) (Rule, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return Rule{}, apperror.ValidationError{Field: "name", Message: "name tidak boleh kosong"}
	}

	ruleType := strings.ToLower(strings.TrimSpace(req.Type))
	if ruleType != "income" && ruleType != "expense" {
		return Rule{}, apperror.ValidationError{Field: "type", Message: "type harus income atau expense"}
	}

	if req.Amount <= 0 {
		return Rule{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
	}

	frequency := strings.ToLower(strings.TrimSpace(req.Frequency))
	if !isAllowedFrequency(frequency) {
		return Rule{}, apperror.ValidationError{Field: "frequency", Message: "frequency harus daily, weekly, monthly, atau yearly"}
	}

	interval := req.Interval
	if interval == 0 {
		interval = 1
	}
	if interval < 1 {
		return Rule{}, apperror.ValidationError{Field: "interval", Message: "interval harus minimal 1"}
	}

	if err := validateDayFields(frequency, req.DayOfMonth, req.DayOfWeek); err != nil {
		return Rule{}, err
	}

	if err := s.wallet.IsWalletExist(ctx, req.WalletID); err != nil {
		return Rule{}, err
	}

	cat, err := s.category.GetCategoryInfo(ctx, req.CategoryID)
	if err != nil {
		return Rule{}, err
	}
	if cat.Type != ruleType {
		return Rule{}, apperror.ValidationError{
			Field:   "category_id",
			Message: "tipe kategori (" + cat.Type + ") tidak cocok dengan tipe rule (" + ruleType + ")",
		}
	}

	if req.StartDate == nil {
		return Rule{}, apperror.ValidationError{Field: "start_date", Message: "start_date wajib diisi"}
	}
	startDate := timeutil.StartOfDay(*req.StartDate)

	if req.EndDate != nil && req.EndDate.Before(startDate) {
		return Rule{}, apperror.ValidationError{Field: "end_date", Message: "end_date tidak boleh lebih awal dari start_date"}
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	// Jalan pertama diselaraskan dulu dengan day_of_month atau day_of_week
	// kalau diisi, supaya tanggalnya konsisten sejak awal.
	nextRunAt := alignFirstRun(startDate, frequency, req.DayOfMonth, req.DayOfWeek)

	return s.repo.CreateRule(ctx, CreateRuleParams{
		Name:       name,
		Type:       ruleType,
		Amount:     req.Amount,
		WalletID:   req.WalletID,
		CategoryID: req.CategoryID,
		Note:       strings.TrimSpace(req.Note),
		Frequency:  frequency,
		Interval:   interval,
		DayOfMonth: req.DayOfMonth,
		DayOfWeek:  req.DayOfWeek,
		StartDate:  startDate,
		EndDate:    req.EndDate,
		NextRunAt:  nextRunAt,
		IsActive:   isActive,
	})
}

func (s *RecurringService) GetAllRules(ctx context.Context, includeDeleted bool) ([]Rule, int, error) {
	return s.repo.GetAllRules(ctx, includeDeleted)
}

func (s *RecurringService) GetRuleByID(ctx context.Context, id string) (Rule, error) {
	return s.repo.GetRuleByID(ctx, id)
}

func (s *RecurringService) PatchRule(ctx context.Context, id string, req PatchRuleRequest) (Rule, error) {
	existing, err := s.repo.GetRuleByID(ctx, id)
	if err != nil {
		return Rule{}, err
	}

	param := PatchRuleParams{
		Note:         req.Note,
		IsActive:     req.IsActive,
		ClearEndDate: req.ClearEndDate,
		EndDate:      req.EndDate,
		DayOfMonth:   req.DayOfMonth,
		DayOfWeek:    req.DayOfWeek,
		NextRunAt:    req.NextRunAt,
	}

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return Rule{}, apperror.ValidationError{Field: "name", Message: "name tidak boleh kosong"}
		}
		param.Name = &name
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return Rule{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
		}
		param.Amount = req.Amount
	}

	frequency := existing.Frequency
	if req.Frequency != nil {
		frequency = strings.ToLower(strings.TrimSpace(*req.Frequency))
		if !isAllowedFrequency(frequency) {
			return Rule{}, apperror.ValidationError{Field: "frequency", Message: "frequency harus daily, weekly, monthly, atau yearly"}
		}
		param.Frequency = &frequency
	}

	if req.Interval != nil {
		if *req.Interval < 1 {
			return Rule{}, apperror.ValidationError{Field: "interval", Message: "interval harus minimal 1"}
		}
		param.Interval = req.Interval
	}

	if err := validateDayFields(frequency, req.DayOfMonth, req.DayOfWeek); err != nil {
		return Rule{}, err
	}

	if req.WalletID != nil {
		if err := s.wallet.IsWalletExist(ctx, *req.WalletID); err != nil {
			return Rule{}, err
		}
		param.WalletID = req.WalletID
	}

	if req.CategoryID != nil {
		cat, err := s.category.GetCategoryInfo(ctx, *req.CategoryID)
		if err != nil {
			return Rule{}, err
		}
		if cat.Type != existing.Type {
			return Rule{}, apperror.ValidationError{
				Field:   "category_id",
				Message: "tipe kategori tidak cocok dengan tipe rule",
			}
		}
		param.CategoryID = req.CategoryID
	}

	if req.StartDate != nil {
		startDate := timeutil.StartOfDay(*req.StartDate)
		param.StartDate = &startDate
	}

	return s.repo.PatchRule(ctx, id, param)
}

func (s *RecurringService) DeleteRule(ctx context.Context, id string) (Rule, error) {
	return s.repo.DeleteRule(ctx, id)
}

func (s *RecurringService) ToggleRule(ctx context.Context, id string) (Rule, error) {
	return s.repo.ToggleRule(ctx, id)
}

// RunDue membuat transaksi untuk semua rule yang sudah jatuh tempo.
//
// Kalau satu rule error, rule berikutnya tetap diproses. Satu rule bermasalah
// tidak boleh menghentikan sisanya.
func (s *RecurringService) RunDue(ctx context.Context) (RunResult, error) {
	today := timeutil.StartOfDay(timeutil.Now())

	rules, err := s.repo.DueRules(ctx, today)
	if err != nil {
		return RunResult{}, err
	}

	result := RunResult{RulesChecked: len(rules)}

	for _, rule := range rules {
		dates, nextRunAt := occurrencesUntil(rule, today)

		created, skipped, err := s.repo.GenerateOccurrences(ctx, rule, dates, nextRunAt)
		if err != nil {
			s.logger.Error("recurring rule gagal", "rule_id", rule.ID, "rule_name", rule.Name, "error", err)
			result.RulesFailed++
			result.FailedRuleIDs = append(result.FailedRuleIDs, rule.ID)
			continue
		}

		result.Created += created
		result.Skipped += skipped
	}

	return result, nil
}

// occurrencesUntil mengumpulkan semua tanggal yang seharusnya sudah tergenerate
// sampai hari ini, lalu memberi tanggal jalan berikutnya.
//
// Loop ini yang membuat server yang sempat mati beberapa hari tetap menyusul
// transaksi yang terlewat.
func occurrencesUntil(rule Rule, today time.Time) ([]time.Time, time.Time) {
	dates := []time.Time{}
	cursor := timeutil.StartOfDay(rule.NextRunAt)

	for i := 0; i < maxOccurrencesPerRun; i++ {
		if cursor.After(today) {
			break
		}
		if rule.EndDate.Valid && cursor.After(timeutil.StartOfDay(rule.EndDate.Time)) {
			break
		}
		dates = append(dates, cursor)
		cursor = nextOccurrence(rule, cursor)
	}

	return dates, cursor
}

// nextOccurrence memajukan tanggal sesuai frequency dan interval.
func nextOccurrence(rule Rule, current time.Time) time.Time {
	interval := rule.Interval
	if interval < 1 {
		interval = 1
	}

	switch rule.Frequency {
	case "daily":
		return current.AddDate(0, 0, interval)

	case "weekly":
		return current.AddDate(0, 0, 7*interval)

	case "monthly":
		// day_of_month = 31 di Februari dipotong ke hari terakhir bulan itu.
		// Perhitungan bulan dimulai dari tanggal 1, karena AddDate pada
		// tanggal 31 bisa meluber ke bulan berikutnya (31 Jan + 1 bulan = 3 Mar).
		day := current.Day()
		if rule.DayOfMonth.Valid {
			day = int(rule.DayOfMonth.Int64)
		}
		base := time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, timeutil.Loc()).AddDate(0, interval, 0)
		return timeutil.ClampDayOfMonth(base.Year(), base.Month(), day)

	case "yearly":
		base := time.Date(current.Year()+interval, current.Month(), 1, 0, 0, 0, 0, timeutil.Loc())
		return timeutil.ClampDayOfMonth(base.Year(), base.Month(), current.Day())

	default:
		return current.AddDate(0, 0, interval)
	}
}

// alignFirstRun menggeser tanggal jalan pertama ke hari yang diminta rule.
func alignFirstRun(startDate time.Time, frequency string, dayOfMonth, dayOfWeek *int) time.Time {
	switch frequency {
	case "monthly":
		if dayOfMonth == nil {
			return startDate
		}
		candidate := timeutil.ClampDayOfMonth(startDate.Year(), startDate.Month(), *dayOfMonth)
		if candidate.Before(startDate) {
			base := time.Date(startDate.Year(), startDate.Month(), 1, 0, 0, 0, 0, timeutil.Loc()).AddDate(0, 1, 0)
			candidate = timeutil.ClampDayOfMonth(base.Year(), base.Month(), *dayOfMonth)
		}
		return candidate

	case "weekly":
		if dayOfWeek == nil {
			return startDate
		}
		offset := (*dayOfWeek - int(startDate.Weekday()) + 7) % 7
		return startDate.AddDate(0, 0, offset)

	default:
		return startDate
	}
}

func isAllowedFrequency(frequency string) bool {
	switch frequency {
	case "daily", "weekly", "monthly", "yearly":
		return true
	default:
		return false
	}
}

func validateDayFields(frequency string, dayOfMonth, dayOfWeek *int) error {
	if dayOfMonth != nil && (*dayOfMonth < 1 || *dayOfMonth > 31) {
		return apperror.ValidationError{Field: "day_of_month", Message: "day_of_month harus di antara 1 dan 31"}
	}
	if dayOfWeek != nil && (*dayOfWeek < 0 || *dayOfWeek > 6) {
		return apperror.ValidationError{Field: "day_of_week", Message: "day_of_week harus di antara 0 (Minggu) dan 6 (Sabtu)"}
	}
	if frequency == "monthly" && dayOfWeek != nil {
		return apperror.ValidationError{Field: "day_of_week", Message: "day_of_week tidak dipakai untuk frequency monthly"}
	}
	if frequency == "weekly" && dayOfMonth != nil {
		return apperror.ValidationError{Field: "day_of_month", Message: "day_of_month tidak dipakai untuk frequency weekly"}
	}
	return nil
}
