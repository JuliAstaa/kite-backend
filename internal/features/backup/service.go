package backup

import (
	"backend/internal/shared/timeutil"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

type BackupServicer interface {
	Export(ctx context.Context, from, to time.Time) (ExportBundle, error)
	Import(ctx context.Context, reader io.Reader, dryRun bool) (ImportResult, error)
}

type BackupService struct {
	repo BackupRepositorer
}

func NewBackupService(repo BackupRepositorer) *BackupService {
	return &BackupService{repo: repo}
}

func (s *BackupService) Export(ctx context.Context, from, to time.Time) (ExportBundle, error) {
	wallets, err := s.repo.ExportWallets(ctx)
	if err != nil {
		return ExportBundle{}, err
	}

	categories, err := s.repo.ExportCategories(ctx)
	if err != nil {
		return ExportBundle{}, err
	}

	transactions, err := s.repo.ExportTransactions(ctx, from, to)
	if err != nil {
		return ExportBundle{}, err
	}

	return ExportBundle{
		ExportedAt:   timeutil.Now(),
		From:         timeutil.FormatDate(from),
		To:           timeutil.FormatDate(to),
		Wallets:      wallets,
		Categories:   categories,
		Transactions: transactions,
	}, nil
}

// Import membaca CSV berkolom date,type,amount,wallet,category,note.
//
// Seluruh baris divalidasi dulu sampai habis, baru ditulis. Dengan begitu
// dry_run dan import sungguhan memakai jalur pengecekan yang sama persis, dan
// laporan errornya lengkap, bukan berhenti di baris pertama yang salah.
func (s *BackupService) Import(ctx context.Context, reader io.Reader, dryRun bool) (ImportResult, error) {
	walletsByName, err := s.repo.WalletsByName(ctx)
	if err != nil {
		return ImportResult{}, err
	}

	categoriesByKey, err := s.repo.CategoriesByNameAndType(ctx)
	if err != nil {
		return ImportResult{}, err
	}

	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.TrimLeadingSpace = true

	result := ImportResult{DryRun: dryRun, Errors: []RowError{}}
	valid := []ImportRow{}

	header, err := csvReader.Read()
	if err == io.EOF {
		return result, nil
	}
	if err != nil {
		return ImportResult{}, fmt.Errorf("gagal membaca header CSV: %w", err)
	}

	columns, err := headerIndex(header)
	if err != nil {
		return ImportResult{}, err
	}

	line := 1
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		line++

		if err != nil {
			result.TotalRows++
			result.InvalidRows++
			result.Errors = append(result.Errors, RowError{Line: line, Message: "baris tidak bisa dibaca: " + err.Error()})
			continue
		}

		if isBlankRecord(record) {
			continue
		}

		result.TotalRows++

		row, rowErr := parseRow(line, record, columns, walletsByName, categoriesByKey)
		if rowErr != nil {
			result.InvalidRows++
			result.Errors = append(result.Errors, RowError{Line: line, Message: rowErr.Error()})
			continue
		}

		result.ValidRows++
		valid = append(valid, row)
	}

	if dryRun {
		return result, nil
	}

	// Import sungguhan menolak file yang masih ada barisnya salah, supaya tidak
	// ada data setengah masuk yang harus dibersihkan manual.
	if result.InvalidRows > 0 {
		return result, nil
	}

	imported, err := s.repo.ImportTransactions(ctx, valid)
	if err != nil {
		return ImportResult{}, err
	}
	result.Imported = imported

	return result, nil
}

func headerIndex(header []string) (map[string]int, error) {
	columns := map[string]int{}
	for i, name := range header {
		// File CSV dari Excel sering diawali BOM, buang dulu biar nama
		// kolom pertama tetap terbaca.
		columns[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))] = i
	}

	for _, required := range []string{"date", "type", "amount", "wallet"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("kolom %q tidak ada di header CSV", required)
		}
	}

	return columns, nil
}

func parseRow(line int, record []string, columns map[string]int, wallets, categories map[string]string) (ImportRow, error) {
	get := func(name string) string {
		index, ok := columns[name]
		if !ok || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}

	occurredAt, err := timeutil.ParseDate(get("date"))
	if err != nil {
		return ImportRow{}, fmt.Errorf("date harus format YYYY-MM-DD")
	}

	rowType := strings.ToLower(get("type"))
	if rowType != "income" && rowType != "expense" {
		// Transfer butuh dua wallet, formatnya tidak muat di CSV ini.
		return ImportRow{}, fmt.Errorf("type harus income atau expense (transfer tidak didukung lewat CSV)")
	}

	amount, err := strconv.Atoi(strings.ReplaceAll(get("amount"), ".", ""))
	if err != nil {
		return ImportRow{}, fmt.Errorf("amount harus angka")
	}
	if amount <= 0 {
		return ImportRow{}, fmt.Errorf("amount harus lebih besar dari 0")
	}

	walletName := strings.ToLower(get("wallet"))
	walletID, ok := wallets[walletName]
	if !ok {
		return ImportRow{}, fmt.Errorf("wallet %q tidak ditemukan", get("wallet"))
	}

	categoryName := strings.ToLower(get("category"))
	if categoryName == "" {
		return ImportRow{}, fmt.Errorf("category wajib diisi untuk income dan expense")
	}
	categoryID, ok := categories[categoryName+"|"+rowType]
	if !ok {
		return ImportRow{}, fmt.Errorf("kategori %q bertipe %s tidak ditemukan", get("category"), rowType)
	}

	return ImportRow{
		Line:       line,
		Type:       rowType,
		Amount:     amount,
		WalletID:   walletID,
		CategoryID: categoryID,
		Note:       get("note"),
		OccurredAt: occurredAt,
	}, nil
}

func isBlankRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}
