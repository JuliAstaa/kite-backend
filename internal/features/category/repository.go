package category

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

type CategoryRepositorer interface {
	CreateCategory(ctx context.Context, name string, catType string, color string, icon string) (Category, error)
	GetAllCategories(ctx context.Context, limit int, offset int, catType string, includeDeleted bool) ([]Category, int, error)
	PatchCategory(ctx context.Context, id string, name *string, catType *string, color *string, icon *string) (Category, error)
	DeleteCategory(ctx context.Context, id string) (Category, error)
	GetCategoryByID(ctx context.Context, id string) (Category, error)
	RestoreCategory(ctx context.Context, id string) (Category, error)
}

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

const categoryColumns = `id, name, type, color, icon, is_default, sort_order, created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCategory(row rowScanner) (Category, error) {
	var c Category
	err := row.Scan(&c.ID, &c.Name, &c.Type, &c.Color, &c.Icon, &c.IsDefault,
		&c.SortOrder, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt)
	return c, err
}

func (r *CategoryRepository) CreateCategory(ctx context.Context, name string, catType string, color string, icon string) (Category, error) {
	query := fmt.Sprintf(`INSERT INTO categories(name, type, color, icon) VALUES($1, $2, $3, $4) RETURNING %s`, categoryColumns)
	category, err := scanCategory(r.db.QueryRowContext(ctx, query, name, catType, color, icon))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, apperror.AlreadyExistsErr{Resource: "categories", Name: name, Type: catType}
		}
		return Category{}, err
	}

	return category, nil
}

func (r *CategoryRepository) GetAllCategories(ctx context.Context, limit int, offset int, catType string, includeDeleted bool) ([]Category, int, error) {
	conditions := []string{}
	args := []any{}

	if !includeDeleted {
		conditions = append(conditions, "deleted_at IS NULL")
	}
	if catType != "" {
		args = append(args, catType)
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)))
	}

	where := "TRUE"
	if len(conditions) > 0 {
		where = joinAnd(conditions)
	}

	countArgs := append([]any{}, args...)
	args = append(args, limit, offset)

	query := fmt.Sprintf(`SELECT %s FROM categories WHERE %s
		ORDER BY type ASC, sort_order ASC, created_at ASC
		LIMIT $%d OFFSET $%d`, categoryColumns, where, len(args)-1, len(args))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, 0, err
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM categories WHERE %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return categories, total, nil
}

func (r *CategoryRepository) PatchCategory(ctx context.Context, id string, name *string, catType *string, color *string, icon *string) (Category, error) {
	query := fmt.Sprintf(`UPDATE categories SET
			name = COALESCE($1, name),
			type = COALESCE($2, type),
			color = COALESCE($3, color),
			icon = COALESCE($4, icon),
			updated_at = now()
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING %s`, categoryColumns)

	category, err := scanCategory(r.db.QueryRowContext(ctx, query, name, catType, color, icon, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, apperror.AlreadyExistsErr{Resource: "categories", Name: derefString(name), Type: derefString(catType)}
		}
		return Category{}, err
	}

	return category, nil
}

func (r *CategoryRepository) DeleteCategory(ctx context.Context, id string) (Category, error) {
	query := fmt.Sprintf(`UPDATE categories SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, categoryColumns)

	category, err := scanCategory(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}
	if err != nil {
		return Category{}, err
	}

	return category, nil
}

func (r *CategoryRepository) GetCategoryByID(ctx context.Context, id string) (Category, error) {
	query := fmt.Sprintf(`SELECT %s FROM categories WHERE id = $1 AND deleted_at IS NULL`, categoryColumns)

	category, err := scanCategory(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}
	if err != nil {
		return Category{}, err
	}

	return category, nil
}

func (r *CategoryRepository) RestoreCategory(ctx context.Context, id string) (Category, error) {
	query := fmt.Sprintf(`UPDATE categories SET deleted_at = NULL, updated_at = now()
		WHERE id = $1 AND deleted_at IS NOT NULL RETURNING %s`, categoryColumns)

	category, err := scanCategory(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, apperror.AlreadyExistsErr{Resource: "categories", Name: "", Type: ""}
		}
		return Category{}, err
	}

	return category, nil
}

func joinAnd(conditions []string) string {
	out := conditions[0]
	for _, c := range conditions[1:] {
		out += " AND " + c
	}
	return out
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
