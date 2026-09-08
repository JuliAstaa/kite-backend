package testutil

import (
	"backend/db"
	"backend/internal/platform/database"
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

func ConnectTestDB(envPath string) *sql.DB {
	godotenv.Load(envPath)
	dsn := os.Getenv("DB_URL_TEST")
	if dsn == "" {
		log.Fatal("DB URL TEST blum di set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	return db
}

// connectTimeout membatasi berapa lama test menunggu database test.
//
// Tanpa batas ini, database yang mati (beda dengan yang memang belum di-set)
// bikin tiap package menunggu timeout TCP dulu, sekitar 75 detik, sebelum
// akhirnya di-skip. Lebih baik gagal cepat.
const connectTimeout = 5 * time.Second

// TryConnectTestDB sama dengan ConnectTestDB tapi mengembalikan nil kalau
// database test tidak tersedia. Dipakai test yang boleh di-skip di mesin yang
// belum menyiapkan Postgres, supaya `go test ./...` tetap bisa jalan.
func TryConnectTestDB(envPath string) *sql.DB {
	godotenv.Load(envPath)
	dsn := os.Getenv("DB_URL_TEST")
	if dsn == "" {
		return nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Printf("DB_URL_TEST tidak bisa dibuka (%v), test yang butuh database di-skip", err)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("database test tidak bisa dihubungi (%v), test yang butuh database di-skip", err)
		db.Close()
		return nil
	}

	return db
}

// HasTable memberi tahu apakah sebuah tabel ada di schema public.
func HasTable(db *sql.DB, name string) bool {
	var exists bool
	err := db.QueryRow(`SELECT to_regclass('public.' || $1) IS NOT NULL`, name).Scan(&exists)
	return err == nil && exists
}

// MigrateTestDB menjalankan migration di database test, supaya test yang
// memakai tabel baru tidak perlu menunggu migration dijalankan manual.
func MigrateTestDB(sqlDB *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return database.RunMigrations(ctx, sqlDB, db.MigrationFS, db.MigrationDir)
}

// TruncateAll mengosongkan tabel data. Dipakai test repository supaya tiap
// test mulai dari keadaan yang sama.
//
// CASCADE dipakai karena wishlist_items dan transactions saling menunjuk
// (purchase_transaction_id dan wishlist_item_id), jadi tidak ada urutan
// penghapusan yang aman kalau dikerjakan satu per satu.
func TruncateAll(sqlDB *sql.DB) error {
	_, err := sqlDB.Exec(`TRUNCATE quick_adds, budgets, savings_targets,
		wishlist_items, recurring_rules, transactions, wallets, categories CASCADE`)
	return err
}

// InsertWallet menyiapkan satu wallet dan mengembalikan id-nya.
func InsertWallet(sqlDB *sql.DB, name, walletType string, initialBalance int) (string, error) {
	var id string
	err := sqlDB.QueryRow(
		`INSERT INTO wallets(name, type, initial_balance) VALUES($1, $2, $3) RETURNING id`,
		name, walletType, initialBalance).Scan(&id)
	return id, err
}

// InsertCategory menyiapkan satu kategori dan mengembalikan id-nya.
func InsertCategory(sqlDB *sql.DB, name, catType string) (string, error) {
	var id string
	err := sqlDB.QueryRow(
		`INSERT INTO categories(name, type) VALUES($1, $2) RETURNING id`,
		name, catType).Scan(&id)
	return id, err
}

// InsertTransaction menyiapkan satu transaksi income atau expense.
func InsertTransaction(sqlDB *sql.DB, txType string, amount int, walletID, categoryID string, occurredAt time.Time) (string, error) {
	var id string
	err := sqlDB.QueryRow(
		`INSERT INTO transactions(type, amount, wallet_id, category_id, note, occurred_at)
		VALUES($1, $2, $3, $4, '', $5) RETURNING id`,
		txType, amount, walletID, categoryID, occurredAt).Scan(&id)
	return id, err
}

// InsertTransfer menyiapkan satu transaksi transfer antar wallet.
func InsertTransfer(sqlDB *sql.DB, amount int, fromWalletID, toWalletID string, occurredAt time.Time) (string, error) {
	var id string
	err := sqlDB.QueryRow(
		`INSERT INTO transactions(type, amount, wallet_id, to_wallet_id, note, occurred_at)
		VALUES('transfer', $1, $2, $3, '', $4) RETURNING id`,
		amount, fromWalletID, toWalletID, occurredAt).Scan(&id)
	return id, err
}

// testDBLockKey adalah angka bebas, cuma perlu sama di semua package.
const testDBLockKey = 918273645

// LockTestDB mengambil advisory lock supaya cuma satu package yang memakai
// database test pada satu waktu.
//
// `go test ./...` menjalankan package secara paralel, sementara test repository
// berbagi satu database dan saling TRUNCATE tabel. Tanpa kunci ini, package
// yang jalan barengan saling menghapus data lawannya dan hasilnya jadi FK
// violation atau duplicate key yang membingungkan, padahal kodenya benar.
//
// Lock-nya dipegang di satu koneksi khusus selama package berjalan. Postgres
// otomatis melepasnya kalau proses test mati di tengah jalan, jadi tidak ada
// kunci nyangkut.
func LockTestDB(sqlDB *sql.DB) (release func(), err error) {
	ctx := context.Background()

	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, testDBLockKey); err != nil {
		conn.Close()
		return nil, err
	}

	return func() {
		conn.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, testDBLockKey)
		conn.Close()
	}, nil
}
