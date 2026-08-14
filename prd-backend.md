# PRD Backend: Personal Finance Tracker API

**Versi:** 2.0
**Bahasa:** Go 1.22+
**Database:** PostgreSQL 15+
**Arsitektur:** Feature-first (vertical slice)
**Status:** Draft untuk implementasi

---

## 1. Ringkasan

REST API untuk aplikasi pencatat keuangan pribadi single-user. Backend menyimpan seluruh data transaksi, dompet, kategori, budget, target tabungan, wishlist, dan aturan transaksi berulang, lalu menyediakannya ke frontend web melalui JSON API.

**Konteks penting:** aplikasi ini hanya dipakai satu orang. Tidak ada registrasi, tidak ada multi-tenancy, tidak ada kolom `user_id` di manapun. Ini keputusan sadar, bukan kelalaian.

### 1.1 Goals

- Satu sumber kebenaran data keuangan yang persisten dan bisa di-backup
- API stabil dan enak dikonsumsi frontend
- Codebase Go yang idiomatik dan cukup sederhana untuk dikerjakan sambil belajar

### 1.2 Non-goals

- Autentikasi, role, permission
- Multi-user
- Integrasi bank atau scraping mutasi rekening
- Mata uang selain IDR
- Concurrency yang rumit. Tidak ada worker pool, tidak ada `errgroup`, tidak ada fan-out. Satu-satunya goroutine di aplikasi ini adalah scheduler transaksi berulang, dan itupun cuma `time.Ticker` sederhana

### 1.3 Perubahan dari v1.0

| Hal | v1.0 | v2.0 |
|---|---|---|
| Database | SQLite | PostgreSQL |
| Struktur | Layer-first | Feature-first |
| Entitas dompet | `accounts` | `wallets` (ganti nama, biar tidak dikira akun login) |
| Delete | Hard delete + archive | Soft delete di semua tabel |
| `color` dan `icon` | Kolom di DB | Dihapus, ditangani frontend |
| Fitur baru | | Savings dan Wishlist |

---

## 2. Tech Stack

| Komponen | Pilihan | Alasan |
|---|---|---|
| Router | `net/http` stdlib (Go 1.22) atau `chi` | Go 1.22 sudah bisa `mux.HandleFunc("GET /wallets/{id}", ...)` |
| Driver DB | `jackc/pgx/v5` | driver Postgres terbaik di Go, pakai `pgxpool` |
| Query | SQL manual dulu | biar kelihatan apa yang terjadi. `sqlc` boleh menyusul kalau sudah nyaman |
| Migration | `pressly/goose` | file SQL biasa, gampang dibaca |
| Validasi | manual di service layer | jangan tambah `validator` dulu, bikin ribet saat belajar |
| Config | `os.Getenv` + struct | |
| Logging | `log/slog` (stdlib) | |
| Testing | `testing` + `testify/require` | |
| Dev DB | Docker Compose | |

**Yang sengaja tidak dipakai dulu:** GORM (terlalu banyak sihir), `errgroup` (belum perlu), dependency injection framework (constructor manual sudah cukup).

---

## 3. Struktur Proyek (Feature-First)

Tiap fitur berdiri sendiri dalam satu folder berisi seluruh layer-nya. Kalau mau paham fitur transaksi, cukup buka satu folder.

```
finance-api/
├── cmd/
│   └── api/
│       └── main.go                  # wiring semua feature, start server
├── internal/
│   ├── features/
│   │   ├── wallet/
│   │   │   ├── model.go             # struct domain + enum
│   │   │   ├── dto.go               # request & response HTTP
│   │   │   ├── repository.go        # interface + implementasi pgx
│   │   │   ├── service.go           # business logic
│   │   │   ├── handler.go           # parse request, panggil service, tulis response
│   │   │   ├── routes.go            # daftar route milik feature ini
│   │   │   └── service_test.go
│   │   ├── category/
│   │   ├── transaction/
│   │   ├── budget/
│   │   ├── recurring/
│   │   ├── saving/
│   │   ├── wishlist/
│   │   ├── quickadd/
│   │   └── analytics/
│   ├── shared/
│   │   ├── apperror/                # error type + kode
│   │   ├── response/                # helper tulis JSON sukses & error
│   │   ├── pagination/
│   │   └── timeutil/                # helper periode, awal minggu, akhir bulan
│   └── platform/
│       ├── config/
│       ├── database/                # koneksi pgxpool, helper transaction
│       ├── middleware/              # logging, recover, cors, request id
│       └── router.go                # gabungkan semua routes.go per feature
├── db/
│   ├── migrations/
│   │   ├── 00001_init.sql
│   │   └── 00002_seed_categories.sql
│   └── seed/
├── docker-compose.yml
├── Makefile
├── .env.example
└── README.md
```

### 3.1 Aturan Antar Feature

Ini bagian yang paling gampang bocor di arsitektur feature-first. Tanpa aturan, dalam dua minggu semua feature saling import dan jadi bola benang.

