package recurring

import (
	"backend/internal/platform/database"
	"backend/internal/shared/apperror"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type RecurringRepositorer interface {
	CreateRule(ctx context.Context, param CreateRuleParams) (Rule, error)
	GetAllRules(ctx context.Context, includeDeleted bool) ([]Rule, int, error)
	GetRuleByID(ctx context.Context, id string) (Rule, error)
	PatchRule(ctx context.Context, id string, param PatchRuleParams) (Rule, error)
	DeleteRule(ctx context.Context, id string) (Rule, error)
	ToggleRule(ctx context.Context, id string) (Rule, error)
	DueRules(ctx context.Context, today time.Time) ([]Rule, error)
	GenerateOccurrences(ctx context.Context, rule Rule, dates []time.Time, nextRunAt time.Time) (created int, skipped int, err error)
}

type RecurringRepository struct {
	db *sql.DB
}

func NewRecurringRepository(db *sql.DB) *RecurringRepository {
	return &RecurringRepository{db: db}
}

const ruleColumns = `id, name, type, amount, wallet_id, category_id, note, frequency,
	interval, day_of_month, day_of_week, start_date, end_date, next_run_at, last_run_at,
	is_active, created_at, updated_at, deleted_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRule(row rowScanner) (Rule, error) {
	var r Rule
	err := row.Scan(&r.ID, &r.Name, &r.Type, &r.Amount, &r.WalletID, &r.CategoryID,
		&r.Note, &r.Frequency, &r.Interval, &r.DayOfMonth, &r.DayOfWeek,
		&r.StartDate, &r.EndDate, &r.NextRunAt, &r.LastRunAt, &r.IsActive,
		&r.CreatedAt, &r.UpdatedAt, &r.DeletedAt)
	return r, err
}

func (r *RecurringRepository) CreateRule(ctx context.Context, param CreateRuleParams) (Rule, error) {
	query := fmt.Sprintf(`INSERT INTO recurring_rules
		(name, type, amount, wallet_id, category_id, note, frequency, interval,
		 day_of_month, day_of_week, start_date, end_date, next_run_at, is_active)
		VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING %s`, ruleColumns)

	return scanRule(r.db.QueryRowContext(ctx, query,
		param.Name, param.Type, param.Amount, param.WalletID, param.CategoryID,
		param.Note, param.Frequency, param.Interval, param.DayOfMonth, param.DayOfWeek,
		param.StartDate, param.EndDate, param.NextRunAt, param.IsActive))
}

func (r *RecurringRepository) GetAllRules(ctx context.Context, includeDeleted bool) ([]Rule, int, error) {
	where := "deleted_at IS NULL"
	if includeDeleted {
		where = "TRUE"
	}

	query := fmt.Sprintf(`SELECT %s FROM recurring_rules WHERE %s ORDER BY next_run_at ASC, name ASC`, ruleColumns, where)
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	rules := []Rule{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, 0, err
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return rules, len(rules), nil
}

func (r *RecurringRepository) GetRuleByID(ctx context.Context, id string) (Rule, error) {
	query := fmt.Sprintf(`SELECT %s FROM recurring_rules WHERE id = $1 AND deleted_at IS NULL`, ruleColumns)
	rule, err := scanRule(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, apperror.NotFoundError{Resource: "recurring_rules", ID: id}
	}
	return rule, err
}

func (r *RecurringRepository) PatchRule(ctx context.Context, id string, param PatchRuleParams) (Rule, error) {
	query := fmt.Sprintf(`UPDATE recurring_rules SET
			name = COALESCE($1, name),
			amount = COALESCE($2, amount),
			wallet_id = COALESCE($3, wallet_id),
			category_id = COALESCE($4, category_id),
			note = COALESCE($5, note),
			frequency = COALESCE($6, frequency),
			interval = COALESCE($7, interval),
			day_of_month = COALESCE($8, day_of_month),
			day_of_week = COALESCE($9, day_of_week),
			start_date = COALESCE($10, start_date),
			end_date = CASE WHEN $11 THEN NULL ELSE COALESCE($12, end_date) END,
			next_run_at = COALESCE($13, next_run_at),
			is_active = COALESCE($14, is_active),
			updated_at = now()
		WHERE id = $15 AND deleted_at IS NULL
		RETURNING %s`, ruleColumns)

	rule, err := scanRule(r.db.QueryRowContext(ctx, query,
		param.Name, param.Amount, param.WalletID, param.CategoryID, param.Note,
		param.Frequency, param.Interval, param.DayOfMonth, param.DayOfWeek,
		param.StartDate, param.ClearEndDate, param.EndDate, param.NextRunAt,
		param.IsActive, id))

	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, apperror.NotFoundError{Resource: "recurring_rules", ID: id}
	}
	return rule, err
}

// DeleteRule cuma menandai rule-nya terhapus. Transaksi yang sudah terlanjur
// digenerate tetap ada (business rule 9).
func (r *RecurringRepository) DeleteRule(ctx context.Context, id string) (Rule, error) {
	query := fmt.Sprintf(`UPDATE recurring_rules SET deleted_at = now(), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, ruleColumns)

	rule, err := scanRule(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, apperror.NotFoundError{Resource: "recurring_rules", ID: id}
	}
	return rule, err
}

