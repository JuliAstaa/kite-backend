package database

import (
	"database/sql"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func ConnectDB(DBUrl string) *sql.DB {
	db, err := sql.Open("pgx", DBUrl)
	if err != nil {
		log.Fatalf("gagal buka koneksi ke database: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("gagal ping database: %v", err)
	}

	log.Println("Koneksi berhasil")
	return db
}
