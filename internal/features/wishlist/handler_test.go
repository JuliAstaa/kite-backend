package wishlist

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

type FakeWishlistService struct {
	View   ItemView
	Views  []ItemView
	Item   Item
	Totals SummaryTotals
	Err    error
	Total  int

	CapturedFilter   ListFilter
	CapturedCreate   CreateItemRequest
	CapturedPatch    PatchItemRequest
	CapturedPurchase PurchaseRequest
	CapturedAmount   int
	CapturedID       string
}

func (f *FakeWishlistService) CreateItem(ctx context.Context, req CreateItemRequest) (ItemView, error) {
	f.CapturedCreate = req
	return f.View, f.Err
}

func (f *FakeWishlistService) GetAllItems(ctx context.Context, filter ListFilter) ([]ItemView, int, error) {
	f.CapturedFilter = filter
	if f.Err != nil {
		return nil, 0, f.Err
	}
	return f.Views, f.Total, nil
}

func (f *FakeWishlistService) GetItemByID(ctx context.Context, id string) (ItemView, error) {
	f.CapturedID = id
	return f.View, f.Err
}

func (f *FakeWishlistService) PatchItem(ctx context.Context, id string, req PatchItemRequest) (ItemView, error) {
	f.CapturedID, f.CapturedPatch = id, req
	return f.View, f.Err
}

func (f *FakeWishlistService) DeleteItem(ctx context.Context, id string) (Item, error) {
	f.CapturedID = id
	return f.Item, f.Err
}

func (f *FakeWishlistService) RestoreItem(ctx context.Context, id string) (Item, error) {
	f.CapturedID = id
	return f.Item, f.Err
}

func (f *FakeWishlistService) Allocate(ctx context.Context, id string, amount int) (ItemView, error) {
	f.CapturedID, f.CapturedAmount = id, amount
	return f.View, f.Err
}

func (f *FakeWishlistService) Purchase(ctx context.Context, id string, req PurchaseRequest) (ItemView, error) {
	f.CapturedID, f.CapturedPurchase = id, req
	return f.View, f.Err
}

func (f *FakeWishlistService) Summary(ctx context.Context) (SummaryTotals, error) {
	return f.Totals, f.Err
}

func TestHandlerWishlistList(t *testing.T) {
	t.Run("bentuk response menyertakan hitungan turunan", func(t *testing.T) {
		months := 2
		readyDate := timeutil.Now().AddDate(0, 2, 0)

		h := NewWishlistHandler(&FakeWishlistService{
			Total: 1,
			Views: []ItemView{{
				Item: Item{
					ID: validUUID, Name: "Keyboard mekanik", EstimatedPrice: 1_200_000,
					Priority: "high", Status: "saving", SavedAmount: 400_000,
					TargetDate: sql.NullTime{Time: timeutil.Now(), Valid: true},
				},
				Affordability: &Affordability{
					AvgMonthlySavable: 4_270_000, MonthsNeeded: &months,
					EstimatedReadyDate: &readyDate, OnTrackForTargetDay: true,
				},
			}},
		})

		req := httptest.NewRequest(http.MethodGet, "/wishlist", nil)
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}

		var resp struct {
			Data []ItemResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}

		got := resp.Data[0]
		if got.Remaining != 800_000 {
			t.Errorf("remaining %d, mau 800000", got.Remaining)
		}
		if got.ProgressPct != 33.33 {
			t.Errorf("progress_pct %v, mau 33.33", got.ProgressPct)
		}
		if got.Affordability == nil || *got.Affordability.MonthsNeeded != 2 {
			t.Errorf("affordability salah: %+v", got.Affordability)
		}
	})

	t.Run("affordability null saat data historis kurang", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{
			Total: 1,
			Views: []ItemView{{Item: Item{ID: validUUID, EstimatedPrice: 100}, Affordability: nil}},
		})

		req := httptest.NewRequest(http.MethodGet, "/wishlist", nil)
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if !strings.Contains(rec.Body.String(), `"affordability":null`) {
			t.Errorf("affordability harus null, body: %s", rec.Body.String())
		}
	})

	t.Run("filter status dan sort diteruskan", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/wishlist?status=planned&sort=priority:desc", nil)
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if fake.CapturedFilter.Status != "planned" {
			t.Errorf("status %q, mau planned", fake.CapturedFilter.Status)
		}
		if fake.CapturedFilter.Sort != "priority:desc" {
			t.Errorf("sort %q, mau priority:desc", fake.CapturedFilter.Sort)
		}
	})

	t.Run("status ngawur ditolak", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{})

		req := httptest.NewRequest(http.MethodGet, "/wishlist?status=entahlah", nil)
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("limit negatif ditolak", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{})

		req := httptest.NewRequest(http.MethodGet, "/wishlist?limit=-1", nil)
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})
}

