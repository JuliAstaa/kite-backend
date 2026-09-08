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
	Details map[string]string `json:"details,omitempty"`
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

func WriteError(w http.ResponseWriter, status int, code string, message string, details map[string]string) {
	writeJSON(w, status, ErrorResponse{Error: APIError{
		Code:    code,
		Message: message,
		Details: details,
	}})
}

// WriteServiceError memetakan error dari service ke status dan kode HTTP.
// ValidationError ikut membawa nama field-nya ke details.
func WriteServiceError(w http.ResponseWriter, err error) {
	var ve apperror.ValidationError
	if errors.As(err, &ve) {
		WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", ve.Message, map[string]string{ve.Field: ve.Message})
		return
	}

	status, code := StatusFromError(err)
	WriteError(w, status, code, err.Error(), nil)
}

func WriteSuccessWithSingleData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, SingleDataResponse{Data: data})
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
	w.WriteHeader(status)
}

func StatusFromError(err error) (int, string) {
	var ae apperror.AlreadyExistsErr
	if errors.As(err, &ae) {
		return http.StatusConflict, "CONFLICT"
	}

	var ce apperror.ConflictError
	if errors.As(err, &ce) {
		return http.StatusConflict, "CONFLICT"
	}

	var ve apperror.ValidationError
	if errors.As(err, &ve) {
		return http.StatusBadRequest, "VALIDATION_ERROR"
	}

	var ue apperror.UnprocessableError
	if errors.As(err, &ue) {
		return http.StatusUnprocessableEntity, "UNPROCESSABLE"
	}

	var nf apperror.NotFoundError
	if errors.As(err, &nf) {
		return http.StatusNotFound, "NOT_FOUND"
	}

	return http.StatusInternalServerError, "INTERNAL_ERROR"
}