**Aturan:** feature tidak boleh import package feature lain. Kalau feature A butuh data dari feature B, deklarasikan **interface kecil di dalam package A**, lalu di `main.go` masukkan service B sebagai implementasinya.

Contoh nyata, wishlist butuh tahu kemampuan menabung dari feature saving:

```go
// internal/features/wishlist/service.go
package wishlist

// interface dideklarasikan di sini, di sisi yang MEMAKAI
type SavingsReader interface {
    AverageMonthlySavings(ctx context.Context, months int) (int64, error)
}

type Service struct {
    repo    Repository
    savings SavingsReader
}

func NewService(repo Repository, savings SavingsReader) *Service {
    return &Service{repo: repo, savings: savings}
}
```

Lalu di `main.go`:

```go
savingSvc := saving.NewService(saving.NewRepository(pool))
wishlistSvc := wishlist.NewService(wishlist.NewRepository(pool), savingSvc)
```

`wishlist` tidak pernah menyebut nama package `saving`. Bonus: test wishlist cukup pakai struct palsu yang memenuhi `SavingsReader`, tanpa perlu database.

**Dependensi antar feature yang diizinkan:**

| Feature | Butuh data dari |
|---|---|
| `transaction` | `wallet`, `category` (cuma untuk validasi keberadaan dan tipe) |
| `budget` | `transaction` (total pengeluaran per kategori) |
| `saving` | `transaction` |
| `wishlist` | `saving` |
| `analytics` | `transaction` |
| `recurring` | `transaction` (untuk membuat transaksi) |
| `quickadd` | `transaction` |

Semuanya lewat interface kecil, bukan import langsung.

---

## 4. Data Model

### 4.1 Aturan Umum

- **Uang: `BIGINT`, satuan Rupiah penuh.** Tidak ada `NUMERIC`, tidak ada `float`. Rupiah tidak punya pecahan praktis
- **ID: `UUID` dengan default `gen_random_uuid()`.** Built-in di Postgres 13+, tidak perlu extension
- **Waktu: `TIMESTAMPTZ`.** Postgres menyimpan UTC, konversi ke `Asia/Jakarta` dilakukan saat query agregasi
- **Enum: pakai `CHECK` constraint, bukan tipe `ENUM` Postgres.** Tipe enum native menyusahkan saat mau menambah nilai baru
- **Soft delete di semua tabel:** kolom `deleted_at TIMESTAMPTZ NULL`
- **Tidak ada kolom `color` dan `icon`.** Tampilan sepenuhnya urusan frontend

### 4.2 Aturan Soft Delete

Kamu bilang sering kena error constraint saat hard delete. Soft delete memang menyelesaikan itu, tapi bawa tiga jebakan yang harus ditangani dari awal, bukan ditambal belakangan.

**Jebakan 1: unique constraint jadi rusak.**
Kalau kamu hapus wallet bernama "BCA" lalu bikin lagi dengan nama sama, unique constraint biasa akan menolak karena baris lama masih ada. Solusinya **partial unique index**:

```sql
CREATE UNIQUE INDEX wallets_name_unique
    ON wallets (lower(name))
    WHERE deleted_at IS NULL;
```

**Jebakan 2: lupa filter.**
Setiap query SELECT wajib punya `WHERE deleted_at IS NULL`. Satu saja yang lupa, data hantu muncul di UI. Taruh filter ini di query dasar tiap repository, jangan diserahkan ke pemanggil.

**Jebakan 3: referensi ke baris yang sudah dihapus.**
Transaksi lama boleh tetap menunjuk wallet yang sudah dihapus. Ini justru yang kita mau, riwayat tidak boleh hilang. Karena itu:

- FK tetap ada, tapi **tanpa** `ON DELETE CASCADE`
- Saat menampilkan transaksi lama, nama wallet yang sudah dihapus tetap ikut ditampilkan, ditandai `"is_deleted": true` supaya frontend bisa memberi tanda
- Wallet atau kategori yang sudah dihapus **tidak boleh dipilih** untuk transaksi baru

**Endpoint restore** disediakan untuk semua entitas yang bisa dihapus. Ini keuntungan gratis dari soft delete, jangan disia-siakan.

### 4.3 Tabel: `wallets`

Tempat uangmu berada. Contoh: Cash, BCA, GoPay, Dana.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK DEFAULT gen_random_uuid() | |
| `name` | TEXT NOT NULL | partial unique index pada `lower(name)` |
| `type` | TEXT NOT NULL | CHECK IN (`cash`, `bank`, `ewallet`, `savings`, `other`) |
| `initial_balance` | BIGINT NOT NULL DEFAULT 0 | saldo saat wallet dibuat |
| `is_excluded_from_total` | BOOLEAN NOT NULL DEFAULT false | untuk deposito atau dana darurat yang tidak mau dihitung di saldo harian |
| `sort_order` | INTEGER NOT NULL DEFAULT 0 | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |
| `updated_at` | TIMESTAMPTZ NOT NULL DEFAULT now() | |

