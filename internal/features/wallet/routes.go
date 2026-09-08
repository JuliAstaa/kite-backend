package wallet

import "net/http"

func RegisterWalletRoutes(mux *http.ServeMux, h *WalletHandler) {
	mux.HandleFunc("/wallets", h.HandlerWallets)
	mux.HandleFunc("/wallets/{id}", h.HandlerWalletByID)
	mux.HandleFunc("POST /wallets/{id}/restore", h.HandlerRestoreWallet)
}
