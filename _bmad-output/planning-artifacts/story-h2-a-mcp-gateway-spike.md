# Story H2.A — MCP Gateway Spike (multi-tenant MCP front door)

**Epic**: 17 (new) — Strategic Moat: MCP Gateway
**Phase**: H2 Bet A (per 12-month Horizon Plan, 2026-05-27)
**Priority**: P1 (护城河 — 市场窗口最紧, est. 6-12m before a hyperscaler occupies it)
**Status**: spike-design (this doc) — execution + customer validation NOT done, see §9
**Type**: Spike → Design doc (the "可执行设计 doc" deliverable; the demo + real-client validation is the gated next step)
**Created**: 2026-05-28
**Decision basis**: D2 = "Bet A MCP Gateway 启动" (user-approved, prior session)
**Related**: `story-h1-1-scim-saml.md` (E1 — auth reuse), Phase E2 (token scope), Phase E3 (audit taxonomy)

---

## 0. Honesty preface (§4.1 self-check applied before writing)

This is a **spike design doc**, not a shipped feature. Per CLAUDE.md §4.1:

- **§4.1②** — every named artifact in the Horizon Plan was `grep`'d before this doc.
  Results below are facts on disk as of 2026-05-28, not aspirations:

  | Plan-named artifact | grep result | status |
  |---|---|---|
  | `internal/mcp/` | no files | **does not exist** (clean slate) |
  | `mcp.lurus.cn` | 0 matches (repo + lurus.yaml) | **not provisioned** (target domain, not live) |
  | `MCP_TOOLS_REGISTRY` env | 0 matches | **does not exist** |
  | `ActionMCPToolInvoked` / `mcp.tool_invoked` | 0 matches | **does not exist** |
  | `mcp_servers` / `mcp_tool_invocations` tables | 0 matches | **do not exist** |
  | `McpServers` field | `internal/pkg/dto/claude.go:211` | exists — but it is the **Anthropic-native request passthrough field**, NOT a gateway (do not conflate) |

- **§4.1③** — the MCP demo's validation (≥10 tool calls) MUST come from a real third-party
  MCP host client, NOT a self-written mock client. A mock driving newhub's own handler is a
  self-validating loop and is explicitly **not** evidence. This doc therefore stops at design;
  it does not claim any tool call has been observed.
- **§4.1④** — go/no-go to actually build the gateway is a **user decision** (see §10), not
  pre-decided here. D2 approved *starting the spike*, not committing the 4-week build.

---

## 1. Objective

Give newhub a `/mcp/v1/` route family that turns Lurus's existing first-party MCP servers
(today: stdio-only, single-user, no auth/audit/billing) into a **remote, multi-tenant,
authenticated, audited, metered** MCP endpoint that enterprise MCP host applications can point at.

The bet is **not** "build more MCP servers" — Lurus already has 6 (see §2). The bet is the
**enterprise gateway layer in front of them**, which no horizontal competitor (LiteLLM/Portkey)
can replicate because they have no first-party MCP tool ecosystem.

---

## 2. Verified ecosystem grounding (why the reframe matters)

`lurus.yaml` already declares these first-party MCP servers — **all stdio-only, no network
port, no domain**:

| Server | dir | transport | auth | maturity |
|---|---|---|---|---|
| `lurus-zitadel-mcp` | `2l-svc-zitadel-mcp` | stdio | Service-Account JWT | prod |
| `lurus-k8s-mcp` | `2l-svc-k8s-mcp` | stdio | SSH to k3s master | prod |
| `lurus-platform-mcp` | `2l-svc-platform-mcp` | stdio | `INTERNAL_API_KEY` | prod |
| `lurus-tally-mcp` | `2b-svc-psi/cmd/tally-mcp` | stdio | PAT bearer | alpha |
| `kova-mcp` | `2b-svc-kova` | stdio | — | — |
| `zita-mcp` | `cmd/zita-mcp` | stdio | — | — |

