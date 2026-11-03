CREATE TABLE payment_sources (
    id bigserial PRIMARY KEY,
    payment_id bigint NOT NULL,
    provider character varying(30) NOT NULL,
    provider_reference character varying(255) NOT NULL DEFAULT '',
    status character varying(20) NOT NULL DEFAULT 'pending',
    failure_reason text NOT NULL DEFAULT '',
    order_payload jsonb NOT NULL DEFAULT '{}',
    expires_at timestamp with time zone,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE UNIQUE INDEX idx_payment_sources_payment_id ON payment_sources (payment_id);
CREATE INDEX idx_payment_sources_provider_reference ON payment_sources (provider_reference);

CREATE TABLE outbox_events (
    id bigserial PRIMARY KEY,
    aggregate_id bigint NOT NULL,
    topic character varying(100) NOT NULL,
    key character varying(255) NOT NULL,
    payload jsonb NOT NULL,
    attempts bigint NOT NULL DEFAULT 0,
    published_at timestamp with time zone,
    created_at timestamp with time zone
);

CREATE INDEX idx_outbox_events_aggregate_id ON outbox_events (aggregate_id);
CREATE INDEX idx_outbox_events_published_at ON outbox_events (published_at);
