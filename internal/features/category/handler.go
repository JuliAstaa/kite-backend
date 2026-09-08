package category

import (
	"backend/internal/shared/queryparam"
	"backend/internal/shared/response"
	"backend/internal/shared/validator"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type CategoryHandler struct {
	service CategoryServicer
}

func NewCategoryHandler(service CategoryServicer) *CategoryHandler {
	return &CategoryHandler{service: service}
}

func (h *CategoryHandler) HandlerCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ctx := r.Context()

		limit, offset, ok := queryparam.Pagination(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"), 10, 200)
		if !ok {
			response.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "limit dan offset tidak boleh negatif", nil)
			return
		}

		catType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
		if catType != "" && !validator.IsOneOf(catType, "income", "expense") {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type harus income atau expense", map[string]string{"type": catType})
			return
		}

		includeDeleted, _ := queryparam.ToBool(r.URL.Query().Get("include_deleted"))

		categories, total, err := h.service.GetAllCategories(ctx, limit, offset, catType, includeDeleted)

		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		resp := []CategoryResponse{}
		for _, category := range categories {
			resp = append(resp, NewCategoryResponse(category))
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: limit, Offset: offset})

	case http.MethodPost:
		ctx := r.Context()

		var reqBody CreateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err), nil)
			return
		}

		reqBody.Name = strings.TrimSpace(reqBody.Name)
		reqBody.Type = strings.ToLower(strings.TrimSpace(reqBody.Type))
		reqBody.Color = strings.TrimSpace(reqBody.Color)
		reqBody.Icon = strings.TrimSpace(reqBody.Icon)

		details := map[string]string{}

		if validator.IsEmptyString(reqBody.Name) {
			details["name"] = "name tidak boleh kosong"
		}

		if validator.IsEmptyString(reqBody.Type) {
			details["type"] = "type tidak boleh kosong"
		} else if reqBody.Type != "expense" && reqBody.Type != "income" {
			details["type"] = "harus expense atau income"
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

		category, err := h.service.CreateCategory(ctx, &reqBody)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, NewCategoryResponse(category))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
	}
}

func (h *CategoryHandler) HandlerCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if validator.IsEmptyString(id) {
		response.WriteError(w, http.StatusBadRequest, "EMPTY_ID", "ID can't be empty!", nil)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		ctx := r.Context()

		var reqBody PatchCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("Invalid JSON: %v", err), nil)
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
			} else if *reqBody.Type != "expense" && *reqBody.Type != "income" {
				details["type"] = "harus expense atau income"
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

		if len(details) > 0 {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ada field yang kosong", details)
			return
		}

		category, err := h.service.PatchCategory(ctx, id, &reqBody)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewCategoryResponse(category))

	case http.MethodDelete:
		ctx := r.Context()

		// Kategori default boleh dihapus. Seeding cuma jalan sekali lewat
		// migration, jadi yang sudah dihapus tidak muncul lagi tiap restart.
		category, err := h.service.DeleteCategory(ctx, id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewCategoryResponse(category))

	case http.MethodGet:
		ctx := r.Context()
		category, err := h.service.GetCategoryByID(ctx, id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewCategoryResponse(category))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed", nil)
	}
}

func (h *CategoryHandler) HandlerRestoreCategory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if validator.IsEmptyString(id) {
		response.WriteError(w, http.StatusBadRequest, "EMPTY_ID", "ID can't be empty!", nil)
		return
	}

	category, err := h.service.RestoreCategory(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewCategoryResponse(category))
}
