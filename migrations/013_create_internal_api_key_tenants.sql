-- Migration: internal_api_key_tenants — tenant whitelist for ScopeProvisioning keys.
--
-- Phase 2 self-audit (2026-05-19) found a cross-tenant Provisioning Create
-- hole: any holder of a ScopeProvisioning InternalApiKey could mint tokens
-- for any tenant slug because creator_user_id was used only for attribution,
-- never as a permission boundary.
--
-- This migration creates a small whitelist table. Each row authorises one
-- InternalApiKey to issue tokens for one tenant. Empty whitelist =
-- deny-all (fail-closed by design).
--
-- Followup: migration 014 (Phase 3) will replace this junction with a
-- first-class tenant_id column on internal_api_keys for keys that are
-- naturally tenant-scoped. The junction stays for platform-admin keys
-- that legitimately span multiple tenants.
--
-- Operational note: after this migration deploys, ALL Provisioning Create
-- requests return 403 until at least one whitelist row is inserted. This
-- is the intended fail-closed behaviour — operators MUST seed the table
-- before the feature can be used.

CREATE TABLE IF NOT EXISTS internal_api_key_tenants (
    api_key_id INTEGER     NOT NULL,
    tenant_id  VARCHAR(36) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (api_key_id, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_internal_api_key_tenants_tenant
    ON internal_api_key_tenants (tenant_id);
