package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Migration runner sederhana, tanpa library.
//
// Aturan penamaan file sama dengan golang-migrate: <versi>_<nama>.up.sql.
// File .down.sql tetap ditulis untuk dijalankan manual lewat `make migrate-down`,
// runner ini hanya peduli pada .up.sql.
//
// Versi yang sudah jalan dicatat di tabel schema_migrations_applied. Kalau di
// database sudah ada tabel schema_migrations bawaan golang-migrate, versinya
// dipakai untuk backfill sekali supaya migration lama tidak dijalankan ulang.

type migrationFile struct {
	version int64
	name    string
	path    string
}

// RunMigrations menjalankan semua file .up.sql yang belum pernah dijalankan.
func RunMigrations(ctx context.Context, db *sql.DB, fsys fs.FS, dir string) error {
	files, err := collectMigrations(fsys, dir)
	if err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations_applied (
		version    BIGINT PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("bikin tabel schema_migrations_applied: %w", err)
	}

	if err := backfillFromGolangMigrate(ctx, db, files); err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	ran := 0
	for _, f := range files {
		if applied[f.version] {
			continue
		}

		body, err := fs.ReadFile(fsys, f.path)
		if err != nil {
			return fmt.Errorf("baca %s: %w", f.path, err)
		}

		if err := runMigration(ctx, db, f, string(body)); err != nil {
			return err
		}

		log.Printf("migration %d_%s dijalankan", f.version, f.name)
		ran++
	}

	if ran == 0 {
		log.Println("migration: tidak ada yang baru")
	}
	return nil
}

// runMigration menjalankan satu file dalam satu database transaction, supaya
// file yang gagal di tengah tidak meninggalkan skema setengah jadi.
func runMigration(ctx context.Context, db *sql.DB, f migrationFile, body string) error {
	return WithTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, body); err != nil {
			return fmt.Errorf("migration %d_%s gagal: %w", f.version, f.name, err)
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations_applied (version, name) VALUES ($1, $2)`,
			f.version, f.name)
		return err
	})
}

func collectMigrations(fsys fs.FS, dir string) ([]migrationFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("baca folder migration: %w", err)
	}

	files := []migrationFile{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		trimmed := strings.TrimSuffix(name, ".up.sql")
		versionPart, namePart, found := strings.Cut(trimmed, "_")
		if !found {
			return nil, fmt.Errorf("nama file migration tidak sesuai format: %s", name)
		}

		version, err := strconv.ParseInt(versionPart, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("versi migration tidak valid pada %s: %w", name, err)
		}

		files = append(files, migrationFile{version: version, name: namePart, path: path.Join(dir, name)})
	}

	sort.Slice(files, func(i, j int) bool { return files[i].version < files[j].version })
	return files, nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[int64]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations_applied`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[int64]bool{}
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// backfillFromGolangMigrate menandai migration lama sebagai sudah jalan kalau
// database ini sebelumnya dikelola golang-migrate CLI. Cuma berlaku sekali,
// yaitu saat schema_migrations_applied masih kosong.
func backfillFromGolangMigrate(ctx context.Context, db *sql.DB, files []migrationFile) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations_applied`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}

	var version int64
	var dirty bool
	err := db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		// tabel bernama sama tapi bentuknya beda, abaikan saja
		return nil
	}
	if dirty {
		return fmt.Errorf("schema_migrations golang-migrate dalam keadaan dirty pada versi %d, benerin manual dulu", version)
	}

	for _, f := range files {
		if f.version > version {
			continue
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations_applied (version, name) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			f.version, f.name); err != nil {
			return err
		}
		log.Printf("migration %d_%s ditandai sudah jalan (backfill dari golang-migrate)", f.version, f.name)
	}
	return nil
}
