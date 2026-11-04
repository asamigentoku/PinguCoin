-- Add a per-user, SHA-256 linked hash chain to the point ledger.
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;

ALTER TABLE point_transactions
    ADD COLUMN chain_sequence bigint,
    ADD COLUMN previous_hash varchar(64),
    ADD COLUMN transaction_hash varchar(64);

DO $$
DECLARE
    entry record;
    active_user bigint := -1;
    next_sequence bigint := 0;
    previous_digest text := repeat('0', 64);
    canonical bytea;
    current_digest text;
BEGIN
    FOR entry IN
        SELECT id, user_id, amount, type, payment_id, COALESCE(reason, '') AS reason, balance_after
        FROM point_transactions
        ORDER BY user_id, id
    LOOP
        IF entry.user_id <> active_user THEN
            active_user := entry.user_id;
            next_sequence := 1;
            previous_digest := repeat('0', 64);
        ELSE
            next_sequence := next_sequence + 1;
        END IF;

        canonical := convert_to('PinguCoin.PointTransaction.v1', 'UTF8')
            || int8send(next_sequence)
            || int8send(entry.user_id)
            || int8send(entry.amount)
            || int4send(octet_length(entry.type)) || convert_to(entry.type, 'UTF8')
            || CASE WHEN entry.payment_id IS NULL
                THEN decode('00', 'hex')
                ELSE decode('01', 'hex') || int8send(entry.payment_id)
               END
            || int4send(octet_length(entry.reason)) || convert_to(entry.reason, 'UTF8')
            || int8send(entry.balance_after)
            || decode(previous_digest, 'hex');
        current_digest := encode(public.digest(canonical, 'sha256'), 'hex');

        UPDATE point_transactions
        SET chain_sequence = next_sequence,
            previous_hash = previous_digest,
            transaction_hash = current_digest
        WHERE id = entry.id;

        previous_digest := current_digest;
    END LOOP;
END $$;

ALTER TABLE point_transactions
    ALTER COLUMN chain_sequence SET NOT NULL,
    ALTER COLUMN previous_hash SET NOT NULL,
    ALTER COLUMN transaction_hash SET NOT NULL;

CREATE UNIQUE INDEX idx_point_transactions_user_chain
    ON point_transactions (user_id, chain_sequence);
CREATE UNIQUE INDEX idx_point_transactions_hash
    ON point_transactions (transaction_hash);

CREATE FUNCTION reject_point_transaction_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'point_transactions is append-only';
END;
$$;

CREATE TRIGGER point_transactions_append_only
BEFORE UPDATE OR DELETE ON point_transactions
FOR EACH ROW EXECUTE FUNCTION reject_point_transaction_mutation();
