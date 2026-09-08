package backup

import (
	"backend/internal/shared/httpx"
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/timeutil"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// maxUploadBytes membatasi ukuran file import. Data pribadi satu orang tidak
// akan sebesar ini, jadi angka segini sudah longgar.
const maxUploadBytes = 10 << 20 // 10 MB

type BackupHandler struct {
	service BackupServicer
}

func NewBackupHandler(service BackupServicer) *BackupHandler {
	return &BackupHandler{service: service}
}

type ImportResponse struct {
	DryRun      bool       `json:"dry_run"`
	TotalRows   int        `json:"total_rows"`
	ValidRows   int        `json:"valid_rows"`
	InvalidRows int        `json:"invalid_rows"`
	Imported    int        `json:"imported"`
	Errors      []RowError `json:"errors"`
}

func (h *BackupHandler) HandlerExport(w http.ResponseWriter, r *http.Request) {
	now := timeutil.Now()

	// default: setahun terakhir, cukup lebar untuk backup rutin
	from, to, err := httpx.DateRange(r, timeutil.StartOfDay(now.AddDate(-1, 0, 0)), timeutil.EndOfDay(now))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "csv" {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "format harus json atau csv", map[string]string{"format": format})
		return
	}

	bundle, err := h.service.Export(r.Context(), from, to)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	if format == "csv" {
		h.writeCSV(w, bundle)
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="finance_%s.json"`, timeutil.FormatDate(now)))
	response.WriteSuccessWithSingleData(w, http.StatusOK, bundle)
}

func (h *BackupHandler) writeCSV(w http.ResponseWriter, bundle ExportBundle) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="finance_%s.csv"`, timeutil.FormatDate(timeutil.Now())))
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"date", "type", "amount", "wallet", "category", "note"})
	for _, t := range bundle.Transactions {
		// Transfer ikut diekspor supaya file backup lengkap. Kolom category-nya
		// kosong, dan baris seperti ini memang tidak bisa diimpor balik.
		writer.Write([]string{
			timeutil.FormatDate(t.OccurredAt),
			t.Type,
			strconv.Itoa(t.Amount),
			t.Wallet,
			t.Category,
			t.Note,
		})
	}
}

func (h *BackupHandler) HandlerImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
		return
	}

	dryRun, ok := queryparam.ToBool(r.URL.Query().Get("dry_run"))
	if !ok {
		// Default aman: kalau tidak disebut, jangan langsung menulis.
		dryRun = true
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"request harus multipart/form-data dengan field \"file\"", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "field \"file\" tidak ditemukan", nil)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "file harus berformat CSV", map[string]string{"file": header.Filename})
		return
	}

	result, err := h.service.Import(r.Context(), file, dryRun)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	status := http.StatusOK
	if !result.DryRun && result.Imported > 0 {
		status = http.StatusCreated
	}

	response.WriteSuccessWithSingleData(w, status, ImportResponse{
		DryRun:      result.DryRun,
		TotalRows:   result.TotalRows,
		ValidRows:   result.ValidRows,
		InvalidRows: result.InvalidRows,
		Imported:    result.Imported,
		Errors:      result.Errors,
	})
}
