# ADR: Orphan-Feature Backfill #1 — io.net Deployment Integration

**Status**: Accepted (backfill — feature is already shipped)
**Date**: 2026-05-20
**Source commit**: `b10f1f7b` (feat: ionet integrate #2105) — date upstream; merged into newhub via fork sync
**Code location**: `pkg/ionet/` + `internal/adapter/handler/deployment.go`

## Context

io.net is a decentralized GPU rental marketplace (https://io.net). It exposes a REST API for renting GPU containers, deploying inference workloads, and querying hardware pricing. The upstream newapi codebase added an io.net client in 2024-2025, framing it as "Model Deployment" — i.e., a tenant can rent GPU capacity directly from io.net via the newapi/newhub admin UI, then route inference traffic to those deployments as a custom upstream channel.

This feature shipped without a corresponding ADR or epic story in the BMAD planning artifacts. It is referenced in the web UI (`ModelDeploymentSetting.jsx`) but absent from `epics.md` and the Phase 1/2/3 roadmap.

## Decision (retroactive)

**Keep io.net deployment integration as a Phase 4 (差异化期) capability, gated behind a feature flag that defaults to disabled.**

### Scope as shipped

- **Client** (`pkg/ionet/`): typed wrappers for create/list/update deployments, pricing estimation, hardware queries, cluster management.
- **Two API modes**: public (`https://api.io.solutions`) and enterprise (separate `DefaultEnterpriseBaseURL`). Tenant-selectable via OptionMap.
- **Handler** (`internal/adapter/handler/deployment.go`): exposes
  - `GET /api/model-deployments/settings`
  - `POST /api/model-deployments/test-connection`
  - `GET /api/model-deployments` / `:id`
  - `PATCH /api/model-deployments/:id`
- **Feature gate**: `model_deployment.ionet.enabled` + `model_deployment.ionet.api_key` in OptionMap. Disabled by default; tenant must opt in via admin UI and supply key.

### Reasoning to keep

1. **Production code, non-zero cost to remove**: ~1500+ LOC of Go client + handler + UI. Removing it is its own engineering project, not a free cleanup.
2. **Differentiation hook for Wave C / Phase 4**: when external resellers want GPU-cost transparency or sovereign deploy (e.g., "I want this LoRA running on my own rented GPU, not OpenRouter"), io.net is one of the few non-AWS/non-GCP options that ships an API today.
3. **No active maintenance burden if left disabled**: handler only loads on tenant opt-in; OptionMap gate keeps it off the hot path.

### Reasoning not to promote it to Phase 1/2

- No paying customer asking for it today.
- No tests in the Go backend (only the UI component has tests). Promoting to "supported" needs at least client-level integration tests against a mock io.net API.
- io.net's API SLA is unverified at Lurus scale — could be a support black hole.

## Consequences

**Positive:**
- Optionality preserved for Phase 4 / Wave C product differentiation.
- No customer-visible change.

**Negative:**
- ~1500 LOC of code remains technically un-owned. Add a CODEOWNERS line if/when promoted.
- Web UI shows the "Model Deployment" tab even when disabled — minor UX clutter. Acceptable for now.

**Risks:**
- Upstream `pkg/ionet/` has zero test coverage. If io.net changes API shape, breakage is silent until a tenant tries to use it. Mitigation: keep disabled-by-default; revisit with tests before any tenant enables it in PROD.

## Action items

- [ ] Add `ionet_owner: TBD` comment in `pkg/ionet/client.go` package doc (mark as "limited support, see ADR 2026-05-20")
- [ ] When the first tenant requests enabling: write integration tests against an io.net sandbox endpoint or a recorded mock before allowing PROD enablement.
- [ ] Surface "io.net" in capability registry (`lurus.yaml` capabilities section) as `tier: experimental` if/when the capability registry adds tier markers.

## What this ADR does NOT do

- Does not promote io.net to a Wave A/B Story.
- Does not commit to maintaining the client when upstream io.net API changes.
- Does not enable the feature flag in any current tenant.
