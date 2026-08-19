package wallet

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type WalletRepositorer interface {
	CreateWallet(ctx context.Context, name string, walletType string, initialBalancae int, color string, icon string, IsExcludedFromTotal bool) (Wallet, error)
	GetAllWallets(ctx context.Context, limit int, offset int) ([]Wallet, int, error)
}

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) CreateWallet(ctx context.Context, name string, walletType string, initialBalancae int, color string, icon string, IsExcludedFromTotal bool) (Wallet, error) {
	var wallet Wallet
	err := r.db.QueryRowContext(ctx, `INSERT INTO wallets(name, type, initial_balance, color, icon, is_excluded_from_total) VALUES($1, $2, $3, $4, $5, $6) RETURNING id, name, type, initial_balance, color, icon, is_excluded_from_total, sort_order, created_at, updated_at, deleted_at`, name, walletType, initialBalancae, color, icon, IsExcludedFromTotal).Scan(&wallet.ID, &wallet.Name, &wallet.Type, &wallet.InitialBalance, &wallet.Color, &wallet.Icon, &wallet.IsExcludedFromTotal, &wallet.SortOrder, &wallet.CreatedAt, &wallet.UpdatedAt, &wallet.DeletedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Wallet{}, apperror.AlreadyExistsErr{Resource: "wallet", Name: name, Type: walletType}
		}
		return Wallet{}, err
	}
	return wallet, nil
}

func (r *WalletRepository) GetAllWallets(ctx context.Context, limit int, offset int) ([]Wallet, int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT * FROM wallets WHERE deleted_at IS NULL LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	wallets := []Wallet{}
	for rows.Next() {
		var wallet Wallet
		if err := rows.Scan(&wallet.ID, &wallet.Name, &wallet.Type, &wallet.InitialBalance, &wallet.Color, &wallet.Icon, &wallet.IsExcludedFromTotal, &wallet.SortOrder, &wallet.CreatedAt, &wallet.UpdatedAt, &wallet.DeletedAt); err != nil {
			return nil, 0, err
		}
		wallets = append(wallets, wallet)
	}

	var total int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wallets WHERE deleted_at IS NULL`).Scan(&total)

	return wallets, total, nil
}
