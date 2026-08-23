-- Organization-scoped API tokens (plan tenara-agent-paas#12, R5).
BEGIN;

CREATE TABLE api_tokens (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id       uuid NOT NULL REFERENCES organizations ON DELETE CASCADE,
    created_by   uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    name         text NOT NULL,
    token_hash   text UNIQUE NOT NULL, -- sha256 hex of full plaintext
    prefix       text NOT NULL,        -- display form, e.g. tenara_ab12
    last_used_at timestamptz,
    revoked_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_tokens_org ON api_tokens (org_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_api_tokens_user ON api_tokens (created_by);

COMMIT;
