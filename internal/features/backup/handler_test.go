package backup

import (
	"backend/internal/shared/timeutil"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type FakeBackupService struct {
	Bundle ExportBundle
	Result ImportResult
	Err    error

	CapturedFrom   time.Time
	CapturedTo     time.Time
	CapturedDryRun bool
	CapturedBody   string
	ImportCalls    int
}

func (f *FakeBackupService) Export(ctx context.Context, from, to time.Time) (ExportBundle, error) {
	f.CapturedFrom, f.CapturedTo = from, to
	return f.Bundle, f.Err
}

func (f *FakeBackupService) Import(ctx context.Context, reader io.Reader, dryRun bool) (ImportResult, error) {
	f.ImportCalls++
	f.CapturedDryRun = dryRun

	body, _ := io.ReadAll(reader)
	f.CapturedBody = string(body)

	if f.Err != nil {
		return ImportResult{}, f.Err
	}
	result := f.Result
	result.DryRun = dryRun
	return result, nil
}

func TestHandlerExportJSON(t *testing.T) {
	t.Run("format default json", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{Bundle: ExportBundle{
			ExportedAt: timeutil.Now(), From: "2026-08-01", To: "2026-08-31",
			Wallets:      []ExportWallet{{ID: "w1", Name: "Cash"}},
			Transactions: []ExportTransaction{{ID: "t1", Type: "expense", Amount: 25_000}},
		}})

		req := httptest.NewRequest(http.MethodGet, "/export", nil)
		rec := httptest.NewRecorder()

		h.HandlerExport(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type %q, mau application/json", ct)
		}
		if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, ".json") {
			t.Errorf("Content-Disposition %q, mau menyebut file json", cd)
		}

		var resp struct {
			Data ExportBundle `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak dibungkus data: %v", err)
		}
		if len(resp.Data.Wallets) != 1 {
			t.Errorf("wallets kurang: %+v", resp.Data)
		}
	})

	t.Run("rentang default setahun terakhir", func(t *testing.T) {
		fake := &FakeBackupService{}
		h := NewBackupHandler(fake)

		req := httptest.NewRequest(http.MethodGet, "/export", nil)
		rec := httptest.NewRecorder()

		h.HandlerExport(rec, req)

		if fake.CapturedFrom.After(timeutil.Now().AddDate(0, -11, 0)) {
			t.Errorf("from %s, mau sekitar setahun lalu", timeutil.FormatDate(fake.CapturedFrom))
		}
	})

	t.Run("format ngawur ditolak", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{})

		req := httptest.NewRequest(http.MethodGet, "/export?format=xml", nil)
		rec := httptest.NewRecorder()

		h.HandlerExport(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("tanggal ngawur ditolak", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{})

		req := httptest.NewRequest(http.MethodGet, "/export?from=kemarin", nil)
		rec := httptest.NewRecorder()

		h.HandlerExport(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("error service jadi 500", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{Err: errors.New("database mati")})

		req := httptest.NewRequest(http.MethodGet, "/export", nil)
		rec := httptest.NewRecorder()

		h.HandlerExport(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status %d, mau 500", rec.Code)
		}
	})
}

func TestHandlerExportCSV(t *testing.T) {
	occurredAt := time.Date(2026, time.August, 11, 9, 30, 0, 0, timeutil.Loc())

	h := NewBackupHandler(&FakeBackupService{Bundle: ExportBundle{
		Transactions: []ExportTransaction{
			{ID: "t1", Type: "expense", Amount: 25_000, Wallet: "Cash", Category: "Makan & Minum", Note: "Kopi susu", OccurredAt: occurredAt},
			{ID: "t2", Type: "transfer", Amount: 100_000, Wallet: "Cash", ToWallet: "BCA", Note: "pindah", OccurredAt: occurredAt},
		},
	}})

	req := httptest.NewRequest(http.MethodGet, "/export?format=csv", nil)
	rec := httptest.NewRecorder()

	h.HandlerExport(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type %q, mau text/csv", ct)
	}

	lines := strings.Split(strings.TrimSpace(rec.Body.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("dapat %d baris, mau 3 (header + 2 transaksi)", len(lines))
	}

	if strings.TrimSpace(lines[0]) != "date,type,amount,wallet,category,note" {
		t.Errorf("header %q tidak sesuai", lines[0])
	}
	// Tanggal ditulis pakai zona waktu aplikasi, bukan UTC.
	if !strings.HasPrefix(lines[1], "2026-08-11,expense,25000,Cash,Makan & Minum,") {
		t.Errorf("baris pertama %q tidak sesuai", lines[1])
	}
	// Transfer tetap diekspor supaya backup lengkap, kolom kategorinya kosong.
	if !strings.Contains(lines[2], "transfer,100000,Cash,,") {
		t.Errorf("baris transfer %q tidak sesuai", lines[2])
	}
}

// multipartBody menyusun body multipart/form-data berisi satu file.
func multipartBody(t *testing.T, fieldName, fileName, content string) (io.Reader, string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("gagal bikin form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("gagal tulis isi file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("gagal tutup writer: %v", err)
	}

	return &buf, writer.FormDataContentType()
}

func TestHandlerImport(t *testing.T) {
	csv := "date,type,amount,wallet,category,note\n2026-08-11,expense,25000,Cash,Makan,Kopi\n"

	t.Run("dry run default true", func(t *testing.T) {
		fake := &FakeBackupService{Result: ImportResult{TotalRows: 1, ValidRows: 1}}
		h := NewBackupHandler(fake)

		body, contentType := multipartBody(t, "file", "data.csv", csv)
		req := httptest.NewRequest(http.MethodPost, "/import", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status %d, mau 200. body: %s", rec.Code, rec.Body.String())
		}
		// Tanpa dry_run yang jelas, jangan langsung menulis ke database.
		if !fake.CapturedDryRun {
			t.Error("dry_run harus default true")
		}
		if fake.CapturedBody != csv {
			t.Errorf("isi file tidak sampai ke service: %q", fake.CapturedBody)
		}
	})

	t.Run("dry_run=false menulis", func(t *testing.T) {
		fake := &FakeBackupService{Result: ImportResult{TotalRows: 1, ValidRows: 1, Imported: 1}}
		h := NewBackupHandler(fake)

		body, contentType := multipartBody(t, "file", "data.csv", csv)
		req := httptest.NewRequest(http.MethodPost, "/import?dry_run=false", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status %d, mau 201", rec.Code)
		}
		if fake.CapturedDryRun {
			t.Error("dry_run harus false")
		}
	})

	t.Run("laporan error per baris ikut dikembalikan", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{Result: ImportResult{
			TotalRows: 3, ValidRows: 1, InvalidRows: 2,
			Errors: []RowError{
				{Line: 3, Message: "amount harus angka"},
				{Line: 4, Message: `wallet "Dompet Hantu" tidak ditemukan`},
			},
		}})

		body, contentType := multipartBody(t, "file", "data.csv", csv)
		req := httptest.NewRequest(http.MethodPost, "/import?dry_run=true", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		var resp struct {
			Data ImportResponse `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response tidak sesuai bentuk: %v", err)
		}
		if len(resp.Data.Errors) != 2 || resp.Data.Errors[0].Line != 3 {
			t.Errorf("daftar error salah: %+v", resp.Data.Errors)
		}
	})

	t.Run("errors array kosong bukan null", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{Result: ImportResult{
			TotalRows: 1, ValidRows: 1, Errors: []RowError{},
		}})

		body, contentType := multipartBody(t, "file", "data.csv", csv)
		req := httptest.NewRequest(http.MethodPost, "/import", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if !strings.Contains(rec.Body.String(), `"errors":[]`) {
			t.Errorf("body %s, mau errors array kosong", rec.Body.String())
		}
	})

	t.Run("bukan multipart ditolak", func(t *testing.T) {
		fake := &FakeBackupService{}
		h := NewBackupHandler(fake)

		req := httptest.NewRequest(http.MethodPost, "/import", strings.NewReader(csv))
		req.Header.Set("Content-Type", "text/csv")
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.ImportCalls != 0 {
			t.Error("service tidak boleh dipanggil")
		}
	})

	t.Run("field file tidak ada", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{})

		body, contentType := multipartBody(t, "berkas", "data.csv", csv)
		req := httptest.NewRequest(http.MethodPost, "/import", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
	})

	t.Run("file bukan csv ditolak", func(t *testing.T) {
		fake := &FakeBackupService{}
		h := NewBackupHandler(fake)

		body, contentType := multipartBody(t, "file", "data.xlsx", csv)
		req := httptest.NewRequest(http.MethodPost, "/import", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status %d, mau 400", rec.Code)
		}
		if fake.ImportCalls != 0 {
			t.Error("service tidak boleh dipanggil")
		}
	})

	t.Run("method selain POST ditolak", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{})

		req := httptest.NewRequest(http.MethodGet, "/import", nil)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status %d, mau 405", rec.Code)
		}
	})

	t.Run("error service jadi 500", func(t *testing.T) {
		h := NewBackupHandler(&FakeBackupService{Err: errors.New("database mati")})

		body, contentType := multipartBody(t, "file", "data.csv", csv)
		req := httptest.NewRequest(http.MethodPost, "/import", body)
		req.Header.Set("Content-Type", contentType)
		rec := httptest.NewRecorder()

		h.HandlerImport(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status %d, mau 500", rec.Code)
		}
	})
}
