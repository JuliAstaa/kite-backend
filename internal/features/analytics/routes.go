package analytics

import "net/http"

func RegisterAnalyticsRoutes(mux *http.ServeMux, h *AnalyticsHandler) {
	mux.HandleFunc("GET /summary", h.HandlerSummary)
	mux.HandleFunc("GET /analytics/by-category", h.HandlerByCategory)
	mux.HandleFunc("GET /analytics/by-wallet", h.HandlerByWallet)
	mux.HandleFunc("GET /analytics/trend", h.HandlerTrend)
}
