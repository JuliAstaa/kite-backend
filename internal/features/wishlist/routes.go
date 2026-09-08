package wishlist

import "net/http"

func RegisterWishlistRoutes(mux *http.ServeMux, h *WishlistHandler) {
	mux.HandleFunc("/wishlist", h.HandlerWishlist)
	// Segmen literal menang atas wildcard, jadi /wishlist/summary tidak
	// pernah ketangkep sebagai /wishlist/{id}.
	mux.HandleFunc("GET /wishlist/summary", h.HandlerSummary)
	mux.HandleFunc("/wishlist/{id}", h.HandlerWishlistByID)
	mux.HandleFunc("POST /wishlist/{id}/restore", h.HandlerRestore)
	mux.HandleFunc("POST /wishlist/{id}/allocate", h.HandlerAllocate)
	mux.HandleFunc("POST /wishlist/{id}/purchase", h.HandlerPurchase)
}