func TestHandlerCreateItem(t *testing.T) {
	t.Run("sukses", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		body := `{"name":"Keyboard","estimated_price":1200000,"priority":"high","target_date":"2026-12-01"}`
		req := httptest.NewRequest(http.MethodPost, "/wishlist", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201. body: %s", rec.Code, rec.Body.String())
		}
		if fake.CapturedCreate.TargetDate == nil ||
			timeutil.FormatDate(*fake.CapturedCreate.TargetDate) != "2026-12-01" {
			t.Errorf("target_date salah: %+v", fake.CapturedCreate.TargetDate)
		}
	})

	t.Run("json rusak", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{})

		req := httptest.NewRequest(http.MethodPost, "/wishlist", strings.NewReader(`{"name":}`))
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("target_date ngawur ditolak", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{})

		body := `{"name":"Keyboard","estimated_price":1200000,"target_date":"desember"}`
		req := httptest.NewRequest(http.MethodPost, "/wishlist", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("error validasi dari service", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{
			Err: apperror.ValidationError{Field: "estimated_price", Message: "harus lebih dari 0"},
		})

		body := `{"name":"Keyboard","estimated_price":0}`
		req := httptest.NewRequest(http.MethodPost, "/wishlist", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerWishlist(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})
}

func TestHandlerWishlistByID(t *testing.T) {
	t.Run("id bukan uuid ditolak", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/wishlist/"+invalidUUID, nil)
		req.SetPathValue("id", invalidUUID)
		rec := httptest.NewRecorder()

		h.HandlerWishlistByID(rec, req)

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
		{"patch", http.MethodPatch, `{"note":"baru"}`, http.StatusOK},
		{"delete", http.MethodDelete, "", http.StatusOK},
		{"method tidak didukung", http.MethodPut, "", http.StatusMethodNotAllowed},
	}

	for _, tt := range methods {
		t.Run(tt.name, func(t *testing.T) {
			h := NewWishlistHandler(&FakeWishlistService{})

			req := httptest.NewRequest(tt.method, "/wishlist/"+validUUID, strings.NewReader(tt.body))
			req.SetPathValue("id", validUUID)
			rec := httptest.NewRecorder()

			h.HandlerWishlistByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, mau %d", rec.Code, tt.wantStatus)
			}
		})
	}

	// target_date dan product_url punya tiga keadaan: tidak dikirim,
	// dikirim null, dikirim isi.
	t.Run("target_date null minta dikosongkan", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/wishlist/"+validUUID, strings.NewReader(`{"target_date":null}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerWishlistByID(rec, req)

		if !fake.CapturedPatch.ClearTargetDate {
			t.Error("ClearTargetDate harus true")
		}
	})

	t.Run("product_url null minta dikosongkan", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/wishlist/"+validUUID, strings.NewReader(`{"product_url":null}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerWishlistByID(rec, req)

		if !fake.CapturedPatch.ClearProductURL {
			t.Error("ClearProductURL harus true")
		}
	})

	t.Run("field yang tidak dikirim tidak menandai apa-apa", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodPatch, "/wishlist/"+validUUID, strings.NewReader(`{"note":"halo"}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerWishlistByID(rec, req)

		if fake.CapturedPatch.ClearTargetDate || fake.CapturedPatch.ClearProductURL {
			t.Error("field yang tidak dikirim tidak boleh ditandai untuk dikosongkan")
		}
	})

	t.Run("unprocessable dari service jadi 422", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{
			Err: apperror.UnprocessableError{Message: "sudah dibeli"},
		})

		req := httptest.NewRequest(http.MethodPatch, "/wishlist/"+validUUID, strings.NewReader(`{"estimated_price":100}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerWishlistByID(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d, mau 422", rec.Code)
		}
	})
}

