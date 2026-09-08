package analytics

import (
	"backend/internal/shared/timeutil"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type FakeAnalyticsService struct {
	SummaryResult    Summary
	CategoriesResult []CategoryBreakdown
	WalletsResult    []WalletBreakdown
	TrendResult      []TrendBucket
	Err              error

	CapturedFrom        time.Time
	CapturedTo          time.Time
	CapturedType        string
	CapturedGranularity string
}

func (f *FakeAnalyticsService) Summary(ctx context.Context, from, to time.Time) (Summary, error) {
	f.CapturedFrom, f.CapturedTo = from, to
	if f.Err != nil {
		return Summary{}, f.Err
	}
	result := f.SummaryResult
	result.From, result.To = from, to
	return result, nil
}

func (f *FakeAnalyticsService) ByCategory(ctx context.Context, from, to time.Time, txType string) ([]CategoryBreakdown, error) {
	f.CapturedFrom, f.CapturedTo, f.CapturedType = from, to, txType
	return f.CategoriesResult, f.Err
}

func (f *FakeAnalyticsService) ByWallet(ctx context.Context, from, to time.Time) ([]WalletBreakdown, error) {
	f.CapturedFrom, f.CapturedTo = from, to
	return f.WalletsResult, f.Err
}

func (f *FakeAnalyticsService) Trend(ctx context.Context, from, to time.Time, granularity string) ([]TrendBucket, error) {
	f.CapturedFrom, f.CapturedTo, f.CapturedGranularity = from, to, granularity
	return f.TrendResult, f.Err
}

func TestHandlerSummary(t *testing.T) {
	t.Run("bentuk response sesuai PRD", func(t *testing.T) {
		fake := &FakeAnalyticsService{SummaryResult: Summary{
			TotalIncome: 8_500_000, TotalExpense: 4_230_000, Net: 4_270_000,
			TotalBalance: 12_750_000, TransactionCount: 87,
			IncomeChangePct: 4.2, ExpenseChangePct: -11.8,
		}}
		h := NewAnalyticsHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/summary?from=2026-08-01&to=2026-08-31", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200. body: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data SummaryResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}

		if resp.Data.Period.From != "2026-08-01" || resp.Data.Period.To != "2026-08-31" {
			t.Errorf("period salah: %+v", resp.Data.Period)
		}
		if resp.Data.TotalBalance != 12_750_000 {
			t.Errorf("total_balance %d, mau 12750000", resp.Data.TotalBalance)
		}
		if resp.Data.Comparison.IncomeChangePct != 4.2 {
			t.Errorf("income_change_pct %v, mau 4.2", resp.Data.Comparison.IncomeChangePct)
		}
	})

	t.Run("default bulan berjalan", func(t *testing.T) {
		fake := &FakeAnalyticsService{}
		h := NewAnalyticsHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/summary", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if fake.CapturedFrom.Day() != 1 {
			t.Errorf("from tanggal %d, mau 1", fake.CapturedFrom.Day())
		}
		if fake.CapturedFrom.Month() != timeutil.Now().Month() {
			t.Errorf("from bulan %s, mau bulan berjalan", fake.CapturedFrom.Month())
		}
	})

	t.Run("tanggal ngawur ditolak", func(t *testing.T) {
		h := NewAnalyticsHandler(&FakeAnalyticsService{})

		req := httptest.NewRequest(http.MethodGet, "/summary?from=kemarin", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("error service jadi 500", func(t *testing.T) {
		h := NewAnalyticsHandler(&FakeAnalyticsService{Err: errors.New("database mati")})

		req := httptest.NewRequest(http.MethodGet, "/summary", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status %d, mau 500", rec.Code)
		}
	})
}

func TestHandlerByCategory(t *testing.T) {
	t.Run("sukses", func(t *testing.T) {
		fake := &FakeAnalyticsService{CategoriesResult: []CategoryBreakdown{
			{CategoryID: "c1", CategoryName: "Makan", CategoryType: "expense", Total: 750_000, Count: 10, Percentage: 75},
		}}
		h := NewAnalyticsHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/analytics/by-category?type=expense", nil)
		rec := httptest.NewRecorder()

		h.HandlerByCategory(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedType != "expense" {
			t.Errorf("type yang diteruskan %q, mau expense", fake.CapturedType)
		}

		var resp struct {
			Data []CategoryBreakdownResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].CategoryName != "Makan" {
			t.Errorf("isi data salah: %+v", resp.Data)
		}
	})

	t.Run("type ngawur ditolak", func(t *testing.T) {
		h := NewAnalyticsHandler(&FakeAnalyticsService{})

		req := httptest.NewRequest(http.MethodGet, "/analytics/by-category?type=transfer", nil)
		rec := httptest.NewRecorder()

		h.HandlerByCategory(rec, req)

		// transfer tidak punya kategori, jadi bukan pilihan yang sah di sini
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("daftar kosong tetap array, bukan null", func(t *testing.T) {
		h := NewAnalyticsHandler(&FakeAnalyticsService{CategoriesResult: nil})

		req := httptest.NewRequest(http.MethodGet, "/analytics/by-category", nil)
		rec := httptest.NewRecorder()

		h.HandlerByCategory(rec, req)

		if got := rec.Body.String(); got != "{\"data\":[]}\n" {
			t.Errorf("body %q, mau {\"data\":[]}", got)
		}
	})
}

func TestHandlerByWallet(t *testing.T) {
	fake := &FakeAnalyticsService{WalletsResult: []WalletBreakdown{
		{WalletID: "w1", WalletName: "Cash", Income: 5_000_000, Expense: 200_000, Net: 4_800_000, Count: 2},
	}}
	h := NewAnalyticsHandler(fake)

	req := httptest.NewRequest(http.MethodGet, "/analytics/by-wallet", nil)
	rec := httptest.NewRecorder()

	h.HandlerByWallet(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}

	var resp struct {
		Data []WalletBreakdownResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}
	if resp.Data[0].Net != 4_800_000 {
		t.Errorf("net %d, mau 4800000", resp.Data[0].Net)
	}
}

func TestHandlerTrend(t *testing.T) {
	t.Run("granularity default month", func(t *testing.T) {
		fake := &FakeAnalyticsService{}
		h := NewAnalyticsHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/analytics/trend", nil)
		rec := httptest.NewRecorder()

		h.HandlerTrend(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedGranularity != "month" {
			t.Errorf("granularity %q, mau month", fake.CapturedGranularity)
		}
	})

	granularities := []struct {
		value      string
		wantStatus int
	}{
		{"day", http.StatusOK},
		{"week", http.StatusOK},
		{"month", http.StatusOK},
		{"quarter", http.StatusBadRequest},
		{"tahunan", http.StatusBadRequest},
	}

	for _, tt := range granularities {
		t.Run("granularity "+tt.value, func(t *testing.T) {
			h := NewAnalyticsHandler(&FakeAnalyticsService{})

			req := httptest.NewRequest(http.MethodGet, "/analytics/trend?granularity="+tt.value, nil)
			rec := httptest.NewRecorder()

			h.HandlerTrend(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, mau %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("tanggal bucket ditulis YYYY-MM-DD", func(t *testing.T) {
		start := time.Date(2026, time.July, 1, 0, 0, 0, 0, timeutil.Loc())
		h := NewAnalyticsHandler(&FakeAnalyticsService{TrendResult: []TrendBucket{
			{Bucket: "2026-07", Start: start, End: start.AddDate(0, 1, -1), Income: 500_000, Expense: 100_000, Net: 400_000},
		}})

		req := httptest.NewRequest(http.MethodGet, "/analytics/trend", nil)
		rec := httptest.NewRecorder()

		h.HandlerTrend(rec, req)

		var resp struct {
			Data []TrendBucketResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if resp.Data[0].Start != "2026-07-01" || resp.Data[0].End != "2026-07-31" {
			t.Errorf("rentang bucket salah: %+v", resp.Data[0])
		}
	})
}
