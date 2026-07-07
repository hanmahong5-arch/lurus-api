-- ============================================================================
-- DEMO seed — tenant-level PRIVATE INFERENCE ROUTING.
--
-- Proves the "data never leaves the intranet" pitch with ZERO business-code
-- change: one tenant (privacy-demo) whose ONLY channel for the model is an
-- on-prem OpenAI-compatible endpoint (ChannelTypeCustom=8, base_url on the
-- loopback). A relay request carrying this tenant's token is dispatched to the
-- private endpoint and NO external provider is reachable — on a fresh DB this
-- is the only channel that exists, so the routing is provable by construction.
--
-- Pure config: a tenants row + a user + a token + a Custom channel + one
-- ability row. No schema change, no code change.
--
-- Apply against the demo Postgres AFTER the backend has auto-migrated once:
--   psql "postgres://postgres:postgres@localhost:5435/newhub" \
--     -f demo/private-inference/seed-tenant-private-endpoint.sql
-- ============================================================================

-- 0) Demo convenience: enable "self-use mode" so the relay does not require a
--    per-model price/ratio to be configured (a real paid deployment would set
--    model pricing instead). Loaded into operation_setting on next boot.
INSERT INTO options (key, value) VALUES ('SelfUseModeEnabled', 'true')
ON CONFLICT (key) DO UPDATE SET value = 'true';

-- 1) The customer tenant.
INSERT INTO tenants (id, zitadel_org_id, slug, name, status, plan_type, max_users, max_quota, created_at, updated_at)
VALUES ('privacy-demo', 'privacy-demo-org', 'privacy-demo', 'Privacy Demo Corp', 1, 'enterprise', 100, 100000000000, NOW(), NOW())
ON CONFLICT (slug) DO NOTHING;

-- 2) A NORMAL (non-admin, role=1) tenant user — proves this is tenant config,
--    not an admin "specify channel" override.
-- newhub users authenticate via OIDC/passkey/session; there is no password column.
INSERT INTO users (id, tenant_id, username, role, status, quota, "group")
VALUES (9100, 'privacy-demo', 'privacy-demo-user', 1, 1, 100000000, 'default')
ON CONFLICT (id) DO NOTHING;

-- 3) A relay token for that user. Key is 48 chars, NO dashes (a dash would be
--    parsed as an admin "specify channel id" suffix). group='' => inherits the
--    user's 'default' group, side-stepping per-group ratio validation.
INSERT INTO tokens (user_id, tenant_id, key, status, name, "group", unlimited_quota, remain_quota, expired_time, created_time, accessed_time)
SELECT 9100, 'privacy-demo', 'privacydemo00000000000000000000000000onprem01', 1, 'privacy-demo relay token', '', true, 0, -1,
       EXTRACT(EPOCH FROM NOW())::bigint, EXTRACT(EPOCH FROM NOW())::bigint
WHERE NOT EXISTS (SELECT 1 FROM tokens WHERE key = 'privacydemo00000000000000000000000000onprem01');

-- 4) The on-prem private-inference channel (OpenAI-compatible, loopback base_url),
--    scoped to the tenant. type 8 = ChannelTypeCustom (internal/pkg/constant/channel.go).
INSERT INTO channels (type, key, name, base_url, models, "group", tenant_id, status, weight, priority, created_time, test_time, channel_info)
SELECT 8, 'sk-onprem-no-auth', 'Private Inference (on-prem · tenant privacy-demo)',
       'http://127.0.0.1:11434', 'qwen2.5-7b-instruct', 'default', 'privacy-demo', 1, 0, 0,
       EXTRACT(EPOCH FROM NOW())::bigint, 0, '{}'
WHERE NOT EXISTS (SELECT 1 FROM channels WHERE name = 'Private Inference (on-prem · tenant privacy-demo)');

-- 5) The ability row the relay selects on: (group, model, channel) enabled.
--    Bound to the channel just inserted (looked up by unique name).
INSERT INTO abilities ("group", model, channel_id, enabled, priority, weight)
SELECT 'default', 'qwen2.5-7b-instruct', c.id, true, 0, 0
FROM channels c
WHERE c.name = 'Private Inference (on-prem · tenant privacy-demo)'
  AND NOT EXISTS (
    SELECT 1 FROM abilities a
    WHERE a."group" = 'default' AND a.model = 'qwen2.5-7b-instruct' AND a.channel_id = c.id
  );

-- Verify
SELECT 'channel' AS kind, c.id::text, c.name, c.base_url, c.tenant_id, c.status::text
FROM channels c WHERE c.name = 'Private Inference (on-prem · tenant privacy-demo)'
UNION ALL
SELECT 'ability', a.channel_id::text, a."group", a.model, '', a.enabled::text
FROM abilities a WHERE a.model = 'qwen2.5-7b-instruct'
UNION ALL
SELECT 'token', t.id::text, t.name, t.tenant_id, t."group", t.status::text
FROM tokens t WHERE t.tenant_id = 'privacy-demo';
