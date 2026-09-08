CREATE TABLE budgets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES categories(id),
    amount      BIGINT NOT NULL CHECK (amount > 0),
    period      TEXT NOT NULL DEFAULT 'monthly' CHECK (period IN ('weekly', 'monthly')),
    start_month DATE NOT NULL,
    end_month   DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

-- satu kategori maksimal punya satu budget yang masih berjalan (end_month NULL)
CREATE UNIQUE INDEX budgets_active_category_unique
    ON budgets (category_id)
    WHERE deleted_at IS NULL AND end_month IS NULL;

CREATE INDEX budgets_category_idx ON budgets (category_id) WHERE deleted_at IS NULL;

CREATE TABLE savings_targets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    period      TEXT NOT NULL CHECK (period IN ('weekly', 'monthly')),
    amount      BIGINT,
    target_rate NUMERIC(5,2),
    start_date  DATE NOT NULL,
    end_date    DATE,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ,
    CONSTRAINT savings_targets_amount_xor_rate CHECK (
        (amount IS NOT NULL AND target_rate IS NULL)
        OR (amount IS NULL AND target_rate IS NOT NULL)
    ),
    CONSTRAINT savings_targets_amount_positive CHECK (amount IS NULL OR amount > 0),
    CONSTRAINT savings_targets_rate_range CHECK (target_rate IS NULL OR (target_rate > 0 AND target_rate <= 100))
);

-- maksimal satu target aktif per periode
CREATE UNIQUE INDEX savings_targets_active_period_unique
    ON savings_targets (period)
    WHERE deleted_at IS NULL AND is_active;

CREATE TABLE quick_adds (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    label       TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    amount      BIGINT CHECK (amount IS NULL OR amount > 0),
    wallet_id   UUID NOT NULL REFERENCES wallets(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    note        TEXT NOT NULL DEFAULT '',
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX quick_adds_label_unique
    ON quick_adds (lower(label))
    WHERE deleted_at IS NULL;

-- dipakai saat menghitung saldo wallet (transfer masuk)
CREATE INDEX transactions_to_wallet_idx ON transactions (to_wallet_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX wishlist_items_name_unique
    ON wishlist_items (lower(name))
    WHERE deleted_at IS NULL;
