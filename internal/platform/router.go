package platform

import (
	"backend/internal/features/category"
	"backend/internal/platform/middleware"
	"net/http"
)

type Handlers struct {
	Category *category.CategoryHandler
}

func NewRouter(h Handlers) http.Handler {
	mux := http.NewServeMux()
	category.RegisterCategoryRoutes(mux, *h.Category)

	return middleware.Logging(mux.ServeHTTP)
}
