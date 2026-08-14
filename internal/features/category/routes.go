package category

import "net/http"

func RegisterCategoryRoutes(mux *http.ServeMux, h CategoryHandler) {
	mux.HandleFunc("/categories", h.HandlerCategories)
	mux.HandleFunc("/category/", h.HandlerCategoryByID)
}
