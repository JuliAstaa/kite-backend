-- Seed dompet awal. Sama seperti seed kategori: dijalankan sekali lewat
-- migration, jadi dompet yang sengaja dihapus user tidak muncul lagi tiap
-- restart.
--
-- Daftarnya dompet yang benar-benar dipakai: Cash, BCA, GoPay.
-- Tabel wallets tidak punya kolom is_default, jadi setelah seed jalan tidak ada
-- cara membedakan dompet bawaan dari buatan user. Itu disengaja: begitu dibuat,
-- ketiganya milik user sepenuhnya dan boleh diubah atau dihapus.
--
-- initial_balance sengaja 0. Saldo awal yang sebenarnya cuma user yang tahu,
-- dan menebak angka di sini berarti seluruh laporan keuangan salah sejak baris
-- pertama. Isi lewat PATCH /wallets/{id} setelah start.
--
-- color memakai swatch dari PALETTE frontend, icon memakai nama di registry
-- lucide frontend (src/lib/palette.ts dan src/lib/icons.ts).
--
-- ON CONFLICT DO NOTHING menyandarkan diri pada partial unique index
-- wallets_name_unique pada lower(name) WHERE deleted_at IS NULL, jadi migration
-- ini aman dijalankan di database yang sudah punya dompet dengan nama sama.
INSERT INTO wallets (name, type, initial_balance, color, icon, is_excluded_from_total, sort_order)
VALUES
    ('Cash',  'cash',    0, '#FFD93D', 'banknote',   FALSE, 1),
    ('BCA',   'bank',    0, '#8FA8FF', 'landmark',   FALSE, 2),
    ('GoPay', 'ewallet', 0, '#7DD8F0', 'smartphone', FALSE, 3)
ON CONFLICT DO NOTHING;
