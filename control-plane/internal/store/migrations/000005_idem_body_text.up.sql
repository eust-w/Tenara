-- Byte-exact idempotent replay (R2): jsonb normalization breaks equality.
ALTER TABLE idempotency_keys
    ALTER COLUMN response_body TYPE text USING response_body::text;
