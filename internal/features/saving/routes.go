package saving

import "net/http"

func RegisterSavingRoutes(mux *http.ServeMux, h *SavingHandler) {
	mux.HandleFunc("GET /savings/summary", h.HandlerSummary)
	mux.HandleFunc("GET /savings/breakdown", h.HandlerBreakdown)
	mux.HandleFunc("/savings/targets", h.HandlerTargets)
	mux.HandleFunc("/savings/targets/{id}", h.HandlerTargetByID)
}
