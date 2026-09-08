package quickadd

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

type FakeQuickAddService struct {
	Items   []QuickAdd
	Result  QuickAdd
	Created CreatedTransaction
	Err     error

	CapturedCreate  CreateQuickAddRequest
	CapturedPatch   PatchQuickAddRequest
	CapturedExecute ExecuteRequest
	CapturedID      string
}

func (f *FakeQuickAddService) CreateQuickAdd(ctx context.Context, req CreateQuickAddRequest) (QuickAdd, error) {
	f.CapturedCreate = req
	return f.Result, f.Err
}

func (f *FakeQuickAddService) GetAllQuickAdds(ctx context.Context, includeDeleted bool) ([]QuickAdd, int, error) {
	return f.Items, len(f.Items), f.Err
}

func (f *FakeQuickAddService) PatchQuickAdd(ctx context.Context, id string, req PatchQuickAddRequest) (QuickAdd, error) {
	f.CapturedID, f.CapturedPatch = id, req
	return f.Result, f.Err
}

func (f *FakeQuickAddService) DeleteQuickAdd(ctx context.Context, id string) (QuickAdd, error) {
	f.CapturedID = id
	return f.Result, f.Err
}

func (f *FakeQuickAddService) Execute(ctx context.Context, id string, req ExecuteRequest) (CreatedTransaction, error) {
	f.CapturedID, f.CapturedExecute = id, req
	return f.Created, f.Err
}