**Saldo berjalan tidak disimpan sebagai kolom.** Dihitung dari `initial_balance` plus agregasi transaksi. Kolom saldo yang di-update manual adalah sumber bug nomor satu di aplikasi keuangan, karena begitu ada satu update yang gagal di tengah jalan, angkanya salah selamanya dan tidak ada cara tahu.

### 4.4 Tabel: `categories`

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `name` | TEXT NOT NULL | |
| `type` | TEXT NOT NULL | CHECK IN (`income`, `expense`) |
| `is_default` | BOOLEAN NOT NULL DEFAULT false | hasil seeding |
| `sort_order` | INTEGER NOT NULL DEFAULT 0 | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

Partial unique index pada `(lower(name), type) WHERE deleted_at IS NULL`.

**Seed default:**
- Expense: Makan & Minum, Transport, Belanja, Tagihan, Hiburan, Kesehatan, Pendidikan, Lainnya
- Income: Gaji, Freelance, Bonus, Hadiah, Lainnya

> **Catatan soal warna dan ikon.** Karena `color` dan `icon` sekarang di frontend, frontend butuh cara memetakan kategori ke warna. ID-nya UUID yang digenerate saat runtime, jadi tidak bisa di-hardcode. Dua opsi, pilih salah satu dan tulis di PRD frontend nanti:
> 1. Frontend simpan mapping `category_id -> color` di localStorage, dengan fallback warna yang dihitung deterministik dari hash UUID
> 2. Backend menambah kolom `slug` yang stabil (`makan-minum`, `transport`) yang bisa di-hardcode frontend
>
> Rekomendasi: opsi 1, karena tidak menambah beban ke backend dan kategori buatan sendiri tetap dapat warna otomatis.

### 4.5 Tabel: `transactions`

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `type` | TEXT NOT NULL | CHECK IN (`income`, `expense`, `transfer`) |
| `amount` | BIGINT NOT NULL | CHECK (amount > 0). Arah ditentukan `type`, bukan tanda minus |
| `wallet_id` | UUID NOT NULL FK | untuk transfer, ini wallet sumber |
| `to_wallet_id` | UUID NULL FK | wajib dan hanya diisi kalau `type = transfer` |
| `category_id` | UUID NULL FK | wajib untuk income/expense, harus NULL untuk transfer |
| `note` | TEXT NOT NULL DEFAULT '' | |
| `occurred_at` | TIMESTAMPTZ NOT NULL | tanggal menurut user, boleh backdate |
| `recurring_rule_id` | UUID NULL FK | terisi kalau hasil generate otomatis |
| `wishlist_item_id` | UUID NULL FK | terisi kalau transaksi ini pembelian item wishlist |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

**Index:**
```sql
CREATE INDEX ON transactions (occurred_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ON transactions (type, occurred_at) WHERE deleted_at IS NULL;
CREATE INDEX ON transactions (category_id) WHERE deleted_at IS NULL;
CREATE INDEX ON transactions (wallet_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX ON transactions (recurring_rule_id, occurred_at)
    WHERE recurring_rule_id IS NOT NULL AND deleted_at IS NULL;
```
Index unik terakhir itu yang membuat scheduler idempoten. Penting.

### 4.6 Tabel: `budgets`

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `category_id` | UUID NOT NULL FK | hanya kategori bertipe `expense` |
| `amount` | BIGINT NOT NULL | batas per periode |
| `period` | TEXT NOT NULL DEFAULT 'monthly' | CHECK IN (`weekly`, `monthly`) |
| `start_month` | DATE NOT NULL | selalu tanggal 1 |
| `end_month` | DATE NULL | NULL berarti berlaku terus |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

Satu kategori maksimal punya satu budget aktif pada satu waktu.

### 4.7 Tabel: `savings_targets` (FITUR BARU)

Target nabung per periode. Opsional, kalau tidak diisi fitur savings tetap jalan dengan angka aktual saja.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `period` | TEXT NOT NULL | CHECK IN (`weekly`, `monthly`) |
| `amount` | BIGINT NOT NULL | target nominal |
| `target_rate` | NUMERIC(5,2) NULL | alternatif berbasis persen, misal 20.00 berarti 20% dari pemasukan |
| `start_date` | DATE NOT NULL | |
| `end_date` | DATE NULL | |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

**Aturan:** isi `amount` atau `target_rate`, tidak boleh dua-duanya, tidak boleh kosong dua-duanya. Maksimal satu target aktif per `period`.

### 4.8 Tabel: `wishlist_items` (FITUR BARU)

Barang yang ingin dibeli.

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `name` | TEXT NOT NULL | |
| `estimated_price` | BIGINT NOT NULL | CHECK (> 0) |
| `priority` | TEXT NOT NULL DEFAULT 'medium' | CHECK IN (`low`, `medium`, `high`) |
| `target_date` | DATE NULL | kapan ingin dibeli |
| `product_url` | TEXT NULL | |
| `note` | TEXT NOT NULL DEFAULT '' | |
| `status` | TEXT NOT NULL DEFAULT 'planned' | CHECK IN (`planned`, `saving`, `purchased`, `cancelled`) |
| `saved_amount` | BIGINT NOT NULL DEFAULT 0 | uang yang sudah dialokasikan manual untuk item ini |
| `purchased_at` | TIMESTAMPTZ NULL | |
| `purchase_transaction_id` | UUID NULL FK | transaksi expense yang tercipta saat item dibeli |
| `sort_order` | INTEGER NOT NULL DEFAULT 0 | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

