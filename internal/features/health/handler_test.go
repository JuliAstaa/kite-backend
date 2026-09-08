package health

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHandlerHealthSaatDatabaseHidup(t *testing.T) {
	db := requireDB(t)
	h := NewHealthHandler(db, "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HandlerHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}

	var resp struct {
		Data HealthResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}

	if resp.Data.Status != "ok" {
		t.Errorf("status %q, mau ok", resp.Data.Status)
	}
	if resp.Data.DB != "ok" {
		t.Errorf("db %q, mau ok", resp.Data.DB)
	}
	if resp.Data.Version != "2.0.0" {
		t.Errorf("version %q, mau 2.0.0", resp.Data.Version)
	}
}

// Database mati harus jadi 503, bukan 200 dengan status ok. Endpoint ini
// dipakai untuk memutuskan apakah aplikasi layak menerima trafik.
func TestHandlerHealthSaatDatabaseMati(t *testing.T) {
	// Koneksi ke alamat yang memang tidak ada, jadi Ping-nya pasti gagal.
	db, err := sql.Open("pgx", "postgres://nobody:nobody@127.0.0.1:1/nihil?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("gagal buka koneksi palsu: %v", err)
	}
	defer db.Close()

	h := NewHealthHandler(db, "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HandlerHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, mau 503", rec.Code)
	}

	var resp struct {
		Data HealthResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response tidak sesuai bentuk: %v", err)
	}

	if resp.Data.DB != "down" {
		t.Errorf("db %q, mau down", resp.Data.DB)
	}
	if resp.Data.Status != "degraded" {
		t.Errorf("status %q, mau degraded", resp.Data.Status)
	}
	// Versi tetap dilaporkan supaya tetap kelihatan build mana yang jalan.
	if resp.Data.Version != "2.0.0" {
		t.Errorf("version %q, mau 2.0.0", resp.Data.Version)
	}
}

// Koneksi yang sudah ditutup juga dihitung sebagai database mati.
func TestHandlerHealthSaatKoneksiDitutup(t *testing.T) {
	requireDB(t)

	db, err := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/nihil")
	if err != nil {
		t.Fatalf("gagal buka koneksi: %v", err)
	}
	db.Close()

	h := NewHealthHandler(db, "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HandlerHealth(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status %d, mau 503", rec.Code)
	}
}
