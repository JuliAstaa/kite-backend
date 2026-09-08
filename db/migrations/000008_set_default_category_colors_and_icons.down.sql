-- Kembalikan kategori default ke warna dan ikon bawaan kolom, yaitu keadaan
-- sebelum migration ini jalan.
--
-- Dibatasi ke pasangan nilai yang persis dipasang migration 8, supaya kategori
-- default yang sudah diwarnai ulang oleh user tidak ikut kena.
UPDATE categories c
SET color = '#FFD93D',
    icon = 'circle',
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
  AND c.color = v.color
  AND c.icon = v.icon;
