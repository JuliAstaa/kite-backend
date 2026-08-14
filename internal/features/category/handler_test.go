package category

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateCategory(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "nama kosong", body: `{"name":"", "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "type selain expense dan icome", body: `{"name":"makanan", "type":"lainnya", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "invalid hex", body: `{"name":"", "type":"expense", "color":"inicolor","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
		{name: "JSON rusak", body: `{"name":, "type":"expense", "color":"#FFFFFF","icon":"iniicon"}`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewCategoryHandler(nil)

			req := httptest.NewRequest(http.MethodPost, "/categories", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.HandlerCategories(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