**The gap this exposes**: a stdio MCP server runs as a subprocess on the *client's* machine.
It cannot be multi-tenant, cannot be reached remotely, has no per-tenant auth, no audit trail,
no usage metering. That is exactly the enterprise-readiness gap the Horizon Plan's Bet A targets —
and Lurus is uniquely positioned because the *tools already exist*; only the front door is missing.

**Gateway = stdio↔HTTP bridge + multi-tenant front door.** This is the core architectural claim.

---

## 3. Architecture

```
┌──────────────────────┐        ┌───────────────────────────────────────────┐
│ MCP host application │        │                  newhub                     │
│ (desktop AI assistant│ HTTPS  │  /mcp/v1/:tenant_slug/  (gin route group)   │
│  / IDE MCP integ.)   │ Stream │  ┌────────────────────────────────────────┐ │
│                      ├───────>│  │ middleware: TokenAuth (reuse E2)        │ │
│  configured endpoint │ MCP    │  │   → token.HasScope("mcp:<server>")      │ │
│  mcp.lurus.cn/<slug> │ JSON-  │  └────────────────────────────────────────┘ │
│                      │ RPC2.0 │  ┌────────────────────────────────────────┐ │
└──────────────────────┘        │  │ internal/mcp/registry.go               │ │
                                │  │   tenant_slug → [mcp_servers] lookup    │ │
                                │  └────────────────────────────────────────┘ │
                                │  ┌────────────────────────────────────────┐ │
                                │  │ internal/mcp/bridge.go                  │ │
                                │  │   spawn / pool stdio child OR proxy to  │ │
                                │  │   an HTTP upstream; relay JSON-RPC      │ │
                                │  └────────────────────────────────────────┘ │
                                │     │ on tools/call →                        │
                                │     ├─ audit: ActionMCPToolInvoked (E3)      │
                                │     └─ bill : platform ReportUsage / Debit   │
                                └───────────────────────────────────────────┘
```

### 3.1 Transport decision

MCP defines two remote transports. The spec's current (2025-03-26 rev) recommendation is
**Streamable HTTP** (single endpoint, POST for requests, optional SSE upgrade for streaming);
the older **HTTP+SSE** (two-endpoint) transport is deprecated. **Decision: implement Streamable
HTTP only**; do not carry the deprecated two-endpoint transport (avoids a dead code path —
cf. the "no speculative branches" lesson).

> ⚠ Spec-evolution risk (§8): MCP transport revs are still moving. The bridge layer must keep
> the wire-protocol adapter isolated in `internal/mcp/transport.go` so a future rev is a
> single-file swap, not a cross-cutting rewrite.

### 3.2 Components (to be built — NOT yet created)

| File | Responsibility | LOC est. |
|---|---|---|
| `internal/adapter/handler/mcp_router.go` | `/mcp/v1/:tenant_slug/*` registration + Streamable-HTTP handler | ~180 |
| `internal/mcp/registry.go` | tenant→servers lookup (Redis-cached, like channel cache) | ~150 |
| `internal/mcp/bridge.go` | stdio child pool OR HTTP upstream proxy; JSON-RPC relay | ~320 |
| `internal/mcp/transport.go` | Streamable-HTTP framing (isolated per §3.1) | ~140 |
| `internal/mcp/policy.go` | per-server scope check (reuse `token.HasScope`) | ~70 |
| `internal/app/governance/audit_action.go` | +`ActionMCPToolInvoked`, +`ActionMCPServerRegistered` | +6 |
| migration `0NN_mcp_servers.sql` | registry table | — |
| migration `0NN_mcp_tool_invocations.sql` | per-call audit/billing source | — |
| `internal/adapter/handler/mcp_admin.go` | tenant MCP-server CRUD (`/api/v2/:slug/mcp/servers`) | ~160 |
| `web/src/.../McpServers/*` | registry UI | ~300 |

