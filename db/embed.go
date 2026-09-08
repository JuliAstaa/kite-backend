// Package db menyimpan file migration dan membungkusnya jadi embed.FS supaya
// binary bisa menjalankan migration sendiri tanpa butuh file di disk.
package db

import "embed"

//go:embed migrations/*.sql
var MigrationFS embed.FS

// MigrationDir adalah nama folder di dalam MigrationFS.
const MigrationDir = "migrations"
