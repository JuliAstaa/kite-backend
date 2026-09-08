# Kite Finance API — Dokumentasi Frontend

Versi 2.0.0 · Base path `/api/v1`

REST API pencatat keuangan pribadi. Single user, tidak ada registrasi, tidak ada
konsep login. Semua request dan response berformat JSON.

---

## 1. Autentikasi

Semua endpoint butuh header:

```
X-API-Token: <token>
```

**Token dikirim terpisah, tidak ada di repo ini.** Simpan di environment
variable frontend (`.env.local` atau sejenisnya), jangan di-hardcode di source
dan jangan di-commit.

Token salah atau tidak dikirim:

```json
HTTP 401
{ "error": { "code": "UNAUTHORIZED", "message": "X-API-Token tidak valid" } }
```

Request `OPTIONS` (preflight CORS) tidak butuh token.

> Kalau backend dijalankan tanpa `API_TOKEN` di `.env`, pengecekan token mati
> total dan semua request lolos. Berguna saat development lokal.

## 2. CORS

Origin yang diizinkan diatur di backend lewat `CORS_ALLOWED_ORIGINS`, default
`http://localhost:5173`. Kalau origin frontend berbeda (port lain, domain
deploy), minta backend menambahkannya — bukan sesuatu yang bisa diakali dari
sisi frontend.

Header yang diizinkan: `Content-Type`, `X-API-Token`.
Method yang diizinkan: `GET`, `POST`, `PATCH`, `DELETE`, `OPTIONS`.

## 3. Bentuk Response

**Satu resource:**

```json
{ "data": { "id": "0192...", "name": "BCA" } }
```

**Daftar dengan pagination:**

```json
{
  "data": [ ... ],
  "meta": { "total": 248, "limit": 50, "offset": 0 }
}
```

Yang punya `meta`: wallets, categories, transactions, wishlist, budgets,
recurring, quick-adds, savings targets. Empat yang terakhir tidak menerima
`limit`/`offset` dan selalu mengembalikan seluruh data — `meta.total` tetap
diisi, `meta.limit` disamakan dengan total.

**Daftar tanpa `meta`** — `data` langsung berupa array. Berlaku untuk seluruh
endpoint analytics, `budgets/status`, `savings/breakdown`, dan `wishlist/summary`:

```json
{ "data": [ ... ] }
```

