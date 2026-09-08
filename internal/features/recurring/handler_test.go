package recurring

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
)

const (
	validUUID   = "0192a1b2-c3d4-4e5f-8a9b-0c1d2e3f4a5b"
	validUUID2  = "0192a1b2-c3d4-4e5f-8a9b-0c1d2e3f4a5c"
	invalidUUID = "bukan-uuid"
)

type FakeRecurringService struct {
	Rules  []Rule
	Result Rule
	Run    RunResult
	Err    error

	CapturedCreate CreateRuleRequest
	CapturedPatch  PatchRuleRequest
	CapturedID     string
	RunCalls       int
}

func (f *FakeRecurringService) CreateRule(ctx context.Context, req CreateRuleRequest) (Rule, error) {
	f.CapturedCreate = req
	return f.Result, f.Err
}

func (f *FakeRecurringService) GetAllRules(ctx context.Context, includeDeleted bool) ([]Rule, int, error) {
	return f.Rules, len(f.Rules), f.Err
}

func (f *FakeRecurringService) GetRuleByID(ctx context.Context, id string) (Rule, error) {
	f.CapturedID = id
	return f.Result, f.Err
}

func (f *FakeRecurringService) PatchRule(ctx context.Context, id string, req PatchRuleRequest) (Rule, error) {
	f.CapturedID, f.CapturedPatch = id, req
	return f.Result, f.Err
}

func (f *FakeRecurringService) DeleteRule(ctx context.Context, id string) (Rule, error) {
	f.CapturedID = id
	return f.Result, f.Err
}

func (f *FakeRecurringService) ToggleRule(ctx context.Context, id string) (Rule, error) {
	f.CapturedID = id
	return f.Result, f.Err
}

func (f *FakeRecurringService) RunDue(ctx context.Context) (RunResult, error) {
	f.RunCalls++
	return f.Run, f.Err
}

func TestHandlerRecurringList(t *testing.T) {
	h := NewRecurringHandler(&FakeRecurringService{Rules: []Rule{{
		ID: validUUID, Name: "Langganan streaming", Type: "expense", Amount: 50_000,
		Frequency: "monthly", Interval: 1,
		DayOfMonth: sql.NullInt64{Int64: 31, Valid: true},
		StartDate:  timeutil.Now(), NextRunAt: timeutil.Now(), IsActive: true,
	}}})

	req := httptest.NewRequest(http.MethodGet, "/recurring", nil)
	rec := httptest.NewRecorder()

	h.HandlerRecurring(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}

	var resp struct {
		Data []RuleResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}

	got := resp.Data[0]
	if got.DayOfMonth == nil || *got.DayOfMonth != 31 {
		t.Errorf("day_of_month salah: %+v", got.DayOfMonth)
	}
	if got.DayOfWeek != nil {
		t.Error("day_of_week harus null untuk rule bulanan")
	}
	if got.LastRunAt != nil {
		t.Error("last_run_at harus null sebelum pernah jalan")
	}
}

