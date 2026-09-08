package quickadd

import "net/http"

func RegisterQuickAddRoutes(mux *http.ServeMux, h *QuickAddHandler) {
	mux.HandleFunc("/quick-adds", h.HandlerQuickAdds)
	mux.HandleFunc("/quick-adds/{id}", h.HandlerQuickAddByID)
	mux.HandleFunc("POST /quick-adds/{id}/execute", h.HandlerExecute)
}
