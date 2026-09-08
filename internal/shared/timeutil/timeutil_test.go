package timeutil

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if err := Init("Asia/Makassar"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func at(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, Loc())
}

// date_trunc('week') Postgres selalu mulai Senin, jadi StartOfWeek harus sama.
func TestStartOfWeekSelaluSenin(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{"Senin tetap di tempat", at(2026, time.August, 10), at(2026, time.August, 10)},
		{"Rabu mundur ke Senin", at(2026, time.August, 12), at(2026, time.August, 10)},
		{"Minggu mundur ke Senin minggu itu", at(2026, time.August, 16), at(2026, time.August, 10)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StartOfWeek(tt.in)
			if !got.Equal(tt.want) {
				t.Errorf("dapat %s, mau %s", got.Format(DateLayout), tt.want.Format(DateLayout))
			}
			if got.Weekday() != time.Monday {
				t.Errorf("hari %s, mau Senin", got.Weekday())
			}
		})
	}
}

func TestLastDayOfMonth(t *testing.T) {
	tests := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2026, time.January, 31},
		{2026, time.February, 28},
		{2028, time.February, 29}, // kabisat
		{2026, time.April, 30},
		{2026, time.December, 31},
	}

	for _, tt := range tests {
		if got := LastDayOfMonth(tt.year, tt.month); got != tt.want {
			t.Errorf("LastDayOfMonth(%d, %s) = %d, mau %d", tt.year, tt.month, got, tt.want)
		}
	}
}

func TestClampDayOfMonth(t *testing.T) {
	tests := []struct {
		name  string
		year  int
		month time.Month
		day   int
		want  time.Time
	}{
		{"tanggal 31 di Februari jadi 28", 2026, time.February, 31, at(2026, time.February, 28)},
		{"tanggal 31 di Februari kabisat jadi 29", 2028, time.February, 31, at(2028, time.February, 29)},
		{"tanggal 15 tetap 15", 2026, time.February, 15, at(2026, time.February, 15)},
		{"tanggal 31 di Maret tetap 31", 2026, time.March, 31, at(2026, time.March, 31)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampDayOfMonth(tt.year, tt.month, tt.day)
			if !got.Equal(tt.want) {
				t.Errorf("dapat %s, mau %s", got.Format(DateLayout), tt.want.Format(DateLayout))
			}
		})
	}
}

func TestEndOfDayInklusif(t *testing.T) {
	day := at(2026, time.August, 11)
	end := EndOfDay(day)

	sore := time.Date(2026, time.August, 11, 23, 59, 59, 0, Loc())
	if end.Before(sore) {
		t.Errorf("EndOfDay %s harus mencakup transaksi jam 23:59", end)
	}

	besok := at(2026, time.August, 12)
	if !end.Before(besok) {
		t.Errorf("EndOfDay tidak boleh masuk ke hari berikutnya")
	}
}

func TestBuckets(t *testing.T) {
	t.Run("bulanan", func(t *testing.T) {
		buckets := Buckets(at(2026, time.June, 1), at(2026, time.August, 31), "month")
		if len(buckets) != 3 {
			t.Fatalf("dapat %d bucket, mau 3", len(buckets))
		}
		if buckets[0].Label != "2026-06" {
			t.Errorf("label pertama %q, mau 2026-06", buckets[0].Label)
		}
	})

	t.Run("mingguan mulai Senin", func(t *testing.T) {
		buckets := Buckets(at(2026, time.August, 12), at(2026, time.August, 25), "week")
		if len(buckets) != 3 {
			t.Fatalf("dapat %d bucket, mau 3", len(buckets))
		}
		for _, b := range buckets {
			if b.Start.Weekday() != time.Monday {
				t.Errorf("bucket %s mulai hari %s, mau Senin", b.Label, b.Start.Weekday())
			}
		}
	})

	t.Run("rentang terbalik memberi kosong", func(t *testing.T) {
		buckets := Buckets(at(2026, time.August, 31), at(2026, time.August, 1), "month")
		if len(buckets) != 0 {
			t.Errorf("dapat %d bucket, mau 0", len(buckets))
		}
	})
}

func TestDaysBetweenInklusif(t *testing.T) {
	got := DaysBetween(at(2026, time.August, 1), at(2026, time.August, 31))
	if got != 31 {
		t.Errorf("DaysBetween = %d, mau 31", got)
	}

	if got := DaysBetween(at(2026, time.August, 11), at(2026, time.August, 11)); got != 1 {
		t.Errorf("hari yang sama = %d, mau 1", got)
	}
}

// Tanggal diparse sebagai jam 00:00 waktu lokal, bukan UTC. Kalau salah,
// transaksi pagi hari WITA bisa jatuh ke hari sebelumnya.
func TestParseDateMemakaiZonaWaktuAplikasi(t *testing.T) {
	parsed, err := ParseDate("2026-08-11")
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}

	if parsed.Hour() != 0 || parsed.Day() != 11 {
		t.Errorf("dapat %s, mau 2026-08-11 00:00 WITA", parsed)
	}

	_, offset := parsed.Zone()
	if offset != 8*3600 {
		t.Errorf("offset %d detik, mau 28800 (WITA, UTC+8)", offset)
	}
}
