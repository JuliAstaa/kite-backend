package wallet

import "net/http"

func RegisterWalletRoutes(mux *http.ServeMux, h WalletHandler) {
	mux.HandleFunc("/wallets", h.HandlerWallets)
}
