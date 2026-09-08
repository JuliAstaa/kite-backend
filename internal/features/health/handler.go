package health

import (
	"backend/internal/shared/response"
	"context"
	"database/sql"
	"net/http"
	"time"
)

type HealthResponse struct {
	Status  string `json:"status"`
	DB      string `json:"db"`
	Version string `json:"version"`
}

type HealthHandler struct {
	db      *sql.DB
	version string
}

func NewHealthHandler(db *sql.DB, version string) *HealthHandler {
	return &HealthHandler{db: db, version: version}
}

// HandlerHealth membalas 200 kalau database masih bisa dihubungi, 503 kalau
// tidak. Ping-nya dibatasi waktu supaya endpoint ini tidak ikut menggantung
// saat database bermasalah.
func (h *HealthHandler) HandlerHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "ok"
	status := http.StatusOK

	if err := h.db.PingContext(ctx); err != nil {
		dbStatus = "down"
		status = http.StatusServiceUnavailable
	}

	appStatus := "ok"
	if status != http.StatusOK {
		appStatus = "degraded"
	}

	response.WriteSuccessWithSingleData(w, status, HealthResponse{
		Status:  appStatus,
		DB:      dbStatus,
		Version: h.version,
	})
}
