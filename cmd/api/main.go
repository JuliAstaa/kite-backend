package main

import (
	"backend/db"
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
	"backend/internal/platform"
	"backend/internal/platform/config"
	"backend/internal/platform/database"
	"backend/internal/platform/wiring"
	"backend/internal/shared/timeutil"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()

	// Zona waktu di-set sekali di sini. Semua perhitungan periode setelah ini
	// memakai waktu lokal, bukan UTC.
	if err := timeutil.Init(cfg.TZ); err != nil {
		log.Fatal(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	conn := database.ConnectDB(cfg.DBUrl)
	defer conn.Close()

	conn.SetMaxOpenConns(cfg.DBMaxConns)
	conn.SetMaxIdleConns(cfg.DBIdleConns)
	conn.SetConnMaxLifetime(time.Hour)

	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 30*time.Second)
	if err := database.RunMigrations(migrateCtx, conn, db.MigrationFS, db.MigrationDir); err != nil {
		cancelMigrate()
		log.Fatalf("migration gagal: %v", err)
	}
	cancelMigrate()

	// --- feature dirakit satu per satu, dari repository ke handler ---

	categoryRepo := category.NewCategoryRepository(conn)
	categoryService := category.NewCategoryService(categoryRepo)
	categoryHandler := category.NewCategoryHandler(categoryService)

	walletRepo := wallet.NewWalletRepository(conn)
	walletService := wallet.NewWalletService(walletRepo)
	walletHandler := wallet.NewWalletHandler(walletService)

	transactionRepo := transaction.NewTransactionRepository(conn)
	transactionService := transaction.NewTransactionService(
		transactionRepo,
		walletService,
		wiring.NewTransactionCategoryReader(categoryService),
	)
	transactionHandler := transaction.NewTransactionHandler(transactionService)

	analyticsRepo := analytics.NewAnalyticsRepository(conn)
	analyticsService := analytics.NewAnalyticsService(analyticsRepo, walletService)
	analyticsHandler := analytics.NewAnalyticsHandler(analyticsService)

	savingRepo := saving.NewSavingRepository(conn)
	savingService := saving.NewSavingService(savingRepo)
	savingHandler := saving.NewSavingHandler(savingService)

	wishlistRepo := wishlist.NewWishlistRepository(conn)
	wishlistService := wishlist.NewWishlistService(
		wishlistRepo,
		wiring.NewWishlistSavingsReader(savingService),
		walletService,
		wiring.NewWishlistCategoryReader(categoryService),
	)
	wishlistHandler := wishlist.NewWishlistHandler(wishlistService)

	budgetRepo := budget.NewBudgetRepository(conn)
	budgetService := budget.NewBudgetService(budgetRepo, wiring.NewBudgetCategoryReader(categoryService))
	budgetHandler := budget.NewBudgetHandler(budgetService)

	recurringRepo := recurring.NewRecurringRepository(conn)
	recurringService := recurring.NewRecurringService(
		recurringRepo,
		walletService,
		wiring.NewRecurringCategoryReader(categoryService),
		logger,
	)
	recurringHandler := recurring.NewRecurringHandler(recurringService)

	quickAddRepo := quickadd.NewQuickAddRepository(conn)
	quickAddService := quickadd.NewQuickAddService(
		quickAddRepo,
		walletService,
		wiring.NewQuickAddCategoryReader(categoryService),
		wiring.NewQuickAddTransactionCreator(transactionService),
	)
	quickAddHandler := quickadd.NewQuickAddHandler(quickAddService)

	backupRepo := backup.NewBackupRepository(conn)
	backupService := backup.NewBackupService(backupRepo)
	backupHandler := backup.NewBackupHandler(backupService)

	healthHandler := health.NewHealthHandler(conn, config.Version)

	router := platform.NewRouter(platform.Handlers{
		Health:      healthHandler,
		Category:    categoryHandler,
		Wallet:      walletHandler,
		Transaction: transactionHandler,
		Analytics:   analyticsHandler,
		Saving:      savingHandler,
		Wishlist:    wishlistHandler,
		Budget:      budgetHandler,
		Recurring:   recurringHandler,
		QuickAdd:    quickAddHandler,
		Backup:      backupHandler,
	}, platform.RouterConfig{
		AllowedOrigins: cfg.CORSAllowedOrigins,
		APIToken:       cfg.APIToken,
	})

	// Scheduler transaksi berulang. Satu goroutine, berhenti saat ctx dibatalkan.
	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	defer stopScheduler()
	recurring.NewScheduler(recurringService, time.Hour, logger).Start(schedulerCtx)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("server jalan", "addr", server.Addr, "env", cfg.AppEnv, "tz", cfg.TZ)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("server dimatikan...")
	stopScheduler()

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}

	logger.Info("server berhenti")
}
