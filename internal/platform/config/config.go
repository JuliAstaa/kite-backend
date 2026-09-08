package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Version dipakai di endpoint /health.
const Version = "2.0.0"

type Config struct {
	AppEnv string
	Host   string
	Port   string
	TZ     string

	DBUrl       string
	DBMaxConns  int
	DBIdleConns int

	CORSAllowedOrigins []string

	// APIToken kosong berarti middleware token jadi no-op. Diisi kalau API mau
	// diakses dari perangkat lain di jaringan lokal.
	APIToken string
}

func Load() Config {
	godotenv.Load()

	dbUrl := os.Getenv("DB_URL")
	if dbUrl == "" {
		log.Fatal("DB_URL belum diisi")
	}

	return Config{
		AppEnv:             envOr("APP_ENV", "development"),
		Host:               envOr("APP_HOST", "127.0.0.1"),
		Port:               envOr("PORT", envOr("APP_PORT", "8080")),
		TZ:                 envOr("APP_TZ", "Asia/Makassar"),
		DBUrl:              dbUrl,
		DBMaxConns:         envIntOr("DB_MAX_CONNS", 10),
		DBIdleConns:        envIntOr("DB_IDLE_CONNS", 5),
		CORSAllowedOrigins: splitCSV(envOr("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		APIToken:           os.Getenv("API_TOKEN"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("%s bukan angka (%q), pakai default %d", key, v, fallback)
		return fallback
	}
	return parsed
}

func splitCSV(v string) []string {
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