**Error:**

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "amount harus lebih besar dari 0",
    "details": { "amount": "amount harus lebih besar dari 0" }
  }
}
```

`details` bersifat opsional dan berisi peta `nama_field → pesan`. Cocok dipakai
langsung untuk menampilkan error di bawah input form.

### Kode error

| Kode | HTTP | Kapan muncul |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Field kosong, format salah, atau melanggar aturan bisnis |
| `INVALID_JSON` | 400 | Body bukan JSON yang valid |
| `UNAUTHORIZED` | 401 | `X-API-Token` salah atau tidak dikirim |
| `NOT_FOUND` | 404 | ID tidak ada, atau resource-nya sudah dihapus |
| `METHOD_NOT_ALLOWED` | 405 | Method tidak didukung di path tersebut |
| `CONFLICT` | 409 | Bentrok dengan data yang ada (nama duplikat, budget ganda) |
| `UNPROCESSABLE` | 422 | Request valid tapi tidak bisa dijalankan pada keadaan sekarang |
| `INTERNAL_ERROR` | 500 | Kesalahan server |

Beda `VALIDATION_ERROR` dan `UNPROCESSABLE`: yang pertama berarti input perlu
diperbaiki, yang kedua berarti input sudah benar tapi state-nya tidak
mengizinkan — misalnya mengalokasikan dana ke wishlist yang sudah dibeli.

## 4. Aturan Umum

**Uang selalu bilangan bulat Rupiah penuh.** `25000` berarti Rp 25.000. Tidak
ada desimal, tidak ada string. Jangan pakai `parseFloat`.

**Zona waktu aplikasi WITA (`Asia/Makassar`, UTC+8).** Timestamp dikembalikan
dalam format RFC3339 dengan offset, misal `2026-09-07T09:30:00+08:00`. Field
yang hanya berupa tanggal memakai string `YYYY-MM-DD`.

**Filter tanggal inklusif di dua ujung.** `from=2026-08-01&to=2026-08-31`
mencakup seluruh 31 Agustus sampai jam 23:59:59.

**Soft delete.** Data yang dihapus tidak hilang, hanya ditandai. Konsekuensinya
untuk frontend:

- Response punya field `deleted_at` (`null` kalau masih aktif)
- List punya query param `include_deleted=true` untuk ikut menampilkannya
- Ada endpoint `POST .../{id}/restore` untuk membatalkan penghapusan
- Transaksi lama tetap menampilkan nama wallet dan kategori yang sudah dihapus,
  ditandai `"is_deleted": true` — tampilkan dengan gaya berbeda (dicoret, abu-abu)
  supaya user tahu itu referensi ke data yang sudah tidak aktif
- Wallet atau kategori yang sudah dihapus **ditolak** untuk transaksi baru (404)

### Semantik PATCH

Semua endpoint PATCH bersifat parsial dengan tiga keadaan berbeda:

| Yang dikirim | Artinya |
|---|---|
| Field tidak ada di body | Jangan diubah |
| `"field": nilai` | Ganti dengan nilai itu |
| `"field": null` | Kosongkan |

Kirim `null` hanya berfungsi untuk field yang memang boleh kosong:
`end_date`, `end_month`, `target_date`, `product_url`, dan `amount` pada
quick-add. Mengirim `null` ke field wajib akan ditolak.

Contoh — mengubah budget jadi berlaku selamanya:

```json
PATCH /budgets/{id}
{ "end_month": null }
```

---

## 5. Health

```
GET /health
```

```json
{ "data": { "status": "ok", "db": "ok", "version": "2.0.0" } }
```

Balas `503` dengan `"status": "degraded"` dan `"db": "down"` kalau database
tidak bisa dihubungi.

Endpoint ini **ikut butuh `X-API-Token`** seperti yang lain. Jadi kalau dipakai
sebagai uptime check dari luar, token tetap harus disertakan.

---

## 6. Wallets

Dompet: Cash, BCA, GoPay, Dana, dan sejenisnya.

```
GET    /wallets
POST   /wallets
GET    /wallets/{id}
PATCH  /wallets/{id}
DELETE /wallets/{id}
POST   /wallets/{id}/restore
```

**Query param `GET /wallets`:** `limit` (default 10, maks 200), `offset`
(default 0), `include_deleted` (default false).

**Bentuk wallet:**

```json
{
  "id": "0192a1b2-c3d4-4e5f-8a9b-0c1d2e3f4a5b",
  "name": "BCA",
  "type": "bank",
  "initial_balance": 5000000,
  "current_balance": 3450000,
  "transaction_count": 87,
  "color": "#3B82F6",
  "icon": "bank",
  "is_excluded_from_total": false,
  "sort_order": 0,
  "created_at": "2026-08-01T10:00:00+08:00",
  "updated_at": "2026-08-01T10:00:00+08:00",
  "deleted_at": null
}
```

`current_balance` dan `transaction_count` **dihitung ulang setiap kali dibaca**
dari saldo awal plus seluruh transaksi. Tidak perlu (dan tidak bisa) dikirim
saat create atau update. Setelah membuat transaksi, ambil ulang wallet-nya
untuk mendapat saldo terbaru.

`is_excluded_from_total: true` berarti wallet ini tidak ikut dihitung di
`total_balance` pada endpoint summary — untuk deposito atau dana darurat.

**Create** (`POST /wallets`) — semua field di bawah wajib kecuali dua yang terakhir:

```json
{
  "name": "BCA",
  "type": "bank",
  "initial_balance": 5000000,
  "color": "#3B82F6",
  "icon": "bank",
  "is_excluded_from_total": false
}
```

- `type` harus salah satu dari: `cash`, `bank`, `ewallet`, `savings`, `other`
- `color` **wajib** dan harus hex 6 digit dengan `#` di depan, contoh `#3B82F6`
- `icon` **wajib**, string bebas — frontend yang menentukan artinya
- `initial_balance` tidak boleh negatif
- Nama tidak boleh sama dengan wallet lain yang masih aktif (409)

