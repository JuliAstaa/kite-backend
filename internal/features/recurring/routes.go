package recurring

import "net/http"

func RegisterRecurringRoutes(mux *http.ServeMux, h *RecurringHandler) {
	mux.HandleFunc("/recurring", h.HandlerRecurring)
	mux.HandleFunc("POST /recurring/run", h.HandlerRun)
	mux.HandleFunc("/recurring/{id}", h.HandlerRecurringByID)
	mux.HandleFunc("POST /recurring/{id}/toggle", h.HandlerToggle)
}
