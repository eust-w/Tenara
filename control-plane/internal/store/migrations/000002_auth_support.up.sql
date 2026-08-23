-- Auth support tables (plan tenara-agent-paas#10).
BEGIN;

CREATE TABLE email_verifications (
    token_hash text PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    expires_at timestamptz NOT NULL DEFAULT now() + interval '24 hours',
    consumed_at timestamptz
);

CREATE TABLE password_resets (
    token_hash  text PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    expires_at  timestamptz NOT NULL DEFAULT now() + interval '1 hour',
    consumed_at timestamptz
);

CREATE TABLE signup_rate_limits (
    ip           inet PRIMARY KEY,
    window_start timestamptz NOT NULL DEFAULT now(),
    count        integer NOT NULL DEFAULT 0
);

CREATE INDEX idx_email_verifications_user ON email_verifications (user_id);
CREATE INDEX idx_password_resets_user ON password_resets (user_id);

COMMIT;
