package transaction

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"strings"
	"time"
)

// WalletsReader dan CategoriesReader dideklarasikan di sini, di sisi yang
// memakai. Feature wallet dan category tidak perlu tahu siapa pemakainya.
type WalletsReader interface {
	IsWalletExist(ctx context.Context, id string) error
}

type CategoriesReader interface {
	GetCategoryInfo(ctx context.Context, id string) (CategoryInfo, error)
}

type TransactionServicer interface {
	CreateTransaction(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error)
	GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error)
	GetTransactionByID(ctx context.Context, id string) (TransactionDetail, error)
	PatchTransaction(ctx context.Context, id string, reqBody PatchTransactionRequest) (TransactionDetail, error)
	DeleteTransaction(ctx context.Context, id string) (TransactionDetail, error)
	RestoreTransaction(ctx context.Context, id string) (TransactionDetail, error)
}

type TransactionService struct {
	repo     TransactionRepositorer
	wallet   WalletsReader
	category CategoriesReader
}

func NewTransactionService(repo TransactionRepositorer, wallet WalletsReader, category CategoriesReader) *TransactionService {
	return &TransactionService{repo: repo, wallet: wallet, category: category}
}

// validated adalah bentuk transaksi yang sudah lolos semua aturan bisnis.
type validated struct {
	Type       string
	Amount     int
	WalletID   string
	ToWalletID *string
	CategoryID *string
	Note       string
	OccurredAt time.Time
}

// validate menegakkan business rules 1 sampai 7 dari PRD. Dipakai bersama oleh
// create dan patch, supaya patch tidak bisa menyelundupkan keadaan yang tidak
// mungkin dibuat lewat create.
func (s *TransactionService) validate(ctx context.Context, in validated) (validated, error) {
	in.Type = strings.ToLower(strings.TrimSpace(in.Type))

	// aturan 1: arah uang ditentukan type, bukan tanda minus
	if in.Amount <= 0 {
		return validated{}, apperror.ValidationError{Field: "amount", Message: "amount harus lebih besar dari 0"}
	}

	// aturan 6: wallet yang sudah dihapus tidak boleh dipakai transaksi baru
	if err := s.wallet.IsWalletExist(ctx, in.WalletID); err != nil {
		return validated{}, err
	}

	switch in.Type {
	case "income", "expense":
		// aturan 4
		if in.CategoryID == nil || strings.TrimSpace(*in.CategoryID) == "" {
			return validated{}, apperror.ValidationError{Field: "category_id", Message: "category_id wajib diisi untuk income dan expense"}
		}

		cat, err := s.category.GetCategoryInfo(ctx, *in.CategoryID)
		if err != nil {
			return validated{}, err
		}

		// aturan 2
		if cat.Type != in.Type {
			return validated{}, apperror.ValidationError{
				Field:   "category_id",
				Message: "tipe kategori (" + cat.Type + ") tidak cocok dengan tipe transaksi (" + in.Type + ")",
			}
		}

		in.ToWalletID = nil

	case "transfer":
		// aturan 3
		if in.ToWalletID == nil || strings.TrimSpace(*in.ToWalletID) == "" {
			return validated{}, apperror.ValidationError{Field: "to_wallet_id", Message: "to_wallet_id wajib diisi untuk transfer"}
		}

		if in.WalletID == *in.ToWalletID {
			return validated{}, apperror.ValidationError{Field: "to_wallet_id", Message: "to_wallet_id tidak boleh sama dengan wallet_id"}
		}

		if err := s.wallet.IsWalletExist(ctx, *in.ToWalletID); err != nil {
			return validated{}, err
		}

		in.CategoryID = nil

	default:
		return validated{}, apperror.ValidationError{Field: "type", Message: "type harus income, expense, atau transfer"}
	}

	if in.OccurredAt.IsZero() {
		return validated{}, apperror.ValidationError{Field: "occurred_at", Message: "occurred_at wajib diisi"}
	}

	// aturan 5: backdate bebas, tapi maksimal 1 hari ke depan
	maxOccurredAt := timeutil.EndOfDay(timeutil.Now().AddDate(0, 0, 1))
	if in.OccurredAt.After(maxOccurredAt) {
		return validated{}, apperror.ValidationError{Field: "occurred_at", Message: "occurred_at tidak boleh lebih dari 1 hari di masa depan"}
	}

	in.Note = strings.TrimSpace(in.Note)

	// aturan 7 (saldo boleh minus) sengaja tidak dicek. Ini pencatatan,
	// bukan sistem pembayaran.
	return in, nil
}

