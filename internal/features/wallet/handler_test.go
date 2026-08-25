package wallet

import (
	"backend/internal/shared/apperror"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type FakeWalletService struct {
	CreateWalletFunc  func(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error)
	GetAllWalletsFunc func(ctx context.Context, limit int, offset int) ([]Wallet, int, error)
	PatchWalletFunc   func(ctx context.Context, id string, reqBody *PatchWalletRequest) (Wallet, error)
	GetWalletByIDFunc func(ctx context.Context, id string) (Wallet, error)
	DeleteWalletFunc  func(ctx context.Context, id string) (Wallet, error)
	RestoreWalletFunc func(ctx context.Context, id string) (Wallet, error)
	IsWalletExistFunc func(ctx context.Context, id string) error
}

func (h *FakeWalletService) CreateWallet(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error) {
	return h.CreateWalletFunc(ctx, reqBody)
}

func (h *FakeWalletService) GetAllWallets(ctx context.Context, limit int, offset int) ([]Wallet, int, error) {
	return h.GetAllWalletsFunc(ctx, limit, offset)
}

func (h *FakeWalletService) PatchWallet(ctx context.Context, id string, reqBody *PatchWalletRequest) (Wallet, error) {
	return h.PatchWalletFunc(ctx, id, reqBody)
}

func (h *FakeWalletService) GetWalletByID(ctx context.Context, id string) (Wallet, error) {
	return h.GetWalletByIDFunc(ctx, id)
}

func (h *FakeWalletService) DeleteWallet(ctx context.Context, id string) (Wallet, error) {
	return h.DeleteWalletFunc(ctx, id)
}

func (h *FakeWalletService) RestoreWallet(ctx context.Context, id string) (Wallet, error) {
	return h.RestoreWalletFunc(ctx, id)
}

func (h *FakeWalletService) IsWalletExist(ctx context.Context, id string) error {
	return h.IsWalletExistFunc(ctx, id)
}

func TestCreateWalletHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "success", body: `{"name":"wallet", "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusCreated},
		{name: "invalid JSON", body: `{"name":, "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "error constraint type", body: `{"name":"wallet", "type":"apanyak", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "invalid hex", body: `{"name":"wallet", "type":"bank", "initial_balance":50000, "color":"#fffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "balance less then 0", body: `{"name":"wallet", "type":"bank", "initial_balance":-50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "empty field", body: `{"name":"", "type":"", "initial_balance":0, "color":"", "icon":"", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeService := &FakeWalletService{
				CreateWalletFunc: func(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error) {
					return Wallet{Name: "wallet"}, nil
				},
			}

			h := NewWalletHandler(fakeService)

			req := httptest.NewRequest(http.MethodPost, "/wallets", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerWallets(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("create wallet handler - error already exist", func(t *testing.T) {
		fakeService := &FakeWalletService{
			CreateWalletFunc: func(ctx context.Context, reqBody *CreateWalletRequest) (Wallet, error) {
				return Wallet{}, apperror.AlreadyExistsErr{Resource: "wallets", Name: reqBody.Name, Type: reqBody.Type}
			},
		}
		h := NewWalletHandler(fakeService)

		body := `{"name":"wallet", "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`
		req := httptest.NewRequest(http.MethodPost, "/wallets", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerWallets(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, want %d", rec.Code, http.StatusConflict)
		}
	})
}

func TestGetAllWalletsHandler(t *testing.T) {
	t.Run("success without param", func(t *testing.T) {
		var getLimit, getOffset int
		fakeService := &FakeWalletService{
			GetAllWalletsFunc: func(ctx context.Context, limit, offset int) ([]Wallet, int, error) {
				getLimit = limit
				getOffset = offset
				return []Wallet{{Name: "wallet"}}, 1, nil
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/wallets", nil)
		rec := httptest.NewRecorder()

		h.HandlerWallets(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}

		if getLimit != 10 {
			t.Errorf("limit %d, mau 10", getLimit)
		}

		if getOffset != 0 {
			t.Errorf("offset %d, mau 0", getOffset)
		}
	})

	t.Run("success with valid param", func(t *testing.T) {
		var getLimit, getOffset int
		fakeService := &FakeWalletService{
			GetAllWalletsFunc: func(ctx context.Context, limit, offset int) ([]Wallet, int, error) {
				getLimit = limit
				getOffset = offset
				return []Wallet{{Name: "wallet"}}, 1, nil
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/wallets?limit=40&offset=2", nil)
		rec := httptest.NewRecorder()

		h.HandlerWallets(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}

		if getLimit != 40 {
			t.Errorf("limit %d, mau 40", getLimit)
		}

		if getOffset != 2 {
			t.Errorf("offset %d, mau 2", getOffset)
		}
	})

	t.Run("success with invalid param", func(t *testing.T) {
		var getLimit, getOffset int
		fakeService := &FakeWalletService{
			GetAllWalletsFunc: func(ctx context.Context, limit, offset int) ([]Wallet, int, error) {
				getLimit = limit
				getOffset = offset
				return []Wallet{{Name: "wallet"}}, 1, nil
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/wallets?limit=abc&offset=2", nil)
		rec := httptest.NewRecorder()

		h.HandlerWallets(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}

		if getLimit != 10 {
			t.Errorf("limit %d, mau 10", getLimit)
		}

		if getOffset != 2 {
			t.Errorf("offset %d, mau 2", getOffset)
		}
	})
}

func TestPatchWalletHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "success", body: `{"name":"new wallet", "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusOK},
		{name: "success beberapa field", body: `{"name":"new wallet", "type":"bank"}`, wantStatus: http.StatusOK},
		{name: "invalid JSON", body: `{"name":, "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "error constraint type", body: `{"name":"wallet", "type":"apanyak", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "invalid hex", body: `{"name":"wallet", "type":"bank", "initial_balance":50000, "color":"#fffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "balance less then 0", body: `{"name":"wallet", "type":"bank", "initial_balance":-50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
		{name: "empty field", body: `{"name":"", "type":"", "initial_balance":0, "color":"", "icon":"", "is_excluded_from_total":false}`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeService := &FakeWalletService{
				PatchWalletFunc: func(ctx context.Context, id string, reqBody *PatchWalletRequest) (Wallet, error) {
					return Wallet{Name: "wallet"}, nil
				},
			}

			h := NewWalletHandler(fakeService)

			req := httptest.NewRequest(http.MethodPatch, "/wallets/some-id", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerWalletByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("service error - already exist", func(t *testing.T) {
		fakeService := &FakeWalletService{
			PatchWalletFunc: func(ctx context.Context, id string, reqBody *PatchWalletRequest) (Wallet, error) {
				return Wallet{}, apperror.AlreadyExistsErr{Resource: "wallets", Name: *reqBody.Name, Type: *reqBody.Type}
			},
		}

		h := NewWalletHandler(fakeService)
		body := `{"name":"new wallet", "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`
		req := httptest.NewRequest(http.MethodPatch, "/wallets/some-id", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("service error - not found", func(t *testing.T) {
		fakeService := &FakeWalletService{
			PatchWalletFunc: func(ctx context.Context, id string, reqBody *PatchWalletRequest) (Wallet, error) {
				return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
			},
		}
		h := NewWalletHandler(fakeService)
		body := `{"name":"new wallet", "type":"bank", "initial_balance":50000, "color":"#ffffff", "icon":"wallet", "is_excluded_from_total":false}`
		req := httptest.NewRequest(http.MethodPatch, "/wallets/some-id", strings.NewReader(body))
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestGetWalletByIDHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeService := &FakeWalletService{
			GetWalletByIDFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{Name: "wallet"}, nil
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/wallets/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("service error - not found", func(t *testing.T) {
		fakeService := &FakeWalletService{
			GetWalletByIDFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/wallets/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestDeleteWalletHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeService := &FakeWalletService{
			DeleteWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{Name: "name"}, nil
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodDelete, "/wallets/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("error service - not found", func(t *testing.T) {
		fakeService := &FakeWalletService{
			DeleteWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
			},
		}

		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodDelete, "/wallets/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestRestoreWalletHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeService := &FakeWalletService{
			RestoreWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{Name: "wallet"}, nil
			},
		}
		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodPost, "/wallets/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("error service - not found", func(t *testing.T) {
		fakeService := &FakeWalletService{
			RestoreWalletFunc: func(ctx context.Context, id string) (Wallet, error) {
				return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
			},
		}
		h := NewWalletHandler(fakeService)

		req := httptest.NewRequest(http.MethodPost, "/wallets/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerWalletByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}
