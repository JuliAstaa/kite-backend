package transaction

import (
	"backend/internal/shared/validator"
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type TransactionRepositorer interface {
	CreateTransaction(ctx context.Context, param CreateTransactionParams) (TransactionDetail, error)
	GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error)
}

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) CreateTransaction(ctx context.Context, param CreateTransactionParams) (TransactionDetail, error) {

	var tsx Transaction
	var txDetail TransactionDetail
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO transactions(type, amount, wallet_id, to_wallet_id, category_id, note, occurred_at, recurring_rule_id, wishlist_item_id) 
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9) 
		RETURNING id`, param.Type, param.Amount, param.WalletID, param.ToWalletID, param.CategoryID, param.Note, param.OccurredAt, param.RecurringRuleID, param.WishlistItemID).Scan(
		&tsx.ID,
	)
	if err != nil {
		return TransactionDetail{}, err
	}

	var catIdTemp, catNameTemp, catTypeTemp sql.NullString
	var walletIdTetmp, walletNameTemp sql.NullString
	var catDelTemp, walletDelTemp, toWalletDelTemp sql.NullTime

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
		&walletDelTemp,
		&catIdTemp,
		&catNameTemp,
		&catTypeTemp,
		&catDelTemp,
		&walletIdTetmp,
		&walletNameTemp,
		&toWalletDelTemp,
	)

	if err != nil {
		return TransactionDetail{}, err
	}

	txDetail.Wallet.IsDeleted = walletDelTemp.Valid

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
			IsDeleted: toWalletDelTemp.Valid,
		}
	}

	return txDetail, nil
}

func (r *TransactionRepository) GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {

	condition := []string{"t.deleted_at IS NULL"}
	args := []any{}

	if !filter.From.IsZero() {
		args = append(args, filter.From)
		condition = append(condition, fmt.Sprintf("t.occurred_at >= $%d", len(args)))
	}

	if !filter.To.IsZero() {
		args = append(args, filter.To)
		condition = append(condition, fmt.Sprintf("t.occurred_at <= $%d", len(args)))
	}

	if !validator.IsEmptyString(filter.Type) {
		args = append(args, filter.Type)
		condition = append(condition, fmt.Sprintf("t.type = $%d", len(args)))
	}

	if !validator.IsEmptyString(filter.CategoryID) {
		args = append(args, filter.CategoryID)
		condition = append(condition, fmt.Sprintf("t.category_id = $%d", len(args)))
	}

	if !validator.IsEmptyString(filter.WalletID) {
		args = append(args, filter.WalletID)
		condition = append(condition, fmt.Sprintf("t.wallet_id = $%d", len(args)))
	}

	if filter.MinAmount > 0 {
		args = append(args, filter.MinAmount)
		condition = append(condition, fmt.Sprintf("t.amount  >= $%d", len(args)))
	}

	if filter.MaxAmount > 0 {
		args = append(args, filter.MaxAmount)
		condition = append(condition, fmt.Sprintf("t.amount  <= $%d", len(args)))
	}

	if !validator.IsEmptyString(filter.Query) {
		args = append(args, "%"+filter.Query+"%")
		condition = append(condition, fmt.Sprintf("t.note ILIKE $%d", len(args)))
	}

	whereClause := strings.Join(condition, "  AND ")

	orderColumn := "t.occurred_at"
	orderDir := "DESC"
	if !validator.IsEmptyString(filter.Sort) {
		parts := strings.SplitN(filter.Sort, ":", 2)
		switch parts[0] {
		case "amount":
			orderColumn = "t.amount"
		case "occurred_at":
			orderColumn = "t.occurred_at"
			// kolom lain nggak dikenal -> tetep fallback ke occurred_at, bukan dipakai mentah
		}
		if len(parts) == 2 && strings.EqualFold(parts[1], "asc") {
			orderDir = "ASC"
		}
	}

	// args buat COUNT (cuma filter, TANPA limit/offset)
	countArgs := append([]any{}, args...)

	args = append(args, filter.Limit)
	limitPos := len(args)
	args = append(args, filter.Offset)
	offsetPos := len(args)

	query := fmt.Sprintf(`SELECT t.id, t.type, t.amount, t.note, t.occurred_at,
										w.id, w.name, w.deleted_at,
										c.id, c.name, c.type, c.deleted_at,
										tw.id, tw.name, tw.deleted_at
										FROM transactions t 
										JOIN wallets w ON w.id = t.wallet_id
										LEFT JOIN categories c ON c.id = t.category_id
										LEFT JOIN wallets tw ON tw.id = t.to_wallet_id
										WHERE %s
										ORDER BY %s %s
										LIMIT $%d OFFSET $%d`, whereClause, orderColumn, orderDir, limitPos, offsetPos)

	rows, err := r.db.QueryContext(ctx, query, args...)

	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	tsxDetails := []TransactionDetail{}
	for rows.Next() {
		var catIdTemp, catNameTemp, catTypeTemp sql.NullString
		var walletIdTetmp, walletNameTemp sql.NullString
		var catDelTemp, walletDelTemp, toWalletDelTemp sql.NullTime
		var tsx TransactionDetail
		if err := rows.Scan(
			&tsx.ID,
			&tsx.Type,
			&tsx.Amount,
			&tsx.Note,
			&tsx.OccurredAt,
			&tsx.Wallet.ID,
			&tsx.Wallet.Name,
			&walletDelTemp,
			&catIdTemp,
			&catNameTemp,
			&catTypeTemp,
			&catDelTemp,
			&walletIdTetmp,
			&walletNameTemp,
			&toWalletDelTemp,
		); err != nil {
			return nil, 0, err
		}

		tsx.Wallet.IsDeleted = walletDelTemp.Valid

		if catIdTemp.Valid {
			tsx.Category = &CategoryRef{
				ID:        catIdTemp.String,
				Name:      catNameTemp.String,
				Type:      catTypeTemp.String,
				IsDeleted: catDelTemp.Valid,
			}
		}

		if walletIdTetmp.Valid {
			tsx.ToWallet = &WalletRef{
				ID:        walletIdTetmp.String,
				Name:      walletNameTemp.String,
				IsDeleted: toWalletDelTemp.Valid,
			}
		}

		tsxDetails = append(tsxDetails, tsx)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	query = fmt.Sprintf(`SELECT COUNT(*) FROM transactions WHERE %s`, whereClause)
	err = r.db.QueryRowContext(ctx, query, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return tsxDetails, total, nil

}
