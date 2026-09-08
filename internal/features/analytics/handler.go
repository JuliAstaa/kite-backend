package analytics

import (
	"backend/internal/shared/httpx"
	"backend/internal/shared/response"
	"backend/internal/shared/timeutil"
	"backend/internal/shared/validator"
	"net/http"
	"strings"
)

type AnalyticsHandler struct {
	service AnalyticsServicer
}

func NewAnalyticsHandler(service AnalyticsServicer) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) HandlerSummary(w http.ResponseWriter, r *http.Request) {
	now := timeutil.Now()
	from, to, err := httpx.DateRange(r, timeutil.StartOfMonth(now), timeutil.EndOfDay(now))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	summary, err := h.service.Summary(r.Context(), from, to)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, SummaryResponse{
		Period: PeriodResponse{
			From: timeutil.FormatDate(summary.From),
			To:   timeutil.FormatDate(summary.To),
		},
		TotalIncome:      summary.TotalIncome,
		TotalExpense:     summary.TotalExpense,
		Net:              summary.Net,
		TotalBalance:     summary.TotalBalance,
		TransactionCount: summary.TransactionCount,
		Comparison: ComparisonResponse{
			IncomeChangePct:  summary.IncomeChangePct,
			ExpenseChangePct: summary.ExpenseChangePct,
		},
	})
}

func (h *AnalyticsHandler) HandlerByCategory(w http.ResponseWriter, r *http.Request) {
	now := timeutil.Now()
	from, to, err := httpx.DateRange(r, timeutil.StartOfMonth(now), timeutil.EndOfDay(now))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	txType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if txType != "" && !validator.IsOneOf(txType, "income", "expense") {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "type harus income atau expense", map[string]string{"type": txType})
		return
	}

	rows, err := h.service.ByCategory(r.Context(), from, to, txType)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	resp := make([]CategoryBreakdownResponse, 0, len(rows))
	for _, c := range rows {
		resp = append(resp, CategoryBreakdownResponse{
			CategoryID:   c.CategoryID,
			CategoryName: c.CategoryName,
			CategoryType: c.CategoryType,
			IsDeleted:    c.IsDeleted,
			Total:        c.Total,
			Count:        c.Count,
			Percentage:   c.Percentage,
		})
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, resp)
}

func (h *AnalyticsHandler) HandlerByWallet(w http.ResponseWriter, r *http.Request) {
	now := timeutil.Now()
	from, to, err := httpx.DateRange(r, timeutil.StartOfMonth(now), timeutil.EndOfDay(now))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	rows, err := h.service.ByWallet(r.Context(), from, to)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	resp := make([]WalletBreakdownResponse, 0, len(rows))
	for _, b := range rows {
		resp = append(resp, WalletBreakdownResponse{
			WalletID:   b.WalletID,
			WalletName: b.WalletName,
			IsDeleted:  b.IsDeleted,
			Income:     b.Income,
			Expense:    b.Expense,
			Net:        b.Net,
			Count:      b.Count,
		})
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, resp)
}

func (h *AnalyticsHandler) HandlerTrend(w http.ResponseWriter, r *http.Request) {
	now := timeutil.Now()
	from, to, err := httpx.DateRange(r, timeutil.StartOfMonth(now).AddDate(0, -5, 0), timeutil.EndOfDay(now))
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	granularity := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("granularity")))
	if granularity == "" {
		granularity = "month"
	}
	if !validator.IsOneOf(granularity, "day", "week", "month") {
		response.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "granularity harus day, week, atau month", map[string]string{"granularity": granularity})
		return
	}

	buckets, err := h.service.Trend(r.Context(), from, to, granularity)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	resp := make([]TrendBucketResponse, 0, len(buckets))
	for _, b := range buckets {
		resp = append(resp, TrendBucketResponse{
			Bucket:  b.Bucket,
			Start:   timeutil.FormatDate(b.Start),
			End:     timeutil.FormatDate(b.End),
			Income:  b.Income,
			Expense: b.Expense,
			Net:     b.Net,
		})
	}

	response.WriteSuccessWithSingleData(w, http.StatusOK, resp)
}
