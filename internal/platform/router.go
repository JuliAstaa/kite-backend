package platform

import (
	"backend/internal/features/category"
	"backend/internal/features/transaction"
	"backend/internal/features/wallet"
	"backend/internal/platform/middleware"
	"net/http"
)

type Handlers struct {
	Category    *category.CategoryHandler
	Wallet      *wallet.WalletHandler
	Transaction *transaction.TransactionHandler
}

func NewRouter(h Handlers) http.Handler {
	mux := http.NewServeMux()
	category.RegisterCategoryRoutes(mux, *h.Category)
	wallet.RegisterWalletRoutes(mux, *h.Wallet)
	transaction.RegisterTransactionRoutes(mux, *h.Transaction)

	return middleware.Logging(mux.ServeHTTP)
}
