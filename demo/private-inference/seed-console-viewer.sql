-- ============================================================================
-- ADDITIVE, OPTIONAL seed — console-viewer account for the private-inference
-- demo. This file is deliberately SEPARATE from seed-tenant-private-endpoint.sql
-- and does NOT touch it: that file's whole point is "a NORMAL (role=1,
-- non-admin) tenant user routed to a private endpoint by pure config" and
-- must keep meaning exactly that.
--
-- The newhub channel-list API (both v1 GET /api/channel and v2
-- GET /api/v2/:tenant_slug/channels) is admin-gated
-- (internal/adapter/handler/router/api-router.go, api-v2-router.go —
-- middleware.AdminAuth() / requireTenantAdmin). A role=1 user can never call
-- it. To let shot-console.mjs render a live "here is this tenant's routing"
-- panel, this file adds ONE SEPARATE admin-role user in the SAME tenant
-- (privacy-demo) — it is a viewer/screenshot convenience account, not part
-- of the routing proof (the proof is the relay call + mock endpoint log).
--
-- Apply AFTER seed-tenant-private-endpoint.sql, against the same DB:
--   docker exec -i newhub-pgdemo psql -U postgres -d newhub \
--     -f demo/private-inference/seed-console-viewer.sql
--
-- Idempotency note: if an earlier manual test promoted the tenant user
-- (id 9100) to an admin role directly (instead of adding a separate viewer),
-- this file resets it back to role=1 so the "normal, non-admin user" claim
-- in seed-tenant-private-endpoint.sql stays true in the database, not just
-- in the SQL comment.
UPDATE users SET role = 1 WHERE id = 9100 AND tenant_id = 'privacy-demo' AND role <> 1;

-- The viewer account: role 10 = RoleAdminUser (internal/pkg/common/constants.go).
INSERT INTO users (id, tenant_id, username, role, status, quota, "group")
VALUES (9101, 'privacy-demo', 'privacy-demo-console-viewer', 10, 1, 0, 'default')
ON CONFLICT (id) DO UPDATE SET role = 10, tenant_id = 'privacy-demo', status = 1;

-- Verify
SELECT id, tenant_id, username, role, status FROM users WHERE tenant_id = 'privacy-demo' ORDER BY id;