**Delete** (`DELETE /wallets/{id}`) memakai bentuk response berbeda:

```json
{
  "data": {
    "id": "0192...",
    "name": "BCA",
    "deleted_at": "2026-09-07T12:08:33+08:00",
    "affected_transactions": 87
  }
}
```

Delete selalu berhasil karena sifatnya soft delete. `affected_transactions`
adalah jumlah transaksi yang menunjuk wallet ini. **Pakai angka ini untuk dialog
konfirmasi sebelum menghapus** — misalnya "Wallet ini punya 87 transaksi.
Riwayatnya tetap tersimpan. Hapus?"

---

## 7. Categories

```
GET    /categories
POST   /categories
GET    /categories/{id}
PATCH  /categories/{id}
DELETE /categories/{id}
POST   /categories/{id}/restore
```

**Query param `GET /categories`:** `type` (`income` / `expense`), `limit`
(default 10, maks 200), `offset`, `include_deleted`.

```json
{
  "id": "0192...",
  "name": "Makan & Minum",
  "type": "expense",
  "color": "#EF4444",
  "icon": "utensils",
  "is_default": true,
  "sort_order": 1,
  "created_at": "2026-08-01T10:00:00+08:00",
  "updated_at": "2026-08-01T10:00:00+08:00",
  "deleted_at": null
}
```

**Create** — `name`, `type`, `color` (hex), `icon` semuanya wajib. `type` harus
`income` atau `expense`.

Kategori default (`is_default: true`) sudah di-seed backend saat migration:
delapan expense (Makan & Minum, Transport, Belanja, Tagihan, Hiburan, Kesehatan,
Pendidikan, Lainnya) dan lima income (Gaji, Freelance, Bonus, Hadiah, Lainnya).
Boleh dihapus user dan tidak akan muncul lagi.

Nama kategori boleh sama antara income dan expense — "Lainnya" ada di keduanya.
Yang tidak boleh adalah nama sama dengan tipe sama.

---

## 8. Transactions

```
GET    /transactions
POST   /transactions
GET    /transactions/{id}
PATCH  /transactions/{id}
DELETE /transactions/{id}
POST   /transactions/{id}/restore
```

### Query param `GET /transactions`

| Param | Tipe | Default | Keterangan |
|---|---|---|---|
| `from` | `YYYY-MM-DD` | awal bulan berjalan | inklusif |
| `to` | `YYYY-MM-DD` | hari ini | inklusif |
| `type` | string | semua | `income`, `expense`, atau `transfer` |
| `category_id` | UUID, boleh CSV | semua | `?category_id=id1,id2` |
| `wallet_id` | UUID, boleh CSV | semua | ikut menangkap transfer **masuk** ke wallet itu |
| `min_amount` | int | | |
| `max_amount` | int | | |
| `q` | string | | cari di `note`, tidak peka huruf besar kecil |
| `sort` | string | `occurred_at:desc` | `occurred_at`, `amount`, atau `created_at`, diikuti `:asc` / `:desc` |
| `limit` | int | 50 | maksimal 200 |
| `offset` | int | 0 | |

Nilai `sort` yang tidak dikenal tidak menghasilkan error, hanya jatuh ke urutan
default.

### Bentuk transaksi

Wallet dan kategori sudah ikut di-expand, jadi tidak perlu request tambahan:

```json
{
  "id": "0192...",
  "type": "expense",
  "amount": 25000,
  "note": "Kopi susu",
  "occurred_at": "2026-09-07T09:30:00+08:00",
  "wallet":   { "id": "0192...", "name": "Cash", "is_deleted": false },
  "category": { "id": "0192...", "name": "Makan & Minum", "type": "expense", "is_deleted": false },
  "to_wallet": null,
  "deleted_at": null
}
```

