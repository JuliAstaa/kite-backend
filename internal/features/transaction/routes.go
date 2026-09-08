package transaction

import "net/http"

func RegisterTransactionRoutes(mux *http.ServeMux, h *TransactionHandler) {
	mux.HandleFunc("/transactions", h.HandlerTransactions)
	mux.HandleFunc("/transactions/{id}", h.HandlerTransactionByID)
	mux.HandleFunc("POST /transactions/{id}/restore", h.HandlerRestoreTransaction)
}