### 4.9 Tabel: `recurring_rules`

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `name` | TEXT NOT NULL | |
| `type` | TEXT NOT NULL | CHECK IN (`income`, `expense`) |
| `amount` | BIGINT NOT NULL | |
| `wallet_id` | UUID NOT NULL FK | |
| `category_id` | UUID NOT NULL FK | |
| `note` | TEXT NOT NULL DEFAULT '' | |
| `frequency` | TEXT NOT NULL | CHECK IN (`daily`, `weekly`, `monthly`, `yearly`) |
| `interval` | INTEGER NOT NULL DEFAULT 1 | tiap N frequency |
| `day_of_month` | SMALLINT NULL | 1-31, untuk monthly |
| `day_of_week` | SMALLINT NULL | 0-6, untuk weekly |
| `start_date` | DATE NOT NULL | |
| `end_date` | DATE NULL | |
| `next_run_at` | DATE NOT NULL | |
| `last_run_at` | DATE NULL | |
| `is_active` | BOOLEAN NOT NULL DEFAULT true | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

**Edge case wajib:** `day_of_month = 31` di bulan Februari. Aturan: pakai hari terakhir bulan tersebut. Cara aman di Go: `time.Date(y, m+1, 0, ...)` memberi hari terakhir bulan `m`.

### 4.10 Tabel: `quick_adds`

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | UUID PK | |
| `label` | TEXT NOT NULL | |
| `type` | TEXT NOT NULL | CHECK IN (`income`, `expense`) |
| `amount` | BIGINT NULL | NULL berarti user isi manual saat dipakai |
| `wallet_id` | UUID NOT NULL FK | |
| `category_id` | UUID NOT NULL FK | |
| `note` | TEXT NOT NULL DEFAULT '' | |
| `sort_order` | INTEGER NOT NULL DEFAULT 0 | |
| `deleted_at` | TIMESTAMPTZ NULL | |
| `created_at`, `updated_at` | TIMESTAMPTZ | |

---

## 5. Spesifikasi API

Base path `/api/v1`. Semua JSON.

### 5.1 Format Response

Resource tunggal:
```json
{ "data": { "id": "...", "name": "..." } }
```

List:
```json
{ "data": [ ... ], "meta": { "total": 248, "limit": 50, "offset": 0 } }
```

