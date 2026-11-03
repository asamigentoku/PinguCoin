ALTER TABLE payments ADD COLUMN IF NOT EXISTS purpose varchar(30) NOT NULL DEFAULT 'purchase';
ALTER TABLE payment_sources ADD COLUMN IF NOT EXISTS action_url text NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS provider_events (
    id bigserial PRIMARY KEY,
    provider varchar(30) NOT NULL,
    event_id varchar(255) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_provider_events_provider_event ON provider_events(provider, event_id);
