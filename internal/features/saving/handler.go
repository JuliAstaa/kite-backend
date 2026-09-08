package saving

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/httpx"
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

type SavingHandler struct {
	service SavingServicer
}

func NewSavingHandler(service SavingServicer) *SavingHandler {
	return &SavingHandler{service: service}
}

func (h *SavingHandler) HandlerSummary(w http.ResponseWriter, r *http.Request) {
	period, ok := readPeriod(w, r)
	if !ok {
		return
	}

	// Kalau from dan to kosong, pakai periode berjalan.
	defaultFrom, defaultTo := timeutil.PeriodRange(period, timeutil.Now())
	explicitRange := r.URL.Query().Get("from") != "" || r.URL.Query().Get("to") != ""

	from, to, err := httpx.DateRange(r, defaultFrom, defaultTo)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	summary, err := h.service.Summary(r.Context(), period, from, to, explicitRange)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewSummaryResponse(summary))
}

func (h *SavingHandler) HandlerBreakdown(w http.ResponseWriter, r *http.Request) {
	period, ok := readPeriod(w, r)
	if !ok {
		return
	}

	// default: 6 bulan terakhir, atau 12 minggu terakhir
	now := timeutil.Now()
	defaultFrom := timeutil.StartOfMonth(now).AddDate(0, -5, 0)
	if period == "week" {
		defaultFrom = timeutil.StartOfWeek(now).AddDate(0, 0, -7*11)
	}

	from, to, err := httpx.DateRange(r, defaultFrom, timeutil.EndOfDay(now))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	buckets, err := h.service.Breakdown(r.Context(), period, from, to)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, NewBreakdownResponse(buckets))
}

func (h *SavingHandler) HandlerTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeDeleted, _ := queryparam.ToBool(r.URL.Query().Get("include_deleted"))

		targets, total, err := h.service.GetAllTargets(r.Context(), includeDeleted)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		resp := make([]TargetResponse, 0, len(targets))
		for _, t := range targets {
			resp = append(resp, NewTargetResponse(t))
		}

		response.WriteSuccessWithMultipleData(w, http.StatusOK, resp, response.APIMeta{Total: total, Limit: total, Offset: 0})

	case http.MethodPost:
		var body createTargetBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		startDate, err := parseDatePointer(body.StartDate, "start_date")
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		var endDate *time.Time
		if body.EndDate != nil {
			endDate, err = parseDatePointer(*body.EndDate, "end_date")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
		}

		target, err := h.service.CreateTarget(r.Context(), CreateTargetRequest{
			Period:     strings.ToLower(strings.TrimSpace(body.Period)),
			Amount:     body.Amount,
			TargetRate: body.TargetRate,
			StartDate:  startDate,
			EndDate:    endDate,
			IsActive:   body.IsActive,
		})
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusCreated, NewTargetResponse(target))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

func (h *SavingHandler) HandlerTargetByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validator.IsValidUUID(id) {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "id bukan UUID yang valid", map[string]string{"id": id})
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var body patchTargetBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			response.WriteError(w, http.StatusBadRequest, "INVALID_JSON", fmt.Sprintf("invalid json: %v", err), nil)
			return
		}

		req := PatchTargetRequest{
			Period:     body.Period,
			Amount:     body.Amount,
			TargetRate: body.TargetRate,
			IsActive:   body.IsActive,
		}

		if body.StartDate != nil {
			parsed, err := parseDatePointer(*body.StartDate, "start_date")
			if err != nil {
				response.WriteServiceError(w, err)
				return
			}
			req.StartDate = parsed
		}

		// end_date: field tidak dikirim, dikirim null, atau dikirim tanggal
		if body.EndDate != nil {
			if string(body.EndDate) == "null" {
				req.ClearEndDate = true
			} else {
				var raw string
				if err := json.Unmarshal(body.EndDate, &raw); err != nil {
					response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "end_date harus string tanggal atau null", nil)
					return
				}
				parsed, err := parseDatePointer(raw, "end_date")
				if err != nil {
					response.WriteServiceError(w, err)
					return
				}
				req.EndDate = parsed
			}
		}

		target, err := h.service.PatchTarget(r.Context(), id, req)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewTargetResponse(target))

	case http.MethodDelete:
		target, err := h.service.DeleteTarget(r.Context(), id)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}

		response.WriteSuccessWithSingleData(w, http.StatusOK, NewTargetResponse(target))

	default:
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed", nil)
	}
}

// readPeriod membaca query param period. Default "month" sesuai PRD.
func readPeriod(w http.ResponseWriter, r *http.Request) (string, bool) {
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	if period == "" {
		period = "month"
	}
	if !validator.IsOneOf(period, "week", "month") {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "period harus week atau month", map[string]string{"period": period})
		return "", false
	}
	return period, true
}

func parseDatePointer(value string, field string) (*time.Time, error) {
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
