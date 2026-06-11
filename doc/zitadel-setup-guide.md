# Zitadel Setup Guide / Zitadel 配置指南

> Created 2026-01-25 · Purpose: lurus-api multi-tenant SaaS — Zitadel auth center config.
> Admin console: https://auth.lurus.cn · default admin `admin` / `admin@lurus.cn` / `Lurus@ops` (change password after first login).

Configures one Organization (= tenant in lurus-api), one Project, one OIDC Application, Project Roles, and SMTP. Each Org has independent users/projects/permissions; each user's roles are embedded in the JWT.

## 1. Create Organization

Console → **Organizations** → **+ Create New Organization** (or https://auth.lurus.cn/ui/console/orgs).

| Field | Value |
|-------|-------|
| Organization Name | `Lurus Platform` |
| Primary Domain | `lurus` (used on login page) |

Record the **Organization ID** (e.g. `123456789012345678`) — used for tenant mapping.

## 2. Create Project

Enter the Org → **Projects** → **+ Create New Project**.

| Field | Value |
|-------|-------|
| Project Name | `lurus-api` |
| Role Assertion | Enabled |
| Role Check | Enabled |

Record the **Project ID** (e.g. `234567890123456789`).

## 3. Create OIDC Application

Project → **Applications** → **+ New** → type **Web** → Continue.

| Setting | Value |
|---------|-------|
| Name | `lurus-api-backend` |
| Auth Method | `PKCE` (recommended) or `Post` |
| Access Token Type | `JWT` |
| Access Token / ID Token Lifetime | `3600s` (1h) |
| Refresh Token Idle Expiration | `2592000s` (30d) |
| Refresh Token Expiration | `7776000s` (90d) |

- **Redirect URIs**: prod `https://api.lurus.cn/api/v2/oauth/callback` · dev `http://localhost:8850/api/v2/oauth/callback`
- **Post Logout Redirect URIs**: `https://api.lurus.cn/logout` · `http://localhost:8850/logout`
- **Grant Types**: Authorization Code + Refresh Token (both required)
- **Response Types**: Code (required)

On Create, record **Client ID** (e.g. `234567890123456789@lurus-api`) and click **Generate Client Secret** — the secret is shown ONCE, save it immediately.

## 4. Project Roles

Project → **Roles** → **+ New** for each. Roles are embedded in the JWT.

| Key | Display Name | Description |
|-----|--------------|-------------|
| `admin` | Administrator | Tenant administrator with full access |
| `user` | User | Regular user with basic access |
| `billing_manager` | Billing Manager | Billing and subscription management access |

Assign roles per user: Org → **Users** → select user → **Authorizations** → **+ New** → Project `lurus-api` → check role → Create.

## 5. SMTP (via Stalwart Mail)

Console → ⚙️ **Instance Settings** → **SMTP**.

| Field | Value |
|-------|-------|
| SMTP Host | `mail.lurus.cn` |
| SMTP Port | `587` (submission, TLS) |
| SMTP User / Sender Email | `noreply@lurus.cn` |
| SMTP Password | `Lurus@ops` |
| Sender Name | `Lurus Platform` |
| TLS | Enabled |

Click **Test Configuration** with a test address; **Save**. On failure: check Stalwart status (`kubectl get pods -n mail`, `kubectl logs -n mail deployment/stalwart-mail --tail=50`), port 587 cluster reachability, and `noreply@lurus.cn` credentials.

## 6. Configuration / Verification

OIDC Discovery: `https://auth.lurus.cn/.well-known/openid-configuration` (`curl ... | jq`). Key endpoints:

```json
{
  "issuer": "https://auth.lurus.cn",
  "authorization_endpoint": "https://auth.lurus.cn/oauth/v2/authorize",
  "token_endpoint": "https://auth.lurus.cn/oauth/v2/token",
  "userinfo_endpoint": "https://auth.lurus.cn/oidc/v1/userinfo",
  "jwks_uri": "https://auth.lurus.cn/oauth/v2/keys",
  "end_session_endpoint": "https://auth.lurus.cn/oidc/v1/end_session",
  "introspection_endpoint": "https://auth.lurus.cn/oauth/v2/introspect"
}
```

Environment variables (see also `2b-svc-newhub/CLAUDE.md` § Zitadel OIDC for the full env set with multi-issuer/audience options):

```bash
ZITADEL_ISSUER=https://auth.lurus.cn
ZITADEL_CLIENT_ID=234567890123456789@lurus-api      # replace with actual
ZITADEL_CLIENT_SECRET=<actual secret>
ZITADEL_REDIRECT_URI=https://api.lurus.cn/api/v2/oauth/callback
ZITADEL_JWKS_URI=https://auth.lurus.cn/oauth/v2/keys
ZITADEL_AUTHORIZATION_ENDPOINT=https://auth.lurus.cn/oauth/v2/authorize
ZITADEL_TOKEN_ENDPOINT=https://auth.lurus.cn/oauth/v2/token
ZITADEL_USERINFO_ENDPOINT=https://auth.lurus.cn/oidc/v1/userinfo
ZITADEL_DEFAULT_ORG_ID=123456789012345678           # replace with actual
ZITADEL_DEFAULT_ORG_NAME=Lurus Platform
```

Test OAuth flow:

```
https://auth.lurus.cn/oauth/v2/authorize?client_id=YOUR_CLIENT_ID&redirect_uri=https://api.lurus.cn/api/v2/oauth/callback&response_type=code&scope=openid%20email%20profile&state=test123&organization=YOUR_ORG_ID
```

Expect: redirect to Zitadel login → after login, redirect to callback URL.

Troubleshooting: `doc/zitadel-troubleshooting.md`. Tenant onboarding (Org→tenant→user mapping): `doc/runbook/tenant-onboarding.md`.
