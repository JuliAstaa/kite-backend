CREATE TABLE recurring_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN('income', 'expense')),
    amount BIGINT NOT NULL,
    wallet_id UUID NOT NULL REFERENCES wallets(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    note TEXT NOT NULL DEFAULT '',
    frequency TEXT NOT NULL CHECK(frequency IN('daily', 'weekly', 'monthly', 'yearly')),
    interval INTEGER NOT NULL DEFAULT 1,
    day_of_month SMALLINT,
    day_of_week SMALLINT,
    start_date DATE NOT NULL,
    end_date DATE,
    next_run_at DATE NOT NULL,
    last_run_at DATE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE wishlist_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    estimated_price BIGINT NOT NULL CHECK(estimated_price > 0),
    priority TEXT NOT NULL CHECK (priority IN('low', 'high', 'medium')) DEFAULT 'medium',
    target_date DATE,
    product_url TEXT,
    note TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN('planned', 'saving', 'purchased', 'cancelled')) DEFAULT 'planned',
    saved_amount BIGINT NOT NULL DEFAULT 0,
    purchased_at TIMESTAMPTZ,
    purchase_transaction_id UUID REFERENCES transactions(id),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

ALTER TABLE transactions
    ADD COLUMN recurring_rule_id UUID REFERENCES recurring_rules(id),
    ADD COLUMN wishlist_item_id UUID REFERENCES wishlist_items(id);

CREATE INDEX ON transactions (occurred_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX ON transactions (type, occurred_at) WHERE deleted_at IS NULL;
CREATE INDEX ON transactions (category_id) WHERE deleted_at IS NULL;
CREATE INDEX ON transactions (wallet_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX ON transactions (recurring_rule_id, occurred_at)
    WHERE recurring_rule_id IS NOT NULL AND deleted_at IS NULL;

