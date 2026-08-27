package transaction

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/response"
	"backend/internal/shared/validator"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type TransactionHandler struct {
	service TransactionServicer
}

func NewTransactionHandler(service TransactionServicer) *TransactionHandler {
	return &TransactionHandler{service: service}
}

func (h *TransactionHandler) HandlerTrasanctions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		ctx := r.Context()
		var reqBody CreateTransactionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err), nil)
			return
		}

		details := map[string]string{}

		if validator.IsEmptyString(reqBody.WalletID) {
			details["wallet_id"] = "wallet_id tidak boleh kosong"
		}

		if reqBody.Amount <= 0 {
			details["amount"] = "amount tidak boleh kurang dari sama dengan 0"
		}

		if reqBody.OccurredAt.IsZero() {
			details["occurred_at"] = "occurred_at tidak boleh kosong"
		}

		if validator.IsEmptyString(reqBody.Type) {
			details["type"] = "type tidak boleh kosong"
		}
		reqBody.Type = strings.ToLower(strings.TrimSpace(reqBody.Type))
		switch reqBody.Type {
		case "expense", "income":
			if reqBody.CategoryID == nil || validator.IsEmptyString(*reqBody.CategoryID) {
				details["category_id"] = "category_id tidak boleh kosong"
			}

		case "transfer":

			if reqBody.ToWalletID == nil || validator.IsEmptyString(*reqBody.ToWalletID) {
				details["to_wallet_id"] = "to_wallet_id tidak boleh kosong"
			}

		default:
			if !validator.IsEmptyString(reqBody.Type) {
				details["type"] = "type tidak sesuai"
			}
		}

		if len(details) > 0 {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang kosong", details)
			return
		}

		detail, err := h.service.CreateTransaction(ctx, reqBody)
		if err != nil {
			var ve apperror.ValidationError
			if errors.As(err, &ve) {
				response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", ve.Message, map[string]string{ve.Field: ve.Message})
				return
			}

			status, code := response.StatusFromError(err)
			response.WriteError(w, status, code, err.Error(), nil)
			return
		}

		resp := TransactionResponse{
			ID:         detail.ID,
			Type:       detail.Type,
			Amount:     detail.Amount,
			Note:       detail.Note,
			OccurredAt: detail.OccurredAt,
			Wallet:     detail.Wallet,
			Category:   detail.Category,
			ToWallet:   detail.ToWallet,
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, resp)

	case http.MethodGet:
	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}
