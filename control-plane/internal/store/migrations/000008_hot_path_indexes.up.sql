-- Hot-path composite indexes surfaced by live QA round.
CREATE INDEX IF NOT EXISTS idx_apps_org_name
    ON applications (org_id, name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_deployments_app_rev
    ON deployments (app_id, revision DESC);
CREATE INDEX IF NOT EXISTS idx_domains_app
    ON domains (app_id);
CREATE INDEX IF NOT EXISTS idx_idem_key_org
    ON idempotency_keys (idempotency_key, org_id);
