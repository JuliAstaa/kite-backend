// Package httpx berisi helper kecil untuk membaca request, dipakai bersama
// oleh banyak feature supaya parsing tanggal tidak ditulis ulang di tiap handler.
package httpx

import (
	"backend/internal/shared/apperror"
	"backend/internal/shared/queryparam"
	"backend/internal/shared/timeutil"
	"net/http"
	"time"
)

// DateRange membaca query param from dan to. Dua-duanya inklusif: `to`
// dinaikkan ke akhir hari supaya transaksi sore hari ikut terhitung.
func DateRange(r *http.Request, defaultFrom, defaultTo time.Time) (time.Time, time.Time, error) {
	from := defaultFrom
	to := defaultTo

	if raw := r.URL.Query().Get("from"); raw != "" {
		parsed, ok := queryparam.ToDate(raw)
		if !ok {
			return time.Time{}, time.Time{}, apperror.ValidationError{Field: "from", Message: "from harus format YYYY-MM-DD"}
		}
		from = timeutil.StartOfDay(parsed)
	}

	if raw := r.URL.Query().Get("to"); raw != "" {
		parsed, ok := queryparam.ToDate(raw)
		if !ok {
			return time.Time{}, time.Time{}, apperror.ValidationError{Field: "to", Message: "to harus format YYYY-MM-DD"}
		}
		to = timeutil.EndOfDay(parsed)
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, apperror.ValidationError{Field: "to", Message: "to tidak boleh lebih awal dari from"}
	}

	return from, to, nil
}
