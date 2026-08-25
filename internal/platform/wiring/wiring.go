package wiring

import (
	"backend/internal/features/category"
	"backend/internal/features/transaction"
	"context"
)

type categoryCheckerAdapter struct {
	svc *category.CategoryService
}

func NewCategoryCheckerAdapter(svc *category.CategoryService) transaction.CategoriesReader {
	return &categoryCheckerAdapter{svc: svc}
}

func (a *categoryCheckerAdapter) GetCategoryInfo(ctx context.Context, id string) (transaction.CategoryInfo, error) {
	cat, err := a.svc.GetCategoryByID(ctx, id)
	if err != nil {
		return transaction.CategoryInfo{}, err
	}
	return transaction.CategoryInfo{
		ID:   cat.ID,
		Type: cat.Type,
	}, nil
}