### Create

```json
POST /transactions
{
  "type": "expense",
  "amount": 25000,
  "wallet_id": "0192...",
  "category_id": "0192...",
  "note": "Kopi susu",
  "occurred_at": "2026-09-07T09:30:00+08:00"
}
```

**Aturan yang ditegakkan backend** — pahami ini supaya form-nya benar sejak awal:

1. `amount` harus lebih besar dari 0. Arah uang ditentukan `type`, **bukan tanda
   minus**. Pengeluaran tetap dikirim sebagai angka positif
2. `type` `income` atau `expense` → `category_id` **wajib**, `to_wallet_id` harus
   kosong
3. `type` `transfer` → `to_wallet_id` **wajib**, `category_id` harus kosong, dan
   `to_wallet_id` tidak boleh sama dengan `wallet_id`
4. Tipe kategori harus cocok dengan tipe transaksi. Kategori `income` tidak bisa
   dipakai untuk transaksi `expense` — **filter dropdown kategori berdasarkan tipe
   yang sedang dipilih**
5. `occurred_at` boleh mundur sejauh apapun, tapi **maksimal 1 hari ke depan**.
   Batasi date picker sampai besok
6. Wallet dan kategori yang sudah dihapus ditolak (404)
7. Saldo wallet boleh minus. Backend tidak akan memblokir pengeluaran yang
   melebihi saldo — ini aplikasi pencatatan, bukan sistem pembayaran

Transfer dibuat lewat endpoint yang sama:

```json
{
  "type": "transfer",
  "amount": 500000,
  "wallet_id": "<asal>",
  "to_wallet_id": "<tujuan>",
  "occurred_at": "2026-09-07T09:30:00+08:00"
}
```

Transfer **tidak** dihitung sebagai pemasukan maupun pengeluaran di summary,
analytics, dan savings — tapi tetap mengubah saldo kedua wallet.

### Patch

Semua field opsional. Backend menggabungkan dengan data lama lalu memeriksa
ulang seluruh aturan di atas. Mengubah `type` dari/ke `transfer` otomatis
membersihkan field lawannya.

---

## 9. Summary (Dashboard)

```
GET /summary?from=2026-08-01&to=2026-08-31
```

Default: bulan berjalan sampai hari ini.

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

`total_balance` adalah saldo seluruh wallet aktif yang tidak dikecualikan —
**tidak dibatasi periode**, beda dengan angka lain di response ini.

`comparison` membandingkan dengan periode sebelumnya yang panjangnya sama.
Bernilai `0` kalau periode pembanding tidak punya data, bukan `null` dan bukan
`Infinity`.

---

## 10. Analytics

Semua endpoint di bawah menerima `from` dan `to`, default bulan berjalan
(kecuali `trend`, default 6 bulan terakhir). Response berupa array polos di
dalam `data`, tanpa `meta`.

```
GET /analytics/by-category?type=expense
```

```json
{
  "data": [
    {
      "category_id": "0192...",
      "category_name": "Makan & Minum",
      "category_type": "expense",
      "is_deleted": false,
      "total": 1850000,
      "count": 42,
      "percentage": 43.7
    }
  ]
}
```

Diurutkan dari yang terbesar. `type` opsional (`income` / `expense`); kalau
dikosongkan, keduanya ikut. `percentage` dihitung terhadap total seluruh baris
di response ini, jadi selalu berjumlah 100.

```
GET /analytics/by-wallet
```

```json
{
  "data": [
    { "wallet_id": "0192...", "wallet_name": "Cash", "is_deleted": false,
      "income": 5000000, "expense": 200000, "net": 4800000, "count": 2 }
  ]
}
```

Semua wallet aktif ikut muncul, termasuk yang belum punya transaksi (nilainya 0).

```
GET /analytics/trend?granularity=month&from=2026-07-01&to=2026-09-30
```

