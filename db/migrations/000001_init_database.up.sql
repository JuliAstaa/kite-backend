CREATE TABLE wallets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    initial_balance BIGINT NOT NULL DEFAULT 0,
    color           TEXT NOT NULL DEFAULT '#FFD93D',
    icon            TEXT NOT NULL DEFAULT 'wallet',
    is_excluded_from_total BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    type       TEXT NOT NULL CHECK (type IN ('income', 'expense')),
    color      TEXT NOT NULL DEFAULT '#FFD93D',
    icon       TEXT NOT NULL DEFAULT 'circle',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE transactions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type         TEXT NOT NULL CHECK (type IN ('income', 'expense', 'transfer')),
    amount       BIGINT NOT NULL CHECK (amount > 0),
    wallet_id    UUID NOT NULL REFERENCES wallets(id),
    to_wallet_id UUID REFERENCES wallets(id),      -- diisi cuma kalau transfer
    category_id  UUID REFERENCES categories(id),   -- dikosongkan kalau transfer
    note         TEXT NOT NULL DEFAULT '',
    occurred_at  TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX transactions_occurred_at_idx ON transactions (occurred_at DESC);
CREATE INDEX transactions_wallet_idx ON transactions (wallet_id);
CREATE INDEX transactions_category_idx ON transactions (category_id);