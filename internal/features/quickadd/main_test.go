package quickadd

import (
	"backend/internal/shared/testutil"
	"backend/internal/shared/timeutil"
	"database/sql"
	"log"
	"os"
	"testing"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	// Zona waktu di-set sekali di sini, sama seperti yang dilakukan main.go.
	// Tanpa ini perhitungan periode di test jatuh ke UTC.
	if err := timeutil.Init("Asia/Makassar"); err != nil {
		panic(err)
	}

	// Test yang butuh database di-skip kalau DB_URL_TEST belum di-set,
	// supaya `go test ./...` tetap jalan di mesin yang belum menyiapkan Postgres.
	testDB = testutil.TryConnectTestDB("../../../.env")

	// Kunci dipegang selama package ini jalan, supaya package lain yang
	// dijalankan barengan oleh `go test ./...` tidak ikut mengubah tabel
	// yang sama di tengah test.
	var unlock func()
	if testDB != nil {
		release, err := testutil.LockTestDB(testDB)
		if err != nil {
			log.Printf("gagal mengunci database test, test yang butuh database akan di-skip: %v", err)
			testDB.Close()
			testDB = nil
		} else {
			unlock = release
		}
	}

	if testDB != nil {
		if err := testutil.MigrateTestDB(testDB); err != nil {
			log.Printf("migration database test gagal, test yang butuh database akan di-skip: %v", err)
			unlock()
			unlock = nil
			testDB.Close()
			testDB = nil
		}
	}

	code := m.Run()

	if unlock != nil {
		unlock()
	}
	if testDB != nil {
		testDB.Close()
	}
	os.Exit(code)
}

func requireDB(t *testing.T) *sql.DB {
	t.Helper()

	if testDB == nil {
		t.Skip("database test tidak tersedia, test yang butuh database di-skip")
	}
	return testDB
}

// resetDB mengosongkan tabel supaya tiap test mulai dari keadaan yang sama.
func resetDB(t *testing.T) *sql.DB {
	t.Helper()

	db := requireDB(t)
	if err := testutil.TruncateAll(db); err != nil {
		t.Fatalf("gagal bersihkan tabel: %v", err)
	}
	return db
}