Error:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "amount harus lebih besar dari 0",
    "details": { "field": "amount" }
  }
}
```

Kode error: `VALIDATION_ERROR` (400), `NOT_FOUND` (404), `CONFLICT` (409), `UNPROCESSABLE` (422), `INTERNAL_ERROR` (500).

### 5.2 Health

```
GET /api/v1/health
→ 200 { "status": "ok", "db": "ok", "version": "2.0.0" }
```

### 5.3 Wallets

```
GET    /api/v1/wallets?include_deleted=false
POST   /api/v1/wallets
GET    /api/v1/wallets/{id}
PATCH  /api/v1/wallets/{id}
DELETE /api/v1/wallets/{id}          # soft delete
POST   /api/v1/wallets/{id}/restore
```

Response menyertakan saldo terhitung:
```json
{
  "data": [
    {
      "id": "0192...",
      "name": "BCA",
      "type": "bank",
      "initial_balance": 5000000,
      "current_balance": 3450000,
      "is_excluded_from_total": false,
      "transaction_count": 87,
      "deleted_at": null
    }
  ]
}
```

**Aturan delete:** selalu berhasil, karena soft delete. Tapi kalau wallet masih punya transaksi, sertakan peringatan di response supaya frontend bisa konfirmasi dulu:
```json
{ "data": { "id": "...", "deleted_at": "...", "affected_transactions": 87 } }
```

### 5.4 Categories

```
GET    /api/v1/categories?type=expense&include_deleted=false
POST   /api/v1/categories
GET    /api/v1/categories/{id}
PATCH  /api/v1/categories/{id}
DELETE /api/v1/categories/{id}
POST   /api/v1/categories/{id}/restore
```

Kategori dengan `is_default = true` boleh dihapus, tapi seeding tidak akan membuatnya lagi. Cek keberadaan saat seeding pakai `WHERE name = ... AND type = ...` tanpa filter `deleted_at`, supaya kategori yang sengaja dihapus tidak muncul lagi tiap restart.

### 5.5 Transactions

```
GET    /api/v1/transactions
POST   /api/v1/transactions
GET    /api/v1/transactions/{id}
PATCH  /api/v1/transactions/{id}
DELETE /api/v1/transactions/{id}       # soft delete
POST   /api/v1/transactions/{id}/restore
```

**Query params `GET /transactions`:**

| Param | Tipe | Default | Keterangan |
|---|---|---|---|
| `from` | `YYYY-MM-DD` | awal bulan berjalan | inklusif |
| `to` | `YYYY-MM-DD` | hari ini | inklusif |
| `type` | string | semua | `income` / `expense` / `transfer` |
| `category_id` | UUID, boleh CSV | semua | |
| `wallet_id` | UUID, boleh CSV | semua | |
| `min_amount`, `max_amount` | int | | |
| `q` | string | | cari di `note` (`ILIKE '%q%'`) |
| `sort` | string | `occurred_at:desc` | |
| `limit` | int | 50 | max 200 |
| `offset` | int | 0 | |

Response item sudah expand wallet dan category, supaya frontend tidak N+1:
```json
{
  "id": "...",
  "type": "expense",
  "amount": 25000,
  "note": "Kopi susu",
  "occurred_at": "2026-08-11T09:30:00+07:00",
  "wallet": { "id": "...", "name": "Cash", "is_deleted": false },
  "category": { "id": "...", "name": "Makan & Minum", "type": "expense", "is_deleted": false },
  "to_wallet": null
}
```

**Transfer** dibuat lewat endpoint yang sama dengan `type: "transfer"`, `to_wallet_id` diisi, `category_id` kosong. Transfer tidak dihitung sebagai pemasukan maupun pengeluaran di summary, tapi tetap mempengaruhi saldo tiap wallet.

### 5.6 Summary (Dashboard)

```
GET /api/v1/summary?from=2026-08-01&to=2026-08-31
```
```json
{
  "data": {
    "period": { "from": "2026-08-01", "to": "2026-08-31" },
    "total_income": 8500000,
    "total_expense": 4230000,
    "net": 4270000,
    "total_balance": 12750000,
    "transaction_count": 87,
    "comparison": { "income_change_pct": 4.2, "expense_change_pct": -11.8 }
  }
}
```
`total_balance` adalah saldo seluruh wallet aktif yang `is_excluded_from_total = false`, bukan terbatas periode. `comparison` membandingkan periode sebelumnya yang panjangnya sama.

### 5.7 Savings (FITUR BARU)

Ini menjawab pertanyaan "berapa uang yang bisa aku tabung".

```
GET /api/v1/savings/summary?period=month&from=&to=
```
`period`: `week` atau `month`. Kalau `from` dan `to` kosong, default ke periode berjalan.

```json
{
  "data": {
    "period": "month",
    "range": { "from": "2026-08-01", "to": "2026-08-31" },
    "income": 8500000,
    "expense": 4230000,
    "savable": 4270000,
    "savings_rate": 50.24,
    "target": {
      "amount": 3000000,
      "achieved": true,
      "progress_pct": 142.3,
      "difference": 1270000
    },
    "daily_average_expense": 136451,
    "projected_savable": 4270000,
    "days_elapsed": 11,
    "days_total": 31
  }
}
```

**Definisi `savable` = income - expense dalam periode tersebut.** Transfer diabaikan. Angka ini boleh negatif kalau memang boros, jangan di-clamp ke 0.

`projected_savable` adalah proyeksi sampai akhir periode kalau pola pengeluaran bertahan:
```
projected_savable = income - (daily_average_expense * days_total)
```
Untuk periode yang sudah lewat, nilainya sama dengan `savable`.

```
GET /api/v1/savings/breakdown?period=week&from=2026-06-01&to=2026-08-31
```
Deret per bucket untuk chart. Bucket kosong tetap dikembalikan dengan nilai 0.
```json
{
  "data": [
    { "bucket": "2026-W23", "start": "2026-06-01", "end": "2026-06-07",
      "income": 0, "expense": 620000, "savable": -620000, "savings_rate": 0 },
    { "bucket": "2026-W24", "start": "2026-06-08", "end": "2026-06-14",
      "income": 8500000, "expense": 1240000, "savable": 7260000, "savings_rate": 85.4 }
  ]
}
```

**Target tabungan:**
```
GET    /api/v1/savings/targets
POST   /api/v1/savings/targets
PATCH  /api/v1/savings/targets/{id}
DELETE /api/v1/savings/targets/{id}
```

**Catatan SQL soal bucket mingguan.** Postgres `date_trunc('week', ...)` selalu mulai hari Senin. Kalau nanti mau minggu dimulai hari Minggu, geser manual, jangan cari flag konfigurasi karena tidak ada. Jangan lupa konversi timezone dulu:
```sql
date_trunc('week', occurred_at AT TIME ZONE 'Asia/Jakarta')
```
Tanpa `AT TIME ZONE`, transaksi jam 7 pagi WIB akan masuk ke hari sebelumnya karena UTC.

### 5.8 Wishlist (FITUR BARU)

```
GET    /api/v1/wishlist?status=planned&sort=priority:desc
POST   /api/v1/wishlist
GET    /api/v1/wishlist/{id}
PATCH  /api/v1/wishlist/{id}
DELETE /api/v1/wishlist/{id}
POST   /api/v1/wishlist/{id}/restore
POST   /api/v1/wishlist/{id}/allocate     # tambah saved_amount
POST   /api/v1/wishlist/{id}/purchase     # tandai dibeli + bikin transaksi expense
```

Response item menyertakan perhitungan keterjangkauan, hasil integrasi dengan feature savings:
```json
{
  "data": {
    "id": "...",
    "name": "Keyboard mekanik",
    "estimated_price": 1200000,
    "priority": "high",
    "target_date": "2026-12-01",
    "status": "saving",
    "saved_amount": 400000,
    "remaining": 800000,
    "progress_pct": 33.3,
    "affordability": {
      "avg_monthly_savable": 4270000,
      "months_needed": 1,
      "estimated_ready_date": "2026-09-11",
      "on_track_for_target_date": true
    }
  }
}
```

**Aturan perhitungan `affordability`:**
- `avg_monthly_savable` = rata-rata `savable` 3 bulan terakhir yang sudah lengkap. Bulan berjalan tidak dihitung karena datanya belum penuh
- Kalau data historis kurang dari 1 bulan penuh, kembalikan `affordability: null`. Jangan tebak-tebakan
- Kalau `avg_monthly_savable <= 0`, kembalikan `months_needed: null` dan `on_track_for_target_date: false`
- `months_needed = ceil(remaining / avg_monthly_savable)`

**Endpoint `/purchase`:**
```json
POST /api/v1/wishlist/{id}/purchase
{
  "wallet_id": "...",
  "category_id": "...",
  "actual_price": 1150000,
  "occurred_at": "2026-09-15T10:00:00+07:00"
}
```
Dalam **satu database transaction**:
1. Insert `transactions` bertipe `expense` dengan `wishlist_item_id` terisi
2. Update wishlist item: `status = 'purchased'`, `purchased_at`, `purchase_transaction_id`

Kalau salah satu gagal, dua-duanya batal. Ini latihan `pgx.Tx` yang paling pas untuk kamu.

**Ringkasan wishlist:**
```
GET /api/v1/wishlist/summary
→ { "total_items": 7, "total_estimated": 8400000, "total_saved": 1200000,
    "by_priority": { "high": 2, "medium": 3, "low": 2 } }
