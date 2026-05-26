-- Migration 016: add retention_until column to audit_events for governance
-- retention enforcement.
--
-- Phase E3 (Enterprise Foundation): every audit event records a per-row
-- retention deadline as unix seconds. The lifecycle cleanup task
-- (internal/lifecycle/audit_cleanup.go) deletes rows where
-- retention_until <= NOW once per day. This replaces the legacy "all rows
-- forever" behavior with a configurable per-tenant policy and gives ops
-- a single column to extend or shorten the retention horizon.
--
-- Default value is 0 ("no expiry") so existing rows backfill safely; the
-- cleanup task treats 0 as "skip" so historical events stay until a
-- second backfill PR (out of scope here) assigns explicit retention_until
-- values per tenant policy. NewAuditEvent in the governance package will
-- set retention_until to Timestamp + 7 years for all new rows.
--
-- BIGINT (not TIMESTAMP) is chosen for consistency with audit_events.timestamp
-- which is already a bigint of unix seconds. Mixed temporal types in one
-- table is a footgun we avoid here.
--
-- PostgreSQL 11+: ADD COLUMN with a constant DEFAULT is metadata-only and
-- does not rewrite the table; online-safe even on large audit_events tables.
-- The new btree index supports the cleanup task's `retention_until <= ? AND
-- retention_until > 0` predicate.

ALTER TABLE audit_events
    ADD COLUMN IF NOT EXISTS retention_until BIGINT DEFAULT 0 NOT NULL;

CREATE INDEX IF NOT EXISTS idx_audit_events_retention_until
    ON audit_events (retention_until)
    WHERE retention_until > 0;
