package transaction

import (
	"context"
	"database/sql"
)

type TransactionRepositorer interface {
}

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) CreateTransaction(ctx context.Context, param *CreateTransactionParams) (Transaction, TransactionDetail, error) {

	var tsx Transaction
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO transactions(type, amount, wallet_id, to_wallet_id, category_id, note, occurred_at, recurring_rule_id, wishlist_item_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9) 
		RETURNING id, type, amount, wallet_id, to_wallet_id, category_id, note, occurred_at, recurring_rule_id, wishlist_item_id, created_at, updated_at, deleted_at`, param.Type, param.Amount, param.WalletID, param.ToWalletID, param.CategoryID, param.Note, param.OccurredAt, param.RecurringRuleID, param.WishlistItemID).Scan(
		&tsx.ID,
		&tsx.Type,
		&tsx.Amount,
		&tsx.WalletID,
		&tsx.ToWalletID,
		&tsx.CategoryID,
		&tsx.Note,
		&tsx.OccurredAt,
		&tsx.RecurringRuleID,
		&tsx.WishlistItemID,
		&tsx.CreatedAt,
		&tsx.UpdatedAt,
		&tsx.DeletedAt,
	)
	if err != nil {
		return Transaction{}, TransactionDetail{}, err
	}

	var catIdTemp, catNameTemp, catTypeTemp sql.NullString
	var walletIdTetmp, walletNameTemp sql.NullString
	var catDelTemp, walletDeleTemp, toWalletDeleTemp sql.NullTime

	var txDetail TransactionDetail
	err = r.db.QueryRowContext(ctx, `SELECT t.id, t.type, t.amount, t.note, t.occurred_at,  
									w.id, w.name, w.deleted_at,
									c.id, c.name, c.type, c.deleted_at,
									tw.id, tw.name, tw.deleted_at FROM transactions t
									JOIN wallets w ON w.id = t.wallet_id
									LEFT JOIN categories c ON c.id = t.category_id
									LEFT JOIN wallets tw ON tw.id = t.to_wallet_id
									WHERE t.id = $1 AND t.deleted_at IS NULL`, tsx.ID).Scan(
		&txDetail.ID,
		&txDetail.Type,
		&txDetail.Amount,
		&txDetail.Note,
		&txDetail.OccurredAt,
		&txDetail.Wallet.ID,
		&txDetail.Wallet.Name,
		&walletDeleTemp,
		&catIdTemp,
		&catNameTemp,
		&catTypeTemp,
		&catDelTemp,
		&walletIdTetmp,
		&walletNameTemp,
		&toWalletDeleTemp,
	)

	if err != nil {
		return Transaction{}, TransactionDetail{}, err
	}

	txDetail.Wallet.IsDeleted = walletDeleTemp.Valid

	if catIdTemp.Valid {
		txDetail.Category = &CategoryRef{
			ID:        catIdTemp.String,
			Name:      catNameTemp.String,
			Type:      catTypeTemp.String,
			IsDeleted: catDelTemp.Valid,
		}
	}

	if walletIdTetmp.Valid {
		txDetail.ToWallet = &WalletRef{
			ID:        walletIdTetmp.String,
			Name:      walletNameTemp.String,
			IsDeleted: toWalletDeleTemp.Valid,
		}
	}

	return tsx, txDetail, nil
}
