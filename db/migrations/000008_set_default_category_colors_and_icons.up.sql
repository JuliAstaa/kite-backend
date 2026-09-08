-- Isi warna dan ikon kategori default.
--
-- Migration 6 menyisipkan kategori tanpa color dan icon, jadi ketiga belasnya
-- jatuh ke default kolom: kuning '#FFD93D' dan ikon 'circle'. Akibatnya semua
-- kategori kelihatan sama di UI dan donut chart pengeluaran jadi satu warna.
--
-- Nilai di bawah diambil dari mock frontend yang dipakai sebelum backend jadi:
-- warna dari PALETTE 12 swatch (frontend/src/lib/palette.ts), nama ikon dari
-- registry lucide (frontend/src/lib/icons.ts). Nama ikon yang tidak dikenal
-- jatuh ke 'circle' di frontend, bukan crash.
--
-- Dijalankan sebagai UPDATE, bukan INSERT baru, karena kategorinya sudah ada
-- di database yang sudah jalan.
UPDATE categories c
SET color = v.color,
    icon = v.icon,
    updated_at = now()
FROM (VALUES
    ('Makan & Minum', 'expense', '#FF7A7A', 'utensils'),
    ('Transport',     'expense', '#7DD8F0', 'bus'),
    ('Belanja',       'expense', '#C9A7FF', 'shopping-bag'),
    ('Tagihan',       'expense', '#FFB84D', 'zap'),
    ('Hiburan',       'expense', '#FF9FD0', 'gamepad-2'),
    ('Kesehatan',     'expense', '#7DE28A', 'heart-pulse'),
    ('Pendidikan',    'expense', '#8FA8FF', 'graduation-cap'),
    ('Lainnya',       'expense', '#D4D4D4', 'more-horizontal'),
    ('Gaji',          'income',  '#7DE28A', 'briefcase'),
    ('Freelance',     'income',  '#7DD8F0', 'laptop'),
    ('Bonus',         'income',  '#FFD93D', 'award'),
    ('Hadiah',        'income',  '#C9A7FF', 'gift'),
    ('Lainnya',       'income',  '#D4D4D4', 'sparkles')
) AS v(name, type, color, icon)
WHERE c.name = v.name
  AND c.type = v.type
  AND c.is_default = TRUE
  -- Hanya menyentuh yang masih memakai default kolom. Kategori default yang
  -- warnanya sudah diubah user tetap dibiarkan, dan migration ini jadi aman
  -- kalau suatu saat dijalankan ulang di database yang sudah rapi.
  AND c.color = '#FFD93D'
  AND c.icon = 'circle';
