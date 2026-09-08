package transaction

import (
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/timeutil"
	"backend/internal/shared/validator"
	"encoding/json"
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

func (h *TransactionHandler) HandlerTransactions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createTransaction(w, r)
	case http.MethodGet:
		h.listTransactions(w, r)
	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *TransactionHandler) createTransaction(w http.ResponseWriter, r *http.Request) {
	var reqBody CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
		return
	}

	reqBody.Type = strings.ToLower(strings.TrimSpace(reqBody.Type))
	reqBody.WalletID = strings.TrimSpace(reqBody.WalletID)

	details := map[string]string{}

	if validator.IsEmptyString(reqBody.WalletID) {
		details["wallet_id"] = "wallet_id tidak boleh kosong"
	} else if !validator.IsValidUUID(reqBody.WalletID) {
		details["wallet_id"] = "wallet_id bukan UUID yang valid"
	}

	if reqBody.Amount <= 0 {
		details["amount"] = "amount harus lebih besar dari 0"
	}

	if reqBody.OccurredAt.IsZero() {
		details["occurred_at"] = "occurred_at tidak boleh kosong"
	}

	switch reqBody.Type {
	case "":
		details["type"] = "type tidak boleh kosong"
	case "income", "expense":
		if reqBody.CategoryID == nil || validator.IsBlank(*reqBody.CategoryID) {
			details["category_id"] = "category_id tidak boleh kosong"
		} else if !validator.IsValidUUID(*reqBody.CategoryID) {
			details["category_id"] = "category_id bukan UUID yang valid"
		}
	case "transfer":
		if reqBody.ToWalletID == nil || validator.IsBlank(*reqBody.ToWalletID) {
			details["to_wallet_id"] = "to_wallet_id tidak boleh kosong"
		} else if !validator.IsValidUUID(*reqBody.ToWalletID) {
			details["to_wallet_id"] = "to_wallet_id bukan UUID yang valid"
		}
	default:
		details["type"] = "type harus income, expense, atau transfer"
	}

	if len(details) > 0 {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang tidak valid", details)
		return
	}

	detail, err := h.service.CreateTransaction(r.Context(), reqBody)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusCreated, NewTransactionResponse(detail))
}

func (h *TransactionHandler) listTransactions(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	now := timeutil.Now()

	// default: dari awal bulan berjalan sampai hari ini, dua-duanya inklusif
	from := timeutil.StartOfMonth(now)
	to := timeutil.EndOfDay(now)

	if raw := query.Get("from"); raw != "" {
		parsed, ok := queryparam.ToDate(raw)
		if !ok {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from harus format YYYY-MM-DD", map[string]string{"from": raw})
			return
		}
		from = timeutil.StartOfDay(parsed)
	}

	if raw := query.Get("to"); raw != "" {
		parsed, ok := queryparam.ToDate(raw)
		if !ok {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "to harus format YYYY-MM-DD", map[string]string{"to": raw})
			return
		}
		to = timeutil.EndOfDay(parsed)
	}

	if to.Before(from) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "to tidak boleh lebih awal dari from", nil)
		return
	}

	txType := strings.ToLower(strings.TrimSpace(query.Get("type")))
	if txType != "" && !validator.IsOneOf(txType, "income", "expense", "transfer") {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type harus income, expense, atau transfer", map[string]string{"type": txType})
		return
	}

	limit, offset, ok := queryparam.Pagination(query.Get("limit"), query.Get("offset"), 50, 200)
	if !ok {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit dan offset tidak boleh negatif", nil)
		return
	}

	minAmount, _ := queryparam.ToInt(query.Get("min_amount"))
	maxAmount, _ := queryparam.ToInt(query.Get("max_amount"))

	filter := TransactionFilter{
		From:        from,
		To:          to,
		Type:        txType,
		CategoryIDs: queryparam.ToCSV(query.Get("category_id")),
		WalletIDs:   queryparam.ToCSV(query.Get("wallet_id")),
		MinAmount:   minAmount,
		MaxAmount:   maxAmount,
		Query:       strings.TrimSpace(query.Get("q")),
		Sort:        strings.TrimSpace(query.Get("sort")),
		Limit:       limit,
		Offset:      offset,
	}

	for _, id := range append(append([]string{}, filter.CategoryIDs...), filter.WalletIDs...) {
		if !validator.IsValidUUID(id) {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id pada filter bukan UUID yang valid", map[string]string{"id": id})
			return
		}
	}

	transactions, total, err := h.service.GetAllTransactions(r.Context(), filter)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	resp := make([]TransactionResponse, 0, len(transactions))
	for _, t := range transactions {
		resp = append(resp, NewTransactionResponse(t))
	}

	response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: limit, Offset: offset})
}

func (h *TransactionHandler) HandlerTransactionByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validator.IsValidUUID(id) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id bukan UUID yang valid", map[string]string{"id": id})
		return
	}

	switch r.Method {
	case http.MethodGet:
		detail, err := h.service.GetTransactionByID(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewTransactionResponse(detail))

	case http.MethodPatch:
		var reqBody PatchTransactionRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		detail, err := h.service.PatchTransaction(r.Context(), id, reqBody)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewTransactionResponse(detail))

	case http.MethodDelete:
		detail, err := h.service.DeleteTransaction(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewTransactionResponse(detail))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *TransactionHandler) HandlerRestoreTransaction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validator.IsValidUUID(id) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id bukan UUID yang valid", map[string]string{"id": id})
		return
	}

	detail, err := h.service.RestoreTransaction(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewTransactionResponse(detail))
}
