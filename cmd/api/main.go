package main

import (
	"backend/internal/features/category"
	"backend/internal/platform/config"
	"backend/internal/platform/database"
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

	mux := http.NewServeMux()
	category.RegisterCategoryRoutes(mux, categoryHandler)

	svr := &http.Server{Addr: fmt.Sprintf(":%s", cfg.Port), Handler: mux}

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