```

### 5.9 Budgets

```
GET    /api/v1/budgets
POST   /api/v1/budgets
PATCH  /api/v1/budgets/{id}
DELETE /api/v1/budgets/{id}
GET    /api/v1/budgets/status?month=2026-08
```
```json
{
  "data": [
    { "budget_id": "...", "category_id": "...", "category_name": "Makan & Minum",
      "limit": 2000000, "spent": 1850000, "remaining": 150000,
      "percentage": 92.5, "status": "warning", "days_left": 20 }
  ]
}
```
`status`: `safe` (<75%), `warning` (75-100%), `exceeded` (>100%).

### 5.10 Analytics

```
GET /api/v1/analytics/by-category?from=&to=&type=expense
GET /api/v1/analytics/trend?from=&to=&granularity=month   # day | week | month
GET /api/v1/analytics/by-wallet?from=&to=
```

### 5.11 Recurring Rules

```
GET    /api/v1/recurring
POST   /api/v1/recurring
GET    /api/v1/recurring/{id}
PATCH  /api/v1/recurring/{id}
DELETE /api/v1/recurring/{id}
POST   /api/v1/recurring/{id}/toggle
POST   /api/v1/recurring/run             # trigger manual, buat testing
```

### 5.12 Quick Adds

```
GET    /api/v1/quick-adds
POST   /api/v1/quick-adds
PATCH  /api/v1/quick-adds/{id}
DELETE /api/v1/quick-adds/{id}
POST   /api/v1/quick-adds/{id}/execute
```

### 5.13 Export, Import, Backup

```
GET  /api/v1/export?format=json&from=&to=
GET  /api/v1/export?format=csv&from=&to=
POST /api/v1/import?dry_run=true         # multipart, field "file"
```

`dry_run=true` mengembalikan preview jumlah baris valid plus daftar error per baris tanpa menulis apapun. Import sesungguhnya jalan dalam satu database transaction, all or nothing.

Format CSV:
```
date,type,amount,wallet,category,note
2026-08-11,expense,25000,Cash,Makan & Minum,Kopi susu
```

Backup database bukan lewat API, tapi lewat `pg_dump` yang dijalankan dari Makefile atau cron di host:
```
make backup   # pg_dump -Fc > backups/finance_$(date +%Y%m%d).dump
```

---

## 6. Business Rules

Semua ditegakkan di layer **service**, bukan handler, bukan database.

1. `amount` harus `> 0`. Arah uang ditentukan `type`, bukan tanda minus
2. `category.type` harus cocok dengan `transaction.type`
3. Transaksi `transfer` wajib `to_wallet_id` terisi, `category_id` NULL, dan `wallet_id != to_wallet_id`
4. Transaksi `income` dan `expense` wajib `category_id` terisi dan `to_wallet_id` NULL
5. `occurred_at` boleh di masa lalu. Lebih dari 1 hari di masa depan ditolak
6. Wallet atau kategori yang sudah di-soft-delete tidak boleh dipakai untuk transaksi baru, tapi transaksi lama yang mereferensikannya tetap valid dan tetap muncul
7. Saldo wallet boleh minus. Jangan diblokir, ini pencatatan bukan sistem pembayaran
8. Update transaksi hasil recurring hanya mengubah transaksi itu, tidak mengubah rule-nya
9. Soft delete recurring rule tidak menghapus transaksi yang sudah digenerate
10. `savable` boleh negatif dan harus ditampilkan apa adanya
11. `wishlist.saved_amount` tidak boleh melebihi `estimated_price`. Kelebihannya ditolak dengan `VALIDATION_ERROR`
12. Wishlist berstatus `purchased` tidak boleh dialokasi tambahan atau diubah harganya. Harus di-restore ke `planned` dulu
13. `savings_targets` wajib mengisi salah satu antara `amount` atau `target_rate`, tidak boleh keduanya
14. Semua operasi yang menyentuh lebih dari satu tabel wajib dalam satu database transaction

---

## 7. Scheduler Transaksi Berulang

Satu goroutine, `time.Ticker` interval 1 jam. Tidak ada `errgroup`, tidak ada worker pool. Cukup begini:

```go
func (s *Scheduler) Start(ctx context.Context) {
    go func() {
        s.runOnce(ctx)                      // jalan sekali saat startup
        ticker := time.NewTicker(time.Hour)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                s.runOnce(ctx)
            }
        }
    }()
}
```

**Algoritma `runOnce`:**
1. Ambil rules dengan `is_active = true`, `deleted_at IS NULL`, `next_run_at <= today`, dan (`end_date IS NULL` OR `end_date >= today`)
2. Untuk tiap rule, loop generate transaksi sampai `next_run_at > today`. Loop ini penting kalau server sempat mati beberapa hari
3. Hitung `next_run_at` berikutnya dari `frequency` dan `interval`
4. Update `last_run_at` dan `next_run_at` dalam transaction yang sama dengan insert

**Idempotensi:** dijamin oleh unique index `(recurring_rule_id, occurred_at)`. Kalau insert kena conflict, perlakukan sebagai skip, jangan sebagai error. Di pgx, cek `pgconn.PgError` dengan `Code == "23505"`, atau lebih simpel pakai `ON CONFLICT DO NOTHING`.

**Kalau satu rule error, jangan hentikan yang lain.** Log error rule tersebut lalu lanjut ke rule berikutnya.

---

## 8. Konfigurasi

```env
APP_ENV=development
APP_PORT=8080
APP_HOST=127.0.0.1
APP_TZ=Asia/Jakarta