func TestHandlerQuickAddsList(t *testing.T) {
	h := NewQuickAddHandler(&FakeQuickAddService{Items: []QuickAdd{
		{ID: validUUID, Label: "Kopi", Type: "expense",
			Amount: sql.NullInt64{Int64: 25_000, Valid: true}, WalletID: validUUID, CategoryID: validUUID2},
		{ID: validUUID2, Label: "Bensin", Type: "expense",
			Amount: sql.NullInt64{Valid: false}, WalletID: validUUID, CategoryID: validUUID2},
	}})

	req := httptest.NewRequest(http.MethodGet, "/quick-adds", nil)
	rec := httptest.NewRecorder()

	h.HandlerQuickAdds(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}

	var resp struct {
		Data []QuickAddResponse `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}

	if resp.Meta.Total != 2 {
		t.Errorf("total %d, mau 2", resp.Meta.Total)
	}
	if resp.Data[0].Amount == nil || *resp.Data[0].Amount != 25_000 {
		t.Errorf("amount pertama salah: %+v", resp.Data[0].Amount)
	}
	// quick add tanpa nominal tetap muncul, amount-nya null
	if resp.Data[1].Amount != nil {
		t.Errorf("amount kedua harus null, dapat %v", *resp.Data[1].Amount)
	}
}

func TestHandlerCreateQuickAdd(t *testing.T) {
	t.Run("sukses", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		body := `{"label":"Kopi","type":"expense","amount":25000,"wallet_id":"` + validUUID +
			`","category_id":"` + validUUID2 + `","note":"kopi susu"}`
		req := httptest.NewRequest(http.MethodPost, "/quick-adds", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerQuickAdds(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201. body: %s", rec.Code, rec.Body.String())
		}
		if fake.CapturedCreate.Amount == nil || *fake.CapturedCreate.Amount != 25_000 {
			t.Errorf("amount tidak diteruskan: %+v", fake.CapturedCreate.Amount)
		}
	})

	t.Run("amount boleh tidak dikirim", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		body := `{"label":"Bensin","type":"expense","wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `"}`
		req := httptest.NewRequest(http.MethodPost, "/quick-adds", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerQuickAdds(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201", rec.Code)
		}
		if fake.CapturedCreate.Amount != nil {
			t.Error("amount harus nil kalau tidak dikirim")
		}
	})

	invalid := []struct {
		name string
		body string
	}{
		{"json rusak", `{"label":}`},
		{"wallet_id bukan uuid", `{"label":"Kopi","type":"expense","wallet_id":"abc","category_id":"` + validUUID2 + `"}`},
		{"category_id bukan uuid", `{"label":"Kopi","type":"expense","wallet_id":"` + validUUID + `","category_id":"abc"}`},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			h := NewQuickAddHandler(&FakeQuickAddService{})

			req := httptest.NewRequest(http.MethodPost, "/quick-adds", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerQuickAdds(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, mau 400", rec.Code)
			}
		})
	}

	t.Run("label duplikat jadi 409", func(t *testing.T) {
		h := NewQuickAddHandler(&FakeQuickAddService{
			Err: apperror.AlreadyExistsErr{Resource: "quick_adds", Name: "Kopi", Type: "expense"},
		})

		body := `{"label":"Kopi","type":"expense","wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `"}`
		req := httptest.NewRequest(http.MethodPost, "/quick-adds", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerQuickAdds(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, mau 409", rec.Code)
		}
	})

	t.Run("method tidak didukung", func(t *testing.T) {
		h := NewQuickAddHandler(&FakeQuickAddService{})

		req := httptest.NewRequest(http.MethodPut, "/quick-adds", nil)
		rec := httptest.NewRecorder()

		h.HandlerQuickAdds(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})
}

func TestHandlerQuickAddByID(t *testing.T) {
	t.Run("id bukan uuid ditolak", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodDelete, "/quick-adds/"+invalidUUID, nil)
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.CapturedID != "" {
			t.Error("service tidak boleh dipanggil")
		}
	})

	// amount punya tiga keadaan: tidak dikirim, dikirim null (nominal diisi
	// manual saat dipakai), dikirim angka.
	t.Run("amount null minta dikosongkan", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/quick-adds/"+validUUID, strings.NewReader(`{"amount":null}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if !fake.CapturedPatch.ClearAmount {
			t.Error("ClearAmount harus true")
		}
	})

	t.Run("amount berisi angka", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/quick-adds/"+validUUID, strings.NewReader(`{"amount":30000}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if fake.CapturedPatch.ClearAmount {
			t.Error("ClearAmount harus false")
		}
		if fake.CapturedPatch.Amount == nil || *fake.CapturedPatch.Amount != 30_000 {
			t.Errorf("amount salah: %+v", fake.CapturedPatch.Amount)
		}
	})

	t.Run("amount tidak dikirim", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/quick-adds/"+validUUID, strings.NewReader(`{"label":"Kopi Susu"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if fake.CapturedPatch.ClearAmount || fake.CapturedPatch.Amount != nil {
			t.Error("amount yang tidak dikirim tidak boleh diapa-apakan")
		}
	})

	t.Run("amount bukan angka ditolak", func(t *testing.T) {
		h := NewQuickAddHandler(&FakeQuickAddService{})

		req := httptest.NewRequest(http.MethodPatch, "/quick-adds/"+validUUID, strings.NewReader(`{"amount":"banyak"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("delete", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodDelete, "/quick-adds/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, mau 200", rec.Code)
		}
	})

	t.Run("method tidak didukung", func(t *testing.T) {
		h := NewQuickAddHandler(&FakeQuickAddService{})

		req := httptest.NewRequest(http.MethodGet, "/quick-adds/"+validUUID, nil)
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerQuickAddByID(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})
}

func TestHandlerExecute(t *testing.T) {
	t.Run("body kosong tetap jalan", func(t *testing.T) {
		fake := &FakeQuickAddService{Created: CreatedTransaction{
			ID: "tx-1", Type: "expense", Amount: 25_000, OccurredAt: timeutil.Now(),
		}}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/quick-adds/"+validUUID+"/execute", strings.NewReader(``))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerExecute(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201. body: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			Data ExecuteResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if resp.Data.TransactionID != "tx-1" {
			t.Errorf("transaction_id %q, mau tx-1", resp.Data.TransactionID)
		}
	})

	t.Run("amount dari body diteruskan", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/quick-adds/"+validUUID+"/execute", strings.NewReader(`{"amount":50000}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerExecute(rec, req)

		if fake.CapturedExecute.Amount == nil || *fake.CapturedExecute.Amount != 50_000 {
			t.Errorf("amount salah: %+v", fake.CapturedExecute.Amount)
		}
	})

	t.Run("id bukan uuid ditolak", func(t *testing.T) {
		fake := &FakeQuickAddService{}
		h := NewQuickAddHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/quick-adds/"+invalidUUID+"/execute", strings.NewReader(`{}`))
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerExecute(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.CapturedID != "" {
			t.Error("service tidak boleh dipanggil")
		}
	})

	t.Run("quick add tanpa amount tetap jadi 400", func(t *testing.T) {
		h := NewQuickAddHandler(&FakeQuickAddService{
			Err: apperror.ValidationError{Field: "amount", Message: "kirim amount di request"},
		})

		req := httptest.NewRequest(http.MethodPost, "/quick-adds/"+validUUID+"/execute", strings.NewReader(`{}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerExecute(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("quick add tidak ketemu jadi 404", func(t *testing.T) {
		h := NewQuickAddHandler(&FakeQuickAddService{
			Err: apperror.NotFoundError{Resource: "quick_adds", ID: validUUID},
		})

		req := httptest.NewRequest(http.MethodPost, "/quick-adds/"+validUUID+"/execute", strings.NewReader(`{}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerExecute(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, mau 404", rec.Code)
		}
	})
}
