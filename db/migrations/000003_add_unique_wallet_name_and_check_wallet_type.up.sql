ALTER TABLE wallets 
ADD CONSTRAINT wallets_type_check 
CHECK (type IN ('cash', 'bank', 'ewallet', 'savings', 'other'));

CREATE UNIQUE INDEX wallets_name_unique 
ON wallets (lower(name))
WHERE deleted_at IS NULL;