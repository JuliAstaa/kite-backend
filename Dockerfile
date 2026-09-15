# syntax=docker/dockerfile:1

# ---- build ----
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Layer dependensi dipisah supaya cache tidak hangus tiap kali kode berubah.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO mati supaya binary statis dan bisa jalan di image runtime yang tipis.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/finance-api ./cmd/api

# ---- runtime ----
FROM alpine:3.21

# tzdata wajib: timeutil.Init memanggil time.LoadLocation("Asia/Makassar"),
# tanpa paket ini aplikasi gagal start di container.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1001 kite

WORKDIR /app
COPY --from=builder /out/finance-api /app/finance-api

USER kite
EXPOSE 8080

# APP_HOST default-nya 127.0.0.1 (lihat internal/platform/config). Di dalam
# container itu berarti tidak bisa dihubungi dari luar, jadi dipaksa 0.0.0.0.
ENV APP_HOST=0.0.0.0 \
    PORT=8080

ENTRYPOINT ["/app/finance-api"]
