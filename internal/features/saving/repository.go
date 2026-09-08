package saving

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type SavingRepositorer interface {
	Totals(ctx context.Context, from, to time.Time) (income int, expense int, err error)
	BucketSums(ctx context.Context, from, to time.Time, granularity string) ([]bucketSum, error)
	ActiveTarget(ctx context.Context, period string, at time.Time) (Target, bool, error)

	CreateTarget(ctx context.Context, param CreateTargetParams) (Target, error)
	GetAllTargets(ctx context.Context, includeDeleted bool) ([]Target, int, error)
	GetTargetByID(ctx context.Context, id string) (Target, error)
	PatchTarget(ctx context.Context, id string, param PatchTargetParams) (Target, error)
	DeleteTarget(ctx context.Context, id string) (Target, error)
}

type SavingRepository struct {
	db *sql.DB
}

func NewSavingRepository(db *sql.DB) *SavingRepository {
	return &SavingRepository{db: db}
}

const targetColumns = `id, period, amount, target_rate, start_date, end_date, is_active, created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTarget(row rowScanner) (Target, error) {
	var t Target
	err := row.Scan(&t.ID, &t.Period, &t.Amount, &t.TargetRate, &t.StartDate,
		&t.EndDate, &t.IsActive, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt)
	return t, err
}

// Totals menjumlahkan pemasukan dan pengeluaran satu periode. Transfer
// diabaikan karena memindahkan uang antar dompet bukan menabung.
func (r *SavingRepository) Totals(ctx context.Context, from, to time.Time) (int, int, error) {
	var income, expense int
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0),
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0)
		FROM transactions
		WHERE deleted_at IS NULL AND occurred_at >= $1 AND occurred_at <= $2`,
		from, to).Scan(&income, &expense)
	return income, expense, err
}

// BucketSums mengelompokkan per minggu atau per bulan. Konversi zona waktu
// dilakukan sebelum date_trunc, kalau tidak transaksi pagi hari WITA masuk ke
// bucket sebelumnya.
func (r *SavingRepository) BucketSums(ctx context.Context, from, to time.Time, granularity string) ([]bucketSum, error) {
	unit := "month"
	if granularity == "week" || granularity == "weekly" {
		unit = "week"
	}
	tz := timeutil.Loc().String()

	rows, err := r.db.QueryContext(ctx, `
		SELECT date_trunc($3, t.occurred_at AT TIME ZONE $4) AT TIME ZONE $4 AS bucket_start,
			COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'income'), 0),
			COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'expense'), 0)
		FROM transactions t
		WHERE t.deleted_at IS NULL
			AND t.occurred_at >= $1 AND t.occurred_at <= $2
			AND t.type <> 'transfer'
		GROUP BY bucket_start
		ORDER BY bucket_start ASC`, from, to, unit, tz)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []bucketSum{}
	for rows.Next() {
		var b bucketSum
		if err := rows.Scan(&b.Start, &b.Income, &b.Expense); err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	return result, rows.Err()
}

// ActiveTarget mencari target yang berlaku pada tanggal tertentu.
func (r *SavingRepository) ActiveTarget(ctx context.Context, period string, at time.Time) (Target, bool, error) {
	query := fmt.Sprintf(`SELECT %s FROM savings_targets
		WHERE deleted_at IS NULL AND is_active AND period = $1
			AND start_date <= $2
			AND (end_date IS NULL OR end_date >= $2)
		ORDER BY start_date DESC
		LIMIT 1`, targetColumns)

	target, err := scanTarget(r.db.QueryRowContext(ctx, query, period, at))
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, false, nil
	}
	if err != nil {
		return Target{}, false, err
	}
	return target, true, nil
}

func (r *SavingRepository) CreateTarget(ctx context.Context, param CreateTargetParams) (Target, error) {
	query := fmt.Sprintf(`INSERT INTO savings_targets(period, amount, target_rate, start_date, end_date, is_active)
		VALUES($1, $2, $3, $4, $5, $6) RETURNING %s`, targetColumns)

	target, err := scanTarget(r.db.QueryRowContext(ctx, query,
		param.Period, param.Amount, param.TargetRate, param.StartDate, param.EndDate, param.IsActive))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Target{}, apperror.ConflictError{Message: "sudah ada target aktif untuk periode " + param.Period}
		}
		return Target{}, err
	}
	return target, nil
}

func (r *SavingRepository) GetAllTargets(ctx context.Context, includeDeleted bool) ([]Target, int, error) {
	where := "deleted_at IS NULL"
	if includeDeleted {
		where = "TRUE"
	}

	query := fmt.Sprintf(`SELECT %s FROM savings_targets WHERE %s ORDER BY period ASC, start_date DESC`, targetColumns, where)
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	targets := []Target{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, 0, err
		}
		targets = append(targets, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return targets, len(targets), nil
}

func (r *SavingRepository) GetTargetByID(ctx context.Context, id string) (Target, error) {
	query := fmt.Sprintf(`SELECT %s FROM savings_targets WHERE id = $1 AND deleted_at IS NULL`, targetColumns)
	target, err := scanTarget(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, apperror.NotFoundError{Resource: "savings_targets", ID: id}
	}
	return target, err
}

// PatchTarget menulis keadaan akhir yang sudah digabung service. amount dan
// target_rate ditulis apa adanya, tidak pakai COALESCE, supaya salah satunya
// benar-benar bisa dikosongkan saat ganti jenis target.
func (r *SavingRepository) PatchTarget(ctx context.Context, id string, param PatchTargetParams) (Target, error) {
	query := fmt.Sprintf(`UPDATE savings_targets SET
			period = COALESCE($1, period),
			amount = $2,
			target_rate = $3,
			start_date = COALESCE($4, start_date),
			end_date = CASE WHEN $5 THEN NULL ELSE COALESCE($6, end_date) END,
			is_active = COALESCE($7, is_active),
			updated_at = now()
		WHERE id = $8 AND deleted_at IS NULL
		RETURNING %s`, targetColumns)

	target, err := scanTarget(r.db.QueryRowContext(ctx, query,
		param.Period, param.Amount, param.TargetRate, param.StartDate,
		param.ClearEndDate, param.EndDate, param.IsActive, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, apperror.NotFoundError{Resource: "savings_targets", ID: id}
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Target{}, apperror.ConflictError{Message: "sudah ada target aktif untuk periode itu"}
		}
		return Target{}, err
	}
	return target, nil
}

func (r *SavingRepository) DeleteTarget(ctx context.Context, id string) (Target, error) {
	query := fmt.Sprintf(`UPDATE savings_targets SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, targetColumns)

	target, err := scanTarget(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, apperror.NotFoundError{Resource: "savings_targets", ID: id}
	}
	return target, err
}
