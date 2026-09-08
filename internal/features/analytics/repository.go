package analytics

import (
	"backend/internal/shared/timeutil"
	"context"
	"database/sql"
	"time"
)

type AnalyticsRepositorer interface {
	Totals(ctx context.Context, from, to time.Time) (Totals, error)
	ByCategory(ctx context.Context, from, to time.Time, txType string) ([]CategoryBreakdown, error)
	ByWallet(ctx context.Context, from, to time.Time) ([]WalletBreakdown, error)
	TrendSums(ctx context.Context, from, to time.Time, granularity string) ([]bucketSum, error)
}

type AnalyticsRepository struct {
	db *sql.DB
}

func NewAnalyticsRepository(db *sql.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

// Totals menjumlahkan pemasukan dan pengeluaran dalam satu rentang.
// FILTER dipakai supaya cukup satu kali baca tabel.
func (r *AnalyticsRepository) Totals(ctx context.Context, from, to time.Time) (Totals, error) {
	var t Totals
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(amount) FILTER (WHERE type = 'income'), 0),
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0),
			COUNT(*)
		FROM transactions
		WHERE deleted_at IS NULL AND occurred_at >= $1 AND occurred_at <= $2`,
		from, to).Scan(&t.Income, &t.Expense, &t.TransactionCount)
	return t, err
}

func (r *AnalyticsRepository) ByCategory(ctx context.Context, from, to time.Time, txType string) ([]CategoryBreakdown, error) {
	// txType kosong berarti income dan expense dua-duanya. Transfer tidak
	// pernah punya kategori, jadi otomatis tersaring lewat JOIN.
	rows, err := r.db.QueryContext(ctx, `
		SELECT c.id, c.name, c.type, c.deleted_at IS NOT NULL,
			COALESCE(SUM(t.amount), 0), COUNT(t.id)
		FROM transactions t
		JOIN categories c ON c.id = t.category_id
		WHERE t.deleted_at IS NULL
			AND t.occurred_at >= $1 AND t.occurred_at <= $2
			AND ($3 = '' OR t.type = $3)
		GROUP BY c.id, c.name, c.type, c.deleted_at
		ORDER BY SUM(t.amount) DESC`, from, to, txType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []CategoryBreakdown{}
	for rows.Next() {
		var c CategoryBreakdown
		if err := rows.Scan(&c.CategoryID, &c.CategoryName, &c.CategoryType, &c.IsDeleted, &c.Total, &c.Count); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *AnalyticsRepository) ByWallet(ctx context.Context, from, to time.Time) ([]WalletBreakdown, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT w.id, w.name, w.deleted_at IS NOT NULL,
			COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'income'), 0),
			COALESCE(SUM(t.amount) FILTER (WHERE t.type = 'expense'), 0),
			COUNT(t.id)
		FROM wallets w
		LEFT JOIN transactions t ON t.wallet_id = w.id
			AND t.deleted_at IS NULL
			AND t.occurred_at >= $1 AND t.occurred_at <= $2
		WHERE w.deleted_at IS NULL
		GROUP BY w.id, w.name, w.deleted_at
		ORDER BY w.sort_order ASC, w.name ASC`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []WalletBreakdown{}
	for rows.Next() {
		var b WalletBreakdown
		if err := rows.Scan(&b.WalletID, &b.WalletName, &b.IsDeleted, &b.Income, &b.Expense, &b.Count); err != nil {
			return nil, err
		}
		b.Net = b.Income - b.Expense
		result = append(result, b)
	}
	return result, rows.Err()
}

// TrendSums mengelompokkan transaksi per potongan waktu.
//
// occurred_at diubah dulu ke zona waktu aplikasi sebelum date_trunc. Tanpa itu,
// transaksi jam 7 pagi WITA akan dihitung masuk hari sebelumnya karena Postgres
// menyimpannya dalam UTC.
func (r *AnalyticsRepository) TrendSums(ctx context.Context, from, to time.Time, granularity string) ([]bucketSum, error) {
	unit := postgresTruncUnit(granularity)
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

func postgresTruncUnit(granularity string) string {
	switch granularity {
	case "day", "daily":
		return "day"
	case "week", "weekly":
		return "week"
	default:
		return "month"
	}
}
