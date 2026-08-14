package testutil

import (
	"database/sql"
	"log"
	"os"

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