DB_HOST=localhost
DB_PORT=5432
DB_USER=finance
DB_PASSWORD=finance
DB_NAME=finance_tracker
DB_SSLMODE=disable
DB_MAX_CONNS=10

CORS_ALLOWED_ORIGINS=http://localhost:5173

API_TOKEN=
```

**Soal keamanan tanpa login:** `APP_HOST` default `127.0.0.1`, jadi API cuma bisa diakses dari mesin sendiri. Kalau nanti mau diakses dari HP di WiFi rumah, isi `API_TOKEN` dan aktifkan middleware pengecek header `X-API-Token`. Tulis middleware-nya dari awal tapi jadikan no-op kalau token kosong. Jauh lebih gampang daripada menambal belakangan.

**`docker-compose.yml` untuk dev:**
```yaml
services:
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: finance
      POSTGRES_PASSWORD: finance
      POSTGRES_DB: finance_tracker
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
volumes:
  pgdata:
```

---

## 9. Non-Functional Requirements

| Aspek | Target |
|---|---|
| Response time | p95 < 150ms. Volume data pribadi kecil, kalau lebih lambat berarti ada query yang lupa index |
| Startup | migration jalan otomatis, seed kategori kalau tabel kosong |
| Shutdown | graceful, `context.WithTimeout` 10 detik |
| Connection pool | `pgxpool` dengan `MaxConns` dari config, jangan buka koneksi per request |
| Logging | JSON via `slog`: request id, method, path, status, duration |
| Panic | middleware recover, log stack trace, balas 500 tanpa bocorin detail |
| Test | business rules di bagian 6 wajib punya test. Handler dan repository menyusul, tidak dikejar coverage number |

---

## 10. Milestone

Disusun supaya tiap tahap menghasilkan sesuatu yang jalan, bukan setengah jadi.

| Tahap | Isi | Selesai kalau |
|---|---|---|
| M0 | Docker Compose, config, koneksi pgxpool, migration, health check | `GET /health` balas 200 dan `db: ok` |
| M1 | Feature `wallet` lengkap (semua layer) + soft delete + restore | Bisa CRUD dompet, delete tidak error constraint |
| M2 | Feature `category` + seeding | Kategori default muncul saat pertama start |
| M3 | Feature `transaction`: CRUD, filter, business rules 1-9 | Bisa catat pemasukan, pengeluaran, transfer |
| M4 | Feature `analytics` + endpoint summary | Angka dashboard lengkap |
| M5 | Feature `saving`: summary, breakdown, targets | Tahu berapa yang bisa ditabung per minggu dan per bulan |
| M6 | Feature `wishlist` + affordability + purchase | Wishlist jalan dan nyambung ke savings |
| M7 | Feature `budget` | Progress bar budget jalan |
| M8 | Feature `recurring` + scheduler | Transaksi berulang tergenerate otomatis |
| M9 | `quickadd`, export, import | Data bisa keluar masuk |

**Frontend sudah bisa mulai setelah M4.**

M1 adalah tahap paling penting untuk dikerjakan pelan-pelan. Begitu satu feature selesai dengan struktur yang rapi, delapan feature sisanya tinggal menyalin pola yang sama. Jangan buru-buru pindah ke M2 sebelum struktur `wallet` terasa nyaman dibaca.

---

## 11. Acceptance Criteria

- [ ] Semua endpoint di bagian 5 ada dan formatnya sesuai 5.1
- [ ] Business rules di bagian 6 punya unit test yang lulus
- [ ] Tidak ada nilai uang bertipe `float` di manapun
- [ ] Tidak ada satupun query SELECT yang lupa `WHERE deleted_at IS NULL`
- [ ] Semua unique constraint pakai partial index dengan `WHERE deleted_at IS NULL`
- [ ] Tidak ada package feature yang meng-import package feature lain
- [ ] Scheduler idempoten, dibuktikan dengan test yang menjalankan siklus dua kali
- [ ] Endpoint `/wishlist/{id}/purchase` benar-benar atomik, dibuktikan dengan test yang memaksa gagal di langkah kedua
- [ ] Semua bucket waktu memakai `AT TIME ZONE` sebelum `date_trunc`
- [ ] `go vet` bersih

---

## 12. Catatan untuk Kamu yang Lagi Belajar Go

Kamu bilang baru sampai database transaction dan belum sampai `errgroup`. Kabar baiknya, proyek ini **tidak butuh `errgroup` sama sekali**. Yang sudah kamu pelajari sudah cukup untuk menyelesaikan seluruh PRD ini.

### Pola yang dipakai di sini dan kenapa

**Interface dideklarasikan di sisi yang memakai, bukan yang menyediakan.** Ini kebalikan dari Java atau C#. Di Go, package `wishlist` yang mendeklarasikan `SavingsReader`, bukan package `saving`. Efeknya: `saving` tidak perlu tahu siapa yang memakainya, dan test `wishlist` tidak butuh database.

**Constructor injection, tanpa framework.** Semua dependensi masuk lewat `NewService(...)` dan dirakit di `main.go`. Jangan pakai variabel global untuk pool database atau config. `main.go` yang panjang dan berisi rakitan itu normal dan bagus, karena satu tempat itulah gambaran utuh aplikasimu.

**Custom error type.** Bikin package `shared/apperror`:
```go
var ErrNotFound = errors.New("not found")

