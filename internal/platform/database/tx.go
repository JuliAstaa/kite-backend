package database

import (
	"context"
	"database/sql"
)

// WithTx membungkus fn dalam satu database transaction.
//
// tx.Rollback setelah Commit itu aman, cuma jadi no-op. Idiom ini dipakai supaya
// jalur error tidak pernah lupa rollback.
func WithTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit()
}
