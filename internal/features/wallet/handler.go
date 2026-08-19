package wallet

import (
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/validator"
	"encoding/json"
	"fmt"
	"net/http"
)

type WalletHandler struct {
	service WalletServicer
}

func NewWalletHandler(service WalletServicer) *WalletHandler {
	return &WalletHandler{service: service}
}

func (h *WalletHandler) HandlerWallets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:

		ctx := r.Context()

		var reqBody CreateWalletRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		details := map[string]string{}

		if validator.IsEmptyString(reqBody.Name) {
			details["name"] = "name cannot be empty!"
		}

		if validator.IsEmptyString(reqBody.Type) {
			details["type"] = "type cannot be empty!"
		} else if reqBody.Type != "cash" && reqBody.Type != "bank" && reqBody.Type != "ewallet" && reqBody.Type != "savings" && reqBody.Type != "other" {
			details["type"] = "type harus diantara cash, bank, ewallet, savings, dan other"
		}

		if validator.IsEmptyString(reqBody.Color) {
			details["color"] = "color tidak boleh kosong"
		} else if !validator.IsValidHexColor(reqBody.Color) {
			details["color"] = "harus berupa kode HEX"
		}

		if validator.IsEmptyString(reqBody.Icon) {
			details["icon"] = "icon tidak boleh kosong"
		}

		if len(details) > 0 {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang kosong", details)
			return
		}

		wallet, err := h.service.CreateWallet(ctx, &reqBody)

		if err != nil {
			status, code := response.StatusFromError(err)
			response.WriteError(w, status, code, err.Error(), nil)
			return
		}

		resp := Wallet{
			ID:                  wallet.ID,
			Name:                wallet.Name,
			Type:                wallet.Type,
			InitialBalance:      wallet.InitialBalance,
			Color:               wallet.Color,
			Icon:                wallet.Icon,
			IsExcludedFromTotal: wallet.IsExcludedFromTotal,
			SortOrder:           wallet.SortOrder,
			CreatedAt:           wallet.CreatedAt,
			UpdatedAt:           wallet.UpdatedAt,
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, resp)

	case http.MethodGet:
		ctx := r.Context()

		limit := 10
		offset := 0

		strLimit := r.URL.Query().Get("limit")
		strOffset := r.URL.Query().Get("offset")

		if parsed, ok := queryparam.ToInt(strLimit); ok {
			limit = parsed
		}

		if parsed, ok := queryparam.ToInt(strOffset); ok {
			offset = parsed
		}

		if limit < 0 || offset < 0 {
			response.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "limit dan offset tidak boleh negatif", nil)
			return
		}

		wallets, total, err := h.service.GetAllWallets(ctx, limit, offset)
		if err != nil {
			status, code := response.StatusFromError(err)
			response.WriteError(w, status, code, err.Error(), nil)
			return
		}

		resp := []WalletResponse{}
		for _, wallet := range wallets {
			w := WalletResponse{
				ID:                  wallet.ID,
				Name:                wallet.Name,
				Type:                wallet.Type,
				InitialBalance:      wallet.InitialBalance,
				Color:               wallet.Color,
				Icon:                wallet.Icon,
				IsExcludedFromTotal: wallet.IsExcludedFromTotal,
				SortOrder:           wallet.SortOrder,
				CreatedAt:           wallet.CreatedAt,
				UpdatedAt:           wallet.UpdatedAt,
			}
			resp = append(resp, w)
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: limit, Offset: offset})

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
	}
}
