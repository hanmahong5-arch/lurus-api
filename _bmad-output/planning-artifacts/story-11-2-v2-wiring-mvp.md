# Story 11-2: V2 Hi-Fi Wiring (MVP)

**Epic**: 11 (v1 Web UI 退役)
**Priority**: P1 — blocks 11-3 (v1 sunset)
**Status**: Draft (2026-05-10)
**Sized**: 2-3 weeks engineering, single-developer pace
**Discovered by**: Layer C bridge browser e2e (2026-05-10)

## Context

The v2 hi-fi screens at `web/src/pages/v2/*` are **100% static design mockups**. Audit on 2026-05-10:

```
$ cd web/src/pages/v2 && for d in */; do
    echo "$d $(grep -rE 'API\.|axios|fetch\(' "$d" 2>/dev/null | wc -l)"
  done

AccountDisabled/ 0   Billing/ 0     Channel/ 0    Chat/ 0
CommandPalette/ 0    Dashboard/ 0   DesignSystem/ 0   Flows/ 0
Log/ 0               Models/ 0      Playground/ 0     Pricing/ 0
Settings/ 0          States/ 0      Tenants/ 0        Token/ 0
Variants/ 0
```

Every page renders hardcoded content. HFShell topbar lacks user/logout UI (added partial-hotfix `2026-05-10` commit `<sha>`).

The `/console` aggressive redirects to `/console/v2/*` were dropping
authenticated users into a button gallery. Hotfix re-routes to
`/console/legacy/*` (real wired v1 pages) until this story ships.

## Why Epic 11 was blocked

Epic 11 plan reads:
> 11-2 v2 OIDC 后台对齐 v1 功能（最后差距）
> 11-3 v1 web UI 灰度下线

11-2 was treated as "minor delta polish" — actually it's the entire wiring effort. 11-3 cannot start until 11-2 ships.

## MVP Scope (this story)

**Wire the management surfaces first** (B2B value), defer playground/chat (consumer-flavored, also pending consumer-feature deletion in Wave 2 of Layer C work):

| Page | Backend already exists | Effort |
|------|------------------------|--------|
| **Token** (CRUD + quota) | `GET/POST/PUT/DELETE /api/v2/:tenant/tokens` | M |
| **Channel** (CRUD + test + sync) | `GET/POST/PUT/DELETE /api/v2/:tenant/channels` | L |
| **Tenants** (admin) | `GET/POST/PUT /api/v2/admin/tenants` | M |
| **Logs** | `GET /api/v2/:tenant/logs` (+ Meilisearch when on) | M |
| **Dashboard** (usage + cost summary) | `GET /api/v2/user/identity-overview` + governance endpoints | M |
| **Settings** (personal) | `GET/PUT /api/v2/:tenant/user/me` | S |

**Out of scope (defer)**:
- Playground / Chat → wait until consumer-feature deletion decision lands (Layer C Wave 2). May get deleted entirely.
- Pricing / Billing → couples to Epic 12 (计费分层); wire after SKU model finalizes.
- Models / Flows / DesignSystem / States / Variants / CommandPalette → design-only, not customer-facing.

## Sequencing

1. **Day 1-2**: Token page (smallest, pattern-establishing)
2. **Day 3-5**: Channel page
3. **Day 6-7**: Logs + Settings
4. **Day 8-10**: Tenants (admin-only, smaller user base)
5. **Day 11-12**: Dashboard aggregation
6. **Day 13-14**: Re-point `/console` redirects back to v2 + remove hotfix
7. **Day 15**: Story 11-3 (v1 sunset) unblocked

## Definition of Done

- All 6 in-scope v2 pages perform their primary CRUD/read against the live backend, no mocks.
- HFShell user area shows real user (already done in hotfix) and tenant switcher reads `/api/v2/admin/tenants`.
- `/console` redirects flipped back to `/console/v2/*` (revert hotfix in App.jsx lines 122-130 + 252-254).
- e2e: log in via Layer C bridge → land on v2 dashboard → create token → see it in token list → log out → log back in → token still there.
- v1 `/console/legacy/*` routes left in place as escape hatch for one sprint after launch (then sunset per 11-3).

## Risk

- **Tenant switcher** is currently a static mock; flipping to live data may surface multi-tenant gaps in `tenant_slug` URL routing (current bridge writes `tenant_id="default"`).
- **No tests for v2 pages** — they're visual-only; need to add at minimum smoke-level cypress/playwright before declaring done.
