ALTER TABLE transactions
    DROP COLUMN IF EXISTS recurring_rule_id,
    DROP COLUMN IF EXISTS wishlist_item_id;

DROP TABLE IF EXISTS wishlist_items;
DROP TABLE IF EXISTS recurring_rules;