package apperror

import (
	"errors"
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

var ErrCategoryAlreadyExists = errors.New("kategori dengan nama dan tipe yang sama sudah ada")
