-- Tenara control plane schema v1 (plan tenara-agent-paas#9, RB-7).
BEGIN;

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email                   citext UNIQUE NOT NULL,
    password_hash           text NOT NULL, -- argon2id encoded hash
    email_verified          boolean NOT NULL DEFAULT false,
    suspended               boolean NOT NULL DEFAULT false,
    github_token_encrypted  bytea,
    github_bound_at         timestamptz,
    github_username         text,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organizations (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    slug        citext UNIQUE NOT NULL,
    tier        text NOT NULL DEFAULT 'free' CHECK (tier IN ('free','pro')),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE organization_members (
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    org_id  uuid NOT NULL REFERENCES organizations ON DELETE CASCADE,
    role    text NOT NULL CHECK (role IN ('platform_admin','workspace_admin','member')),
    PRIMARY KEY (user_id, org_id)
);

CREATE INDEX idx_org_members_org ON organization_members (org_id);

CREATE TABLE applications (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      uuid NOT NULL REFERENCES organizations,
    name        text NOT NULL,
    slug        text NOT NULL,
    created_by  uuid REFERENCES users,
    deleted_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (org_id, name)
);

CREATE INDEX idx_apps_org ON applications (org_id) WHERE deleted_at IS NULL;

CREATE TABLE services (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    name       text NOT NULL,
    type       text NOT NULL CHECK (type IN ('frontend','backend')),
    runtime    text NOT NULL CHECK (runtime IN ('node','python','go')),
    port       integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (app_id, name)
);

CREATE TABLE environments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    name            text NOT NULL,
    namespace_name  text UNIQUE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (app_id, name)
);

CREATE TABLE builds (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id           uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    environment_name text NOT NULL DEFAULT 'production',
    repo_url         text NOT NULL,
    git_sha          text,
    token_ref        text,
    phase            text NOT NULL DEFAULT 'CREATED'
                     CHECK (phase IN ('CREATED','CLONING','BUILDING','SCANNING','SCAN_FAILED','SIGNING','PUSHED','FAILED')),
    image_digest     text,
    sbom_ref         text,
    scan_report_ref  text,
    signature_ref    text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_builds_app ON builds (app_id, created_at DESC);

CREATE TABLE artifacts (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    build_id    uuid NOT NULL REFERENCES builds ON DELETE CASCADE,
    kind        text NOT NULL CHECK (kind IN ('sbom','scan_report','signature')),
    storage_uri text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE deployments (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id          uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    environment_id  uuid NOT NULL REFERENCES environments,
    state           text NOT NULL DEFAULT 'CREATED'
                    CHECK (state IN ('CREATED','AWAITING_APPROVAL','PLANNED','QUEUED','BUILDING',
                                     'DEPLOYING','VERIFYING','RUNNING','DEGRADED','FAILED','STOPPED',
                                     'ROLLING_BACK','DELETING','DELETED','RESTORED')),
    plan_id         uuid,
    plan_snapshot   jsonb,
    delete_steps    jsonb,
    git_sha         text,
    image_digest    text,
    revision        integer NOT NULL DEFAULT 0,
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_deployments_app ON deployments (app_id, created_at DESC);
CREATE INDEX idx_deployments_state ON deployments (state) WHERE state NOT IN ('RUNNING','DELETED');

CREATE TABLE deployment_revisions (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id   uuid NOT NULL REFERENCES deployments ON DELETE CASCADE,
    revision        integer NOT NULL,
    git_sha         text,
    build_id        uuid REFERENCES builds,
    image_digest    text NOT NULL CHECK (image_digest ~ '^sha256:[0-9a-f]{64}$'),
    config_version  integer NOT NULL DEFAULT 1,
    secret_revision integer NOT NULL DEFAULT 1,
    appspec_version text NOT NULL DEFAULT 'v1',
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (deployment_id, revision)
);

CREATE TABLE domains (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id        uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    hostname      citext UNIQUE NOT NULL,
    verified      boolean NOT NULL DEFAULT false,
    txt_challenge text,
    cname_target  text,
    is_default    boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE databases (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    type       text NOT NULL CHECK (type IN ('mongodb')),
    isolation  text NOT NULL DEFAULT 'shared' CHECK (isolation IN ('shared','dedicated')),
    state      text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','ready','failed')),
    provider   text NOT NULL DEFAULT 'local',
    db_name    text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (app_id, type)
);

CREATE TABLE database_bindings (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    database_id          uuid NOT NULL REFERENCES databases ON DELETE CASCADE,
    app_id               uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    credential_secret_ref text,
    state                text NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','ready','failed')),
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE secrets (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id      uuid NOT NULL REFERENCES applications ON DELETE CASCADE,
    name        text NOT NULL,
    ciphertext  bytea NOT NULL, -- AES-256-GCM payload incl. nonce + version header
    key_version integer NOT NULL DEFAULT 1,
    created_by  uuid REFERENCES users,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (app_id, name)
);

CREATE TABLE secret_revisions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    secret_id  uuid NOT NULL REFERENCES secrets ON DELETE CASCADE,
    version    integer NOT NULL,
    ciphertext bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (secret_id, version)
);

CREATE TABLE resource_quotas (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id     uuid UNIQUE NOT NULL REFERENCES organizations,
    tier       text NOT NULL DEFAULT 'free' CHECK (tier IN ('free','pro')),
    limits     jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE usage_records (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    org_id     uuid NOT NULL REFERENCES organizations,
    app_id     uuid REFERENCES applications ON DELETE CASCADE,
    metric     text NOT NULL CHECK (metric IN ('cpu_millicores','memory_mb','storage_mb','build_minutes','db_size_mb')),
    value      double precision NOT NULL DEFAULT 0,
    sampled_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_usage_org_time ON usage_records (org_id, sampled_at DESC);

CREATE TABLE audit_logs (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_type    text NOT NULL CHECK (actor_type IN ('user','agent','admin','controller')),
    actor_id      text,
    agent         text,
    workspace_id  uuid REFERENCES organizations,
    app_id        uuid REFERENCES applications,
    action        text NOT NULL,
    source_ip     inet,
    request_id    text,
    before        jsonb,
    after         jsonb,
    result        text NOT NULL,
    occurred_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_workspace_time ON audit_logs (workspace_id, occurred_at DESC);
CREATE INDEX idx_audit_action ON audit_logs (action);

CREATE TABLE security_events (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind        text NOT NULL,
    actor_id    text,
    source_ip   inet,
    detail      jsonb,
    occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_security_kind_time ON security_events (kind, occurred_at DESC);

CREATE TABLE idempotency_keys (
    idempotency_key text NOT NULL,
    org_id          uuid NOT NULL,
    request_hash    text NOT NULL,
    response_status integer,
    response_body   jsonb,
    expires_at      timestamptz NOT NULL DEFAULT now() + interval '24 hours',
    PRIMARY KEY (idempotency_key, org_id)
);

COMMIT;
