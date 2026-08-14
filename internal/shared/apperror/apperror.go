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

var ErrCategoryAlreadyExists = errors.New("kategori dengan nama dan tipe yang sama sudah ada")