func TestHandlerAllocate(t *testing.T) {
	t.Run("meneruskan amount ke service", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/allocate", strings.NewReader(`{"amount":400000}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerAllocate(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if fake.CapturedAmount != 400_000 {
			t.Errorf("amount %d, mau 400000", fake.CapturedAmount)
		}
	})

	t.Run("kelebihan harga jadi 400", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{
			Err: apperror.ValidationError{Field: "amount", Message: "melebihi estimated_price"},
		})

		req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/allocate", strings.NewReader(`{"amount":9999999}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerAllocate(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("item sudah dibeli jadi 422", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{
			Err: apperror.UnprocessableError{Message: "sudah dibeli"},
		})

		req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/allocate", strings.NewReader(`{"amount":1000}`))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerAllocate(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d, mau 422", rec.Code)
		}
	})
}

func TestHandlerPurchase(t *testing.T) {
	validBody := `{"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","actual_price":1150000,"occurred_at":"2026-09-15T10:00:00+07:00"}`

	t.Run("sukses", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/purchase", strings.NewReader(validBody))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerPurchase(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200. body: %s", rec.Code, rec.Body.String())
		}
		if fake.CapturedPurchase.ActualPrice != 1_150_000 {
			t.Errorf("actual_price %d, mau 1150000", fake.CapturedPurchase.ActualPrice)
		}
		if fake.CapturedPurchase.OccurredAt.IsZero() {
			t.Error("occurred_at harus diteruskan")
		}
	})

	t.Run("occurred_at boleh kosong", func(t *testing.T) {
		fake := &FakeWishlistService{}
		h := NewWishlistHandler(fake)

		body := `{"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","actual_price":1150000}`
		req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/purchase", strings.NewReader(body))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerPurchase(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, mau 200", rec.Code)
		}
	})

	invalid := []struct {
		name string
		body string
	}{
		{"json rusak", `{"actual_price":}`},
		{"wallet_id bukan uuid", `{"wallet_id":"abc","category_id":"` + validUUID2 + `","actual_price":1000}`},
		{"category_id bukan uuid", `{"wallet_id":"` + validUUID + `","category_id":"abc","actual_price":1000}`},
		{"actual_price nol", `{"wallet_id":"` + validUUID + `","category_id":"` + validUUID2 + `","actual_price":0}`},
	}

	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			fake := &FakeWishlistService{}
			h := NewWishlistHandler(fake)

			req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/purchase", strings.NewReader(tt.body))
			req.SetPathValue("id", validUUID)
			rec := httptest.NewRecorder()

			h.HandlerPurchase(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status %d, mau 400", rec.Code)
			}
			if fake.CapturedID != "" {
				t.Error("service tidak boleh dipanggil untuk request yang tidak valid")
			}
		})
	}

	t.Run("pembelian kedua jadi 422", func(t *testing.T) {
		h := NewWishlistHandler(&FakeWishlistService{
			Err: apperror.UnprocessableError{Message: "item ini sudah ditandai dibeli"},
		})

		req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/purchase", strings.NewReader(validBody))
		req.SetPathValue("id", validUUID)
		rec := httptest.NewRecorder()

		h.HandlerPurchase(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %d, mau 422", rec.Code)
		}
	})
}

func TestHandlerRestoreWishlist(t *testing.T) {
	fake := &FakeWishlistService{Item: Item{ID: validUUID, Name: "Keyboard"}}
	h := NewWishlistHandler(fake)

	req := httptest.NewRequest(http.MethodPost, "/wishlist/"+validUUID+"/restore", nil)
	req.SetPathValue("id", validUUID)
	rec := httptest.NewRecorder()

	h.HandlerRestore(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status %d, mau 200", rec.Code)
	}
	if fake.CapturedID != validUUID {
		t.Errorf("id %q, mau %q", fake.CapturedID, validUUID)
	}
}

func TestHandlerWishlistSummary(t *testing.T) {
	h := NewWishlistHandler(&FakeWishlistService{Totals: SummaryTotals{
		TotalItems: 7, TotalEstimated: 8_400_000, TotalSaved: 1_200_000,
		ByPriority: map[string]int{"high": 2, "medium": 3, "low": 2},
	}})

	req := httptest.NewRequest(http.MethodGet, "/wishlist/summary", nil)
	rec := httptest.NewRecorder()

	h.HandlerSummary(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}

	var resp struct {
		Data SummaryResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}
	if resp.Data.TotalItems != 7 || resp.Data.ByPriority["high"] != 2 {
		t.Errorf("isi salah: %+v", resp.Data)
	}
}
