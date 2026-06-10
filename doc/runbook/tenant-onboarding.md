# Tenant Onboarding Runbook

> Auth: Zitadel (auth.lurus.cn) · API: api.lurus.cn · Flow: Zitadel Org → API Tenant Record → User Identity Mapping.

Two modes: **auto-create** (`ZITADEL_AUTO_CREATE_TENANT=true` — tenant created on first user login) or **manual** (admin creates tenant via API, maps to Zitadel Org ID).

## Phase 1: Zitadel Setup (manual)

Create Organization (record Org ID, e.g. `285895506344386561`), Project `lurus-api` (Role Assertion + Role Check enabled), OIDC Application `lurus-api-backend` (Web, PKCE, redirect `https://api.lurus.cn/api/v2/oauth/callback`, post-logout `https://api.lurus.cn/logout`, Grant Types Authorization Code + Refresh Token, JWT token 3600s / refresh 30d idle / 90d), and Project Roles `admin` / `user` / `billing_manager`. **Full step-by-step + token settings: `doc/zitadel-setup-guide.md`.**

Update K8s secret + restart:

```bash
kubectl create secret generic lurus-api-secrets \
  --from-literal=ZITADEL_CLIENT_ID='<client_id>' --from-literal=ZITADEL_CLIENT_SECRET='<client_secret>' \
  --from-literal=SQL_DSN='postgres://...' --from-literal=SESSION_SECRET='...' \
  -n lurus-system --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deployment/lurus-api -n lurus-system
```

## Phase 2: API Tenant Creation

**Option A — auto-create (recommended)**: with `ZITADEL_AUTO_CREATE_TENANT=true`, first login from a new Zitadel Org auto-creates the tenant. No API call.

**Option B — manual via admin API**:

```bash
curl -X POST https://api.lurus.cn/api/v2/admin/tenants \
  -H "Content-Type: application/json" -H "Cookie: session=<platform_admin_session>" \
  -d '{"zitadel_org_id":"285895506344386561","slug":"acme-corp","name":"Acme Corporation","plan_type":"pro","max_users":500,"max_quota":5000000}'
# 201 → {"success":true,"data":{"id":"uuid","zitadel_org_id":...,"slug":...,"name":...,"status":1,"plan_type":"pro"}}
```

Tenant status: `1` Enabled (normal) · `2` Disabled (login blocked, data preserved) · `3` Suspended (login + API blocked).

## Phase 3: User First Login (automatic)

`/api/v2/acme-corp/auth/login` → 302 to `auth.lurus.cn/oauth/v2/authorize` → Zitadel login/consent → 302 to `/api/v2/oauth/callback?code&state` → exchange code → ZitadelAuth middleware auto-maps user → session created.

Automatic steps: JWT validated via JWKS → tenant resolved from `urn:zitadel:iam:org:id` claim → `tenants.zitadel_org_id` → user mapped from `sub` claim → `user_identity_mappings` row → Lurus user created with tenant-plan default quota → tenant context injected for isolation.

## Phase 4: Verification

```bash
curl -s https://api.lurus.cn/api/v2/admin/tenants -H "Cookie: session=<admin_session>" | jq '.data[] | {id,slug,name,status}'
psql "$DSN" -c "SELECT id, slug, name, status, plan_type FROM tenants;"
psql "$DSN" -c "SELECT zitadel_user_id, lurus_user_id, tenant_id, email FROM user_identity_mappings WHERE tenant_id='<tenant_id>';"
curl -v https://api.lurus.cn/api/v2/acme-corp/auth/login?redirect_url=/dashboard   # expect 302 → auth.lurus.cn/oauth/v2/authorize
```

## Phase 5: Tenant Management

| Operation | Endpoint | Method |
|-----------|----------|--------|
| List / Create | `/api/v2/admin/tenants` | GET / POST |
| Get / Update | `/api/v2/admin/tenants/:id` | GET / PUT |
| Enable / Disable / Suspend | `/api/v2/admin/tenants/:id/{enable,disable,suspend}` | POST |
| Stats | `/api/v2/admin/tenants/:id/stats` | GET |

```bash
curl -X POST https://api.lurus.cn/api/v2/admin/tenants/<id>/disable -H "Cookie: session=<admin_session>"  # all users lose login, data preserved
curl -X PUT https://api.lurus.cn/api/v2/admin/tenants/<id> -H "Content-Type: application/json" -H "Cookie: session=<admin_session>" -d '{"max_quota":10000000,"max_users":1000}'
```

## Troubleshooting

| Problem | Check |
|---------|-------|
| Login redirects but never completes | OIDC redirect URI matches exactly |
| "Tenant not found" on login | `ZITADEL_AUTO_CREATE_TENANT=true` or create manually |
| JWT verification fails | `ZITADEL_ISSUER`, JWKS endpoint reachable |
| User not created on login | `ZITADEL_AUTO_CREATE_USER=true` |
| Cross-tenant data visible | tenant_id in request context, GORM plugin |

Env: `ZITADEL_ENABLED=true`, `ZITADEL_ISSUER=https://auth.lurus.cn`, `ZITADEL_CLIENT_ID`/`_SECRET` (Phase 1.3), `ZITADEL_REDIRECT_URI=https://api.lurus.cn/api/v2/oauth/callback`, `ZITADEL_JWKS_URI=https://auth.lurus.cn/oauth/v2/keys`, `ZITADEL_AUTO_CREATE_TENANT=true`, `ZITADEL_AUTO_CREATE_USER=true`, `ZITADEL_ENABLE_PKCE=true`.
