ALTER TABLE idempotency_keys
    ALTER COLUMN response_body TYPE jsonb USING NULLIF(response_body,'')::jsonb;
