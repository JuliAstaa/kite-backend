package quickadd

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type QuickAddRepositorer interface {
	CreateQuickAdd(ctx context.Context, param CreateQuickAddParams) (QuickAdd, error)
	GetAllQuickAdds(ctx context.Context, includeDeleted bool) ([]QuickAdd, int, error)
	GetQuickAddByID(ctx context.Context, id string) (QuickAdd, error)
	PatchQuickAdd(ctx context.Context, id string, param PatchQuickAddParams) (QuickAdd, error)
	DeleteQuickAdd(ctx context.Context, id string) (QuickAdd, error)
}

type QuickAddRepository struct {
	db *sql.DB
}

func NewQuickAddRepository(db *sql.DB) *QuickAddRepository {
	return &QuickAddRepository{db: db}
}

const quickAddColumns = `id, label, type, amount, wallet_id, category_id, note, sort_order,
	created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanQuickAdd(row rowScanner) (QuickAdd, error) {
	var q QuickAdd
	err := row.Scan(&q.ID, &q.Label, &q.Type, &q.Amount, &q.WalletID, &q.CategoryID,
		&q.Note, &q.SortOrder, &q.CreatedAt, &q.UpdatedAt, &q.DeletedAt)
	return q, err
}

func (r *QuickAddRepository) CreateQuickAdd(ctx context.Context, param CreateQuickAddParams) (QuickAdd, error) {
	query := fmt.Sprintf(`INSERT INTO quick_adds(label, type, amount, wallet_id, category_id, note)
		VALUES($1, $2, $3, $4, $5, $6) RETURNING %s`, quickAddColumns)

	quickAdd, err := scanQuickAdd(r.db.QueryRowContext(ctx, query,
		param.Label, param.Type, param.Amount, param.WalletID, param.CategoryID, param.Note))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return QuickAdd{}, apperror.AlreadyExistsErr{Resource: "quick_adds", Name: param.Label, Type: param.Type}
		}
		return QuickAdd{}, err
	}
	return quickAdd, nil
}

func (r *QuickAddRepository) GetAllQuickAdds(ctx context.Context, includeDeleted bool) ([]QuickAdd, int, error) {
	where := "deleted_at IS NULL"
	if includeDeleted {
		where = "TRUE"
	}

	query := fmt.Sprintf(`SELECT %s FROM quick_adds WHERE %s ORDER BY sort_order ASC, created_at ASC`, quickAddColumns, where)
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	quickAdds := []QuickAdd{}
	for rows.Next() {
		q, err := scanQuickAdd(rows)
		if err != nil {
			return nil, 0, err
		}
		quickAdds = append(quickAdds, q)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return quickAdds, len(quickAdds), nil
}

func (r *QuickAddRepository) GetQuickAddByID(ctx context.Context, id string) (QuickAdd, error) {
	query := fmt.Sprintf(`SELECT %s FROM quick_adds WHERE id = $1 AND deleted_at IS NULL`, quickAddColumns)
	quickAdd, err := scanQuickAdd(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return QuickAdd{}, apperror.NotFoundError{Resource: "quick_adds", ID: id}
	}
	return quickAdd, err
}

func (r *QuickAddRepository) PatchQuickAdd(ctx context.Context, id string, param PatchQuickAddParams) (QuickAdd, error) {
	query := fmt.Sprintf(`UPDATE quick_adds SET
			label = COALESCE($1, label),
			type = COALESCE($2, type),
			amount = CASE WHEN $3 THEN NULL ELSE COALESCE($4, amount) END,
			wallet_id = COALESCE($5, wallet_id),
			category_id = COALESCE($6, category_id),
			note = COALESCE($7, note),
			sort_order = COALESCE($8, sort_order),
			updated_at = now()
		WHERE id = $9 AND deleted_at IS NULL
		RETURNING %s`, quickAddColumns)

	quickAdd, err := scanQuickAdd(r.db.QueryRowContext(ctx, query,
		param.Label, param.Type, param.ClearAmount, param.Amount,
		param.WalletID, param.CategoryID, param.Note, param.SortOrder, id))

	if errors.Is(err, sql.ErrNoRows) {
		return QuickAdd{}, apperror.NotFoundError{Resource: "quick_adds", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return QuickAdd{}, apperror.AlreadyExistsErr{Resource: "quick_adds", Name: "", Type: ""}
		}
		return QuickAdd{}, err
	}
	return quickAdd, nil
}

func (r *QuickAddRepository) DeleteQuickAdd(ctx context.Context, id string) (QuickAdd, error) {
	query := fmt.Sprintf(`UPDATE quick_adds SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, quickAddColumns)

	quickAdd, err := scanQuickAdd(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return QuickAdd{}, apperror.NotFoundError{Resource: "quick_adds", ID: id}
	}
	return quickAdd, err
}
