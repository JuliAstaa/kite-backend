package wishlist

import (
	"backend/internal/platform/database"
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

type WishlistRepositorer interface {
	CreateItem(ctx context.Context, param CreateItemParams) (Item, error)
	GetAllItems(ctx context.Context, filter ListFilter) ([]Item, int, error)
	GetItemByID(ctx context.Context, id string) (Item, error)
	PatchItem(ctx context.Context, id string, param PatchItemParams) (Item, error)
	DeleteItem(ctx context.Context, id string) (Item, error)
	RestoreItem(ctx context.Context, id string) (Item, error)
	Allocate(ctx context.Context, id string, amount int) (Item, error)
	Purchase(ctx context.Context, id string, param PurchaseParams) (Item, error)
	Summary(ctx context.Context) (SummaryTotals, error)
}

type WishlistRepository struct {
	db *sql.DB
}

func NewWishlistRepository(db *sql.DB) *WishlistRepository {
	return &WishlistRepository{db: db}
}

const itemColumns = `id, name, estimated_price, priority, target_date, product_url, note,
	status, saved_amount, purchased_at, purchase_transaction_id, sort_order,
	created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (Item, error) {
	var i Item
	err := row.Scan(&i.ID, &i.Name, &i.EstimatedPrice, &i.Priority, &i.TargetDate,
		&i.ProductURL, &i.Note, &i.Status, &i.SavedAmount, &i.PurchasedAt,
		&i.PurchaseTransactionID, &i.SortOrder, &i.CreatedAt, &i.UpdatedAt, &i.DeletedAt)
	return i, err
}

func (r *WishlistRepository) CreateItem(ctx context.Context, param CreateItemParams) (Item, error) {
	query := fmt.Sprintf(`INSERT INTO wishlist_items(name, estimated_price, priority, target_date, product_url, note, status)
		VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING %s`, itemColumns)

	item, err := scanItem(r.db.QueryRowContext(ctx, query,
		param.Name, param.EstimatedPrice, param.Priority, param.TargetDate,
		param.ProductURL, param.Note, param.Status))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Item{}, apperror.AlreadyExistsErr{Resource: "wishlist_items", Name: param.Name, Type: "wishlist"}
		}
		return Item{}, err
	}
	return item, nil
}

func (r *WishlistRepository) GetAllItems(ctx context.Context, filter ListFilter) ([]Item, int, error) {
	conditions := []string{}
	args := []any{}

	if !filter.IncludeDeleted {
		conditions = append(conditions, "deleted_at IS NULL")
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}

	where := "TRUE"
	if len(conditions) > 0 {
		where = strings.Join(conditions, " AND ")
	}

	countArgs := append([]any{}, args...)
	args = append(args, filter.Limit, filter.Offset)

	query := fmt.Sprintf(`SELECT %s FROM wishlist_items WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		itemColumns, where, orderClause(filter.Sort), len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM wishlist_items WHERE %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// orderClause memetakan sort dari query param ke kolom yang dikenal.
// Nilai yang tidak dikenal jatuh ke urutan default, tidak pernah dipakai mentah.
func orderClause(sort string) string {
	field, dir, _ := strings.Cut(sort, ":")

	direction := "ASC"
	if strings.EqualFold(dir, "desc") {
		direction = "DESC"
	}

	switch field {
	case "priority":
		// low < medium < high, jadi priority:desc menaruh yang high di atas
		return fmt.Sprintf(`CASE priority WHEN 'low' THEN 1 WHEN 'medium' THEN 2 WHEN 'high' THEN 3 ELSE 0 END %s, created_at DESC`, direction)
	case "estimated_price":
		return fmt.Sprintf("estimated_price %s, created_at DESC", direction)
	case "target_date":
		return fmt.Sprintf("target_date %s NULLS LAST, created_at DESC", direction)
	case "name":
		return fmt.Sprintf("lower(name) %s", direction)
	case "created_at":
		return fmt.Sprintf("created_at %s", direction)
	default:
		return "sort_order ASC, created_at DESC"
	}
}

func (r *WishlistRepository) GetItemByID(ctx context.Context, id string) (Item, error) {
	query := fmt.Sprintf(`SELECT %s FROM wishlist_items WHERE id = $1 AND deleted_at IS NULL`, itemColumns)
	item, err := scanItem(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, apperror.NotFoundError{Resource: "wishlist_items", ID: id}
	}
	return item, err
}

func (r *WishlistRepository) PatchItem(ctx context.Context, id string, param PatchItemParams) (Item, error) {
	query := fmt.Sprintf(`UPDATE wishlist_items SET
			name = COALESCE($1, name),
			estimated_price = COALESCE($2, estimated_price),
			priority = COALESCE($3, priority),
			target_date = CASE WHEN $4 THEN NULL ELSE COALESCE($5, target_date) END,
			product_url = CASE WHEN $6 THEN NULL ELSE COALESCE($7, product_url) END,
			note = COALESCE($8, note),
			status = COALESCE($9, status),
			sort_order = COALESCE($10, sort_order),
			updated_at = now()
		WHERE id = $11 AND deleted_at IS NULL
		RETURNING %s`, itemColumns)

	item, err := scanItem(r.db.QueryRowContext(ctx, query,
		param.Name, param.EstimatedPrice, param.Priority,
		param.ClearTargetDate, param.TargetDate,
		param.ClearProductURL, param.ProductURL,
		param.Note, param.Status, param.SortOrder, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, apperror.NotFoundError{Resource: "wishlist_items", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Item{}, apperror.AlreadyExistsErr{Resource: "wishlist_items", Name: "", Type: "wishlist"}
		}
		return Item{}, err
	}
	return item, nil
}

func (r *WishlistRepository) DeleteItem(ctx context.Context, id string) (Item, error) {
	query := fmt.Sprintf(`UPDATE wishlist_items SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, itemColumns)

	item, err := scanItem(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, apperror.NotFoundError{Resource: "wishlist_items", ID: id}
	}
	return item, err
}

func (r *WishlistRepository) RestoreItem(ctx context.Context, id string) (Item, error) {
	query := fmt.Sprintf(`UPDATE wishlist_items SET deleted_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL RETURNING %s`, itemColumns)

	item, err := scanItem(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, apperror.NotFoundError{Resource: "wishlist_items", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Item{}, apperror.AlreadyExistsErr{Resource: "wishlist_items", Name: "", Type: "wishlist"}
		}
		return Item{}, err
	}
	return item, nil
}

// Allocate menambah saved_amount. Batas saved_amount <= estimated_price
// ditegakkan sekali lagi di sini lewat WHERE, supaya dua request yang datang
// bersamaan tidak bisa membuat total melebihi harga.
func (r *WishlistRepository) Allocate(ctx context.Context, id string, amount int) (Item, error) {
	query := fmt.Sprintf(`UPDATE wishlist_items SET
			saved_amount = saved_amount + $1,
			status = CASE WHEN status = 'planned' THEN 'saving' ELSE status END,
			updated_at = now()
		WHERE id = $2 AND deleted_at IS NULL AND saved_amount + $1 <= estimated_price
		RETURNING %s`, itemColumns)

	item, err := scanItem(r.db.QueryRowContext(ctx, query, amount, id))
	if errors.Is(err, sql.ErrNoRows) {
		// Bedakan "tidak ada" dari "melebihi harga", supaya pesan errornya jelas.
		if _, findErr := r.GetItemByID(ctx, id); findErr != nil {
			return Item{}, findErr
		}
		return Item{}, apperror.ValidationError{
			Field:   "amount",
			Message: "total saved_amount tidak boleh melebihi estimated_price",
		}
	}
	return item, err
}

// Purchase menandai item sudah dibeli sekaligus membuat transaksi expense-nya.
//
// Dua tulisan ini harus jadi satu kesatuan: kalau update wishlist gagal,
// transaksinya ikut batal, supaya tidak ada pengeluaran nyangkut tanpa item.
func (r *WishlistRepository) Purchase(ctx context.Context, id string, param PurchaseParams) (Item, error) {
	var item Item

	err := database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		var transactionID string
		err := tx.QueryRowContext(ctx,
			`INSERT INTO transactions(type, amount, wallet_id, category_id, note, occurred_at, wishlist_item_id)
			VALUES('expense', $1, $2, $3, $4, $5, $6)
			RETURNING id`,
			param.ActualPrice, param.WalletID, param.CategoryID, param.Note, param.OccurredAt, id).Scan(&transactionID)
		if err != nil {
			return err
		}

		query := fmt.Sprintf(`UPDATE wishlist_items SET
				status = 'purchased',
				purchased_at = $1,
				purchase_transaction_id = $2,
				updated_at = now()
			WHERE id = $3 AND deleted_at IS NULL
			RETURNING %s`, itemColumns)

		item, err = scanItem(tx.QueryRowContext(ctx, query, param.OccurredAt, transactionID, id))
		if errors.Is(err, sql.ErrNoRows) {
			return apperror.NotFoundError{Resource: "wishlist_items", ID: id}
		}
		return err
	})

	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func (r *WishlistRepository) Summary(ctx context.Context) (SummaryTotals, error) {
	var s SummaryTotals
	var high, medium, low int

	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
			COALESCE(SUM(estimated_price), 0),
			COALESCE(SUM(saved_amount), 0),
			COUNT(*) FILTER (WHERE priority = 'high'),
			COUNT(*) FILTER (WHERE priority = 'medium'),
			COUNT(*) FILTER (WHERE priority = 'low')
		FROM wishlist_items
		WHERE deleted_at IS NULL AND status <> 'cancelled'`).
		Scan(&s.TotalItems, &s.TotalEstimated, &s.TotalSaved, &high, &medium, &low)
	if err != nil {
		return SummaryTotals{}, err
	}

	s.ByPriority = map[string]int{"high": high, "medium": medium, "low": low}
	return s, nil
}
