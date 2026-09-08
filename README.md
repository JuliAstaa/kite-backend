# Kite Finance API

REST API pencatat keuangan pribadi single-user. Go 1.22+ dengan `net/http` stdlib,
PostgreSQL, arsitektur feature-first. Tanpa framework web, tanpa ORM.

## Menjalankan

```bash
cp .env.example .env      # sesuaikan DB_URL
make db-up                # Postgres lewat Docker Compose
make run                  # migration jalan otomatis saat startup
curl localhost:8080/api/v1/health
```

## Perintah

```bash
make help     # daftar semua perintah
make test     # semua test
make vet      # go vet
make backup   # pg_dump ke backups/
```

Tiap feature punya test di ketiga layernya: `service_test.go` (aturan bisnis,
pakai repository palsu), `repository_test.go` (SQL sungguhan ke Postgres), dan
`handler_test.go` (parsing request dan status code, pakai service palsu).

Test repository butuh `DB_URL_TEST`. Kalau kosong atau databasenya tidak bisa
dihubungi, test itu di-skip dan test service serta handler tetap jalan. Pesan
skip-nya muncul di output, jadi kelihatan bedanya antara "lulus" dan
"tidak dijalankan".

Test repository berbagi satu database dan saling `TRUNCATE` tabel, jadi tidak
boleh jalan barengan. Yang menjaganya adalah Postgres advisory lock yang
dipegang tiap package selama berjalan, bukan kedisiplinan memakai flag, jadi
`go test ./...` biasa pun tetap benar. `make test` tetap memakai `-p 1` karena
lebih cepat: package tidak perlu antre di kunci.

## Struktur

```
cmd/api/main.go            wiring semua feature, start server
db/migrations/             file SQL, dijalankan otomatis saat startup
internal/features/<nama>/  model, dto, repository, service, handler, routes
internal/platform/         config, database, middleware, router, wiring
internal/shared/           apperror, response, validator, queryparam, timeutil, httpx
```

Tiap feature berdiri sendiri dalam satu folder berisi seluruh layernya.

### Aturan antar feature

**Feature tidak boleh import package feature lain.** Kalau feature A butuh data
dari feature B, deklarasikan interface kecil **di dalam package A**, lalu
`internal/platform/wiring` yang menyambungkannya ke service B.

Contohnya `wishlist` butuh tahu kemampuan menabung:

```go
// internal/features/wishlist/service.go
type SavingsReader interface {
    AverageMonthlySavable(ctx context.Context, months int) (avg int, sampleMonths int, err error)
}
```

`wishlist` tidak pernah menyebut nama package `saving`. Bonus: test wishlist
cukup pakai struct palsu yang memenuhi interface itu, tanpa perlu database.

Untuk bacaan agregat (analytics, savings, budget status, saldo wallet),
repository feature tersebut query tabel `transactions` langsung lewat SQL.
Tidak ada import antar package, dan tidak perlu adapter untuk sekadar
menjumlahkan angka.

## Endpoint

Base path `/api/v1`.

| Endpoint | Keterangan |
|---|---|
| `GET /health` | status aplikasi dan database |
| `GET/POST /wallets`, `GET/PATCH/DELETE /wallets/{id}`, `POST /wallets/{id}/restore` | dompet, saldo dihitung dari transaksi |
| `GET/POST /categories`, `GET/PATCH/DELETE /categories/{id}`, `POST /categories/{id}/restore` | kategori |
| `GET/POST /transactions`, `GET/PATCH/DELETE /transactions/{id}`, `POST /transactions/{id}/restore` | transaksi, termasuk transfer |
| `GET /summary` | angka dashboard |
| `GET /analytics/by-category`, `/analytics/by-wallet`, `/analytics/trend` | breakdown dan tren |
| `GET /savings/summary`, `/savings/breakdown` | berapa yang bisa ditabung |
| `GET/POST /savings/targets`, `PATCH/DELETE /savings/targets/{id}` | target menabung |
| `GET/POST /wishlist`, `GET/PATCH/DELETE /wishlist/{id}` | wishlist |
| `POST /wishlist/{id}/restore`, `/allocate`, `/purchase` | alokasi dan pembelian |
| `GET /wishlist/summary` | ringkasan wishlist |
| `GET/POST /budgets`, `PATCH/DELETE /budgets/{id}`, `GET /budgets/status` | budget |
| `GET/POST /recurring`, `GET/PATCH/DELETE /recurring/{id}` | transaksi berulang |
| `POST /recurring/{id}/toggle`, `POST /recurring/run` | aktifkan dan trigger manual |
| `GET/POST /quick-adds`, `PATCH/DELETE /quick-adds/{id}`, `POST /quick-adds/{id}/execute` | pintasan transaksi |
| `GET /export?format=json\|csv`, `POST /import?dry_run=true` | backup dan restore data |

### Bentuk response

```json
{ "data": { "id": "..." } }
{ "data": [ ], "meta": { "total": 248, "limit": 50, "offset": 0 } }
{ "error": { "code": "VALIDATION_ERROR", "message": "...", "details": { "field": "amount" } } }
```

Kode error: `VALIDATION_ERROR` (400), `NOT_FOUND` (404), `CONFLICT` (409),
`UNPROCESSABLE` (422), `INTERNAL_ERROR` (500).

## Keputusan yang perlu diingat

**Uang disimpan `BIGINT` rupiah penuh.** Tidak ada `NUMERIC`, tidak ada `float`.

**Saldo wallet tidak disimpan sebagai kolom,** dihitung dari `initial_balance`
plus agregasi transaksi setiap kali dibaca. Kolom saldo yang di-update manual
adalah sumber bug nomor satu di aplikasi keuangan.

**Soft delete di semua tabel.** Konsekuensinya sudah ditangani sejak awal:
unique constraint pakai partial index `WHERE deleted_at IS NULL`, setiap SELECT
memfilter `deleted_at IS NULL` di query dasar repository, dan FK tidak pakai
`ON DELETE CASCADE` supaya riwayat transaksi tidak ikut hilang.

**Semua bucket waktu dikonversi ke `Asia/Makassar` sebelum `date_trunc`.** Tanpa
itu, transaksi jam 7 pagi WITA masuk ke hari sebelumnya karena Postgres
menyimpan dalam UTC.

**Scheduler transaksi berulang idempoten** berkat unique index
`(recurring_rule_id, occurred_at)` dan `ON CONFLICT DO NOTHING`. Satu goroutine
dengan `time.Ticker` satu jam, tanpa worker pool.

**Aturan bisnis ditegakkan di layer service,** bukan di handler dan bukan di
database. Handler cuma memeriksa bentuk request.

## Keamanan

`APP_HOST` default `127.0.0.1`, jadi API hanya bisa diakses dari mesin sendiri.
Kalau mau diakses dari HP di WiFi rumah, isi `API_TOKEN` di `.env` lalu kirim
header `X-API-Token`. Middleware-nya sudah ada dan jadi no-op saat token kosong.
