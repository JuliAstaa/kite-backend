package transaction

import "net/http"

func RegisterTransactionRoutes(mux *http.ServeMux, h TransactionHandler) {
	mux.HandleFunc("/transactions", h.HandlerTrasanctions)
}
