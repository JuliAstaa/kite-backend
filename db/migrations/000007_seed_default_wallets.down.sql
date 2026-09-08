-- Hanya menghapus dompet seed yang masih perawan.
--
-- Tabel wallets tidak punya is_default, jadi satu-satunya penanda adalah nama.
-- Karena itu penghapusan dibatasi tiga syarat sekaligus: namanya persis nama
-- seed, saldo awalnya masih 0, dan belum pernah dipakai transaksi apapun.
-- Tanpa syarat terakhir, DELETE-nya juga akan gagal karena FK transaksi memang
-- sengaja tidak memakai ON DELETE CASCADE.
DELETE FROM wallets w
WHERE lower(w.name) IN ('cash', 'bca', 'gopay')
  AND w.initial_balance = 0
  AND NOT EXISTS (
      SELECT 1 FROM transactions t
      WHERE t.wallet_id = w.id OR t.to_wallet_id = w.id
  )
  AND NOT EXISTS (SELECT 1 FROM recurring_rules r WHERE r.wallet_id = w.id)
  AND NOT EXISTS (SELECT 1 FROM quick_adds q WHERE q.wallet_id = w.id);