type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string { return e.Message }
```
Service melempar error ini, handler memetakannya ke status code pakai `errors.Is` dan `errors.As`. Jangan pernah membandingkan error dengan `err.Error() == "not found"`.

**Helper database transaction.** Kamu sudah belajar ini, jadi bungkus jadi satu fungsi yang dipakai ulang:
```go
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)   // aman dipanggil setelah Commit, jadi no-op
    if err := fn(tx); err != nil {
        return err
    }
    return tx.Commit(ctx)
}
```
`defer tx.Rollback(ctx)` setelah commit itu tidak error, cuma no-op. Ini idiom standar dan menghindari lupa rollback di jalur error.

**Table-driven test.** Untuk validasi business rules, ini idiom Go yang paling sering dipakai di dunia nyata:
```go
tests := []struct{
    name    string
    input   CreateTransactionRequest
    wantErr bool
}{
    {"amount nol ditolak", CreateTransactionRequest{Amount: 0}, true},
    {"transfer tanpa to_wallet ditolak", ..., true},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

**`context.Context` sebagai argumen pertama** di semua method service dan repository, diteruskan sampai query pgx. Ini yang membuat request bisa dibatalkan saat client menutup koneksi.

### Yang boleh ditunda dulu

- `sqlc` dan code generation. Tulis SQL manual dulu sampai kamu hafal bentuk querynya
- Repository test yang pakai database asli. Test service dengan repository palsu sudah memberi nilai paling besar per usaha
- Caching. Tidak perlu, datanya kecil
- Rate limiting. Penggunanya kamu sendiri

### Soal bakal banyak bug

Wajar dan memang begitu prosesnya. Yang membedakan proyek yang selesai dan yang mangkrak bukan jumlah bug, tapi apakah strukturnya memudahkan bug ditemukan. Struktur feature-first plus business rules yang ditulis eksplisit di bagian 6 sudah menyiapkan itu: kalau ada angka yang salah, kamu tahu harus buka file mana.

Satu saran praktis: kerjakan M1 sampai benar-benar rapi sebelum lanjut. Feature `wallet` adalah cetakanmu. Kalau cetakannya bagus, delapan feature sisanya jadi pekerjaan mekanis.
