package wallet

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type WalletRepositorer interface {
	CreateWallet(ctx context.Context, name string, walletType string, initialBalancae *int, color string, icon string, IsExcludedFromTotal *bool) (Wallet, error)
}

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) CreateWallet(ctx context.Context, name string, walletType string, initialBalancae *int, color string, icon string, IsExcludedFromTotal *bool) (Wallet, error) {
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
