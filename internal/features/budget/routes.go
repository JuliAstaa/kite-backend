package budget

import "net/http"

func RegisterBudgetRoutes(mux *http.ServeMux, h *BudgetHandler) {
	mux.HandleFunc("/budgets", h.HandlerBudgets)
	mux.HandleFunc("GET /budgets/status", h.HandlerStatus)
	mux.HandleFunc("/budgets/{id}", h.HandlerBudgetByID)
}
