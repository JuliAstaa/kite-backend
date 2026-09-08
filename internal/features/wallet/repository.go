package wallet

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type WalletRepositorer interface {
	CreateWallet(ctx context.Context, name string, walletType string, initialBalancae int, color string, icon string, IsExcludedFromTotal bool) (Wallet, error)
	GetAllWallets(ctx context.Context, limit int, offset int, includeDeleted bool) ([]Wallet, int, error)
	PatchWallet(ctx context.Context, id string, name *string, walletType *string, initialBalancae *int, color *string, icon *string, IsExcludedFromTotal *bool) (Wallet, error)
	GetWalletByID(ctx context.Context, id string) (Wallet, error)
	DeleteWallet(ctx context.Context, id string) (Wallet, error)
	RestoreWallet(ctx context.Context, id string) (Wallet, error)
	TotalBalance(ctx context.Context) (int, error)
}

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// walletColumns menghitung saldo berjalan langsung di query:
// saldo awal, ditambah semua transaksi keluar-masuk wallet ini.
// income menambah, expense dan transfer keluar mengurangi, transfer masuk
// menambah lewat subquery kedua.
const walletColumns = `w.id, w.name, w.type, w.initial_balance, w.color, w.icon,
	w.is_excluded_from_total, w.sort_order, w.created_at, w.updated_at, w.deleted_at,
	w.initial_balance
		+ COALESCE((SELECT SUM(CASE t.type WHEN 'income' THEN t.amount ELSE -t.amount END)
			FROM transactions t WHERE t.wallet_id = w.id AND t.deleted_at IS NULL), 0)
		+ COALESCE((SELECT SUM(t.amount)
			FROM transactions t WHERE t.to_wallet_id = w.id AND t.type = 'transfer' AND t.deleted_at IS NULL), 0)
		AS current_balance,
	(SELECT COUNT(*) FROM transactions t
		WHERE (t.wallet_id = w.id OR t.to_wallet_id = w.id) AND t.deleted_at IS NULL) AS transaction_count`

// walletReturning sama dengan walletColumns tapi memakai nama tabel, bukan
// alias, karena RETURNING tidak bisa memakai alias. Dengan ini insert dan
// update cukup satu perjalanan ke database, tidak perlu SELECT ulang.
const walletReturning = `id, name, type, initial_balance, color, icon,
	is_excluded_from_total, sort_order, created_at, updated_at, deleted_at,
	initial_balance
		+ COALESCE((SELECT SUM(CASE t.type WHEN 'income' THEN t.amount ELSE -t.amount END)
			FROM transactions t WHERE t.wallet_id = wallets.id AND t.deleted_at IS NULL), 0)
		+ COALESCE((SELECT SUM(t.amount)
			FROM transactions t WHERE t.to_wallet_id = wallets.id AND t.type = 'transfer' AND t.deleted_at IS NULL), 0)
		AS current_balance,
	(SELECT COUNT(*) FROM transactions t
		WHERE (t.wallet_id = wallets.id OR t.to_wallet_id = wallets.id) AND t.deleted_at IS NULL) AS transaction_count`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanWallet(row rowScanner) (Wallet, error) {
	var w Wallet
	err := row.Scan(
		&w.ID, &w.Name, &w.Type, &w.InitialBalance, &w.Color, &w.Icon,
		&w.IsExcludedFromTotal, &w.SortOrder, &w.CreatedAt, &w.UpdatedAt, &w.DeletedAt,
		&w.CurrentBalance, &w.TransactionCount,
	)
	return w, err
}

// walletByID mengambil bentuk lengkap wallet setelah insert atau update.
func (r *WalletRepository) walletByID(ctx context.Context, id string, includeDeleted bool) (Wallet, error) {
	where := "w.id = $1 AND w.deleted_at IS NULL"
	if includeDeleted {
		where = "w.id = $1"
	}

	query := fmt.Sprintf(`SELECT %s FROM wallets w WHERE %s`, walletColumns, where)
	wallet, err := scanWallet(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
	}
	return wallet, err
}

