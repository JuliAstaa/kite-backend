package response

import (
	"backend/internal/shared/apperror"
	"encoding/json"
	"errors"
	"net/http"
)

type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Detail  map[string]string `json:"detail,omitempty"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

type SingleDataResponse struct {
	Data any `json:"data"`
}

type APIMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type MultipleDataResponse struct {
	Data any     `json:"data"`
	Meta APIMeta `json:"meta"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, code string, message string, detail map[string]string) {
	writeJSON(w, status, APIError{
		Code:    code,
		Message: message,
		Detail:  detail,
	})
}

func WriteSuccessWithSingleData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, data)
}

func WriteSuccessWithMultipleData(w http.ResponseWriter, status int, data any, meta APIMeta) {
	writeJSON(w, status, MultipleDataResponse{
		Data: data,
		Meta: APIMeta{
			Total:  meta.Total,
			Limit:  meta.Limit,
			Offset: meta.Offset,
		},
	})
}

func WriteSuccessNoData(w http.ResponseWriter, status int) {
	writeJSON(w, status, nil)
}

func StatusFromError(err error) (int, string) {
	if errors.Is(err, apperror.ErrCategoryAlreadyExists) {
		return http.StatusConflict, "ALREADY_EXIST"
	}

	var nf apperror.NotFoundError
	if errors.As(err, &nf) {
		return http.StatusNotFound, "NOT_FOUND"
	}

	return http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"
}
