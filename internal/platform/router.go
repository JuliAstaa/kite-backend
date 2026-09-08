package platform

import (
	"backend/internal/features/analytics"
	"backend/internal/features/backup"
	"backend/internal/features/budget"
	"backend/internal/features/category"
	"backend/internal/features/health"
	"backend/internal/features/quickadd"
	"backend/internal/features/recurring"
	"backend/internal/features/saving"
	"backend/internal/features/transaction"
	"backend/internal/features/wallet"
	"backend/internal/features/wishlist"
	"backend/internal/platform/middleware"
	"net/http"
)

// BasePath adalah awalan semua endpoint. Tiap feature mendaftarkan route-nya
// tanpa awalan ini, lalu StripPrefix yang memasangnya sekali di sini.
const BasePath = "/api/v1"

type Handlers struct {
	Health      *health.HealthHandler
	Category    *category.CategoryHandler
	Wallet      *wallet.WalletHandler
	Transaction *transaction.TransactionHandler
	Analytics   *analytics.AnalyticsHandler
	Saving      *saving.SavingHandler
	Wishlist    *wishlist.WishlistHandler
	Budget      *budget.BudgetHandler
	Recurring   *recurring.RecurringHandler
	QuickAdd    *quickadd.QuickAddHandler
	Backup      *backup.BackupHandler
}

type RouterConfig struct {
	AllowedOrigins []string
	APIToken       string
}

func NewRouter(h Handlers, cfg RouterConfig) http.Handler {
	api := http.NewServeMux()

	health.RegisterHealthRoutes(api, h.Health)
	category.RegisterCategoryRoutes(api, h.Category)
	wallet.RegisterWalletRoutes(api, h.Wallet)
	transaction.RegisterTransactionRoutes(api, h.Transaction)
	analytics.RegisterAnalyticsRoutes(api, h.Analytics)
	saving.RegisterSavingRoutes(api, h.Saving)
	wishlist.RegisterWishlistRoutes(api, h.Wishlist)
	budget.RegisterBudgetRoutes(api, h.Budget)
	recurring.RegisterRecurringRoutes(api, h.Recurring)
	quickadd.RegisterQuickAddRoutes(api, h.QuickAdd)
	backup.RegisterBackupRoutes(api, h.Backup)

	root := http.NewServeMux()
	root.Handle(BasePath+"/", http.StripPrefix(BasePath, api))

	// Urutannya dari luar ke dalam: recover paling luar supaya panic di
	// middleware manapun tetap tertangkap, logging berikutnya supaya semua
	// request tercatat termasuk yang ditolak token.
	return middleware.Chain(root,
		middleware.Recover,
		middleware.Logging,
		middleware.CORS(cfg.AllowedOrigins),
		middleware.APIToken(cfg.APIToken),
	)
}
