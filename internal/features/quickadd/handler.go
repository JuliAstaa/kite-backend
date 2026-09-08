package quickadd

import (
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/validator"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type QuickAddHandler struct {
	service QuickAddServicer
}

func NewQuickAddHandler(service QuickAddServicer) *QuickAddHandler {
	return &QuickAddHandler{service: service}
}

func (h *QuickAddHandler) HandlerQuickAdds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeDeleted, _ := queryparam.ToBool(r.URL.Query().Get("include_deleted"))

		quickAdds, total, err := h.service.GetAllQuickAdds(r.Context(), includeDeleted)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		resp := make([]QuickAddResponse, 0, len(quickAdds))
		for _, q := range quickAdds {
			resp = append(resp, NewQuickAddResponse(q))
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: total, Offset: 0})

	case http.MethodPost:
		var body createQuickAddBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		details := map[string]string{}
		if !validator.IsValidUUID(strings.TrimSpace(body.WalletID)) {
			details["wallet_id"] = "wallet_id bukan UUID yang valid"
		}
		if !validator.IsValidUUID(strings.TrimSpace(body.CategoryID)) {
			details["category_id"] = "category_id bukan UUID yang valid"
		}
		if len(details) > 0 {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang tidak valid", details)
			return
		}

		quickAdd, err := h.service.CreateQuickAdd(r.Context(), CreateQuickAddRequest{
			Label:      body.Label,
			Type:       body.Type,
			Amount:     body.Amount,
			WalletID:   strings.TrimSpace(body.WalletID),
			CategoryID: strings.TrimSpace(body.CategoryID),
			Note:       body.Note,
		})
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, NewQuickAddResponse(quickAdd))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *QuickAddHandler) HandlerQuickAddByID(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body patchQuickAddBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		req := PatchQuickAddRequest{
			Label:      body.Label,
			Type:       body.Type,
			WalletID:   body.WalletID,
			CategoryID: body.CategoryID,
			Note:       body.Note,
			SortOrder:  body.SortOrder,
		}

		if body.Amount != nil {
			if string(body.Amount) == "null" {
				req.ClearAmount = true
			} else {
				var amount int
				if err := json.Unmarshal(body.Amount, &amount); err != nil {
					response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "amount harus angka atau null", nil)
					return
				}
				req.Amount = &amount
			}
		}

		quickAdd, err := h.service.PatchQuickAdd(r.Context(), id, req)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewQuickAddResponse(quickAdd))

	case http.MethodDelete:
		quickAdd, err := h.service.DeleteQuickAdd(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewQuickAddResponse(quickAdd))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *QuickAddHandler) HandlerExecute(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	// Body boleh kosong: quick add yang sudah punya amount tetap tidak butuh
	// input apa-apa.
	var body executeBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}
	}

	req := ExecuteRequest{Amount: body.Amount, Note: body.Note}
	if body.OccurredAt != nil {
		req.OccurredAt = *body.OccurredAt
	}

	created, err := h.service.Execute(r.Context(), id, req)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusCreated, ExecuteResponse{
		TransactionID: created.ID,
		Type:          created.Type,
		Amount:        created.Amount,
		Note:          created.Note,
		OccurredAt:    created.OccurredAt,
	})
}

func readID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !validator.IsValidUUID(id) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id bukan UUID yang valid", map[string]string{"id": id})
		return "", false
	}
	return id, true
}
