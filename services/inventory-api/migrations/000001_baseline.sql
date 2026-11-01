CREATE TABLE IF NOT EXISTS product_inventory (
    id bigserial PRIMARY KEY,
    product_id bigint NOT NULL,
    quantity bigint NOT NULL,
    reserved bigint NOT NULL DEFAULT 0,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz,
    updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_inventory_product_id ON product_inventory(product_id);
CREATE TABLE IF NOT EXISTS product_inventory_transactions (
    id bigserial PRIMARY KEY,
    product_id bigint NOT NULL,
    amount bigint NOT NULL,
    reason varchar(255),
    idempotency_key varchar(255) NOT NULL,
    quantity_after bigint NOT NULL,
    created_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_product_inventory_transactions_idempotency_key ON product_inventory_transactions(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_product_inventory_transactions_product_id ON product_inventory_transactions(product_id);
