// Package timeutil menyimpan helper periode. Semua perhitungan tanggal di
// aplikasi ini memakai zona waktu lokal yang di-set sekali di startup
// (default Asia/Makassar), bukan UTC, supaya transaksi jam 7 pagi tidak
// dihitung masuk hari sebelumnya.
package timeutil

import (
	"fmt"
	"time"
)

const DateLayout = "2006-01-02"

var loc = time.UTC

// Init memuat zona waktu aplikasi. Dipanggil sekali dari main.
func Init(name string) error {
	if name == "" {
		name = "Asia/Makassar"
	}
	l, err := time.LoadLocation(name)
	if err != nil {
		return fmt.Errorf("zona waktu %q tidak dikenal: %w", name, err)
	}
	loc = l
	return nil
}

// Loc mengembalikan zona waktu aplikasi.
func Loc() *time.Location { return loc }

// Now adalah waktu sekarang di zona waktu aplikasi.
func Now() time.Time { return time.Now().In(loc) }

// ParseDate membaca "YYYY-MM-DD" sebagai jam 00:00 di zona waktu aplikasi.
func ParseDate(value string) (time.Time, error) {
	t, err := time.ParseInLocation(DateLayout, value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("tanggal harus format YYYY-MM-DD: %w", err)
	}
	return t, nil
}

// FormatDate menulis tanggal sebagai "YYYY-MM-DD" di zona waktu aplikasi.
func FormatDate(t time.Time) string { return t.In(loc).Format(DateLayout) }

// StartOfDay mengembalikan jam 00:00:00 hari itu.
func StartOfDay(t time.Time) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// EndOfDay mengembalikan detik terakhir hari itu. Dipakai untuk filter `to`
// yang sifatnya inklusif.
func EndOfDay(t time.Time) time.Time {
	return StartOfDay(t).AddDate(0, 0, 1).Add(-time.Nanosecond)
}

// StartOfMonth mengembalikan tanggal 1 jam 00:00.
func StartOfMonth(t time.Time) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

// EndOfMonth mengembalikan hari terakhir bulan itu jam 00:00.
func EndOfMonth(t time.Time) time.Time {
	return StartOfMonth(t).AddDate(0, 1, -1)
}

// StartOfWeek mengembalikan hari Senin minggu itu, mengikuti date_trunc('week')
// Postgres yang juga selalu mulai Senin.
func StartOfWeek(t time.Time) time.Time {
	d := StartOfDay(t)
	offset := (int(d.Weekday()) + 6) % 7 // Minggu=0 -> 6, Senin=1 -> 0
	return d.AddDate(0, 0, -offset)
}

// EndOfWeek mengembalikan hari Minggu minggu itu jam 00:00.
func EndOfWeek(t time.Time) time.Time {
	return StartOfWeek(t).AddDate(0, 0, 6)
}

// LastDayOfMonth memberi jumlah hari pada bulan tersebut. time.Date dengan
// tanggal 0 di bulan berikutnya otomatis mundur ke hari terakhir bulan ini,
// jadi Februari tidak perlu diperlakukan khusus.
func LastDayOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, loc).Day()
}

// ClampDayOfMonth membatasi tanggal ke hari terakhir bulan tersebut.
// day_of_month = 31 di bulan Februari jadi tanggal 28 atau 29.
func ClampDayOfMonth(year int, month time.Month, day int) time.Time {
	last := LastDayOfMonth(year, month)
	if day > last {
		day = last
	}
	if day < 1 {
		day = 1
	}
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

// MonthLabel menulis bucket bulanan, misal "2026-08".
func MonthLabel(t time.Time) string { return t.In(loc).Format("2006-01") }

// WeekLabel menulis bucket mingguan ISO, misal "2026-W23".
func WeekLabel(t time.Time) string {
	year, week := t.In(loc).ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// DayLabel menulis bucket harian, misal "2026-08-11".
func DayLabel(t time.Time) string { return FormatDate(t) }

// PeriodRange memberi rentang periode berjalan untuk "week" atau "month".
func PeriodRange(period string, ref time.Time) (from time.Time, to time.Time) {
	if period == "week" || period == "weekly" {
		return StartOfWeek(ref), EndOfDay(EndOfWeek(ref))
	}
	return StartOfMonth(ref), EndOfDay(EndOfMonth(ref))
}

// Bucket adalah satu potongan waktu pada grafik.
type Bucket struct {
	Label string
	Start time.Time
	End   time.Time
}

// Buckets memecah rentang jadi potongan harian, mingguan, atau bulanan.
// Potongan yang tidak punya transaksi tetap dikembalikan supaya grafik di
// frontend tidak bolong.
func Buckets(from, to time.Time, granularity string) []Bucket {
	buckets := []Bucket{}
	if to.Before(from) {
		return buckets
	}

	cursor := bucketStart(from, granularity)
	limit := StartOfDay(to)

	for !cursor.After(limit) {
		next := nextBucket(cursor, granularity)
		buckets = append(buckets, Bucket{
			Label: BucketLabel(cursor, granularity),
			Start: cursor,
			End:   next.AddDate(0, 0, -1),
		})
		cursor = next
	}

	return buckets
}

// BucketLabel menulis nama potongan sesuai granularity.
func BucketLabel(t time.Time, granularity string) string {
	switch granularity {
	case "day", "daily":
		return DayLabel(t)
	case "week", "weekly":
		return WeekLabel(t)
	default:
		return MonthLabel(t)
	}
}

func bucketStart(t time.Time, granularity string) time.Time {
	switch granularity {
	case "day", "daily":
		return StartOfDay(t)
	case "week", "weekly":
		return StartOfWeek(t)
	default:
		return StartOfMonth(t)
	}
}

func nextBucket(t time.Time, granularity string) time.Time {
	switch granularity {
	case "day", "daily":
		return t.AddDate(0, 0, 1)
	case "week", "weekly":
		return t.AddDate(0, 0, 7)
	default:
		return t.AddDate(0, 1, 0)
	}
}

// DaysBetween menghitung jumlah hari inklusif antara dua tanggal.
func DaysBetween(from, to time.Time) int {
	d := int(StartOfDay(to).Sub(StartOfDay(from)).Hours()/24) + 1
	if d < 0 {
		return 0
	}
	return d
}
