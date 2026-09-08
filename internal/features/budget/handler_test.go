package budget

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
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

type FakeBudgetService struct {
	Budgets  []Budget
	Statuses []Status
	Result   Budget
	Err      error

	CapturedCreate CreateBudgetRequest
	CapturedPatch  PatchBudgetRequest
	CapturedID     string
	CapturedMonth  time.Time
}

func (f *FakeBudgetService) CreateBudget(ctx context.Context, req CreateBudgetRequest) (Budget, error) {
	f.CapturedCreate = req
	return f.Result, f.Err
}

func (f *FakeBudgetService) GetAllBudgets(ctx context.Context, includeDeleted bool) ([]Budget, int, error) {
	return f.Budgets, len(f.Budgets), f.Err
}

func (f *FakeBudgetService) PatchBudget(ctx context.Context, id string, req PatchBudgetRequest) (Budget, error) {
	f.CapturedID, f.CapturedPatch = id, req
	return f.Result, f.Err
}

func (f *FakeBudgetService) DeleteBudget(ctx context.Context, id string) (Budget, error) {
	f.CapturedID = id
	return f.Result, f.Err
}

func (f *FakeBudgetService) StatusForMonth(ctx context.Context, month time.Time) ([]Status, error) {
	f.CapturedMonth = month
	return f.Statuses, f.Err
}

func TestHandlerBudgetsList(t *testing.T) {
	h := NewBudgetHandler(&FakeBudgetService{Budgets: []Budget{
		{ID: validUUID, CategoryID: "c1", CategoryName: "Makan", Amount: 2_000_000,
			Period: "monthly", StartMonth: timeutil.StartOfMonth(timeutil.Now())},
	}})

	req := httptest.NewRequest(http.MethodGet, "/budgets", nil)
	rec := httptest.NewRecorder()

	h.HandlerBudgets(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}

	var resp struct {
		Data []BudgetResponse `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}
	if resp.Meta.Total != 1 || resp.Data[0].CategoryName != "Makan" {
		t.Errorf("isi salah: %+v", resp)
	}
	if resp.Data[0].EndMonth != nil {
		t.Error("end_month harus null")
	}
}

func TestHandlerCreateBudget(t *testing.T) {
	t.Run("start_month format YYYY-MM diterima", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		body := `{"category_id":"` + validUUID + `","amount":2000000,"period":"monthly","start_month":"2026-08"}`
		req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerBudgets(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201. body: %s", rec.Code, rec.Body.String())
		}
		if fake.CapturedCreate.StartMonth == nil ||
			timeutil.FormatDate(*fake.CapturedCreate.StartMonth) != "2026-08-01" {
			t.Errorf("start_month salah: %+v", fake.CapturedCreate.StartMonth)
		}
	})

	t.Run("start_month format lengkap juga diterima", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		body := `{"category_id":"` + validUUID + `","amount":2000000,"start_month":"2026-08-17"}`
		req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerBudgets(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201", rec.Code)
		}
		// tanggal berapapun dinormalkan ke tanggal 1
		if timeutil.FormatDate(*fake.CapturedCreate.StartMonth) != "2026-08-01" {
			t.Errorf("start_month %s, mau dinormalkan ke 2026-08-01",
				timeutil.FormatDate(*fake.CapturedCreate.StartMonth))
		}
	})

	invalid := []struct {
		name string
		body string
	}{
		{"json rusak", `{"amount":}`},
		{"category_id bukan uuid", `{"category_id":"` + invalidUUID + `","amount":2000000,"start_month":"2026-08"}`},
		{"start_month kosong", `{"category_id":"` + validUUID + `","amount":2000000}`},
		{"start_month ngawur", `{"category_id":"` + validUUID + `","amount":2000000,"start_month":"agustus"}`},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			h := NewBudgetHandler(&FakeBudgetService{})

			req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerBudgets(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, mau 400", rec.Code)
			}
		})
	}

	t.Run("conflict dari service jadi 409", func(t *testing.T) {
		h := NewBudgetHandler(&FakeBudgetService{
			Err: apperror.ConflictError{Message: "kategori sudah punya budget berjalan"},
		})

		body := `{"category_id":"` + validUUID + `","amount":2000000,"start_month":"2026-08"}`
		req := httptest.NewRequest(http.MethodPost, "/budgets", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerBudgets(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, mau 409", rec.Code)
		}
	})

	t.Run("method tidak didukung", func(t *testing.T) {
		h := NewBudgetHandler(&FakeBudgetService{})

		req := httptest.NewRequest(http.MethodPatch, "/budgets", nil)
		rec := httptest.NewRecorder()

		h.HandlerBudgets(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})
}

func TestHandlerBudgetByID(t *testing.T) {
	t.Run("id bukan uuid ditolak", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		req := httptest.NewRequest(http.MethodDelete, "/budgets/"+invalidUUID, nil)
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerBudgetByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.CapturedID != "" {
			t.Error("service tidak boleh dipanggil")
		}
	})

	t.Run("end_date null minta dikosongkan", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/budgets/"+validUUID, strings.NewReader(`{"end_month":null}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerBudgetByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if !fake.CapturedPatch.ClearEndMonth {
			t.Error("ClearEndMonth harus true")
		}
	})

	t.Run("end_month berisi bulan", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/budgets/"+validUUID, strings.NewReader(`{"end_month":"2026-12"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerBudgetByID(rec, req)

		if fake.CapturedPatch.EndMonth == nil ||
			timeutil.FormatDate(*fake.CapturedPatch.EndMonth) != "2026-12-01" {
			t.Errorf("end_month salah: %+v", fake.CapturedPatch.EndMonth)
		}
	})

	t.Run("delete", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		req := httptest.NewRequest(http.MethodDelete, "/budgets/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerBudgetByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedID != validUUID {
			t.Errorf("id %q, mau %q", fake.CapturedID, validUUID)
		}
	})

	t.Run("not found dari service", func(t *testing.T) {
		h := NewBudgetHandler(&FakeBudgetService{
			Err: apperror.NotFoundError{Resource: "budgets", ID: validUUID},
		})

		req := httptest.NewRequest(http.MethodDelete, "/budgets/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerBudgetByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, mau 404", rec.Code)
		}
	})
}

