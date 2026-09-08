package recurring

import (
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, timeutil.Loc())
}

// FakeRecurringRepository menirukan unique index (recurring_rule_id, occurred_at):
// tanggal yang sudah pernah dibuat dihitung sebagai skip, bukan error.
type FakeRecurringRepository struct {
	Rules    []Rule
	Existing map[string]bool
	Calls    int
	NextRuns []time.Time
}

func (f *FakeRecurringRepository) CreateRule(ctx context.Context, param CreateRuleParams) (Rule, error) {
	return Rule{Name: param.Name, Frequency: param.Frequency, NextRunAt: param.NextRunAt}, nil
}
func (f *FakeRecurringRepository) GetAllRules(ctx context.Context, includeDeleted bool) ([]Rule, int, error) {
	return f.Rules, len(f.Rules), nil
}
func (f *FakeRecurringRepository) GetRuleByID(ctx context.Context, id string) (Rule, error) {
	return f.Rules[0], nil
}
func (f *FakeRecurringRepository) PatchRule(ctx context.Context, id string, param PatchRuleParams) (Rule, error) {
	return f.Rules[0], nil
}
func (f *FakeRecurringRepository) DeleteRule(ctx context.Context, id string) (Rule, error) {
	return f.Rules[0], nil
}
func (f *FakeRecurringRepository) ToggleRule(ctx context.Context, id string) (Rule, error) {
	return f.Rules[0], nil
}

func (f *FakeRecurringRepository) DueRules(ctx context.Context, today time.Time) ([]Rule, error) {
	due := []Rule{}
	for _, r := range f.Rules {
		if !r.NextRunAt.After(today) {
			due = append(due, r)
		}
	}
	return due, nil
}

func (f *FakeRecurringRepository) GenerateOccurrences(ctx context.Context, rule Rule, dates []time.Time, nextRunAt time.Time) (int, int, error) {
	f.Calls++
	f.NextRuns = append(f.NextRuns, nextRunAt)

	if f.Existing == nil {
		f.Existing = map[string]bool{}
	}

	created, skipped := 0, 0
	for _, d := range dates {
		key := rule.ID + "|" + d.Format(time.RFC3339)
		if f.Existing[key] {
			skipped++
			continue
		}
		f.Existing[key] = true
		created++
	}

	// menirukan UPDATE next_run_at di akhir transaksi
	for i := range f.Rules {
		if f.Rules[i].ID == rule.ID {
			f.Rules[i].NextRunAt = nextRunAt
		}
	}

	return created, skipped, nil
}

// Edge case wajib dari PRD: day_of_month 31 di bulan yang tidak punya
// tanggal 31 harus jatuh ke hari terakhir bulan tersebut, bukan meluber.
func TestNextOccurrenceMonthlyClampsToLastDay(t *testing.T) {
	rule := Rule{
		Frequency:  "monthly",
		Interval:   1,
		DayOfMonth: sql.NullInt64{Int64: 31, Valid: true},
	}

	tests := []struct {
		name    string
		current time.Time
		want    time.Time
	}{
		{"Januari ke Februari tahun biasa", date(2026, time.January, 31), date(2026, time.February, 28)},
		{"Januari ke Februari tahun kabisat", date(2028, time.January, 31), date(2028, time.February, 29)},
		{"Februari kembali ke Maret 31", date(2026, time.February, 28), date(2026, time.March, 31)},
		{"April yang cuma 30 hari", date(2026, time.March, 31), date(2026, time.April, 30)},
		{"Desember menyeberang tahun", date(2026, time.December, 31), date(2027, time.January, 31)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextOccurrence(rule, tt.current)
			if !got.Equal(tt.want) {
				t.Errorf("dapat %s, mau %s", got.Format("2006-01-02"), tt.want.Format("2006-01-02"))
			}
		})
	}
}

func TestNextOccurrenceFrequencies(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		current time.Time
		want    time.Time
	}{
		{"harian", Rule{Frequency: "daily", Interval: 1}, date(2026, time.August, 11), date(2026, time.August, 12)},
		{"tiap 3 hari", Rule{Frequency: "daily", Interval: 3}, date(2026, time.August, 11), date(2026, time.August, 14)},
		{"mingguan", Rule{Frequency: "weekly", Interval: 1}, date(2026, time.August, 11), date(2026, time.August, 18)},
		{"tiap 2 minggu", Rule{Frequency: "weekly", Interval: 2}, date(2026, time.August, 11), date(2026, time.August, 25)},
		{"tahunan", Rule{Frequency: "yearly", Interval: 1}, date(2026, time.August, 11), date(2027, time.August, 11)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextOccurrence(tt.rule, tt.current)
			if !got.Equal(tt.want) {
				t.Errorf("dapat %s, mau %s", got.Format("2006-01-02"), tt.want.Format("2006-01-02"))
			}
		})
	}
}

