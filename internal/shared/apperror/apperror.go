package apperror

import (
	"fmt"
)

type NotFoundError struct {
	Resource string
	ID       string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s dengan id %s tidak ditemukan", e.Resource, e.ID)
}

type AlreadyExistsErr struct {
	Resource string
	Name     string
	Type     string
}

func (e AlreadyExistsErr) Error() string {
	return fmt.Sprintf("%s dengan name %s dan %s yang sama sudah ada", e.Resource, e.Name, e.Type)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ConflictError dipakai saat request bentrok dengan keadaan data sekarang,
// misalnya budget aktif kedua untuk kategori yang sama.
type ConflictError struct {
	Message string
}

func (e ConflictError) Error() string { return e.Message }

// UnprocessableError dipakai saat request-nya valid secara bentuk tapi tidak
// bisa dijalankan, misalnya membeli item wishlist yang sudah berstatus purchased.
type UnprocessableError struct {
	Message string
}

func (e UnprocessableError) Error() string { return e.Message }