func TestHandlerCreateRule(t *testing.T) {
	validBody := `{"name":"Langganan","type":"expense","amount":50000,"wallet_id":"` + validUUID +
		`","category_id":"` + validUUID2 + `","frequency":"monthly","interval":1,"day_of_month":31,"start_date":"2026-01-31"}`

	t.Run("sukses", func(t *testing.T) {
		fake := &FakeRecurringService{}
		h := NewRecurringHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/recurring", strings.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.HandlerRecurring(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201. body: %s", rec.Code, rec.Body.String())
		}
		if fake.CapturedCreate.DayOfMonth == nil || *fake.CapturedCreate.DayOfMonth != 31 {
			t.Errorf("day_of_month tidak diteruskan: %+v", fake.CapturedCreate.DayOfMonth)
		}
		if timeutil.FormatDate(*fake.CapturedCreate.StartDate) != "2026-01-31" {
			t.Errorf("start_date salah: %v", fake.CapturedCreate.StartDate)
		}
	})

	invalid := []struct {
		name string
		body string
	}{
		{"json rusak", `{"amount":}`},
		{"wallet_id bukan uuid", `{"name":"x","type":"expense","amount":1000,"wallet_id":"abc","category_id":"` + validUUID2 + `","frequency":"daily","start_date":"2026-01-01"}`},
		{"category_id bukan uuid", `{"name":"x","type":"expense","amount":1000,"wallet_id":"` + validUUID + `","category_id":"abc","frequency":"daily","start_date":"2026-01-01"}`},
		{"start_date kosong", `{"name":"x","type":"expense","amount":1000,"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","frequency":"daily"}`},
		{"start_date ngawur", `{"name":"x","type":"expense","amount":1000,"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","frequency":"daily","start_date":"januari"}`},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			h := NewRecurringHandler(&FakeRecurringService{})

			req := httptest.NewRequest(http.MethodPost, "/recurring", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerRecurring(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, mau 400", rec.Code)
			}
		})
	}

	t.Run("error validasi service diteruskan", func(t *testing.T) {
		h := NewRecurringHandler(&FakeRecurringService{
			Err: apperror.ValidationError{Field: "frequency", Message: "tidak dikenal"},
		})

		req := httptest.NewRequest(http.MethodPost, "/recurring", strings.NewReader(validBody))
		rec := httptest.NewRecorder()

		h.HandlerRecurring(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("method tidak didukung", func(t *testing.T) {
		h := NewRecurringHandler(&FakeRecurringService{})

		req := httptest.NewRequest(http.MethodDelete, "/recurring", nil)
		rec := httptest.NewRecorder()

		h.HandlerRecurring(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})
}

func TestHandlerRecurringByID(t *testing.T) {
	t.Run("id bukan uuid ditolak", func(t *testing.T) {
		fake := &FakeRecurringService{}
		h := NewRecurringHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/recurring/"+invalidUUID, nil)
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerRecurringByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.CapturedID != "" {
			t.Error("service tidak boleh dipanggil")
		}
	})

	methods := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{"get", http.MethodGet, "", http.StatusOK},
		{"patch", http.MethodPatch, `{"amount":75000}`, http.StatusOK},
		{"delete", http.MethodDelete, "", http.StatusOK},
		{"method tidak didukung", http.MethodPut, "", http.StatusMethodNotAllowed},
	}

	for _, tt := range methods {
		t.Run(tt.name, func(t *testing.T) {
			h := NewRecurringHandler(&FakeRecurringService{})

			req := httptest.NewRequest(tt.method, "/recurring/"+validUUID, strings.NewReader(tt.body))
			req.SetPathValue("id", validUUID)
			rec := httptest.NewRecorder()

			h.HandlerRecurringByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, mau %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("end_date null minta dikosongkan", func(t *testing.T) {
		fake := &FakeRecurringService{}
		h := NewRecurringHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/recurring/"+validUUID, strings.NewReader(`{"end_date":null}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerRecurringByID(rec, req)

		if !fake.CapturedPatch.ClearEndDate {
			t.Error("ClearEndDate harus true")
		}
	})

	t.Run("next_run_at bisa digeser manual", func(t *testing.T) {
		fake := &FakeRecurringService{}
		h := NewRecurringHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/recurring/"+validUUID, strings.NewReader(`{"next_run_at":"2026-10-01"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerRecurringByID(rec, req)

		if fake.CapturedPatch.NextRunAt == nil ||
			timeutil.FormatDate(*fake.CapturedPatch.NextRunAt) != "2026-10-01" {
			t.Errorf("next_run_at salah: %+v", fake.CapturedPatch.NextRunAt)
		}
	})

	t.Run("not found dari service", func(t *testing.T) {
		h := NewRecurringHandler(&FakeRecurringService{
			Err: apperror.NotFoundError{Resource: "recurring_rules", ID: validUUID},
		})

		req := httptest.NewRequest(http.MethodGet, "/recurring/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerRecurringByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, mau 404", rec.Code)
		}
	})
}

func TestHandlerToggle(t *testing.T) {
	fake := &FakeRecurringService{Result: Rule{ID: validUUID, IsActive: false, StartDate: timeutil.Now(), NextRunAt: timeutil.Now()}}
	h := NewRecurringHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/recurring/"+validUUID+"/toggle", nil)
	req.SetPathValue("id", validUUID)
	rec := httptest.NewRecorder()

	h.HandlerToggle(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}
	if fake.CapturedID != validUUID {
		t.Errorf("id %q, mau %q", fake.CapturedID, validUUID)
	}
}

func TestHandlerRun(t *testing.T) {
	t.Run("melaporkan hasil putaran", func(t *testing.T) {
		fake := &FakeRecurringService{Run: RunResult{
			RulesChecked: 3, Created: 8, Skipped: 2, RulesFailed: 1,
			FailedRuleIDs: []string{validUUID},
		}}
		h := NewRecurringHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/recurring/run", nil)
		rec := httptest.NewRecorder()

		h.HandlerRun(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if fake.RunCalls != 1 {
			t.Errorf("RunDue dipanggil %d kali, mau 1", fake.RunCalls)
		}

		var resp struct {
			Data RunResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if resp.Data.Created != 8 || resp.Data.Skipped != 2 || resp.Data.RulesFailed != 1 {
			t.Errorf("hasil salah: %+v", resp.Data)
		}
	})

	// Kalau tidak ada rule yang gagal, field-nya array kosong, bukan null,
	// supaya frontend tidak perlu menangani dua bentuk.
	t.Run("failed_rule_ids array kosong bukan null", func(t *testing.T) {
		h := NewRecurringHandler(&FakeRecurringService{Run: RunResult{RulesChecked: 1, Created: 1}})

		req := httptest.NewRequest(http.MethodPost, "/recurring/run", nil)
		rec := httptest.NewRecorder()

		h.HandlerRun(rec, req)

		if !strings.Contains(rec.Body.String(), `"failed_rule_ids":[]`) {
			t.Errorf("body %s, mau failed_rule_ids berupa array kosong", rec.Body.String())
		}
	})
}