Total first cut ≈ **1,300–1,600 LOC** (gateway core + admin CRUD + UI). Within H2 16-week budget.

---

## 4. Data model (migrations — IDs reserved via migration-ledger before writing SQL)

```sql
-- mcp_servers: per-tenant registry of fronted MCP servers
CREATE TABLE mcp_servers (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    name        VARCHAR(128) NOT NULL,       -- shown to operators
    slug        VARCHAR(64)  NOT NULL,        -- URL component: /mcp/v1/<tenant>/<slug>
    kind        VARCHAR(16)  NOT NULL,        -- 'stdio' | 'http'
    endpoint    TEXT         NOT NULL,        -- exec path (stdio) or upstream URL (http)
    scopes      VARCHAR(255) NOT NULL DEFAULT '',  -- comma list, mirrors token.Scopes
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, slug)
);

-- mcp_tool_invocations: the billing + audit source of truth for tool calls
CREATE TABLE mcp_tool_invocations (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL,
    server_id   BIGINT NOT NULL,
    token_id    BIGINT,
    tool_name   VARCHAR(128) NOT NULL,
    status      VARCHAR(16)  NOT NULL,        -- 'ok' | 'error' | 'rejected'
    latency_ms  INT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_mcp_inv_tenant_ts ON mcp_tool_invocations (tenant_id, created_at DESC);
```

GORM auto-migrate drives table creation at boot (per CLAUDE.md "DB 自动迁移"); the SQL above is
the hand-migration mirror for PostgreSQL, committed under `migrations/` once ledger IDs are reserved.

---

## 5. Reuse map (zero net-new auth/audit/billing primitives)

| Need | Reuse | Evidence |
|---|---|---|
| Per-tool authorization | `token.HasScope("mcp:<server-slug>")` | `entity/token.go:106` — empty scopes = no restriction (migration 015) |
| Rejection audit | `ActionAuthScopeRejected` (E2) | `audit_action.go:20` |
| Tool-call audit | **new** `ActionMCPToolInvoked` = `"mcp.tool_invoked"` | follows `audit_action.go` add pattern (const + validAuditActions map) |
| Registry-change audit | **new** `ActionMCPServerRegistered` = `"mcp.server_registered"` | same |
| Usage metering | `ReportLLMUsageGRPC` / `DebitWalletGRPC` | newhub CLAUDE.md gRPC table — extend memo with `mcp:` prefix so `wallet_ledger` rows are greppable (§9 verification) |
| Tenant routing | `:tenant_slug` group + tenant cache | mirrors every existing `/api/v2/:tenant_slug/*` route |
| Registry cache | Redis channel-cache pattern | CLAUDE.md "渠道缓存" |

Net-new audit taxonomy: **51 → 53 actions** (2 added). No new auth model, no new billing path.

---

## 6. Route family (to register in api-v2-router.go OR a dedicated mcp-router.go)

```
POST /mcp/v1/:tenant_slug/:server_slug      # Streamable HTTP MCP endpoint (TokenAuth + scope)
GET  /mcp/v1/:tenant_slug/:server_slug      # SSE upgrade (optional, same handler)
# Admin CRUD (session/admin auth, mirrors tenantChannels group):
GET    /api/v2/:tenant_slug/mcp/servers
POST   /api/v2/:tenant_slug/mcp/servers
PUT    /api/v2/:tenant_slug/mcp/servers/:id
DELETE /api/v2/:tenant_slug/mcp/servers/:id
```

`mcp.lurus.cn` is a **future ingress** — it does not exist yet. Until provisioned, the gateway is
reachable at `test-newhub.lurus.cn/mcp/v1/...`. The vanity domain is an ops task, not a code dep;
the doc does not assume it.

---

## 7. First fronted server (spike demo target)

