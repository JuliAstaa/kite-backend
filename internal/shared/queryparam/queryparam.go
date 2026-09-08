package queryparam

import (
	"backend/internal/shared/timeutil"
	"strconv"
	"strings"
	"time"
)

func ToInt(v string) (int, bool) {
	if v == "" {
		return 0, false
	}

	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, false
	}

	return parsed, true
}

// ToBool membaca "true"/"false"/"1"/"0".
func ToBool(v string) (bool, bool) {
	if v == "" {
		return false, false
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, false
	}
	return parsed, true
}

// ToFloat membaca angka desimal, dipakai untuk target_rate.
func ToFloat(v string) (float64, bool) {
	if v == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

// ToDate membaca "YYYY-MM-DD" di zona waktu aplikasi.
func ToDate(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	parsed, err := timeutil.ParseDate(v)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// ToCSV memecah "a,b,c" jadi slice, membuang bagian yang kosong.
func ToCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Pagination membaca limit dan offset dengan batas atas, sesuai PRD:
// limit default 50, maksimal 200.
func Pagination(rawLimit, rawOffset string, defaultLimit, maxLimit int) (limit int, offset int, ok bool) {
	limit = defaultLimit
	offset = 0

	if parsed, found := ToInt(rawLimit); found {
		limit = parsed
	}
	if parsed, found := ToInt(rawOffset); found {
		offset = parsed
	}

	if limit < 0 || offset < 0 {
		return 0, 0, false
	}
	if limit == 0 || limit > maxLimit {
		limit = maxLimit
	}

	return limit, offset, true
}