`granularity`: `day`, `week`, atau `month` (default `month`).

```json
{
  "data": [
    { "bucket": "2026-07", "start": "2026-07-01", "end": "2026-07-31",
      "income": 0, "expense": 80000, "net": -80000 }
  ]
}
```

**Bucket kosong tetap dikembalikan dengan nilai 0**, jadi grafik tidak bolong dan
frontend tidak perlu mengisi celah sendiri. Label bucket: `2026-09-07` (day),
`2026-W37` (week, ISO), `2026-09` (month). Minggu selalu mulai hari Senin.

---

## 11. Savings

Menjawab "berapa uang yang bisa saya tabung".

```
GET /savings/summary?period=month
GET /savings/breakdown?period=week&from=&to=
```

`period`: `week` atau `month` (default `month`).

Rentang default berbeda antara keduanya. Pada `summary`, kalau `from` dan `to`
kosong dipakai **periode berjalan** — bulan ini atau minggu ini. Pada
`breakdown`, defaultnya **6 bulan terakhir** (`period=month`) atau **12 minggu
terakhir** (`period=week`), karena gunanya memang untuk grafik deret waktu.

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

- `savable` = `income - expense` dalam periode itu. Transfer diabaikan.
  **Nilainya boleh negatif dan harus ditampilkan apa adanya** — jangan di-clamp
  ke 0, itu justru informasi paling penting saat user boros
- `target` bernilai `null` kalau belum ada target aktif untuk periode itu
- `projected_savable` adalah proyeksi sampai akhir periode kalau pola
  pengeluaran bertahan. Untuk periode yang sudah lewat, nilainya sama dengan
  `savable`
- `savings_rate` bernilai 0 kalau tidak ada pemasukan

**Breakdown** mengembalikan deret per bucket untuk grafik. Bucket kosong tetap
ikut dengan nilai 0:

```json
{
  "data": [
    { "bucket": "2026-W23", "start": "2026-06-01", "end": "2026-06-07",
      "income": 0, "expense": 620000, "savable": -620000, "savings_rate": 0 }
  ]
}
```

### Target menabung

```
GET    /savings/targets
POST   /savings/targets
PATCH  /savings/targets/{id}
DELETE /savings/targets/{id}
```

```json
POST /savings/targets
{
  "period": "monthly",
  "amount": 3000000,
  "start_date": "2026-08-01",
  "end_date": null,
  "is_active": true
}
```

- `period`: `weekly` atau `monthly`
- **Isi salah satu antara `amount` (nominal) atau `target_rate` (persen dari
  pemasukan, 0–100). Tidak boleh dua-duanya, tidak boleh kosong dua-duanya.**
  Di form, jadikan ini pilihan radio, bukan dua input bebas
- `start_date` wajib, `end_date` opsional (`null` = berlaku terus)
- Maksimal satu target aktif per periode — target kedua ditolak dengan `409`

Response target selalu memuat `amount` dan `target_rate`, salah satunya `null`.

---

## 12. Wishlist

```
GET    /wishlist?status=planned&sort=priority:desc
POST   /wishlist
GET    /wishlist/summary
GET    /wishlist/{id}
PATCH  /wishlist/{id}
DELETE /wishlist/{id}
POST   /wishlist/{id}/restore
POST   /wishlist/{id}/allocate
POST   /wishlist/{id}/purchase
```

**Query param list:** `status` (`planned` / `saving` / `purchased` /
`cancelled`), `sort` (`priority`, `estimated_price`, `target_date`, `name`,
`created_at` + `:asc` / `:desc`), `limit` (default 50, maks 200), `offset`,
`include_deleted`.

