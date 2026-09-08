package backup

import (
	"backend/internal/platform/database"
	"context"
	"database/sql"
	"strings"
	"time"
)

type BackupRepositorer interface {
	ExportTransactions(ctx context.Context, from, to time.Time) ([]ExportTransaction, error)
	ExportWallets(ctx context.Context) ([]ExportWallet, error)
	ExportCategories(ctx context.Context) ([]ExportCategory, error)
	WalletsByName(ctx context.Context) (map[string]string, error)
	CategoriesByNameAndType(ctx context.Context) (map[string]string, error)
	ImportTransactions(ctx context.Context, rows []ImportRow) (int, error)
}

type BackupRepository struct {
	db *sql.DB
}

func NewBackupRepository(db *sql.DB) *BackupRepository {
	return &BackupRepository{db: db}
}

func (r *BackupRepository) ExportTransactions(ctx context.Context, from, to time.Time) ([]ExportTransaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.id, t.type, t.amount, w.name, COALESCE(tw.name, ''), COALESCE(c.name, ''), t.note, t.occurred_at
		FROM transactions t
		JOIN wallets w ON w.id = t.wallet_id
		LEFT JOIN wallets tw ON tw.id = t.to_wallet_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.deleted_at IS NULL AND t.occurred_at >= $1 AND t.occurred_at <= $2
		ORDER BY t.occurred_at ASC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ExportTransaction{}
	for rows.Next() {
		var t ExportTransaction
		if err := rows.Scan(&t.ID, &t.Type, &t.Amount, &t.Wallet, &t.ToWallet, &t.Category, &t.Note, &t.OccurredAt); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (r *BackupRepository) ExportWallets(ctx context.Context) ([]ExportWallet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type, initial_balance, is_excluded_from_total
		FROM wallets WHERE deleted_at IS NULL ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ExportWallet{}
	for rows.Next() {
		var w ExportWallet
		if err := rows.Scan(&w.ID, &w.Name, &w.Type, &w.InitialBalance, &w.IsExcludedFromTotal); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

func (r *BackupRepository) ExportCategories(ctx context.Context) ([]ExportCategory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, type FROM categories WHERE deleted_at IS NULL
		ORDER BY type ASC, sort_order ASC, name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []ExportCategory{}
	for rows.Next() {
		var c ExportCategory
		if err := rows.Scan(&c.ID, &c.Name, &c.Type); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// WalletsByName memetakan nama wallet (huruf kecil) ke id, dipakai import CSV
// yang isinya nama, bukan UUID.
func (r *BackupRepository) WalletsByName(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, lower(name) FROM wallets WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lookup := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		lookup[name] = id
	}
	return lookup, rows.Err()
}

// CategoriesByNameAndType memakai kunci "nama|tipe", karena nama kategori boleh
// sama antara income dan expense (misalnya "Lainnya").
func (r *BackupRepository) CategoriesByNameAndType(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, lower(name), type FROM categories WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lookup := map[string]string{}
	for rows.Next() {
		var id, name, catType string
		if err := rows.Scan(&id, &name, &catType); err != nil {
			return nil, err
		}
		lookup[name+"|"+strings.ToLower(catType)] = id
	}
	return lookup, rows.Err()
}

// ImportTransactions menulis semua baris dalam satu database transaction.
// Kalau satu baris gagal, tidak ada satupun yang masuk.
func (r *BackupRepository) ImportTransactions(ctx context.Context, rows []ImportRow) (int, error) {
	imported := 0

	err := database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		for _, row := range rows {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO transactions(type, amount, wallet_id, category_id, note, occurred_at)
				VALUES($1, $2, $3, $4, $5, $6)`,
				row.Type, row.Amount, row.WalletID, row.CategoryID, row.Note, row.OccurredAt)
			if err != nil {
				return err
			}
			imported++
		}
		return nil
	})

	if err != nil {
		return 0, err
	}
	return imported, nil
}
