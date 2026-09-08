package budget

import (
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type BudgetRepositorer interface {
	CreateBudget(ctx context.Context, param CreateBudgetParams) (Budget, error)
	GetAllBudgets(ctx context.Context, includeDeleted bool) ([]Budget, int, error)
	GetBudgetByID(ctx context.Context, id string) (Budget, error)
	PatchBudget(ctx context.Context, id string, param PatchBudgetParams) (Budget, error)
	DeleteBudget(ctx context.Context, id string) (Budget, error)
	ActiveBudgets(ctx context.Context, monthStart time.Time) ([]Budget, error)
	SpentOnCategory(ctx context.Context, categoryID string, from, to time.Time) (int, error)
}

type BudgetRepository struct {
	db *sql.DB
}

func NewBudgetRepository(db *sql.DB) *BudgetRepository {
	return &BudgetRepository{db: db}
}

// Nama kategori ikut diambil supaya frontend tidak perlu request kedua.
// Kategori yang sudah di-soft-delete tetap ikut, riwayat tidak boleh hilang.
const budgetColumns = `b.id, b.category_id, c.name, c.type, b.amount, b.period,
	b.start_month, b.end_month, b.created_at, b.updated_at, b.deleted_at`

const budgetJoins = `FROM budgets b JOIN categories c ON c.id = b.category_id`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBudget(row rowScanner) (Budget, error) {
	var b Budget
	err := row.Scan(&b.ID, &b.CategoryID, &b.CategoryName, &b.CategoryType, &b.Amount,
		&b.Period, &b.StartMonth, &b.EndMonth, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt)
	return b, err
}

func (r *BudgetRepository) budgetByID(ctx context.Context, id string, includeDeleted bool) (Budget, error) {
	where := "b.id = $1 AND b.deleted_at IS NULL"
	if includeDeleted {
		where = "b.id = $1"
	}

	query := fmt.Sprintf(`SELECT %s %s WHERE %s`, budgetColumns, budgetJoins, where)
	budget, err := scanBudget(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Budget{}, apperror.NotFoundError{Resource: "budgets", ID: id}
	}
	return budget, err
}

func (r *BudgetRepository) CreateBudget(ctx context.Context, param CreateBudgetParams) (Budget, error) {
	var id string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO budgets(category_id, amount, period, start_month, end_month)
		VALUES($1, $2, $3, $4, $5) RETURNING id`,
		param.CategoryID, param.Amount, param.Period, param.StartMonth, param.EndMonth).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Budget{}, apperror.ConflictError{Message: "kategori ini sudah punya budget yang masih berjalan"}
		}
		return Budget{}, err
	}

	return r.budgetByID(ctx, id, false)
}

func (r *BudgetRepository) GetAllBudgets(ctx context.Context, includeDeleted bool) ([]Budget, int, error) {
	where := "b.deleted_at IS NULL"
	if includeDeleted {
		where = "TRUE"
	}

	query := fmt.Sprintf(`SELECT %s %s WHERE %s ORDER BY c.name ASC, b.start_month DESC`,
		budgetColumns, budgetJoins, where)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	budgets := []Budget{}
	for rows.Next() {
		b, err := scanBudget(rows)
		if err != nil {
			return nil, 0, err
		}
		budgets = append(budgets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return budgets, len(budgets), nil
}

func (r *BudgetRepository) GetBudgetByID(ctx context.Context, id string) (Budget, error) {
	return r.budgetByID(ctx, id, false)
}

func (r *BudgetRepository) PatchBudget(ctx context.Context, id string, param PatchBudgetParams) (Budget, error) {
	var updatedID string
	err := r.db.QueryRowContext(ctx, `UPDATE budgets SET
			amount = COALESCE($1, amount),
			period = COALESCE($2, period),
			start_month = COALESCE($3, start_month),
			end_month = CASE WHEN $4 THEN NULL ELSE COALESCE($5, end_month) END,
			updated_at = now()
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING id`,
		param.Amount, param.Period, param.StartMonth, param.ClearEndMonth, param.EndMonth, id).Scan(&updatedID)

	if errors.Is(err, sql.ErrNoRows) {
		return Budget{}, apperror.NotFoundError{Resource: "budgets", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Budget{}, apperror.ConflictError{Message: "kategori ini sudah punya budget yang masih berjalan"}
		}
		return Budget{}, err
	}

	return r.budgetByID(ctx, updatedID, false)
}

func (r *BudgetRepository) DeleteBudget(ctx context.Context, id string) (Budget, error) {
	var deletedID string
	err := r.db.QueryRowContext(ctx,
		`UPDATE budgets SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING id`, id).Scan(&deletedID)

	if errors.Is(err, sql.ErrNoRows) {
		return Budget{}, apperror.NotFoundError{Resource: "budgets", ID: id}
	}
	if err != nil {
		return Budget{}, err
	}

	return r.budgetByID(ctx, deletedID, true)
}

// ActiveBudgets mengambil budget yang berlaku pada bulan tersebut.
func (r *BudgetRepository) ActiveBudgets(ctx context.Context, monthStart time.Time) ([]Budget, error) {
	query := fmt.Sprintf(`SELECT %s %s
		WHERE b.deleted_at IS NULL
			AND b.start_month <= $1
			AND (b.end_month IS NULL OR b.end_month >= $1)
		ORDER BY c.name ASC`, budgetColumns, budgetJoins)

	rows, err := r.db.QueryContext(ctx, query, monthStart)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	budgets := []Budget{}
	for rows.Next() {
		b, err := scanBudget(rows)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, b)
	}
	return budgets, rows.Err()
}

// SpentOnCategory menjumlahkan pengeluaran satu kategori dalam satu rentang.
func (r *BudgetRepository) SpentOnCategory(ctx context.Context, categoryID string, from, to time.Time) (int, error) {
	var spent int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0) FROM transactions
		WHERE deleted_at IS NULL AND type = 'expense' AND category_id = $1
			AND occurred_at >= $2 AND occurred_at <= $3`,
		categoryID, from, to).Scan(&spent)
	return spent, err
}
