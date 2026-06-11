# ADR: Orphan-Feature Backfill #1 — io.net Deployment Integration

**Status**: Accepted (backfill — already shipped) · **Date**: 2026-05-20
**Source commit**: `b10f1f7b` (feat: ionet integrate #2105, merged via fork sync) · **Code**: `pkg/ionet/` + `internal/adapter/handler/deployment.go`

## Context

io.net (https://io.net) is a decentralized GPU rental marketplace with a REST API (rent GPU containers, deploy inference, query pricing). Upstream newapi added an io.net client framed as "Model Deployment" — a tenant rents io.net GPU capacity via admin UI, then routes inference to it as a custom upstream channel. Shipped without ADR/epic story; referenced in `ModelDeploymentSetting.jsx` but absent from `epics.md`.

## Decision (retroactive)

**Keep as a Phase 4 (差异化期) capability behind a feature flag defaulting to disabled.**

Scope as shipped: client `pkg/ionet/` (create/list/update deployments, pricing, hardware, clusters) with two API modes (public `https://api.io.solutions` + enterprise `DefaultEnterpriseBaseURL`, tenant-selectable via OptionMap). Handler `internal/adapter/handler/deployment.go`: `GET /api/model-deployments/settings`, `POST /api/model-deployments/test-connection`, `GET /api/model-deployments[/:id]`, `PATCH /api/model-deployments/:id`. Feature gate: `model_deployment.ionet.enabled` + `model_deployment.ionet.api_key` in OptionMap, disabled by default (tenant opts in + supplies key via admin UI).

**Keep because**: production code (~1500+ LOC), non-zero removal cost; differentiation hook for Wave C / Phase 4 (sovereign GPU deploy, non-AWS/GCP API); no maintenance burden while disabled (only loads on tenant opt-in). **Don't promote to Phase 1/2 because**: no paying customer asking; no Go backend tests (only UI component tested); io.net API SLA at Lurus scale unverified.

**Risk**: `pkg/ionet/` has zero test coverage — io.net API shape change = silent breakage. Mitigation: keep disabled-by-default; add integration tests against io.net sandbox/mock before any PROD enablement.

## Action items

- [ ] Add `ionet_owner: TBD` package-doc comment in `pkg/ionet/client.go` ("limited support, see this ADR").
- [ ] Before first tenant enables: write integration tests against io.net sandbox/recorded mock.
- [ ] Surface `io.net` in `lurus.yaml` capabilities as `tier: experimental` if/when tier markers exist.

Does NOT promote to a Story, commit to maintaining the client on upstream API changes, or enable the flag in any tenant.
