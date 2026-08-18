package platform

import (
	"backend/internal/features/category"
	"backend/internal/features/wallet"
	"backend/internal/platform/middleware"
	"net/http"
)

type Handlers struct {
	Category *category.CategoryHandler
	Wallet   *wallet.WalletHandler
}

func NewRouter(h Handlers) http.Handler {
	mux := http.NewServeMux()
	category.RegisterCategoryRoutes(mux, *h.Category)
	wallet.RegisterWalletRoutes(mux, *h.Wallet)

	return middleware.Logging(mux.ServeHTTP)
}
