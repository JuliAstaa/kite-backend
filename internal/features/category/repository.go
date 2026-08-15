package category

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type CategoryRepositorer interface {
	CreateCategory(ctx context.Context, name string, catType string, color string, icon string) (Category, error)
	GetAllCategories(ctx context.Context, limit int, offset int) ([]Category, int, error)
	PatchCategory(ctx context.Context, id string, name *string, catType *string, color *string, icon *string) (Category, error)
	DeleteCategory(ctx context.Context, id string) (Category, error)
}

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) CreateCategory(ctx context.Context, name string, catType string, color string, icon string) (Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `INSERT INTO categories(name, type, color, icon) VALUES($1, $2, $3, $4) RETURNING id, name, type, color, icon, is_default, sort_order, created_at, updated_at, deleted_at`, name, catType, color, icon).Scan(&category.ID, &category.Name, &category.Type, &category.Color, &category.Icon, &category.IsDefault, &category.SortOrder, &category.CreatedAt, &category.UpdatedAt, &category.DeletedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, apperror.ErrCategoryAlreadyExists
		}
		return Category{}, err
	}

	return category, nil
}

func (r *CategoryRepository) GetAllCategories(ctx context.Context, limit int, offset int) ([]Category, int, error) {

	rows, err := r.db.QueryContext(ctx, `SELECT * FROM categories WHERE deleted_at IS NULL LIMIT $1 OFFSET $2`, limit, offset)

	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	categories := []Category{}

	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Color, &c.Icon, &c.IsDefault, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt, &c.DeletedAt); err != nil {
			return nil, 0, err
		}
		categories = append(categories, c)
	}

	var total int
	err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE deleted_at IS NULL`).Scan(&total)

	return categories, total, rows.Err()
}

func (r *CategoryRepository) PatchCategory(ctx context.Context, id string, name *string, catType *string, color *string, icon *string) (Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `UPDATE categories SET  name = COALESCE($1, name),
														type = COALESCE($2, type),
														color = COALESCE($3, color),
														icon = COALESCE($4, icon),
														updated_at = now()
														WHERE id = $5 AND deleted_at IS NULL
														RETURNING id, name, type, color, icon, is_default, sort_order, created_at, updated_at, deleted_at`, name, catType, color, icon, id).Scan(
		&category.ID,
		&category.Name,
		&category.Type,
		&category.Color,
		&category.Icon,
		&category.IsDefault,
		&category.SortOrder,
		&category.CreatedAt,
		&category.UpdatedAt,
		&category.DeletedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return Category{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Category{}, apperror.ErrCategoryAlreadyExists
		}
		return Category{}, err
	}

	return category, nil
}

func (r *CategoryRepository) DeleteCategory(ctx context.Context, id string) (Category, error) {
	var category Category
	err := r.db.QueryRowContext(ctx, `UPDATE categories SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL RETURNING id, name, type, color, icon, is_default, sort_order, created_at, updated_at, deleted_at`, id).Scan(
		&category.ID,
		&category.Name,
		&category.Type,
		&category.Color,
		&category.Icon,
		&category.IsDefault,
		&category.SortOrder,
		&category.CreatedAt,
		&category.UpdatedAt,
		&category.DeletedAt,
	)

	if err != nil {
		return Category{}, err
	}

	if errors.As(err, sql.ErrNoRows) {
		return Category{}, apperror.NotFoundError{Resource: "categories", ID: id}
	}

	return category, nil
}
