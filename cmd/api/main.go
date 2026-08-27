package main

import (
	"backend/internal/features/category"
	"backend/internal/features/transaction"
	"backend/internal/features/wallet"
	"backend/internal/platform"
	"backend/internal/platform/config"
	"backend/internal/platform/database"
	"backend/internal/platform/wiring"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {

	cfg := config.Load()
	db := database.ConnectDB(cfg.DBUrl)
	defer db.Close()

	categoryRepo := category.NewCategoryRepository(db)
	categoryService := category.NewCategoryService(categoryRepo)
	categoryHandler := category.NewCategoryHandler(categoryService)

	walletRepo := wallet.NewWalletRepository(db)
	walletService := wallet.NewWalletService(walletRepo)
	walletHandler := wallet.NewWalletHandler(walletService)

	// adapter
	categoryAdapter := wiring.NewCategoryCheckerAdapter(categoryService)

	transactionRepo := transaction.NewTransactionRepository(db)
	transactionService := transaction.NewTransactionService(transactionRepo, walletService, categoryAdapter)
	transactionHandler := transaction.NewTransactionHandler(transactionService)

	router := platform.NewRouter(platform.Handlers{
		Category:    categoryHandler,
		Wallet:      walletHandler,
		Transaction: transactionHandler,
	})

	svr := &http.Server{Addr: fmt.Sprintf(":%s", cfg.Port), Handler: router}

	go func() {
		if err := svr.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	log.Println("server started")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Println("shutting down sever...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svr.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("server stopped")
}
