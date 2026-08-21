package category

import (
	"backend/internal/shared/apperror"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type FakeCategoryService struct {
	CreateCategoryFunc   func(ctx context.Context, requestBody *CreateCategoryRequest) (Category, error)
	GetAllCategoriesFunc func(ctx context.Context, limit int, offset int) ([]Category, int, error)
	PatchCategoryFunc    func(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error)
	DeleteCategoryFunc   func(ctx context.Context, id string) (Category, error)
	GetCategoryByIDFunc  func(ctx context.Context, id string) (Category, error)
	RestoreCategoryFunc  func(ctx context.Context, id string) (Category, error)
}

func (s *FakeCategoryService) CreateCategory(ctx context.Context, requestBody *CreateCategoryRequest) (Category, error) {
	return s.CreateCategoryFunc(ctx, requestBody)
}

func (s *FakeCategoryService) GetAllCategories(ctx context.Context, limit int, offset int) ([]Category, int, error) {
	return s.GetAllCategoriesFunc(ctx, limit, offset)
}

func (s *FakeCategoryService) PatchCategory(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error) {
	return s.PatchCategoryFunc(ctx, id, requestBody)
}

func (s *FakeCategoryService) DeleteCategory(ctx context.Context, id string) (Category, error) {
	return s.DeleteCategoryFunc(ctx, id)
}

func (s *FakeCategoryService) GetCategoryByID(ctx context.Context, id string) (Category, error) {
	return s.GetCategoryByIDFunc(ctx, id)
}

func (s *FakeCategoryService) RestoreCategory(ctx context.Context, id string) (Category, error) {
	return s.RestoreCategoryFunc(ctx, id)
}

func TestCreateCategoryHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "nama kosong", body: `{"name":"", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "type selain expense dan icome", body: `{"name":"makanan", "type":"lainnya", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "invalid hex", body: `{"name":"", "type":"expense", "color":"inicolor","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "JSON rusak", body: `{"name":, "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "sukses", body: `{"name":"Makan", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusCreated},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fakeService := &FakeCategoryService{
				CreateCategoryFunc: func(ctx context.Context, requestBody *CreateCategoryRequest) (Category, error) {
					return Category{Name: "makan"}, nil
				},
			}

			h := NewCategoryHandler(fakeService)

			req := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerCategories(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("service error - already exist", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			CreateCategoryFunc: func(ctx context.Context, requestBody *CreateCategoryRequest) (Category, error) {
				return Category{}, apperror.AlreadyExistsErr{Resource: "categories", Name: requestBody.Name, Type: requestBody.Type}
			},
		}

		h := NewCategoryHandler(fakeService)

		body := `{"name":"Makan", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`
		req := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandlerCategories(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, want %d", rec.Code, http.StatusConflict)
		}
	})
}

func TestGetAllCategoriesHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var getLimit, getOffset int
		fakeService := &FakeCategoryService{
			GetAllCategoriesFunc: func(ctx context.Context, limit, offset int) ([]Category, int, error) {
				getLimit = limit
				getOffset = offset
				return []Category{{Name: "hallo"}}, 1, nil
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/categories", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategories(rec, req)

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

	t.Run("success - valid param", func(t *testing.T) {
		var getLimit, getOffset int

		fakeService := &FakeCategoryService{
			GetAllCategoriesFunc: func(ctx context.Context, limit, offset int) ([]Category, int, error) {
				getLimit = limit
				getOffset = offset
				return []Category{{Name: "hallo"}}, 1, nil
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/categories?limit=5&offset=10", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategories(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}

		if getLimit != 5 {
			t.Errorf("limit %d, mau 5", getLimit)
		}

		if getOffset != 10 {
			t.Errorf("offset %d, mau 10", getOffset)
		}
	})

	t.Run("success - invalid param", func(t *testing.T) {
		var getLimit, getOffset int

		fakeService := &FakeCategoryService{
			GetAllCategoriesFunc: func(ctx context.Context, limit, offset int) ([]Category, int, error) {
				getLimit = limit
				getOffset = offset
				return []Category{{Name: "hallo"}}, 1, nil
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/categories?limit=abc&offset=10", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategories(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}

		if getLimit != 10 {
			t.Errorf("limit %d, mau 10", getLimit)
		}

		if getOffset != 10 {
			t.Errorf("offset %d, mau 10", getOffset)
		}
	})

}

func TestPatchCategoryHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "nama kosong", body: `{"name":"", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "type selain expense dan icome", body: `{"name":"makanan", "type":"lainnya", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "invalid hex", body: `{"name":"", "type":"expense", "color":"inicolor","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "JSON rusak", body: `{"name":, "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "sukses", body: `{"name":"Makan", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusOK},
		{name: "patch sebagian field saja", body: `{"color":"#FF0000"}`, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fakeService := &FakeCategoryService{
				PatchCategoryFunc: func(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error) {
					return Category{Name: "ikan"}, nil
				},
			}

			h := NewCategoryHandler(fakeService)

			req := httptest.NewRequest(http.MethodPatch, "/category/some-id", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerCategoryByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}

	t.Run("service error - already exist", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			PatchCategoryFunc: func(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error) {
				return Category{}, apperror.AlreadyExistsErr{Resource: "categories", Name: *requestBody.Name, Type: *requestBody.Type}
			},
		}

		h := NewCategoryHandler(fakeService)

		body := `{"name":"Makan", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`
		req := httptest.NewRequest(http.MethodPatch, "/category/some-id", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("service error - not found", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			PatchCategoryFunc: func(ctx context.Context, id string, requestBody *PatchCategoryRequest) (Category, error) {
				return Category{}, apperror.NotFoundError{Resource: "categories", ID: "some-id"}
			},
		}

		h := NewCategoryHandler(fakeService)

		body := `{"name":"Makan", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`
		req := httptest.NewRequest(http.MethodPatch, "/category/some-id", strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

}

func TestDeleteCategoryHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			DeleteCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{Name: "hai"}, nil
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodDelete, "/category/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			DeleteCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{}, apperror.NotFoundError{Resource: "categories", ID: "some-id"}
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodDelete, "/category/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

}
func TestGetCategoryByIDHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			GetCategoryByIDFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{Name: "hai"}, nil
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/category/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			GetCategoryByIDFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{}, apperror.NotFoundError{Resource: "categories", ID: "some-id"}
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodGet, "/category/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

}
func TestRestoreCategoryHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			RestoreCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{Name: "hai"}, nil
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodPost, "/category/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("not found", func(t *testing.T) {
		fakeService := &FakeCategoryService{
			RestoreCategoryFunc: func(ctx context.Context, id string) (Category, error) {
				return Category{}, apperror.NotFoundError{Resource: "categories", ID: "some-id"}
			},
		}

		h := NewCategoryHandler(fakeService)

		req := httptest.NewRequest(http.MethodPost, "/category/some-id", nil)
		rec := httptest.NewRecorder()

		h.HandlerCategoryByID(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

}
