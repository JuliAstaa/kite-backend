package budget

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

type BudgetHandler struct {
	service BudgetServicer
}

func NewBudgetHandler(service BudgetServicer) *BudgetHandler {
	return &BudgetHandler{service: service}
}

func (h *BudgetHandler) HandlerBudgets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeDeleted, _ := queryparam.ToBool(r.URL.Query().Get("include_deleted"))

		budgets, total, err := h.service.GetAllBudgets(r.Context(), includeDeleted)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		resp := make([]BudgetResponse, 0, len(budgets))
		for _, b := range budgets {
			resp = append(resp, NewBudgetResponse(b))
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: total, Offset: 0})

	case http.MethodPost:
		var body createBudgetBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		if !validator.IsValidUUID(strings.TrimSpace(body.CategoryID)) {
			response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "category_id bukan UUID yang valid",
				map[string]string{"category_id": body.CategoryID})
			return
		}

		startMonth, err := parseMonth(body.StartMonth, "start_month")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		var endMonth *time.Time
		if body.EndMonth != nil {
			endMonth, err = parseMonth(*body.EndMonth, "end_month")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
		}

		budget, err := h.service.CreateBudget(r.Context(), CreateBudgetRequest{
			CategoryID: strings.TrimSpace(body.CategoryID),
			Amount:     body.Amount,
			Period:     body.Period,
			StartMonth: startMonth,
			EndMonth:   endMonth,
		})
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, NewBudgetResponse(budget))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *BudgetHandler) HandlerBudgetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validator.IsValidUUID(id) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id bukan UUID yang valid", map[string]string{"id": id})
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body patchBudgetBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		req := PatchBudgetRequest{Amount: body.Amount, Period: body.Period}

		if body.StartMonth != nil {
			parsed, err := parseMonth(*body.StartMonth, "start_month")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
			req.StartMonth = parsed
		}

		if body.EndMonth != nil {
			if string(body.EndMonth) == "null" {
				req.ClearEndMonth = true
			} else {
				var raw string
				if err := json.Unmarshal(body.EndMonth, &raw); err != nil {
					response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "end_month harus string tanggal atau null", nil)
					return
				}
				parsed, err := parseMonth(raw, "end_month")
				if err != nil {
					response.WriteServiceError(w, err)
					return
				}
				req.EndMonth = parsed
			}
		}

		budget, err := h.service.PatchBudget(r.Context(), id, req)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewBudgetResponse(budget))

	case http.MethodDelete:
		budget, err := h.service.DeleteBudget(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewBudgetResponse(budget))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *BudgetHandler) HandlerStatus(w http.ResponseWriter, r *http.Request) {
	month := timeutil.Now()

	if raw := r.URL.Query().Get("month"); raw != "" {
		parsed, err := parseMonth(raw, "month")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		month = *parsed
	}

	statuses, err := h.service.StatusForMonth(r.Context(), month)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	resp := make([]StatusResponse, 0, len(statuses))
	for _, s := range statuses {
		resp = append(resp, NewStatusResponse(s))
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, resp)
}

// parseMonth menerima "2026-08" maupun "2026-08-01", dua-duanya dianggap
// menunjuk bulan Agustus 2026.
func parseMonth(value string, field string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, apperror.ValidationError{Field: field, Message: field + " tidak boleh kosong"}
	}

	if len(value) == 7 {
		value += "-01"
	}

	parsed, err := timeutil.ParseDate(value)
	if err != nil {
		return nil, apperror.ValidationError{Field: field, Message: field + " harus format YYYY-MM atau YYYY-MM-DD"}
	}

	month := timeutil.StartOfMonth(parsed)
	return &month, nil
}
