package transaction

import (
	"backend/internal/shared/apperror"
	"context"
	"time"
)

type WalletsReader interface {
	IsWalletExist(ctx context.Context, id string) error
}

type CategoriesReader interface {
	GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error)
}

type TransactionServicer interface {
	CreateTransaction(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error)
	GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error)
}

type TransactionService struct {
	repo     TransactionRepositorer
	wallet   WalletsReader
	category CategoriesReader
}

func NewTransactionService(repo TransactionRepositorer, wallet WalletsReader, category CategoriesReader) *TransactionService {
	return &TransactionService{repo: repo, wallet: wallet, category: category}
}

func (s *TransactionService) CreateTransaction(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error) {
	if err := s.wallet.IsWalletExist(ctx, reqBody.WalletID); err != nil {
		return TransactionDetail{}, err
	}
	switch reqBody.Type {
	case "income", "expense":

		if reqBody.CategoryID == nil {
			return TransactionDetail{}, apperror.ValidationError{Field: "category_id", Message: "id kategori wajib diisi"}
		}

		cat, err := s.category.GetCategoryInfo(ctx, *reqBody.CategoryID)
		if err != nil {
			return TransactionDetail{}, err
		}

		if reqBody.Type != cat.Type {
			return TransactionDetail{}, apperror.ValidationError{Field: "category_type", Message: "category tidak seusai dengan expese maupun income"}
		}

		reqBody.ToWalletID = nil

	case "transfer":
		if reqBody.ToWalletID == nil {
			return TransactionDetail{}, apperror.ValidationError{Field: "to_wallet_id", Message: "to_wallet_id wajib diisi"}
		}

		if err := s.wallet.IsWalletExist(ctx, *reqBody.ToWalletID); err != nil {
			return TransactionDetail{}, err
		}

		if reqBody.WalletID == *reqBody.ToWalletID {
			return TransactionDetail{}, apperror.ValidationError{Field: "to_wallet_id", Message: "to_wallet_id tidak boleh sama dengan wallet id"}
		}

		reqBody.CategoryID = nil

	default:
		return TransactionDetail{}, apperror.ValidationError{Field: "type", Message: "tipe transaksi tidak ada yang sesuai"}
	}

	if reqBody.Amount <= 0 {
		return TransactionDetail{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih dari 0"}
	}

	now := time.Now()
	today := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		0, 0, 0, 0,
		now.Location(),
	)

	maxDay := today.AddDate(0, 0, 2)

	if reqBody.OccurredAt.After(maxDay) {
		return TransactionDetail{}, apperror.ValidationError{Field: "occurred_at", Message: "occurred_at hanya bsia hari ini, sebelumnya, dan besok"}
	}

	params := CreateTransactionParams{
		Type:       reqBody.Type,
		Amount:     reqBody.Amount,
		WalletID:   reqBody.WalletID,
		CategoryID: reqBody.CategoryID,
		ToWalletID: reqBody.ToWalletID,
		Note:       reqBody.Note,
		OccurredAt: reqBody.OccurredAt,
	}

	return s.repo.CreateTransaction(ctx, params)

}

func (s *TransactionService) GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {
	return s.repo.GetAllTransactions(ctx, filter)
}
