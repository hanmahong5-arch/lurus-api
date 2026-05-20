# ADR: Orphan-Feature Backfill #2 — OpenRouter Multi-Key Pool

**Status**: Accepted (backfill — feature is already shipped + production-active)
**Date**: 2026-05-20
**Source commits**: `1897` (openrouter-enterprise), `2433` (openrouter test fixes), plus migration `010_openrouter_sync.sql`
**Code location**:
- `internal/app/openrouter_pool/` (cooldown, reaper, writer)
- `internal/adapter/handler/openrouter_pool.go`
- `internal/adapter/repo/channel_pool.go`

## Context

A single OpenRouter API key has per-key rate limits. Tenants doing high-throughput LLM workloads quickly bump against the 429 wall. The upstream newapi fix was "multi-key channels": one logical OpenRouter channel can hold a **list** of API keys, and the relay rotates between them with per-key cooldown tracking when individual keys hit 429s.

This is production code with test coverage. It was inherited from upstream new-api but never received its own ADR in the lurus-newhub planning track.

## Decision (retroactive)

**Document OpenRouter multi-key pool as a first-class production feature; promote its visibility in epics.md as part of Epic 11 (Reliability Hard Floor) or Epic 12 (Plans Tier).**

### What it does

1. **Per-key cooldown state**: tracks `MultiKeyStatusList`, `MultiKeyCooldownUntil`, `MultiKeyDisabledReason`, `MultiKeyDisabledTime` on each channel.
2. **Cooldown windows**: 30s (mild rate-limit) → escalates up to 24h on repeated failures, per upstream cooldown algorithm.
3. **Reaper**: master-only ticker (every 30s) that scans cooled-down keys, recovers expired ones, and flips the channel back to `Enabled` if any single key has recovered.
4. **Relay hook**: when an upstream returns 429, relay calls `MaybeMarkCooldown()` to mark the specific key as cooling without taking the whole channel offline.
5. **Visibility endpoint**: `GET /api/openrouter-sync/api-pool` returns a status snapshot with masked key prefixes (e.g., `sk-or-v1-abc...`) for admin debugging.

### Reasoning to keep

1. **Already shipped, already tested**: `cooldown_test.go` + `reaper_test.go` exist. No engineering debt to retire.
2. **Net reliability uplift**: a tenant with 4 keys and one being cooled gets 75% capacity rather than 0% capacity. This is the kind of resilience the Reliability Hard Floor epic is exactly trying to deliver.
3. **Touches the cash path**: when OpenRouter is the cheapest provider for a model class, having the pool means a single key-rate-limit doesn't push traffic to a more-expensive backup. Direct cost savings.

### Reasoning not to expand right now

- The cooldown algorithm assumes OpenRouter-specific 429 semantics. Generalizing to "any provider with rate-limited keys" is a real refactor (multi-key support for Anthropic, OpenAI, etc.). Out of Wave A/B scope.
- The reaper is master-only, so HA isn't a current concern, but if newhub goes multi-master in the future, leader election needs to gate the reaper.

## Consequences

**Positive:**
- Reliability hardening already in production; no further work needed for Wave A.
- Visibility endpoint provides on-call debugging without exposing raw keys.

**Negative:**
- The "channel pool" concept is OpenRouter-specific in code. Tenants asking "can I do the same for my Anthropic keys?" → answer is no, today. Manageable expectation gap.

**Risks:**
- Single-master assumption: if newhub HA introduces an active-active topology, the reaper could double-process keys. Mitigation: add leader election before any HA migration.
- Cooldown algorithm tuning is hardcoded (30s → 24h escalation). If OpenRouter changes 429 semantics, retuning is a code change, not a config. Acceptable for now.

## Action items

- [ ] Mention OpenRouter multi-key pool in `epics.md` under Epic 7 (Reliability) close-out summary as a "delivered" reliability feature.
- [ ] When HA work begins (post Wave B): add a `IsMasterNode` guard around `reaper.Start()` if not already present, or wire to leader election.
- [ ] If a customer requests multi-key support for non-OpenRouter providers, that is a new Story under Epic 7 follow-up, not a config flip.

## What this ADR does NOT do

- Does not commit to generalizing multi-key pool to all providers.
- Does not change the cooldown algorithm or the 30s ticker cadence.
- Does not expose the pool status endpoint to non-admin users.
