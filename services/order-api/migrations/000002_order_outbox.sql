ALTER TABLE orders ADD COLUMN IF NOT EXISTS failure_reason varchar(255);
ALTER TABLE orders ALTER COLUMN payment_id SET DEFAULT 0;
CREATE TABLE IF NOT EXISTS order_outbox (
 id bigserial PRIMARY KEY, aggregate_id bigint NOT NULL, topic varchar(255) NOT NULL,
 event_key varchar(255) NOT NULL, payload jsonb NOT NULL, published_at timestamptz,
 attempts integer NOT NULL DEFAULT 0, last_error text, created_at timestamptz
);
CREATE INDEX IF NOT EXISTS idx_order_outbox_unpublished ON order_outbox(id) WHERE published_at IS NULL;
