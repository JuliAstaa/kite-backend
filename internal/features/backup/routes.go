package backup

import "net/http"

func RegisterBackupRoutes(mux *http.ServeMux, h *BackupHandler) {
	mux.HandleFunc("GET /export", h.HandlerExport)
	mux.HandleFunc("POST /import", h.HandlerImport)
}