// Server yang sempat mati beberapa hari harus menyusul semua tanggal terlewat.
func TestOccurrencesUntilMenyusulTanggalTerlewat(t *testing.T) {
	today := date(2026, time.August, 11)
	rule := Rule{
		ID:        "r1",
		Frequency: "daily",
		Interval:  1,
		NextRunAt: date(2026, time.August, 8),
	}

	dates, nextRun := occurrencesUntil(rule, today)

	if len(dates) != 4 {
		t.Fatalf("dapat %d tanggal, mau 4 (8,9,10,11 Agustus)", len(dates))
	}
	if !nextRun.Equal(date(2026, time.August, 12)) {
		t.Errorf("next_run_at %s, mau 2026-08-12", nextRun.Format("2006-01-02"))
	}
}

func TestOccurrencesUntilBerhentiDiEndDate(t *testing.T) {
	today := date(2026, time.August, 11)
	rule := Rule{
		ID:        "r1",
		Frequency: "daily",
		Interval:  1,
		NextRunAt: date(2026, time.August, 8),
		EndDate:   sql.NullTime{Time: date(2026, time.August, 9), Valid: true},
	}

	dates, _ := occurrencesUntil(rule, today)

	if len(dates) != 2 {
		t.Fatalf("dapat %d tanggal, mau 2 (8 dan 9 Agustus)", len(dates))
	}
}

// Acceptance criteria: scheduler idempoten, dibuktikan dengan menjalankan
// siklus dua kali. Putaran kedua tidak boleh membuat transaksi baru.
func TestRunDueIdempotenSaatDijalankanDuaKali(t *testing.T) {
	rule := Rule{
		ID:         "r1",
		Name:       "Langganan streaming",
		Type:       "expense",
		Amount:     50000,
		WalletID:   "w1",
		CategoryID: "c1",
		Frequency:  "daily",
		Interval:   1,
		IsActive:   true,
		NextRunAt:  timeutil.StartOfDay(timeutil.Now()).AddDate(0, 0, -3),
	}

	repo := &FakeRecurringRepository{Rules: []Rule{rule}}
	service := NewRecurringService(repo, nil, nil, nil)

	first, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("putaran pertama error: %v", err)
	}
	if first.Created != 4 {
		t.Fatalf("putaran pertama membuat %d transaksi, mau 4", first.Created)
	}

	second, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("putaran kedua error: %v", err)
	}
	if second.Created != 0 {
		t.Errorf("putaran kedua membuat %d transaksi, mau 0 (harus idempoten)", second.Created)
	}
}

// Satu rule yang error tidak boleh menghentikan rule berikutnya.
func TestRunDueLanjutSaatSatuRuleGagal(t *testing.T) {
	yesterday := timeutil.StartOfDay(timeutil.Now()).AddDate(0, 0, -1)

	repo := &failingRepository{
		FakeRecurringRepository: FakeRecurringRepository{
			Rules: []Rule{
				{ID: "rusak", Frequency: "daily", Interval: 1, NextRunAt: yesterday},
				{ID: "sehat", Frequency: "daily", Interval: 1, NextRunAt: yesterday},
			},
		},
		failFor: "rusak",
	}

	service := NewRecurringService(repo, nil, nil, nil)

	result, err := service.RunDue(context.Background())
	if err != nil {
		t.Fatalf("RunDue tidak boleh gagal total: %v", err)
	}
	if result.RulesFailed != 1 {
		t.Errorf("rules_failed %d, mau 1", result.RulesFailed)
	}
	if result.Created == 0 {
		t.Errorf("rule yang sehat harus tetap menghasilkan transaksi")
	}
}

type failingRepository struct {
	FakeRecurringRepository
	failFor string
}

func (f *failingRepository) GenerateOccurrences(ctx context.Context, rule Rule, dates []time.Time, nextRunAt time.Time) (int, int, error) {
	if rule.ID == f.failFor {
		return 0, 0, sql.ErrConnDone
	}
	return f.FakeRecurringRepository.GenerateOccurrences(ctx, rule, dates, nextRunAt)
}
