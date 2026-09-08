package health

import "net/http"

func RegisterHealthRoutes(mux *http.ServeMux, h *HealthHandler) {
	mux.HandleFunc("GET /health", h.HandlerHealth)
}