Recommendation: front **`lurus-platform-mcp`** or **`lurus-tally-mcp`** first — both are read-heavy
first-party servers with clear, low-risk tool surfaces, and both already authenticate with a bearer
key the gateway can hold server-side. A throwaway "lurus.yaml-reader" demo is **not** built here:
it would only be exercised by a self-written client, which §4.1③ forbids counting as evidence.

The demo's *real* validation (next gated step) is: an internal user configures a real third-party
MCP host client to point at the gateway and the platform-mcp tools resolve through it.

---

## 8. Risk register + kill criteria

| Risk | Severity | Mitigation | Kill trigger |
|---|---|---|---|
| MCP spec still evolving (transport revs) | High | isolate wire protocol in `transport.go` | spec breaks twice in 4w → pause |
| Customers don't actually consume MCP (agent wave routes via Assistants/GPTs/custom instead) | High | **customer validation gate before build** (§9) | 0 real tool calls from a real client in 4w → no-go, delete `internal/mcp/` |
| stdio child-process pooling is fragile under concurrency | Med | prefer fronting HTTP-capable upstreams; pool with bounded size + idle reap | stdio bridge can't hold 10 concurrent sessions → http-only |
| Billing double-count vs existing relay metering | Med | `mcp:` memo prefix + separate `mcp_tool_invocations` source | wallet_ledger shows dup rows → reconcile before GA |

**Sunk-cost ceiling**: all code lands under `internal/mcp/` + 2 migrations + 1 route file. If the
customer-validation gate fails, the rollback is a single directory + 2 reverts. This bounded blast
radius is *why* Bet A was the recommended H2 entry (Plan §H2: "若失败,沉默成本可控").

---

## 9. Verification (copied from Horizon Plan — status: NOT done, gated)

```bash
# (1) real third-party MCP host client configured at the gateway, ≥10 tool calls
psql -c "SELECT count(*) FROM mcp_tool_invocations WHERE tenant_id=<slug>"   # ≥ 10
psql -c "SELECT count(*) FROM audit_events WHERE action='mcp.tool_invoked'"  # ≥ 10
# (2) billing actually flowed — grep the real table, not audit_events alone (§4.1③):
psql -c "SELECT SUM(amount) FROM wallet_ledger WHERE memo LIKE '%mcp%'"      # ≥ 0.01
```

None of the above has been run. They require: (a) the gateway built, (b) a real MCP host client,
(c) an internal/friendly customer. This doc is the design that precedes that work.

---

## 10. Recommendation to user (go/no-go is your call — §4.1④)

The spike design is complete and the architecture is **low-risk + high-reuse** (no net-new auth,
+2 audit actions, bounded blast radius). My recommendation: **proceed to a 1-week build of the
gateway core (registry + bridge + Streamable-HTTP handler) fronting `lurus-platform-mcp`, gated on
a real-client validation before any further investment.** But three open decisions are yours:

- **D2.1** — build the gateway core now, or hold for an explicit internal customer first?
- **D2.2** — first fronted server: `lurus-platform-mcp` (recommended) vs `lurus-tally-mcp`?
- **D2.3** — does an internal/friendly customer exist to run the ≥10-tool-call validation? If not,
  the build proceeds but the verification gate (§9) stays open and Bet A is NOT declared validated.

---

## 11. Effort + phasing

| Step | Deliverable | Effort | Gate |
|---|---|---|---|
| **This doc** | executable design + grounding | done | — |
| Build-1 | registry + bridge + transport + 1 route, unit tests on pure framing/routing logic | ~1w | doc review |
| Build-2 | admin CRUD + audit/billing wiring + migrations | ~1w | Build-1 green |
| Validate | real-client ≥10 tool calls → §9 queries | external | **go/no-go for full Bet A** |
| Build-3 | registry UI + `lurus-mcp-tools` OSS lib | ~2w | validation pass (release cadence: cut OSS lib only after a real consumer integrates — cf. memory) |

---

_Spike doc authored 2026-05-28. Decision D2 (start Bet A) approved prior session; D2.1–D2.3 open._
