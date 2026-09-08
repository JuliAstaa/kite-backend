package transaction

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/validator"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type TransactionRepositorer interface {
	CreateTransaction(ctx context.Context, param CreateTransactionParams) (TransactionDetail, error)
	GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error)
	GetTransactionByID(ctx context.Context, id string) (TransactionDetail, error)
	GetTransactionRaw(ctx context.Context, id string) (Transaction, error)
	UpdateTransaction(ctx context.Context, id string, param UpdateTransactionParams) (TransactionDetail, error)
	DeleteTransaction(ctx context.Context, id string) (TransactionDetail, error)
	RestoreTransaction(ctx context.Context, id string) (TransactionDetail, error)
}

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// detailColumns dipakai bersama oleh semua query yang mengembalikan
// TransactionDetail, supaya urutan kolom dan urutan Scan tidak pernah beda.
// Wallet dan category yang sudah di-soft-delete tetap ikut di-JOIN, karena
// riwayat transaksi lama tidak boleh kehilangan namanya.
const detailColumns = `t.id, t.type, t.amount, t.note, t.occurred_at, t.deleted_at,
	w.id, w.name, w.deleted_at,
	c.id, c.name, c.type, c.deleted_at,
	tw.id, tw.name, tw.deleted_at`

const detailJoins = `FROM transactions t
	JOIN wallets w ON w.id = t.wallet_id
	LEFT JOIN categories c ON c.id = t.category_id
	LEFT JOIN wallets tw ON tw.id = t.to_wallet_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDetail(row rowScanner) (TransactionDetail, error) {
	var d TransactionDetail
	var catID, catName, catType sql.NullString
	var toWalletID, toWalletName sql.NullString
	var catDeleted, walletDeleted, toWalletDeleted sql.NullTime

	err := row.Scan(
		&d.ID, &d.Type, &d.Amount, &d.Note, &d.OccurredAt, &d.DeletedAt,
		&d.Wallet.ID, &d.Wallet.Name, &walletDeleted,
		&catID, &catName, &catType, &catDeleted,
		&toWalletID, &toWalletName, &toWalletDeleted,
	)
	if err != nil {
		return TransactionDetail{}, err
	}

	d.Wallet.IsDeleted = walletDeleted.Valid

	if catID.Valid {
		d.Category = &CategoryRef{
			ID:        catID.String,
			Name:      catName.String,
			Type:      catType.String,
			IsDeleted: catDeleted.Valid,
		}
	}

	if toWalletID.Valid {
		d.ToWallet = &WalletRef{
			ID:        toWalletID.String,
			Name:      toWalletName.String,
			IsDeleted: toWalletDeleted.Valid,
		}
	}

	return d, nil
}

// detailByID dipakai setelah insert atau update untuk mengambil bentuk lengkap
// transaksi. includeDeleted dipakai oleh delete, karena barisnya sudah bukan
// deleted_at IS NULL lagi saat mau dikembalikan ke handler.
func (r *TransactionRepository) detailByID(ctx context.Context, id string, includeDeleted bool) (TransactionDetail, error) {
	where := "t.id = $1 AND t.deleted_at IS NULL"
	if includeDeleted {
		where = "t.id = $1"
	}

	query := fmt.Sprintf(`SELECT %s %s WHERE %s`, detailColumns, detailJoins, where)
	detail, err := scanDetail(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return TransactionDetail{}, apperror.NotFoundError{Resource: "transactions", ID: id}
	}
	return detail, err
}

func (r *TransactionRepository) CreateTransaction(ctx context.Context, param CreateTransactionParams) (TransactionDetail, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO transactions(type, amount, wallet_id, to_wallet_id, category_id, note, occurred_at, recurring_rule_id, wishlist_item_id)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		param.Type, param.Amount, param.WalletID, param.ToWalletID, param.CategoryID,
		param.Note, param.OccurredAt, param.RecurringRuleID, param.WishlistItemID).Scan(&id)
	if err != nil {
		return TransactionDetail{}, err
	}

	return r.detailByID(ctx, id, false)
}

func (r *TransactionRepository) GetTransactionByID(ctx context.Context, id string) (TransactionDetail, error) {
	return r.detailByID(ctx, id, false)
}

// GetTransactionRaw mengambil baris apa adanya. Dipakai service saat patch,
// supaya field yang tidak dikirim client bisa digabung dengan data lama
// sebelum aturan bisnis dicek ulang.
func (r *TransactionRepository) GetTransactionRaw(ctx context.Context, id string) (Transaction, error) {
	var t Transaction
	err := r.db.QueryRowContext(ctx,
		`SELECT id, type, amount, wallet_id, to_wallet_id, category_id, note, occurred_at,
			recurring_rule_id, wishlist_item_id, created_at, updated_at, deleted_at
		FROM transactions WHERE id = $1 AND deleted_at IS NULL`, id).Scan(
		&t.ID, &t.Type, &t.Amount, &t.WalletID, &t.ToWalletID, &t.CategoryID,
		&t.Note, &t.OccurredAt, &t.RecurringRuleID, &t.WishlistItemID,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Transaction{}, apperror.NotFoundError{Resource: "transactions", ID: id}
	}
	return t, err
}

func (r *TransactionRepository) UpdateTransaction(ctx context.Context, id string, param UpdateTransactionParams) (TransactionDetail, error) {
	var updatedID string
	err := r.db.QueryRowContext(ctx,
		`UPDATE transactions SET
			type = $1,
			amount = $2,
			wallet_id = $3,
			to_wallet_id = $4,
			category_id = $5,
			note = $6,
			occurred_at = $7,
			updated_at = now()
		WHERE id = $8 AND deleted_at IS NULL
		RETURNING id`,
		param.Type, param.Amount, param.WalletID, param.ToWalletID, param.CategoryID,
		param.Note, param.OccurredAt, id).Scan(&updatedID)

	if errors.Is(err, sql.ErrNoRows) {
		return TransactionDetail{}, apperror.NotFoundError{Resource: "transactions", ID: id}
	}
	if err != nil {
		return TransactionDetail{}, err
	}

	return r.detailByID(ctx, updatedID, false)
}

func (r *TransactionRepository) DeleteTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	var deletedID string
	err := r.db.QueryRowContext(ctx,
		`UPDATE transactions SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING id`, id).Scan(&deletedID)

	if errors.Is(err, sql.ErrNoRows) {
		return TransactionDetail{}, apperror.NotFoundError{Resource: "transactions", ID: id}
	}
	if err != nil {
		return TransactionDetail{}, err
	}

	return r.detailByID(ctx, deletedID, true)
}

func (r *TransactionRepository) RestoreTransaction(ctx context.Context, id string) (TransactionDetail, error) {
	var restoredID string
	err := r.db.QueryRowContext(ctx,
		`UPDATE transactions SET deleted_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL RETURNING id`, id).Scan(&restoredID)

	if errors.Is(err, sql.ErrNoRows) {
		return TransactionDetail{}, apperror.NotFoundError{Resource: "transactions", ID: id}
	}
	if err != nil {
		return TransactionDetail{}, err
	}

	return r.detailByID(ctx, restoredID, false)
}

func (r *TransactionRepository) GetAllTransactions(ctx context.Context, filter TransactionFilter) ([]TransactionDetail, int, error) {
	conditions := []string{"t.deleted_at IS NULL"}
	args := []any{}

	addCondition := func(format string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}

	if !filter.From.IsZero() {
		addCondition("t.occurred_at >= $%d", filter.From)
	}
	if !filter.To.IsZero() {
		addCondition("t.occurred_at <= $%d", filter.To)
	}
	if !validator.IsEmptyString(filter.Type) {
		addCondition("t.type = $%d", filter.Type)
	}
	if filter.MinAmount > 0 {
		addCondition("t.amount >= $%d", filter.MinAmount)
	}
	if filter.MaxAmount > 0 {
		addCondition("t.amount <= $%d", filter.MaxAmount)
	}
	if !validator.IsEmptyString(filter.Query) {
		addCondition("t.note ILIKE $%d", "%"+filter.Query+"%")
	}

	// category_id dan wallet_id boleh berisi beberapa id (CSV), jadi
	// placeholder-nya dibuat sebanyak isinya.
	if len(filter.CategoryIDs) > 0 {
		placeholders := make([]string, 0, len(filter.CategoryIDs))
		for _, id := range filter.CategoryIDs {
			args = append(args, id)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		conditions = append(conditions, fmt.Sprintf("t.category_id IN (%s)", strings.Join(placeholders, ", ")))
	}

	if len(filter.WalletIDs) > 0 {
		placeholders := make([]string, 0, len(filter.WalletIDs))
		for _, id := range filter.WalletIDs {
			args = append(args, id)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		// transfer masuk juga dianggap milik wallet tujuan
		conditions = append(conditions, fmt.Sprintf("(t.wallet_id IN (%s) OR t.to_wallet_id IN (%s))",
			strings.Join(placeholders, ", "), strings.Join(placeholders, ", ")))
	}

	whereClause := strings.Join(conditions, " AND ")

	// Kolom sort tidak pernah diambil mentah dari query param, cuma dipetakan
	// dari daftar yang sudah dikenal. Kalau tidak dikenal, jatuh ke occurred_at.
	orderColumn := "t.occurred_at"
	orderDir := "DESC"
	if !validator.IsEmptyString(filter.Sort) {
		field, dir, _ := strings.Cut(filter.Sort, ":")
		switch field {
		case "amount":
			orderColumn = "t.amount"
		case "created_at":
			orderColumn = "t.created_at"
		case "occurred_at":
			orderColumn = "t.occurred_at"
		}
		if strings.EqualFold(dir, "asc") {
			orderDir = "ASC"
		}
	}

	countArgs := append([]any{}, args...)

	args = append(args, filter.Limit)
	limitPos := len(args)
	args = append(args, filter.Offset)
	offsetPos := len(args)

	query := fmt.Sprintf(`SELECT %s %s
		WHERE %s
		ORDER BY %s %s, t.id DESC
		LIMIT $%d OFFSET $%d`,
		detailColumns, detailJoins, whereClause, orderColumn, orderDir, limitPos, offsetPos)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	details := []TransactionDetail{}
	for rows.Next() {
		detail, err := scanDetail(rows)
		if err != nil {
			return nil, 0, err
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// COUNT memakai JOIN yang sama karena whereClause bisa menyebut alias w/c/tw.
	countQuery := fmt.Sprintf(`SELECT COUNT(*) %s WHERE %s`, detailJoins, whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return details, total, nil
}
