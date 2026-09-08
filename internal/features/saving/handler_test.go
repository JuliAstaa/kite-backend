package saving

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	validUUID   = "0192a1b2-c3d4-4e5f-8a9b-0c1d2e3f4a5b"
	invalidUUID = "bukan-uuid"
)

type FakeSavingService struct {
	SummaryResult   Summary
	BreakdownResult []BreakdownBucket
	Targets         []Target
	TargetResult    Target
	Err             error

	CapturedPeriod        string
	CapturedFrom          time.Time
	CapturedTo            time.Time
	CapturedExplicitRange bool
	CapturedCreate        CreateTargetRequest
	CapturedPatch         PatchTargetRequest
	CapturedID            string
}

func (f *FakeSavingService) Summary(ctx context.Context, period string, from, to time.Time, explicitRange bool) (Summary, error) {
	f.CapturedPeriod, f.CapturedFrom, f.CapturedTo, f.CapturedExplicitRange = period, from, to, explicitRange
	if f.Err != nil {
		return Summary{}, f.Err
	}
	result := f.SummaryResult
	result.Period, result.From, result.To = period, from, to
	return result, nil
}

func (f *FakeSavingService) Breakdown(ctx context.Context, period string, from, to time.Time) ([]BreakdownBucket, error) {
	f.CapturedPeriod, f.CapturedFrom, f.CapturedTo = period, from, to
	return f.BreakdownResult, f.Err
}

func (f *FakeSavingService) AverageMonthlySavable(ctx context.Context, months int) (int, int, error) {
	return 0, 0, f.Err
}

func (f *FakeSavingService) CreateTarget(ctx context.Context, req CreateTargetRequest) (Target, error) {
	f.CapturedCreate = req
	return f.TargetResult, f.Err
}

func (f *FakeSavingService) GetAllTargets(ctx context.Context, includeDeleted bool) ([]Target, int, error) {
	return f.Targets, len(f.Targets), f.Err
}

func (f *FakeSavingService) PatchTarget(ctx context.Context, id string, req PatchTargetRequest) (Target, error) {
	f.CapturedID, f.CapturedPatch = id, req
	return f.TargetResult, f.Err
}

func (f *FakeSavingService) DeleteTarget(ctx context.Context, id string) (Target, error) {
	f.CapturedID = id
	return f.TargetResult, f.Err
}

func TestHandlerSavingSummary(t *testing.T) {
	t.Run("bentuk response sesuai PRD", func(t *testing.T) {
		fake := &FakeSavingService{SummaryResult: Summary{
			Income: 8_500_000, Expense: 4_230_000, Savable: 4_270_000, SavingsRate: 50.24,
			Target:              &TargetProgress{Amount: 3_000_000, Achieved: true, ProgressPct: 142.3, Difference: 1_270_000},
			DailyAverageExpense: 136_451, ProjectedSavable: 4_270_000, DaysElapsed: 11, DaysTotal: 31,
		}}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/savings/summary?period=month&from=2026-08-01&to=2026-08-31", nil)
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

		if resp.Data.Range.From != "2026-08-01" || resp.Data.Range.To != "2026-08-31" {
			t.Errorf("range salah: %+v", resp.Data.Range)
		}
		if resp.Data.Target == nil || resp.Data.Target.ProgressPct != 142.3 {
			t.Errorf("target salah: %+v", resp.Data.Target)
		}
	})

	t.Run("target null kalau tidak ada", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{SummaryResult: Summary{Income: 100, Target: nil}})

		req := httptest.NewRequest(http.MethodGet, "/savings/summary", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if !strings.Contains(rec.Body.String(), `"target":null`) {
			t.Errorf("target harus null, body: %s", rec.Body.String())
		}
	})

	t.Run("period default month dan rentangnya periode berjalan", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/savings/summary", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if fake.CapturedPeriod != "month" {
			t.Errorf("period %q, mau month", fake.CapturedPeriod)
		}
		if fake.CapturedFrom.Day() != 1 {
			t.Errorf("from tanggal %d, mau 1", fake.CapturedFrom.Day())
		}
		if fake.CapturedExplicitRange {
			t.Error("tanpa from/to, explicitRange harus false")
		}
	})

	t.Run("from atau to yang dikirim menandai rentang eksplisit", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/savings/summary?from=2026-07-01", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if !fake.CapturedExplicitRange {
			t.Error("explicitRange harus true kalau client mengirim from")
		}
	})

	t.Run("period week diterima", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/savings/summary?period=week", nil)
		rec := httptest.NewRecorder()

		h.HandlerSummary(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedFrom.Weekday() != time.Monday {
			t.Errorf("from hari %s, mau Senin", fake.CapturedFrom.Weekday())
		}
	})

	invalid := []struct {
		name  string
		query string
	}{
		{"period ngawur", "?period=harian"},
		{"from bukan tanggal", "?from=kemarin"},
		{"to lebih awal dari from", "?from=2026-08-31&to=2026-08-01"},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSavingHandler(&FakeSavingService{})

			req := httptest.NewRequest(http.MethodGet, "/savings/summary"+tt.query, nil)
			rec := httptest.NewRecorder()

			h.HandlerSummary(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, mau 400", rec.Code)
			}
		})
	}
}