func (r *WalletRepository) CreateWallet(ctx context.Context, name string, walletType string, initialBalancae int, color string, icon string, IsExcludedFromTotal bool) (Wallet, error) {
	query := fmt.Sprintf(`INSERT INTO wallets(name, type, initial_balance, color, icon, is_excluded_from_total)
		VALUES($1, $2, $3, $4, $5, $6) RETURNING %s`, walletReturning)

	wallet, err := scanWallet(r.db.QueryRowContext(ctx, query, name, walletType, initialBalancae, color, icon, IsExcludedFromTotal))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Wallet{}, apperror.AlreadyExistsErr{Resource: "wallet", Name: name, Type: walletType}
		}
		return Wallet{}, err
	}

	return wallet, nil
}

func (r *WalletRepository) GetAllWallets(ctx context.Context, limit int, offset int, includeDeleted bool) ([]Wallet, int, error) {
	where := "w.deleted_at IS NULL"
	if includeDeleted {
		where = "TRUE"
	}

	query := fmt.Sprintf(`SELECT %s FROM wallets w WHERE %s
		ORDER BY w.sort_order ASC, w.created_at ASC
		LIMIT $1 OFFSET $2`, walletColumns, where)

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	wallets := []Wallet{}
	for rows.Next() {
		wallet, err := scanWallet(rows)
		if err != nil {
			return nil, 0, err
		}
		wallets = append(wallets, wallet)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM wallets w WHERE %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}

	return wallets, total, nil
}

func (r *WalletRepository) PatchWallet(ctx context.Context, id string, name *string, walletType *string, initialBalancae *int, color *string, icon *string, IsExcludedFromTotal *bool) (Wallet, error) {
	query := fmt.Sprintf(`UPDATE wallets SET
			name = COALESCE($1, name),
			type = COALESCE($2, type),
			initial_balance = COALESCE($3, initial_balance),
			color = COALESCE($4, color),
			icon = COALESCE($5, icon),
			is_excluded_from_total = COALESCE($6, is_excluded_from_total),
			updated_at = now()
		WHERE id = $7 AND deleted_at IS NULL
		RETURNING %s`, walletReturning)

	wallet, err := scanWallet(r.db.QueryRowContext(ctx, query, name, walletType, initialBalancae, color, icon, IsExcludedFromTotal, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
	}

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Wallet{}, apperror.AlreadyExistsErr{Resource: "wallets", Name: derefString(name), Type: derefString(walletType)}
		}
		return Wallet{}, err
	}

	return wallet, nil
}

func (r *WalletRepository) GetWalletByID(ctx context.Context, id string) (Wallet, error) {
	return r.walletByID(ctx, id, false)
}

func (r *WalletRepository) DeleteWallet(ctx context.Context, id string) (Wallet, error) {
	query := fmt.Sprintf(`UPDATE wallets SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, walletReturning)

	wallet, err := scanWallet(r.db.QueryRowContext(ctx, query, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
	}
	if err != nil {
		return Wallet{}, err
	}

	return wallet, nil
}

func (r *WalletRepository) RestoreWallet(ctx context.Context, id string) (Wallet, error) {
	query := fmt.Sprintf(`UPDATE wallets SET deleted_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL RETURNING %s`, walletReturning)

	wallet, err := scanWallet(r.db.QueryRowContext(ctx, query, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{}, apperror.NotFoundError{Resource: "wallets", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Wallet{}, apperror.AlreadyExistsErr{Resource: "wallets", Name: "", Type: ""}
		}
		return Wallet{}, err
	}

	return wallet, nil
}

// TotalBalance menjumlahkan saldo semua wallet aktif yang tidak dikecualikan
// dari total. Dipakai endpoint summary.
func (r *WalletRepository) TotalBalance(ctx context.Context) (int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			w.initial_balance
			+ COALESCE((SELECT SUM(CASE t.type WHEN 'income' THEN t.amount ELSE -t.amount END)
				FROM transactions t WHERE t.wallet_id = w.id AND t.deleted_at IS NULL), 0)
			+ COALESCE((SELECT SUM(t.amount)
				FROM transactions t WHERE t.to_wallet_id = w.id AND t.type = 'transfer' AND t.deleted_at IS NULL), 0)
		), 0)
		FROM wallets w
		WHERE w.deleted_at IS NULL AND w.is_excluded_from_total = FALSE`).Scan(&total)
	return total, err
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
