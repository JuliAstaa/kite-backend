CREATE UNIQUE INDEX idx_categories_unique_name_type 
ON categories (lower(name), type) 
WHERE deleted_at IS NULL;