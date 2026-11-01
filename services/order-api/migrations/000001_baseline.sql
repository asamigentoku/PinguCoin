CREATE TABLE IF NOT EXISTS orders (
 id bigserial PRIMARY KEY, user_id bigint NOT NULL, product_id bigint NOT NULL,
 quantity bigint NOT NULL, unit_price bigint NOT NULL, total_amount bigint NOT NULL,
 payment_id bigint NOT NULL DEFAULT 0, status varchar(20) NOT NULL DEFAULT 'processing',
 idempotency_key varchar(255) NOT NULL, failure_reason varchar(255),
 created_at timestamptz, updated_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_idempotency_key ON orders(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_product_id ON orders(product_id);
CREATE TABLE IF NOT EXISTS order_outbox (
 id bigserial PRIMARY KEY, aggregate_id bigint NOT NULL, topic varchar(255) NOT NULL,
 event_key varchar(255) NOT NULL, payload jsonb NOT NULL, published_at timestamptz,
 attempts integer NOT NULL DEFAULT 0, last_error text, created_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_order_outbox_unpublished ON order_outbox(id) WHERE published_at IS NULL;
