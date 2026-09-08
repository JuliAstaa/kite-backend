// Package wiring berisi adapter antar feature.
//
// Feature tidak boleh saling import. Tiap feature mendeklarasikan interface
// kecil di sisi yang memakai, lalu package ini yang menjembatani interface itu
// ke service milik feature lain. Dengan begitu satu-satunya tempat yang tahu
// dua feature sekaligus adalah package wiring dan main.go.
package wiring

import (
	"backend/internal/features/budget"
	"backend/internal/features/category"
	"backend/internal/features/quickadd"
	"backend/internal/features/recurring"
	"backend/internal/features/saving"
	"backend/internal/features/transaction"
	"backend/internal/features/wishlist"
	"context"
)

// --- category -> transaction, budget, wishlist, recurring, quickadd ---

type categoryReaderAdapter struct {
	svc *category.CategoryService
}

func (a *categoryReaderAdapter) info(ctx context.Context, id string) (string, string, error) {
	cat, err := a.svc.GetCategoryByID(ctx, id)
	if err != nil {
		return "", "", err
	}
	return cat.ID, cat.Type, nil
}

func NewTransactionCategoryReader(svc *category.CategoryService) transaction.CategoriesReader {
	return &transactionCategoryReader{categoryReaderAdapter{svc: svc}}
}

type transactionCategoryReader struct{ categoryReaderAdapter }

func (a *transactionCategoryReader) GetCategoryInfo(ctx context.Context, id string) (transaction.CategoryInfo, error) {
	catID, catType, err := a.info(ctx, id)
	if err != nil {
		return transaction.CategoryInfo{}, err
	}
	return transaction.CategoryInfo{ID: catID, Type: catType}, nil
}

func NewBudgetCategoryReader(svc *category.CategoryService) budget.CategoriesReader {
	return &budgetCategoryReader{categoryReaderAdapter{svc: svc}}
}

type budgetCategoryReader struct{ categoryReaderAdapter }

func (a *budgetCategoryReader) GetCategoryInfo(ctx context.Context, id string) (budget.CategoryInfo, error) {
	catID, catType, err := a.info(ctx, id)
	if err != nil {
		return budget.CategoryInfo{}, err
	}
	return budget.CategoryInfo{ID: catID, Type: catType}, nil
}

func NewWishlistCategoryReader(svc *category.CategoryService) wishlist.CategoriesReader {
	return &wishlistCategoryReader{categoryReaderAdapter{svc: svc}}
}

type wishlistCategoryReader struct{ categoryReaderAdapter }

func (a *wishlistCategoryReader) GetCategoryInfo(ctx context.Context, id string) (wishlist.CategoryInfo, error) {
	catID, catType, err := a.info(ctx, id)
	if err != nil {
		return wishlist.CategoryInfo{}, err
	}
	return wishlist.CategoryInfo{ID: catID, Type: catType}, nil
}

func NewRecurringCategoryReader(svc *category.CategoryService) recurring.CategoriesReader {
	return &recurringCategoryReader{categoryReaderAdapter{svc: svc}}
}

type recurringCategoryReader struct{ categoryReaderAdapter }

func (a *recurringCategoryReader) GetCategoryInfo(ctx context.Context, id string) (recurring.CategoryInfo, error) {
	catID, catType, err := a.info(ctx, id)
	if err != nil {
		return recurring.CategoryInfo{}, err
	}
	return recurring.CategoryInfo{ID: catID, Type: catType}, nil
}

func NewQuickAddCategoryReader(svc *category.CategoryService) quickadd.CategoriesReader {
	return &quickAddCategoryReader{categoryReaderAdapter{svc: svc}}
}

type quickAddCategoryReader struct{ categoryReaderAdapter }

func (a *quickAddCategoryReader) GetCategoryInfo(ctx context.Context, id string) (quickadd.CategoryInfo, error) {
	catID, catType, err := a.info(ctx, id)
	if err != nil {
		return quickadd.CategoryInfo{}, err
	}
	return quickadd.CategoryInfo{ID: catID, Type: catType}, nil
}

// --- saving -> wishlist ---

type savingsReaderAdapter struct {
	svc *saving.SavingService
}

// NewWishlistSavingsReader menyambungkan perhitungan kemampuan menabung ke
// fitur wishlist, tanpa wishlist perlu tahu package saving.
func NewWishlistSavingsReader(svc *saving.SavingService) wishlist.SavingsReader {
	return &savingsReaderAdapter{svc: svc}
}

func (a *savingsReaderAdapter) AverageMonthlySavable(ctx context.Context, months int) (int, int, error) {
	return a.svc.AverageMonthlySavable(ctx, months)
}

// --- transaction -> quickadd ---

type transactionCreatorAdapter struct {
	svc *transaction.TransactionService
}

// NewQuickAddTransactionCreator membuat quick add menulis transaksi lewat
// service transaction, sehingga seluruh aturan bisnis transaksi tetap jalan.
func NewQuickAddTransactionCreator(svc *transaction.TransactionService) quickadd.TransactionCreator {
	return &transactionCreatorAdapter{svc: svc}
}

func (a *transactionCreatorAdapter) CreateTransaction(ctx context.Context, input quickadd.CreateTransactionInput) (quickadd.CreatedTransaction, error) {
	detail, err := a.svc.CreateTransaction(ctx, transaction.CreateTransactionRequest{
		Type:       input.Type,
		Amount:     input.Amount,
		WalletID:   input.WalletID,
		CategoryID: input.CategoryID,
		Note:       input.Note,
		OccurredAt: input.OccurredAt,
	})
	if err != nil {
		return quickadd.CreatedTransaction{}, err
	}

	return quickadd.CreatedTransaction{
		ID:         detail.ID,
		Type:       detail.Type,
		Amount:     detail.Amount,
		Note:       detail.Note,
		OccurredAt: detail.OccurredAt,
	}, nil
}