func TestHandlerBudgetStatus(t *testing.T) {
	t.Run("bentuk response sesuai PRD", func(t *testing.T) {
		monthStart := timeutil.StartOfMonth(timeutil.Now())
		h := NewBudgetHandler(&FakeBudgetService{Statuses: []Status{
			{
				BudgetID: validUUID, CategoryID: "c1", CategoryName: "Makan & Minum",
				Limit: 2_000_000, Spent: 1_850_000, Remaining: 150_000,
				Percentage: 92.5, State: "warning", DaysLeft: 20,
				Period: "monthly", From: monthStart, To: timeutil.EndOfMonth(monthStart),
			},
		}})

		req := httptest.NewRequest(http.MethodGet, "/budgets/status?month=2026-08", nil)
		rec := httptest.NewRecorder()

		h.HandlerStatus(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}

		var resp struct {
			Data []StatusResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}

		got := resp.Data[0]
		if got.Limit != 2_000_000 || got.Spent != 1_850_000 || got.Remaining != 150_000 {
			t.Errorf("angka salah: %+v", got)
		}
		// field-nya bernama "status", bukan "state"
		if got.Status != "warning" {
			t.Errorf("status %q, mau warning", got.Status)
		}
	})

	t.Run("month diteruskan ke service", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/budgets/status?month=2026-08", nil)
		rec := httptest.NewRecorder()

		h.HandlerStatus(rec, req)

		if timeutil.FormatDate(fake.CapturedMonth) != "2026-08-01" {
			t.Errorf("month %s, mau 2026-08-01", timeutil.FormatDate(fake.CapturedMonth))
		}
	})

	t.Run("tanpa month pakai bulan berjalan", func(t *testing.T) {
		fake := &FakeBudgetService{}
		h := NewBudgetHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/budgets/status", nil)
		rec := httptest.NewRecorder()

		h.HandlerStatus(rec, req)

		if fake.CapturedMonth.Month() != timeutil.Now().Month() {
			t.Errorf("month %s, mau bulan berjalan", fake.CapturedMonth.Month())
		}
	})

	t.Run("month ngawur ditolak", func(t *testing.T) {
		h := NewBudgetHandler(&FakeBudgetService{})

		req := httptest.NewRequest(http.MethodGet, "/budgets/status?month=agustus", nil)
		rec := httptest.NewRecorder()

		h.HandlerStatus(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("tanpa budget tetap array kosong", func(t *testing.T) {
		h := NewBudgetHandler(&FakeBudgetService{Statuses: nil})

		req := httptest.NewRequest(http.MethodGet, "/budgets/status", nil)
		rec := httptest.NewRecorder()

		h.HandlerStatus(rec, req)

		if got := rec.Body.String(); got != "{\"data\":[]}\n" {
			t.Errorf("body %q, mau {\"data\":[]}", got)
		}
	})
}
