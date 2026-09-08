.PHONY: help run build test test-unit vet fmt tidy db-up db-down db-logs migrate-status backup clean

APP_NAME := finance-api
BIN_DIR  := bin
DB_URL   ?= $(shell grep -E '^DB_URL=' .env 2>/dev/null | cut -d= -f2-)
BACKUP_DIR := backups

help: ## Tampilkan daftar perintah
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

run: ## Jalankan server (migration otomatis jalan saat startup)
	go run ./cmd/api

build: ## Build binary ke bin/
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/api

test: ## Jalankan semua test (butuh DB_URL_TEST untuk test repository)
	# -p 1 supaya package tidak jalan barengan. Test repository berbagi satu
	# database test dan saling TRUNCATE tabel, jadi harus antre.
	go test -p 1 ./...

test-unit: ## Jalankan test yang tidak butuh database
	go test -short ./internal/features/... ./internal/shared/...

vet: ## go vet
	go vet ./...

fmt: ## Rapikan format kode
	gofmt -w ./cmd ./db ./internal

tidy: ## Rapikan go.mod
	go mod tidy

db-up: ## Nyalakan Postgres lewat Docker Compose
	docker compose up -d

db-down: ## Matikan Postgres
	docker compose down

db-logs: ## Lihat log Postgres
	docker compose logs -f db

migrate-status: ## Lihat migration yang sudah jalan
	@echo "Migration dijalankan otomatis saat 'make run'."
	@echo "File yang tersedia:"
	@ls -1 db/migrations/*.up.sql

backup: ## Dump database ke backups/
	@mkdir -p $(BACKUP_DIR)
	pg_dump -Fc "$(DB_URL)" > $(BACKUP_DIR)/finance_$$(date +%Y%m%d_%H%M%S).dump
	@echo "backup tersimpan di $(BACKUP_DIR)/"

clean: ## Bersihkan hasil build
	rm -rf $(BIN_DIR)