```json
{
  "data": {
    "id": "0192...",
    "name": "Keyboard mekanik",
    "estimated_price": 1200000,
    "priority": "high",
    "target_date": "2026-12-01",
    "product_url": null,
    "note": "",
    "status": "saving",
    "saved_amount": 400000,
    "remaining": 800000,
    "progress_pct": 33.33,
    "purchased_at": null,
    "purchase_transaction_id": null,
    "sort_order": 0,
    "affordability": {
      "avg_monthly_savable": 4270000,
      "months_needed": 1,
      "estimated_ready_date": "2026-10-07",
      "on_track_for_target_date": true
    },
    "created_at": "2026-09-01T10:00:00+08:00",
    "updated_at": "2026-09-07T11:00:00+08:00",
    "deleted_at": null
  }
}
```

### Affordability — perlu penanganan khusus di UI

Objek ini punya tiga keadaan yang berbeda artinya:

| Keadaan | Arti | Saran tampilan |
|---|---|---|
| `affordability: null` | Data historis belum ada satu bulan penuh | "Belum cukup data untuk memperkirakan" |
| `months_needed: null` | Kemampuan menabung 0 atau minus | "Dengan pola sekarang, belum bisa diperkirakan" |
| terisi lengkap | Perkiraan tersedia | Tampilkan tanggal dan jumlah bulan |

`avg_monthly_savable` adalah rata-rata 3 bulan terakhir **yang sudah lengkap** —
bulan berjalan sengaja tidak ikut karena datanya belum penuh.

### Alokasi dana

```json
POST /wishlist/{id}/allocate
{ "amount": 400000 }
```

Menambah `saved_amount`. Status `planned` otomatis berubah jadi `saving`.

- Total `saved_amount` tidak boleh melebihi `estimated_price` → `400`
- Item berstatus `purchased` atau `cancelled` tidak bisa dialokasi → `422`

### Pembelian

```json
POST /wishlist/{id}/purchase
{
  "wallet_id": "0192...",
  "category_id": "0192...",
  "actual_price": 1150000,
  "occurred_at": "2026-09-15T10:00:00+08:00",
  "note": ""
}
```

Satu request ini melakukan dua hal sekaligus dalam satu database transaction:
membuat transaksi expense baru, dan menandai item jadi `purchased`. Keduanya
berhasil atau keduanya batal — **tidak perlu memanggil `POST /transactions`
secara terpisah.**

- `category_id` harus bertipe `expense` → kalau tidak, `400`
- `occurred_at` opsional, default sekarang
- `note` opsional, default "Pembelian wishlist: <nama item>"
- Item yang sudah `purchased` tidak bisa dibeli lagi → `422`

Setelah berhasil, `purchase_transaction_id` terisi id transaksi yang dibuat.

Status `purchased` **tidak bisa di-set lewat PATCH** — harus lewat endpoint ini,
supaya transaksinya selalu ikut tercatat. Harga item yang sudah dibeli juga
tidak bisa diubah (`422`); ubah dulu statusnya ke `planned`.

### Ringkasan

```
GET /wishlist/summary
```

```json
{
  "data": {
    "total_items": 7,
    "total_estimated": 8400000,
    "total_saved": 1200000,
    "by_priority": { "high": 2, "medium": 3, "low": 2 }
  }
}
```

Item berstatus `cancelled` tidak ikut dihitung.

---

## 13. Budgets

```
GET    /budgets
POST   /budgets
PATCH  /budgets/{id}
DELETE /budgets/{id}
GET    /budgets/status?month=2026-08
```

```json
POST /budgets
{
  "category_id": "0192...",
  "amount": 2000000,
  "period": "monthly",
  "start_month": "2026-08",
  "end_month": null
}
```

- Hanya untuk kategori bertipe `expense` → kalau tidak, `400`
- `period`: `weekly` atau `monthly` (default `monthly`)
- `start_month` dan `end_month` menerima `YYYY-MM` maupun `YYYY-MM-DD`, keduanya
  dinormalkan ke tanggal 1
- Satu kategori hanya boleh punya satu budget yang masih berjalan
  (`end_month` kosong) → yang kedua ditolak `409`

**Status budget** untuk progress bar:

