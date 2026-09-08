package quickadd

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"strings"
)

type WalletsReader interface {
	IsWalletExist(ctx context.Context, id string) error
}

type CategoriesReader interface {
	GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error)
}

// TransactionCreator membuat transaksi lewat service transaction, bukan lewat
// SQL langsung, supaya seluruh aturan bisnis transaksi tetap berlaku.
type TransactionCreator interface {
	CreateTransaction(ctx context.Context, input CreateTransactionInput) (CreatedTransaction, error)
}

type QuickAddServicer interface {
	CreateQuickAdd(ctx context.Context, req CreateQuickAddRequest) (QuickAdd, error)
	GetAllQuickAdds(ctx context.Context, includeDeleted bool) ([]QuickAdd, int, error)
	PatchQuickAdd(ctx context.Context, id string, req PatchQuickAddRequest) (QuickAdd, error)
	DeleteQuickAdd(ctx context.Context, id string) (QuickAdd, error)
	Execute(ctx context.Context, id string, req ExecuteRequest) (CreatedTransaction, error)
}

type QuickAddService struct {
	repo        QuickAddRepositorer
	wallet      WalletsReader
	category    CategoriesReader
	transaction TransactionCreator
}

func NewQuickAddService(repo QuickAddRepositorer, wallet WalletsReader, category CategoriesReader, transaction TransactionCreator) *QuickAddService {
	return &QuickAddService{repo: repo, wallet: wallet, category: category, transaction: transaction}
}

func (s *QuickAddService) CreateQuickAdd(ctx context.Context, req CreateQuickAddRequest) (QuickAdd, error) {
	label := strings.TrimSpace(req.Label)
	if label == "" {
		return QuickAdd{}, apperror.ValidationError{Field: "label", Message: "label tidak boleh kosong"}
	}

	quickType := strings.ToLower(strings.TrimSpace(req.Type))
	if quickType != "income" && quickType != "expense" {
		return QuickAdd{}, apperror.ValidationError{Field: "type", Message: "type harus income atau expense"}
	}

	// amount boleh kosong, artinya nominalnya diisi manual saat dipakai
	if req.Amount != nil && *req.Amount <= 0 {
		return QuickAdd{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
	}

	if err := s.validateWalletAndCategory(ctx, req.WalletID, req.CategoryID, quickType); err != nil {
		return QuickAdd{}, err
	}

	return s.repo.CreateQuickAdd(ctx, CreateQuickAddParams{
		Label:      label,
		Type:       quickType,
		Amount:     req.Amount,
		WalletID:   req.WalletID,
		CategoryID: req.CategoryID,
		Note:       strings.TrimSpace(req.Note),
	})
}

func (s *QuickAddService) GetAllQuickAdds(ctx context.Context, includeDeleted bool) ([]QuickAdd, int, error) {
	return s.repo.GetAllQuickAdds(ctx, includeDeleted)
}

func (s *QuickAddService) PatchQuickAdd(ctx context.Context, id string, req PatchQuickAddRequest) (QuickAdd, error) {
	existing, err := s.repo.GetQuickAddByID(ctx, id)
	if err != nil {
		return QuickAdd{}, err
	}

	param := PatchQuickAddParams{
		Note:        req.Note,
		SortOrder:   req.SortOrder,
		ClearAmount: req.ClearAmount,
	}

	if req.Label != nil {
		label := strings.TrimSpace(*req.Label)
		if label == "" {
			return QuickAdd{}, apperror.ValidationError{Field: "label", Message: "label tidak boleh kosong"}
		}
		param.Label = &label
	}

	quickType := existing.Type
	if req.Type != nil {
		quickType = strings.ToLower(strings.TrimSpace(*req.Type))
		if quickType != "income" && quickType != "expense" {
			return QuickAdd{}, apperror.ValidationError{Field: "type", Message: "type harus income atau expense"}
		}
		param.Type = &quickType
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return QuickAdd{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
		}
		param.Amount = req.Amount
	}

	if req.WalletID != nil {
		if err := s.wallet.IsWalletExist(ctx, *req.WalletID); err != nil {
			return QuickAdd{}, err
		}
		param.WalletID = req.WalletID
	}

	// Kategori dicek ulang terhadap tipe akhir, supaya quick add bertipe expense
	// tidak bisa menunjuk kategori income.
	categoryID := existing.CategoryID
	if req.CategoryID != nil {
		categoryID = *req.CategoryID
		param.CategoryID = req.CategoryID
	}
	if req.CategoryID != nil || req.Type != nil {
		cat, err := s.category.GetCategoryInfo(ctx, categoryID)
		if err != nil {
			return QuickAdd{}, err
		}
		if cat.Type != quickType {
			return QuickAdd{}, apperror.ValidationError{
				Field:   "category_id",
				Message: "tipe kategori (" + cat.Type + ") tidak cocok dengan tipe quick add (" + quickType + ")",
			}
		}
	}

	return s.repo.PatchQuickAdd(ctx, id, param)
}

func (s *QuickAddService) DeleteQuickAdd(ctx context.Context, id string) (QuickAdd, error) {
	return s.repo.DeleteQuickAdd(ctx, id)
}

// Execute membuat transaksi dari satu quick add.
//
// Nominal diambil dari quick add, kecuali client mengirim amount sendiri.
// Kalau dua-duanya kosong, request ditolak.
func (s *QuickAddService) Execute(ctx context.Context, id string, req ExecuteRequest) (CreatedTransaction, error) {
	quickAdd, err := s.repo.GetQuickAddByID(ctx, id)
	if err != nil {
		return CreatedTransaction{}, err
	}

	amount := 0
	switch {
	case req.Amount != nil:
		amount = *req.Amount
	case quickAdd.Amount.Valid:
		amount = int(quickAdd.Amount.Int64)
	default:
		return CreatedTransaction{}, apperror.ValidationError{
			Field:   "amount",
			Message: "quick add ini tidak punya amount tetap, kirim amount di request",
		}
	}

	occurredAt := req.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = timeutil.Now()
	}

	note := strings.TrimSpace(req.Note)
	if note == "" {
		note = quickAdd.Note
	}

	categoryID := quickAdd.CategoryID

	return s.transaction.CreateTransaction(ctx, CreateTransactionInput{
		Type:       quickAdd.Type,
		Amount:     amount,
		WalletID:   quickAdd.WalletID,
		CategoryID: &categoryID,
		Note:       note,
		OccurredAt: occurredAt,
	})
}

func (s *QuickAddService) validateWalletAndCategory(ctx context.Context, walletID, categoryID, quickType string) error {
	if err := s.wallet.IsWalletExist(ctx, walletID); err != nil {
		return err
	}

	cat, err := s.category.GetCategoryInfo(ctx, categoryID)
	if err != nil {
		return err
	}
	if cat.Type != quickType {
		return apperror.ValidationError{
			Field:   "category_id",
			Message: "tipe kategori (" + cat.Type + ") tidak cocok dengan tipe quick add (" + quickType + ")",
		}
	}

	return nil
}
