BEGIN;
CREATE TABLE oauth_states (
    state      text PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
COMMIT;