```json
GET /budgets/status?month=2026-08

{
  "data": [
    {
      "budget_id": "0192...",
      "category_id": "0192...",
      "category_name": "Makan & Minum",
      "limit": 2000000,
      "spent": 1850000,
      "remaining": 150000,
      "percentage": 92.5,
      "status": "warning",
      "days_left": 20,
      "period": "monthly",
      "from": "2026-08-01",
      "to": "2026-08-31"
    }
  ]
}
```

- `status`: `safe` (di bawah 75%), `warning` (75–100%), `exceeded` (di atas 100%)
- `remaining` **bisa negatif** kalau melewati batas — tampilkan apa adanya
- `month` opsional, default bulan berjalan
- Budget mingguan dinilai untuk satu minggu: minggu berjalan kalau bulan yang
  diminta adalah bulan sekarang, atau minggu terakhir bulan itu kalau bulan
  lampau. `from` dan `to` di response memberi tahu rentang yang dipakai

---

## 14. Recurring Rules

Transaksi berulang. Backend punya scheduler yang jalan tiap jam dan membuat
transaksinya otomatis — frontend hanya mengatur aturannya.

```
GET    /recurring
POST   /recurring
GET    /recurring/{id}
PATCH  /recurring/{id}
DELETE /recurring/{id}
POST   /recurring/{id}/toggle
POST   /recurring/run
```

```json
POST /recurring
{
  "name": "Langganan streaming",
  "type": "expense",
  "amount": 50000,
  "wallet_id": "0192...",
  "category_id": "0192...",
  "note": "",
  "frequency": "monthly",
  "interval": 1,
  "day_of_month": 31,
  "day_of_week": null,
  "start_date": "2026-01-31",
  "end_date": null,
  "is_active": true
}
```

- `type`: `income` atau `expense`, harus cocok dengan tipe kategori
- `frequency`: `daily`, `weekly`, `monthly`, `yearly`
- `interval`: tiap berapa kali frequency, minimal 1. `frequency: weekly` +
  `interval: 2` berarti dua minggu sekali
- `day_of_month` (1–31) hanya untuk `monthly`, `day_of_week` (0 = Minggu sampai
  6 = Sabtu) hanya untuk `weekly`. Mengirim yang tidak sesuai frequency ditolak
- **`day_of_month: 31` di bulan yang tidak punya tanggal 31 otomatis jatuh ke
  hari terakhir bulan itu.** Februari jadi tanggal 28 atau 29, April jadi 30

Response memuat `next_run_at` dan `last_run_at` (`YYYY-MM-DD`, `last_run_at`
bernilai `null` sebelum pernah jalan) — bagus untuk ditampilkan sebagai
"jadwal berikutnya".

`POST /recurring/{id}/toggle` membalik `is_active`, tidak butuh body.

`POST /recurring/run` memicu scheduler manual, berguna untuk testing tanpa
menunggu satu jam:

```json
{
  "data": { "rules_checked": 3, "created": 8, "skipped": 2,
            "rules_failed": 0, "failed_rule_ids": [] }
}
```

Aman dipanggil berkali-kali — transaksi yang sudah dibuat tidak akan ganda.

Menghapus rule **tidak** menghapus transaksi yang sudah terlanjur dibuat.

---

## 15. Quick Adds

Tombol pintasan untuk transaksi yang sering diulang.

```
GET    /quick-adds
POST   /quick-adds
PATCH  /quick-adds/{id}
DELETE /quick-adds/{id}
POST   /quick-adds/{id}/execute
```

```json
POST /quick-adds
{
  "label": "Kopi",
  "type": "expense",
  "amount": 25000,
  "wallet_id": "0192...",
  "category_id": "0192...",
  "note": "kopi susu"
}
```

`amount` boleh `null`, artinya nominalnya diisi manual tiap kali dipakai.
Tipe kategori harus cocok dengan `type`. Label tidak boleh duplikat (`409`).

**Menjalankan:**

```json
POST /quick-adds/{id}/execute
{ }
```

