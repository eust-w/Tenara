-- Reverse of 000001_schema_v1.up.sql
BEGIN;

DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS security_events;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS usage_records;
DROP TABLE IF EXISTS resource_quotas;
DROP TABLE IF EXISTS secret_revisions;
DROP TABLE IF EXISTS secrets;
DROP TABLE IF EXISTS database_bindings;
DROP TABLE IF EXISTS databases;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS deployment_revisions;
DROP TABLE IF EXISTS deployments;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS builds;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS services;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS users;

DROP EXTENSION IF EXISTS citext;

COMMIT;