func (r *RecurringRepository) ToggleRule(ctx context.Context, id string) (Rule, error) {
	query := fmt.Sprintf(`UPDATE recurring_rules SET is_active = NOT is_active, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL RETURNING %s`, ruleColumns)

	rule, err := scanRule(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, apperror.NotFoundError{Resource: "recurring_rules", ID: id}
	}
	return rule, err
}

// DueRules mengambil rule yang sudah waktunya jalan.
func (r *RecurringRepository) DueRules(ctx context.Context, today time.Time) ([]Rule, error) {
	query := fmt.Sprintf(`SELECT %s FROM recurring_rules
		WHERE deleted_at IS NULL
			AND is_active
			AND next_run_at <= $1
			AND (end_date IS NULL OR end_date >= $1)
		ORDER BY next_run_at ASC`, ruleColumns)

	rows, err := r.db.QueryContext(ctx, query, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []Rule{}
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

// GenerateOccurrences menulis transaksi untuk tanggal-tanggal yang lewat,
// sekaligus memajukan next_run_at dan last_run_at, semuanya dalam satu
// database transaction.
//
// ON CONFLICT DO NOTHING mengandalkan unique index
// (recurring_rule_id, occurred_at). Itulah yang membuat scheduler ini idempoten:
// dijalankan dua kali untuk rentang yang sama tidak menghasilkan transaksi ganda.
func (r *RecurringRepository) GenerateOccurrences(ctx context.Context, rule Rule, dates []time.Time, nextRunAt time.Time) (int, int, error) {
	created := 0
	skipped := 0

	err := database.WithTx(ctx, r.db, func(tx *sql.Tx) error {
		for _, date := range dates {
			var insertedID string
			err := tx.QueryRowContext(ctx,
				`INSERT INTO transactions(type, amount, wallet_id, category_id, note, occurred_at, recurring_rule_id)
				VALUES($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT DO NOTHING
				RETURNING id`,
				rule.Type, rule.Amount, rule.WalletID, rule.CategoryID, rule.Note, date, rule.ID).Scan(&insertedID)

			if errors.Is(err, sql.ErrNoRows) {
				// sudah pernah dibuat, bukan error
				skipped++
				continue
			}
			if err != nil {
				return err
			}
			created++
		}

		var lastRunAt any
		if len(dates) > 0 {
			lastRunAt = dates[len(dates)-1]
		}

		_, err := tx.ExecContext(ctx,
			`UPDATE recurring_rules SET next_run_at = $1, last_run_at = COALESCE($2, last_run_at), updated_at = now()
			WHERE id = $3`, nextRunAt, lastRunAt, rule.ID)
		return err
	})

	if err != nil {
		return 0, 0, err
	}
	return created, skipped, nil
}
