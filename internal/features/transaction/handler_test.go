package transaction

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FakeTransactionService memenuhi TransactionServicer, jadi handler bisa diuji
// tanpa database sama sekali.
type FakeTransactionService struct {
	CreateFunc     func(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error)
	GetAllFunc     func(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error)
	GetByIDFunc    func(ctx context.Context, id string) (TransactionDetail, error)
	PatchFunc      func(ctx context.Context, id string, reqBody PatchTransactionRequest) (TransactionDetail, error)
	DeleteFunc     func(ctx context.Context, id string) (TransactionDetail, error)
	RestoreFunc    func(ctx context.Context, id string) (TransactionDetail, error)
	CapturedFilter TransactionFilter
	CapturedID     string
}

func (f *FakeTransactionService) CreateTransaction(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error) {
	if f.CreateFunc != nil {
		return f.CreateFunc(ctx, reqBody)
	}
	return TransactionDetail{ID: "t1", Type: reqBody.Type, Amount: reqBody.Amount}, nil
}

func (f *FakeTransactionService) GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {
	f.CapturedFilter = filter
	if f.GetAllFunc != nil {
		return f.GetAllFunc(ctx, filter)
	}
	return []TransactionDetail{{ID: "t1", Type: "expense", Amount: 25000}}, 1, nil
}

func (f *FakeTransactionService) GetTransactionByID(ctx context.Context, id string) (TransactionDetail, error) {
	f.CapturedID = id
	if f.GetByIDFunc != nil {
		return f.GetByIDFunc(ctx, id)
	}
	return TransactionDetail{ID: id}, nil
}

func (f *FakeTransactionService) PatchTransaction(ctx context.Context, id string, reqBody PatchTransactionRequest) (TransactionDetail, error) {
	f.CapturedID = id
	if f.PatchFunc != nil {
		return f.PatchFunc(ctx, id, reqBody)
	}
	return TransactionDetail{ID: id}, nil
}

func (f *FakeTransactionService) DeleteTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	f.CapturedID = id
	if f.DeleteFunc != nil {
		return f.DeleteFunc(ctx, id)
	}
	return TransactionDetail{ID: id}, nil
}

func (f *FakeTransactionService) RestoreTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	f.CapturedID = id
	if f.RestoreFunc != nil {
		return f.RestoreFunc(ctx, id)
	}
	return TransactionDetail{ID: id}, nil
}

const (
	validUUID   = "0192a1b2-c3d4-4e5f-8a9b-0c1d2e3f4a5b"
	validUUID2  = "0192a1b2-c3d4-4e5f-8a9b-0c1d2e3f4a5c"
	invalidUUID = "bukan-uuid"
)

