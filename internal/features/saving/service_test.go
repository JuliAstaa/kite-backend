package saving

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

type FakeSavingRepository struct {
	Income    int
	Expense   int
	Target    Target
	HasTarget bool
	Buckets   []bucketSum
}

func (f *FakeSavingRepository) Totals(ctx context.Context, from, to time.Time) (int, int, error) {
	return f.Income, f.Expense, nil
}

func (f *FakeSavingRepository) BucketSums(ctx context.Context, from, to time.Time, granularity string) ([]bucketSum, error) {
	return f.Buckets, nil
}

func (f *FakeSavingRepository) ActiveTarget(ctx context.Context, period string, at time.Time) (Target, bool, error) {
	return f.Target, f.HasTarget, nil
}

func (f *FakeSavingRepository) CreateTarget(ctx context.Context, param CreateTargetParams) (Target, error) {
	return Target{Period: param.Period}, nil
}
func (f *FakeSavingRepository) GetAllTargets(ctx context.Context, includeDeleted bool) ([]Target, int, error) {
	return nil, 0, nil
}
func (f *FakeSavingRepository) GetTargetByID(ctx context.Context, id string) (Target, error) {
	return f.Target, nil
}
func (f *FakeSavingRepository) PatchTarget(ctx context.Context, id string, param PatchTargetParams) (Target, error) {
	return f.Target, nil
}
func (f *FakeSavingRepository) DeleteTarget(ctx context.Context, id string) (Target, error) {
	return f.Target, nil
}

// Business rule 10: savable boleh negatif dan ditampilkan apa adanya.
func TestSavableBolehNegatif(t *testing.T) {
	repo := &FakeSavingRepository{Income: 2_000_000, Expense: 3_500_000}
	service := NewSavingService(repo)

	from := timeutil.StartOfMonth(timeutil.Now().AddDate(0, -1, 0))
	to := timeutil.EndOfDay(timeutil.EndOfMonth(from))

	summary, err := service.Summary(context.Background(), "month", from, to, true)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if summary.Savable != -1_500_000 {
		t.Errorf("savable %d, mau -1500000 (tidak boleh di-clamp ke 0)", summary.Savable)
	}
}

func TestSavingsRateTanpaPemasukan(t *testing.T) {
	repo := &FakeSavingRepository{Income: 0, Expense: 500_000}
	service := NewSavingService(repo)

	from := timeutil.StartOfMonth(timeutil.Now().AddDate(0, -1, 0))
	to := timeutil.EndOfDay(timeutil.EndOfMonth(from))

	summary, err := service.Summary(context.Background(), "month", from, to, true)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if summary.SavingsRate != 0 {
		t.Errorf("savings_rate %v, mau 0 saat tidak ada pemasukan", summary.SavingsRate)
	}
}

// Periode yang sudah lewat: proyeksi sama dengan angka aktualnya.
func TestProjectedSavablePeriodeLampau(t *testing.T) {
	repo := &FakeSavingRepository{Income: 8_500_000, Expense: 4_230_000}
	service := NewSavingService(repo)

	from := timeutil.StartOfMonth(timeutil.Now().AddDate(0, -2, 0))
	to := timeutil.EndOfDay(timeutil.EndOfMonth(from))

	summary, err := service.Summary(context.Background(), "month", from, to, true)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if summary.ProjectedSavable != summary.Savable {
		t.Errorf("projected_savable %d, mau sama dengan savable %d", summary.ProjectedSavable, summary.Savable)
	}
}

func TestTargetProgress(t *testing.T) {
	t.Run("target nominal tercapai", func(t *testing.T) {
		target := Target{Amount: sql.NullInt64{Int64: 3_000_000, Valid: true}}

		got := buildTargetProgress(target, 8_500_000, 4_270_000)

		if !got.Achieved {
			t.Errorf("harusnya tercapai")
		}
		if got.Difference != 1_270_000 {
			t.Errorf("difference %d, mau 1270000", got.Difference)
		}
	})

	t.Run("target persen dihitung dari pemasukan", func(t *testing.T) {
		target := Target{TargetRate: sql.NullFloat64{Float64: 20, Valid: true}}

		got := buildTargetProgress(target, 10_000_000, 1_500_000)

		if got.Amount != 2_000_000 {
			t.Errorf("amount %d, mau 2000000 (20%% dari 10 juta)", got.Amount)
		}
		if got.Achieved {
			t.Errorf("1,5 juta belum mencapai target 2 juta")
		}
	})
}

// Business rule 13: isi salah satu antara amount atau target_rate.
func TestValidateAmountOrRate(t *testing.T) {
	amount := 1_000_000
	rate := 20.0
	zero := 0
	tooBig := 150.0

	tests := []struct {
		name    string
		amount  *int
		rate    *float64
		wantErr bool
	}{
		{"cuma amount", &amount, nil, false},
		{"cuma target_rate", nil, &rate, false},
		{"dua-duanya kosong", nil, nil, true},
		{"dua-duanya diisi", &amount, &rate, true},
		{"amount nol", &zero, nil, true},
		{"target_rate di atas 100", nil, &tooBig, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAmountOrRate(tt.amount, tt.rate)

			if tt.wantErr && err == nil {
				t.Fatal("mau error, dapat nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("mau sukses, dapat %v", err)
			}
			if tt.wantErr {
				var ve apperror.ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("mau ValidationError, dapat %T", err)
				}
			}
		})
	}
}

// Bucket kosong tetap dikembalikan dengan nilai 0.
func TestBreakdownMengisiBucketKosong(t *testing.T) {
	repo := &FakeSavingRepository{Buckets: nil}
	service := NewSavingService(repo)

	from := timeutil.StartOfMonth(timeutil.Now().AddDate(0, -2, 0))
	to := timeutil.EndOfDay(timeutil.EndOfMonth(timeutil.Now()))

	buckets, err := service.Breakdown(context.Background(), "month", from, to)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if len(buckets) != 3 {
		t.Fatalf("dapat %d bucket, mau 3", len(buckets))
	}
	for _, b := range buckets {
		if b.Income != 0 || b.Expense != 0 || b.Savable != 0 {
			t.Errorf("bucket %s harusnya nol semua, dapat %+v", b.Bucket, b)
		}
	}
}
