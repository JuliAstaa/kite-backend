package wishlist

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/timeutil"
	"backend/internal/shared/validator"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WishlistHandler struct {
	service WishlistServicer
}

func NewWishlistHandler(service WishlistServicer) *WishlistHandler {
	return &WishlistHandler{service: service}
}

func (h *WishlistHandler) HandlerWishlist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listItems(w, r)
	case http.MethodPost:
		h.createItem(w, r)
	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *WishlistHandler) listItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	status := strings.ToLower(strings.TrimSpace(query.Get("status")))
	if status != "" && !validator.IsOneOf(status, allowedStatuses...) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status tidak dikenal", map[string]string{"status": status})
		return
	}

	limit, offset, ok := queryparam.Pagination(query.Get("limit"), query.Get("offset"), 50, 200)
	if !ok {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit dan offset tidak boleh negatif", nil)
		return
	}

	includeDeleted, _ := queryparam.ToBool(query.Get("include_deleted"))

	views, total, err := h.service.GetAllItems(r.Context(), ListFilter{
		Status:         status,
		Sort:           strings.TrimSpace(query.Get("sort")),
		IncludeDeleted: includeDeleted,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	resp := make([]ItemResponse, 0, len(views))
	for _, view := range views {
		resp = append(resp, NewItemResponse(view))
	}

	response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: limit, Offset: offset})
}

func (h *WishlistHandler) createItem(w http.ResponseWriter, r *http.Request) {
	var body createItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
		return
	}

	req := CreateItemRequest{
		Name:           body.Name,
		EstimatedPrice: body.EstimatedPrice,
		Priority:       body.Priority,
		ProductURL:     body.ProductURL,
		Note:           body.Note,
		Status:         body.Status,
	}

	if body.TargetDate != nil {
		parsed, err := parseDate(*body.TargetDate, "target_date")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		req.TargetDate = parsed
	}

	view, err := h.service.CreateItem(r.Context(), req)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusCreated, NewItemResponse(view))
}

func (h *WishlistHandler) HandlerWishlistByID(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		view, err := h.service.GetItemByID(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewItemResponse(view))

	case http.MethodPatch:
		h.patchItem(w, r, id)

	case http.MethodDelete:
		item, err := h.service.DeleteItem(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewItemOnlyResponse(item))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *WishlistHandler) patchItem(w http.ResponseWriter, r *http.Request, id string) {
	var body patchItemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
		return
	}

	req := PatchItemRequest{
		Name:           body.Name,
		EstimatedPrice: body.EstimatedPrice,
		Priority:       body.Priority,
		Note:           body.Note,
		Status:         body.Status,
		SortOrder:      body.SortOrder,
	}

	if body.TargetDate != nil {
		if string(body.TargetDate) == "null" {
			req.ClearTargetDate = true
		} else {
			var raw string
			if err := json.Unmarshal(body.TargetDate, &raw); err != nil {
				response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_date harus string tanggal atau null", nil)
				return
			}
			parsed, err := parseDate(raw, "target_date")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
			req.TargetDate = parsed
		}
	}

	if body.ProductURL != nil {
		if string(body.ProductURL) == "null" {
			req.ClearProductURL = true
		} else {
			var raw string
			if err := json.Unmarshal(body.ProductURL, &raw); err != nil {
				response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "product_url harus string atau null", nil)
				return
			}
			req.ProductURL = &raw
		}
	}

	view, err := h.service.PatchItem(r.Context(), id, req)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewItemResponse(view))
}

func (h *WishlistHandler) HandlerRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	item, err := h.service.RestoreItem(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewItemOnlyResponse(item))
}

func (h *WishlistHandler) HandlerAllocate(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	var body allocateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
		return
	}

	view, err := h.service.Allocate(r.Context(), id, body.Amount)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewItemResponse(view))
}

func (h *WishlistHandler) HandlerPurchase(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	var body purchaseBody
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
	if body.ActualPrice <= 0 {
		details["actual_price"] = "actual_price harus lebih besar dari 0"
	}
	if len(details) > 0 {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang tidak valid", details)
		return
	}

	req := PurchaseRequest{
		WalletID:    strings.TrimSpace(body.WalletID),
		CategoryID:  strings.TrimSpace(body.CategoryID),
		ActualPrice: body.ActualPrice,
		Note:        body.Note,
	}
	if body.OccurredAt != nil {
		req.OccurredAt = *body.OccurredAt
	}

	view, err := h.service.Purchase(r.Context(), id, req)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewItemResponse(view))
}

func (h *WishlistHandler) HandlerSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := h.service.Summary(r.Context())
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, SummaryResponse{
		TotalItems:     summary.TotalItems,
		TotalEstimated: summary.TotalEstimated,
		TotalSaved:     summary.TotalSaved,
		ByPriority:     summary.ByPriority,
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

func parseDate(value string, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := timeutil.ParseDate(value)
	if err != nil {
		return nil, apperror.ValidationError{Field: field, Message: field + " harus format YYYY-MM-DD"}
	}
	return &parsed, nil
}
