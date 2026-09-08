-- Seed kategori default. Dijalankan sekali lewat migration, jadi kategori default
-- yang sengaja dihapus user tidak akan muncul lagi tiap restart.
INSERT INTO categories (name, type, is_default, sort_order)
VALUES
    ('Makan & Minum', 'expense', TRUE, 1),
    ('Transport',     'expense', TRUE, 2),
    ('Belanja',       'expense', TRUE, 3),
    ('Tagihan',       'expense', TRUE, 4),
    ('Hiburan',       'expense', TRUE, 5),
    ('Kesehatan',     'expense', TRUE, 6),
    ('Pendidikan',    'expense', TRUE, 7),
    ('Lainnya',       'expense', TRUE, 8),
    ('Gaji',          'income',  TRUE, 1),
    ('Freelance',     'income',  TRUE, 2),
    ('Bonus',         'income',  TRUE, 3),
    ('Hadiah',        'income',  TRUE, 4),
    ('Lainnya',       'income',  TRUE, 5)
ON CONFLICT DO NOTHING;
