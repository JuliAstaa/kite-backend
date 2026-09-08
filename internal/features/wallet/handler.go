package wallet

import (
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/validator"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var allowedWalletTypes = []string{"cash", "bank", "ewallet", "savings", "other"}

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

		reqBody.Name = strings.TrimSpace(reqBody.Name)
		reqBody.Type = strings.ToLower(strings.TrimSpace(reqBody.Type))
		reqBody.Color = strings.TrimSpace(reqBody.Color)
		reqBody.Icon = strings.TrimSpace(reqBody.Icon)

		details := map[string]string{}

		if validator.IsEmptyString(reqBody.Name) {
			details["name"] = "name cannot be empty!"
		}

		if validator.IsEmptyString(reqBody.Type) {
			details["type"] = "type cannot be empty!"
		} else if !validator.IsOneOf(reqBody.Type, allowedWalletTypes...) {
			details["type"] = "type harus salah satu dari: " + strings.Join(allowedWalletTypes, ", ")
		}

		if validator.IsEmptyString(reqBody.Color) {
			details["color"] = "color tidak boleh kosong"
		} else if !validator.IsValidHexColor(reqBody.Color) {
			details["color"] = "harus berupa kode HEX"
		}

		if validator.IsEmptyString(reqBody.Icon) {
			details["icon"] = "icon tidak boleh kosong"
		}

		if reqBody.InitialBalance < 0 {
			details["initial_balance"] = "saldo awal tidak boleh negatif"
		}

		if len(details) > 0 {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang kosong", details)
			return
		}

		wallet, err := h.service.CreateWallet(ctx, &reqBody)

		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, NewWalletResponse(wallet))

	case http.MethodGet:
		ctx := r.Context()

		limit, offset, ok := queryparam.Pagination(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"), 10, 200)
		if !ok {
			response.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "limit dan offset tidak boleh negatif", nil)
			return
		}

		includeDeleted, _ := queryparam.ToBool(r.URL.Query().Get("include_deleted"))

		wallets, total, err := h.service.GetAllWallets(ctx, limit, offset, includeDeleted)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		resp := []WalletResponse{}
		for _, wallet := range wallets {
			resp = append(resp, NewWalletResponse(wallet))
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: limit, Offset: offset})

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
	}
}

func (h *WalletHandler) HandlerWalletByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if validator.IsEmptyString(id) {
		response.WriteError(w, http.StatusBadRequest, "EMPTY_ID", "ID can't be empty!", nil)
		return
	}

	switch r.Method {
	case http.MethodPatch:

		ctx := r.Context()

		var reqBody PatchWalletRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		if reqBody.Name != nil {
			trimmed := strings.TrimSpace(*reqBody.Name)
			reqBody.Name = &trimmed
		}
		if reqBody.Type != nil {
			trimmed := strings.ToLower(strings.TrimSpace(*reqBody.Type))
			reqBody.Type = &trimmed
		}
		if reqBody.Color != nil {
			trimmed := strings.TrimSpace(*reqBody.Color)
			reqBody.Color = &trimmed
		}
		if reqBody.Icon != nil {
			trimmed := strings.TrimSpace(*reqBody.Icon)
			reqBody.Icon = &trimmed
		}

		details := map[string]string{}

		if reqBody.Name != nil {
			if validator.IsEmptyString(*reqBody.Name) {
				details["name"] = "name tidak boleh kosong"
			}
		}

		if reqBody.Type != nil {
			if validator.IsEmptyString(*reqBody.Type) {
				details["type"] = "type tidak boleh kosong"
			} else if !validator.IsOneOf(*reqBody.Type, allowedWalletTypes...) {
				details["type"] = "type harus salah satu dari: " + strings.Join(allowedWalletTypes, ", ")
			}
		}

		if reqBody.Color != nil {
			if validator.IsEmptyString(*reqBody.Color) {
				details["color"] = "color tidak boleh kosong"
			} else if !validator.IsValidHexColor(*reqBody.Color) {
				details["color"] = "harus berupa kode HEX"
			}
		}

		if reqBody.Icon != nil {
			if validator.IsEmptyString(*reqBody.Icon) {
				details["icon"] = "icon tidak boleh kosong"
			}
		}

		if reqBody.InitialBalance != nil {
			if *reqBody.InitialBalance < 0 {
				details["initial_balance"] = "saldo awal tidak boleh negatif"
			}
		}

		if len(details) > 0 {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang kosong", details)
			return
		}

		wallet, err := h.service.PatchWallet(ctx, id, &reqBody)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewWalletResponse(wallet))

	case http.MethodGet:
		ctx := r.Context()

		wallet, err := h.service.GetWalletByID(ctx, id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewWalletResponse(wallet))

	case http.MethodDelete:
		ctx := r.Context()

		// Delete selalu berhasil karena soft delete, tapi jumlah transaksi yang
		// ikut terpengaruh dikembalikan supaya frontend bisa memberi peringatan.
		wallet, err := h.service.DeleteWallet(ctx, id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewDeleteWalletResponse(wallet))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
	}

}

func (h *WalletHandler) HandlerRestoreWallet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if validator.IsEmptyString(id) {
		response.WriteError(w, http.StatusBadRequest, "EMPTY_ID", "ID can't be empty!", nil)
		return
	}

	wallet, err := h.service.RestoreWallet(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewWalletResponse(wallet))
}
