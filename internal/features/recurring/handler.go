package recurring

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

type RecurringHandler struct {
	service RecurringServicer
}

func NewRecurringHandler(service RecurringServicer) *RecurringHandler {
	return &RecurringHandler{service: service}
}

func (h *RecurringHandler) HandlerRecurring(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeDeleted, _ := queryparam.ToBool(r.URL.Query().Get("include_deleted"))

		rules, total, err := h.service.GetAllRules(r.Context(), includeDeleted)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		resp := make([]RuleResponse, 0, len(rules))
		for _, rule := range rules {
			resp = append(resp, NewRuleResponse(rule))
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: total, Offset: 0})

	case http.MethodPost:
		var body createRuleBody
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

		startDate, err := parseDate(body.StartDate, "start_date")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		var endDate *time.Time
		if body.EndDate != nil {
			endDate, err = parseDate(*body.EndDate, "end_date")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
		}

		rule, err := h.service.CreateRule(r.Context(), CreateRuleRequest{
			Name:       body.Name,
			Type:       body.Type,
			Amount:     body.Amount,
			WalletID:   strings.TrimSpace(body.WalletID),
			CategoryID: strings.TrimSpace(body.CategoryID),
			Note:       body.Note,
			Frequency:  body.Frequency,
			Interval:   body.Interval,
			DayOfMonth: body.DayOfMonth,
			DayOfWeek:  body.DayOfWeek,
			StartDate:  startDate,
			EndDate:    endDate,
			IsActive:   body.IsActive,
		})
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, NewRuleResponse(rule))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *RecurringHandler) HandlerRecurringByID(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		rule, err := h.service.GetRuleByID(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewRuleResponse(rule))

	case http.MethodPatch:
		h.patchRule(w, r, id)

	case http.MethodDelete:
		rule, err := h.service.DeleteRule(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		response.WriteSuccessWithSingleData(w, http.StatusOK, NewRuleResponse(rule))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *RecurringHandler) patchRule(w http.ResponseWriter, r *http.Request, id string) {
	var body patchRuleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
		return
	}

	req := PatchRuleRequest{
		Name:       body.Name,
		Amount:     body.Amount,
		WalletID:   body.WalletID,
		CategoryID: body.CategoryID,
		Note:       body.Note,
		Frequency:  body.Frequency,
		Interval:   body.Interval,
		DayOfMonth: body.DayOfMonth,
		DayOfWeek:  body.DayOfWeek,
		IsActive:   body.IsActive,
	}

	if body.StartDate != nil {
		parsed, err := parseDate(*body.StartDate, "start_date")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		req.StartDate = parsed
	}

	if body.NextRunAt != nil {
		parsed, err := parseDate(*body.NextRunAt, "next_run_at")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		req.NextRunAt = parsed
	}

	if body.EndDate != nil {
		if string(body.EndDate) == "null" {
			req.ClearEndDate = true
		} else {
			var raw string
			if err := json.Unmarshal(body.EndDate, &raw); err != nil {
				response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "end_date harus string tanggal atau null", nil)
				return
			}
			parsed, err := parseDate(raw, "end_date")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
			req.EndDate = parsed
		}
	}

	rule, err := h.service.PatchRule(r.Context(), id, req)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewRuleResponse(rule))
}

func (h *RecurringHandler) HandlerToggle(w http.ResponseWriter, r *http.Request) {
	id, ok := readID(w, r)
	if !ok {
		return
	}

	rule, err := h.service.ToggleRule(r.Context(), id)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewRuleResponse(rule))
}

// HandlerRun memicu scheduler secara manual. Berguna buat testing tanpa harus
// menunggu ticker satu jam.
func (h *RecurringHandler) HandlerRun(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.RunDue(r.Context())
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	failed := result.FailedRuleIDs
	if failed == nil {
		failed = []string{}
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, RunResponse{
		RulesChecked:  result.RulesChecked,
		Created:       result.Created,
		Skipped:       result.Skipped,
		RulesFailed:   result.RulesFailed,
		FailedRuleIDs: failed,
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
		return nil, apperror.ValidationError{Field: field, Message: field + " tidak boleh kosong"}
	}
	parsed, err := timeutil.ParseDate(value)
	if err != nil {
		return nil, apperror.ValidationError{Field: field, Message: field + " harus format YYYY-MM-DD"}
	}
	return &parsed, nil
}