func TestHandlerBreakdown(t *testing.T) {
	t.Run("sukses", func(t *testing.T) {
		start := time.Date(2026, time.June, 1, 0, 0, 0, 0, timeutil.Loc())
		h := NewSavingHandler(&FakeSavingService{BreakdownResult: []BreakdownBucket{
			{Bucket: "2026-W23", Start: start, End: start.AddDate(0, 0, 6), Income: 0, Expense: 620_000, Savable: -620_000},
		}})

		req := httptest.NewRequest(http.MethodGet, "/savings/breakdown?period=week", nil)
		rec := httptest.NewRecorder()

		h.HandlerBreakdown(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}

		var resp struct {
			Data []BreakdownBucketResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if resp.Data[0].Savable != -620_000 {
			t.Errorf("savable %d, mau -620000 (angka negatif tidak boleh di-clamp)", resp.Data[0].Savable)
		}
		if resp.Data[0].Start != "2026-06-01" {
			t.Errorf("start %q, mau 2026-06-01", resp.Data[0].Start)
		}
	})

	t.Run("period ngawur ditolak", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{})

		req := httptest.NewRequest(http.MethodGet, "/savings/breakdown?period=harian", nil)
		rec := httptest.NewRecorder()

		h.HandlerBreakdown(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})
}

func TestHandlerTargets(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{Targets: []Target{
			{ID: validUUID, Period: "monthly", Amount: sql.NullInt64{Int64: 3_000_000, Valid: true},
				StartDate: timeutil.Now(), IsActive: true},
		}})

		req := httptest.NewRequest(http.MethodGet, "/savings/targets", nil)
		rec := httptest.NewRecorder()

		h.HandlerTargets(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}

		var resp struct {
			Data []TargetResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if resp.Data[0].Amount == nil || *resp.Data[0].Amount != 3_000_000 {
			t.Errorf("amount salah: %+v", resp.Data[0].Amount)
		}
		if resp.Data[0].TargetRate != nil {
			t.Error("target_rate harus null kalau yang dipakai amount")
		}
	})

	t.Run("create sukses", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		body := `{"period":"monthly","amount":3000000,"start_date":"2026-08-01"}`
		req := httptest.NewRequest(http.MethodPost, "/savings/targets", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerTargets(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201. body: %s", rec.Code, rec.Body.String())
		}
		if fake.CapturedCreate.StartDate == nil || timeutil.FormatDate(*fake.CapturedCreate.StartDate) != "2026-08-01" {
			t.Errorf("start_date tidak diteruskan dengan benar: %+v", fake.CapturedCreate.StartDate)
		}
	})

	t.Run("start_date wajib", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{})

		req := httptest.NewRequest(http.MethodPost, "/savings/targets", strings.NewReader(`{"period":"monthly","amount":3000000}`))
		rec := httptest.NewRecorder()

		h.HandlerTargets(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("json rusak", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{})

		req := httptest.NewRequest(http.MethodPost, "/savings/targets", strings.NewReader(`{"period":}`))
		rec := httptest.NewRecorder()

		h.HandlerTargets(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("error validasi service diteruskan", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{
			Err: apperror.ValidationError{Field: "amount", Message: "isi salah satu"},
		})

		body := `{"period":"monthly","amount":3000000,"target_rate":20,"start_date":"2026-08-01"}`
		req := httptest.NewRequest(http.MethodPost, "/savings/targets", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerTargets(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("method tidak didukung", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{})

		req := httptest.NewRequest(http.MethodDelete, "/savings/targets", nil)
		rec := httptest.NewRecorder()

		h.HandlerTargets(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})
}

func TestHandlerTargetByID(t *testing.T) {
	t.Run("id bukan uuid ditolak", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/savings/targets/"+invalidUUID, strings.NewReader(`{}`))
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.CapturedID != "" {
			t.Error("service tidak boleh dipanggil")
		}
	})

	// end_date punya tiga keadaan: tidak dikirim, dikirim null, dikirim tanggal.
	t.Run("end_date null berarti minta dikosongkan", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/savings/targets/"+validUUID, strings.NewReader(`{"end_date":null}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if !fake.CapturedPatch.ClearEndDate {
			t.Error("ClearEndDate harus true saat end_date dikirim null")
		}
	})

	t.Run("end_date tidak dikirim berarti jangan diapa-apakan", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/savings/targets/"+validUUID, strings.NewReader(`{"amount":5000000}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if fake.CapturedPatch.ClearEndDate {
			t.Error("ClearEndDate harus false kalau end_date tidak dikirim")
		}
		if fake.CapturedPatch.EndDate != nil {
			t.Error("EndDate harus nil kalau tidak dikirim")
		}
	})

	t.Run("end_date berisi tanggal", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/savings/targets/"+validUUID, strings.NewReader(`{"end_date":"2026-12-31"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if fake.CapturedPatch.ClearEndDate {
			t.Error("ClearEndDate harus false")
		}
		if fake.CapturedPatch.EndDate == nil || timeutil.FormatDate(*fake.CapturedPatch.EndDate) != "2026-12-31" {
			t.Errorf("EndDate salah: %+v", fake.CapturedPatch.EndDate)
		}
	})

	t.Run("end_date bukan tanggal ditolak", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{})

		req := httptest.NewRequest(http.MethodPatch, "/savings/targets/"+validUUID, strings.NewReader(`{"end_date":"besok"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("delete", func(t *testing.T) {
		fake := &FakeSavingService{}
		h := NewSavingHandler(fake)

		req := httptest.NewRequest(http.MethodDelete, "/savings/targets/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedID != validUUID {
			t.Errorf("id %q, mau %q", fake.CapturedID, validUUID)
		}
	})

	t.Run("not found dari service", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{
			Err: apperror.NotFoundError{Resource: "savings_targets", ID: validUUID},
		})

		req := httptest.NewRequest(http.MethodDelete, "/savings/targets/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, mau 404", rec.Code)
		}
	})

	t.Run("method tidak didukung", func(t *testing.T) {
		h := NewSavingHandler(&FakeSavingService{})

		req := httptest.NewRequest(http.MethodPost, "/savings/targets/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTargetByID(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})
}
