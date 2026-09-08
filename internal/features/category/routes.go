package category

import "net/http"

func RegisterCategoryRoutes(mux *http.ServeMux, h *CategoryHandler) {
	mux.HandleFunc("/categories", h.HandlerCategories)
	mux.HandleFunc("/categories/{id}", h.HandlerCategoryByID)
	mux.HandleFunc("POST /categories/{id}/restore", h.HandlerRestoreCategory)
}