Body boleh kosong. Field opsional yang bisa dikirim: `amount` (menimpa nominal
bawaan, **wajib** kalau quick-add-nya tidak punya nominal), `note`, `occurred_at`.

```json
HTTP 201
{
  "data": {
    "transaction_id": "0192...",
    "type": "expense",
    "amount": 25000,
    "note": "kopi susu",
    "occurred_at": "2026-09-07T12:07:09+08:00"
  }
}
```

Transaksi dibuat lewat jalur yang sama dengan `POST /transactions`, jadi seluruh
aturan bisnis transaksi tetap berlaku.

---

## 16. Export dan Import

```
GET  /export?format=json&from=&to=
GET  /export?format=csv&from=&to=
POST /import?dry_run=true
```

`format` default `json`, rentang default setahun terakhir. Response memakai
header `Content-Disposition: attachment`, jadi bisa langsung dijadikan link
unduhan.

Export JSON memuat wallets, categories, dan transactions sekaligus. Export CSV
berkolom `date,type,amount,wallet,category,note`.

**Import** memakai `multipart/form-data` dengan field bernama `file`, maksimal
10 MB, hanya menerima `.csv`.

```js
const form = new FormData()
form.append("file", file)

await fetch("/api/v1/import?dry_run=true", {
  method: "POST",
  headers: { "X-API-Token": token },   // jangan set Content-Type manual
  body: form,
})
```

```json
{
  "data": {
    "dry_run": true,
    "total_rows": 3,
    "valid_rows": 1,
    "invalid_rows": 2,
    "imported": 0,
    "errors": [
      { "line": 3, "message": "amount harus angka" },
      { "line": 4, "message": "wallet \"Dompet Hantu\" tidak ditemukan" }
    ]
  }
}
```

- **`dry_run` default `true`.** Harus dikirim `dry_run=false` secara eksplisit
  untuk benar-benar menulis. Alur yang disarankan: dry run dulu, tampilkan
  daftar error ke user, baru jalankan yang sungguhan
- `line` menunjuk nomor baris di file (baris 1 adalah header), jadi bisa
  ditampilkan langsung ke user
- Import sungguhan berjalan dalam satu database transaction. **Kalau masih ada
  satu saja baris yang salah, tidak ada satupun yang masuk** dan `imported`
  bernilai 0
- Wallet dan kategori dicocokkan berdasarkan **nama**, tidak peka huruf besar
  kecil. Nama yang tidak ada akan ditolak, bukan dibuat otomatis
- Baris `transfer` tidak didukung lewat CSV karena butuh dua wallet

Backup penuh database bukan lewat API, tapi lewat `pg_dump` di sisi server.

---

## 17. Catatan Praktis

**Warna dan ikon ada di backend.** Field `color` (hex) dan `icon` (string bebas)
tersimpan di wallet dan kategori, dan **wajib diisi** saat create. Frontend yang
menentukan arti string `icon` — misalnya memetakannya ke nama ikon di icon set
yang dipakai. Sediakan color picker dan icon picker di form.

**Jangan cache saldo.** `current_balance` berubah setiap ada transaksi yang
menyentuh wallet itu, termasuk transfer dari wallet lain. Ambil ulang setelah
operasi tulis apapun.

**Urutan pemanggilan yang wajar saat aplikasi dibuka:**

1. `GET /wallets` dan `GET /categories` — untuk isi dropdown, cache di state
2. `GET /summary` — angka besar di dashboard
3. `GET /transactions` — daftar terbaru
4. Sisanya sesuai halaman yang dibuka

**Tangani `details` pada error 400.** Isinya peta field ke pesan, siap
ditempelkan ke input yang bersangkutan tanpa parsing tambahan.

**Angka negatif itu fitur, bukan bug.** `savable`, `net`, dan `remaining` pada
budget memang bisa minus. Tampilkan dengan warna berbeda, jangan disembunyikan
atau dijadikan nol.

Kalau ada endpoint yang perilakunya berbeda dari dokumen ini, kabari — berarti
dokumennya yang perlu diperbaiki.