func (s *TransactionService) CreateTransaction(ctx context.Context, reqBody CreateTransactionRequest) (TransactionDetail, error) {
	v, err := s.validate(ctx, validated{
		Type:       reqBody.Type,
		Amount:     reqBody.Amount,
		WalletID:   reqBody.WalletID,
		ToWalletID: reqBody.ToWalletID,
		CategoryID: reqBody.CategoryID,
		Note:       reqBody.Note,
		OccurredAt: reqBody.OccurredAt,
	})
	if err != nil {
		return TransactionDetail{}, err
	}

	return s.repo.CreateTransaction(ctx, CreateTransactionParams{
		Type:       v.Type,
		Amount:     v.Amount,
		WalletID:   v.WalletID,
		ToWalletID: v.ToWalletID,
		CategoryID: v.CategoryID,
		Note:       v.Note,
		OccurredAt: v.OccurredAt,
	})
}

func (s *TransactionService) GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {
	return s.repo.GetAllTransactions(ctx, filter)
}

func (s *TransactionService) GetTransactionByID(ctx context.Context, id string) (TransactionDetail, error) {
	return s.repo.GetTransactionByID(ctx, id)
}

// PatchTransaction menggabungkan field yang dikirim dengan data lama, lalu
// mengecek ulang seluruh aturan. Aturan 8: yang berubah cuma transaksinya,
// recurring rule asalnya tidak disentuh.
func (s *TransactionService) PatchTransaction(ctx context.Context, id string, reqBody PatchTransactionRequest) (TransactionDetail, error) {
	existing, err := s.repo.GetTransactionRaw(ctx, id)
	if err != nil {
		return TransactionDetail{}, err
	}

	merged := validated{
		Type:       existing.Type,
		Amount:     existing.Amount,
		WalletID:   existing.WalletID,
		Note:       existing.Note,
		OccurredAt: existing.OccurredAt,
	}
	if existing.ToWalletID.Valid {
		toWalletID := existing.ToWalletID.String
		merged.ToWalletID = &toWalletID
	}
	if existing.CategoryID.Valid {
		categoryID := existing.CategoryID.String
		merged.CategoryID = &categoryID
	}

	if reqBody.Type != nil {
		merged.Type = *reqBody.Type
	}
	if reqBody.Amount != nil {
		merged.Amount = *reqBody.Amount
	}
	if reqBody.WalletID != nil {
		merged.WalletID = *reqBody.WalletID
	}
	if reqBody.ToWalletID != nil {
		merged.ToWalletID = reqBody.ToWalletID
	}
	if reqBody.CategoryID != nil {
		merged.CategoryID = reqBody.CategoryID
	}
	if reqBody.Note != nil {
		merged.Note = *reqBody.Note
	}
	if reqBody.OccurredAt != nil {
		merged.OccurredAt = *reqBody.OccurredAt
	}

	// Ganti tipe dari transfer ke income/expense (atau sebaliknya) berarti
	// field lawannya harus dibuang, bukan dibawa dari data lama.
	if reqBody.Type != nil && *reqBody.Type != existing.Type {
		switch strings.ToLower(strings.TrimSpace(*reqBody.Type)) {
		case "income", "expense":
			merged.ToWalletID = nil
			if reqBody.CategoryID == nil {
				merged.CategoryID = nil
			}
		case "transfer":
			merged.CategoryID = nil
			if reqBody.ToWalletID == nil {
				merged.ToWalletID = nil
			}
		}
	}

	v, err := s.validate(ctx, merged)
	if err != nil {
		return TransactionDetail{}, err
	}

	return s.repo.UpdateTransaction(ctx, id, UpdateTransactionParams{
		Type:       v.Type,
		Amount:     v.Amount,
		WalletID:   v.WalletID,
		ToWalletID: v.ToWalletID,
		CategoryID: v.CategoryID,
		Note:       v.Note,
		OccurredAt: v.OccurredAt,
	})
}

func (s *TransactionService) DeleteTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	return s.repo.DeleteTransaction(ctx, id)
}

func (s *TransactionService) RestoreTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	return s.repo.RestoreTransaction(ctx, id)
}
