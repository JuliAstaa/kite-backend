package middleware

import (
	"backend/internal/shared/response"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"time"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

var requestCounter atomic.Uint64

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Chain merangkai middleware supaya urutannya kebaca dari luar ke dalam.
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		reqID := fmt.Sprintf("%d-%d", start.UnixNano(), requestCounter.Add(1))
		reqLogger := logger.With("request_id", reqID)

		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(rec, r)

		reqLogger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

// Recover menangkap panic supaya server tidak mati, dan membalas 500 tanpa
// membocorkan isi stack trace ke client.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error("panic",
					"path", r.URL.Path,
					"panic", fmt.Sprint(rec),
					"stack", string(debug.Stack()))
				response.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "terjadi kesalahan di server", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORS mengizinkan origin yang terdaftar di config. Origin "*" berarti bebas.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(allowedOrigins, "*")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			switch {
			case allowAll:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case origin != "" && slices.Contains(allowedOrigins, origin):
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Token")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// APIToken memeriksa header X-API-Token. Kalau token di config kosong,
// middleware ini jadi no-op. Ditulis dari awal supaya tidak perlu ditambal
// belakangan saat API mau diakses dari HP.
func APIToken(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if token == "" {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			if r.Header.Get("X-API-Token") != token {
				response.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "X-API-Token tidak valid", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
