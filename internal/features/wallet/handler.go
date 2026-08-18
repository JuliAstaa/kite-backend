package wallet

import (
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
	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
	}
}