func TestCreateTransactionHandler(t *testing.T) {
	occurredAt := timeutil.Now().Format("2006-01-02T15:04:05Z07:00")

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "expense valid",
			body:       `{"type":"expense","amount":25000,"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","note":"Kopi","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "transfer valid",
			body:       `{"type":"transfer","amount":25000,"wallet_id":"` + validUUID + `","to_wallet_id":"` + validUUID2 + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "json rusak",
			body:       `{"type":,"amount":25000}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "type kosong",
			body:       `{"amount":25000,"wallet_id":"` + validUUID + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "type ngawur",
			body:       `{"type":"kredit","amount":25000,"wallet_id":"` + validUUID + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "amount nol",
			body:       `{"type":"expense","amount":0,"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wallet_id bukan uuid",
			body:       `{"type":"expense","amount":25000,"wallet_id":"` + invalidUUID + `","category_id":"` + validUUID2 + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "expense tanpa category_id",
			body:       `{"type":"expense","amount":25000,"wallet_id":"` + validUUID + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "transfer tanpa to_wallet_id",
			body:       `{"type":"transfer","amount":25000,"wallet_id":"` + validUUID + `","occurred_at":"` + occurredAt + `"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "occurred_at kosong",
			body:       `{"type":"expense","amount":25000,"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewTransactionHandler(&FakeTransactionService{})

			req := httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerTransactions(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, mau %d. body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

// Error dari service dipetakan ke status yang benar, bukan selalu 500.
func TestCreateTransactionHandlerMemetakanErrorService(t *testing.T) {
	occurredAt := timeutil.Now().Format("2006-01-02T15:04:05Z07:00")
	body := `{"type":"expense","amount":25000,"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","occurred_at":"` + occurredAt + `"}`

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"validation", apperror.ValidationError{Field: "amount", Message: "salah"}, http.StatusBadRequest, "VALIDATION_ERROR"},
		{"not found", apperror.NotFoundError{Resource: "wallets", ID: validUUID}, http.StatusNotFound, "NOT_FOUND"},
		{"conflict", apperror.ConflictError{Message: "bentrok"}, http.StatusConflict, "CONFLICT"},
		{"unprocessable", apperror.UnprocessableError{Message: "tidak bisa"}, http.StatusUnprocessableEntity, "UNPROCESSABLE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewTransactionHandler(&FakeTransactionService{
				CreateFunc: func(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error) {
					return TransactionDetail{}, tt.err
				},
			})

			req := httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(body))
			rec := httptest.NewRecorder()

			h.HandlerTransactions(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, mau %d", rec.Code, tt.wantStatus)
			}

			var resp struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response bukan JSON error: %v", err)
			}
			if resp.Error.Code != tt.wantCode {
				t.Errorf("code %q, mau %q", resp.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestListTransactionsHandler(t *testing.T) {
	t.Run("default dari awal bulan sampai hari ini", func(t *testing.T) {
		fake := &FakeTransactionService{}
		h := NewTransactionHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
		rec := httptest.NewRecorder()

		h.HandlerTransactions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedFilter.From.Day() != 1 {
			t.Errorf("from tanggal %d, mau 1", fake.CapturedFilter.From.Day())
		}
		if fake.CapturedFilter.Limit != 50 {
			t.Errorf("limit default %d, mau 50", fake.CapturedFilter.Limit)
		}
	})

	t.Run("limit dibatasi maksimal 200", func(t *testing.T) {
		fake := &FakeTransactionService{}
		h := NewTransactionHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/transactions?limit=9999", nil)
		rec := httptest.NewRecorder()

		h.HandlerTransactions(rec, req)

		if fake.CapturedFilter.Limit != 200 {
			t.Errorf("limit %d, mau dibatasi ke 200", fake.CapturedFilter.Limit)
		}
	})

	t.Run("category_id CSV dipecah", func(t *testing.T) {
		fake := &FakeTransactionService{}
		h := NewTransactionHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/transactions?category_id="+validUUID+","+validUUID2, nil)
		rec := httptest.NewRecorder()

		h.HandlerTransactions(rec, req)

		if len(fake.CapturedFilter.CategoryIDs) != 2 {
			t.Errorf("dapat %d category_id, mau 2", len(fake.CapturedFilter.CategoryIDs))
		}
	})

	t.Run("meta ikut di response list", func(t *testing.T) {
		h := NewTransactionHandler(&FakeTransactionService{
			GetAllFunc: func(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {
				return []TransactionDetail{{ID: "t1"}}, 248, nil
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/transactions?limit=50&offset=10", nil)
		rec := httptest.NewRecorder()

		h.HandlerTransactions(rec, req)

		var resp struct {
			Data []map[string]any `json:"data"`
			Meta struct {
				Total  int `json:"total"`
				Limit  int `json:"limit"`
				Offset int `json:"offset"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai format list: %v", err)
		}
		if resp.Meta.Total != 248 || resp.Meta.Limit != 50 || resp.Meta.Offset != 10 {
			t.Errorf("meta salah: %+v", resp.Meta)
		}
	})

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"from bukan tanggal", "?from=kemarin", http.StatusBadRequest},
		{"to bukan tanggal", "?to=besok", http.StatusBadRequest},
		{"to lebih awal dari from", "?from=2026-08-31&to=2026-08-01", http.StatusBadRequest},
		{"type ngawur", "?type=kredit", http.StatusBadRequest},
		{"limit negatif", "?limit=-5", http.StatusBadRequest},
		{"category_id bukan uuid", "?category_id=abc", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewTransactionHandler(&FakeTransactionService{})

			req := httptest.NewRequest(http.MethodGet, "/transactions"+tt.query, nil)
			rec := httptest.NewRecorder()

			h.HandlerTransactions(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, mau %d. body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestHandlerTransactionByID(t *testing.T) {
	t.Run("id bukan uuid ditolak sebelum menyentuh service", func(t *testing.T) {
		fake := &FakeTransactionService{}
		h := NewTransactionHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/transactions/"+invalidUUID, nil)
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerTransactionByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.CapturedID != "" {
			t.Error("service tidak boleh dipanggil untuk id yang tidak valid")
		}
	})

	methods := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{"get", http.MethodGet, "", http.StatusOK},
		{"patch", http.MethodPatch, `{"note":"baru"}`, http.StatusOK},
		{"delete", http.MethodDelete, "", http.StatusOK},
		{"method tidak didukung", http.MethodPut, "", http.StatusMethodNotAllowed},
	}

	for _, tt := range methods {
		t.Run(tt.name, func(t *testing.T) {
			h := NewTransactionHandler(&FakeTransactionService{})

			req := httptest.NewRequest(tt.method, "/transactions/"+validUUID, strings.NewReader(tt.body))
			req.SetPathValue("id", validUUID)
			rec := httptest.NewRecorder()

			h.HandlerTransactionByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, mau %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("patch dengan json rusak", func(t *testing.T) {
		h := NewTransactionHandler(&FakeTransactionService{})

		req := httptest.NewRequest(http.MethodPatch, "/transactions/"+validUUID, strings.NewReader(`{"note":}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTransactionByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("not found dari service jadi 404", func(t *testing.T) {
		h := NewTransactionHandler(&FakeTransactionService{
			GetByIDFunc: func(ctx context.Context, id string) (TransactionDetail, error) {
				return TransactionDetail{}, apperror.NotFoundError{Resource: "transactions", ID: id}
			},
		})

		req := httptest.NewRequest(http.MethodGet, "/transactions/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerTransactionByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, mau 404", rec.Code)
		}
	})
}

func TestHandlerRestoreTransaction(t *testing.T) {
	t.Run("sukses", func(t *testing.T) {
		fake := &FakeTransactionService{}
		h := NewTransactionHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/transactions/"+validUUID+"/restore", nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerRestoreTransaction(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedID != validUUID {
			t.Errorf("id yang diteruskan %q, mau %q", fake.CapturedID, validUUID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := NewTransactionHandler(&FakeTransactionService{
			RestoreFunc: func(ctx context.Context, id string) (TransactionDetail, error) {
				return TransactionDetail{}, apperror.NotFoundError{Resource: "transactions", ID: id}
			},
		})

		req := httptest.NewRequest(http.MethodPost, "/transactions/"+validUUID+"/restore", nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerRestoreTransaction(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, mau 404", rec.Code)
		}
	})
}

// Response sukses selalu dibungkus {"data": ...} sesuai PRD bagian 5.1.
func TestResponseDibungkusData(t *testing.T) {
	h := NewTransactionHandler(&FakeTransactionService{
		GetByIDFunc: func(ctx context.Context, id string) (TransactionDetail, error) {
			return TransactionDetail{ID: id, Type: "expense", Amount: 25000, Note: "Kopi"}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/transactions/"+validUUID, nil)
	req.SetPathValue("id", validUUID)
	rec := httptest.NewRecorder()

	h.HandlerTransactionByID(rec, req)

	var resp struct {
		Data TransactionResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak dibungkus data: %v", err)
	}
	if resp.Data.Amount != 25000 {
		t.Errorf("amount %d, mau 25000", resp.Data.Amount)
	}
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type %q, mau application/json", rec.Header().Get("Content-Type"))
	}
}
