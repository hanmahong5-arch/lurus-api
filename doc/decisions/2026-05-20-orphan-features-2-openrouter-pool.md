# ADR: Orphan-Feature Backfill #2 — OpenRouter Multi-Key Pool

**Status**: Accepted (backfill — shipped + production-active) · **Date**: 2026-05-20
**Source commits**: `1897` (openrouter-enterprise), `2433` (test fixes), migration `010_openrouter_sync.sql`
**Code**: `internal/app/openrouter_pool/` (cooldown, reaper, writer) · `internal/adapter/handler/openrouter_pool.go` · `internal/adapter/repo/channel_pool.go`

## Context

A single OpenRouter key has per-key rate limits; high-throughput tenants hit 429s. Upstream newapi fix: multi-key channels — one logical OpenRouter channel holds a **list** of keys; relay rotates with per-key cooldown on 429. Production code with test coverage (`cooldown_test.go`, `reaper_test.go`), inherited from upstream, never got its own ADR.

## Decision (retroactive)

**Document as a first-class production feature; promote visibility in `epics.md` under Epic 7 (Reliability) close-out.**

What it does:
1. Per-key cooldown state on each channel: `MultiKeyStatusList`, `MultiKeyCooldownUntil`, `MultiKeyDisabledReason`, `MultiKeyDisabledTime`.
2. Cooldown windows: 30s (mild) → escalates to 24h on repeated failures.
3. Reaper: master-only ticker (every 30s) scans cooled keys, recovers expired ones, flips channel back to `Enabled` if any key recovered.
4. Relay hook: on upstream 429, `MaybeMarkCooldown()` cools the specific key without taking the channel offline.
5. Visibility: `GET /api/openrouter-sync/api-pool` → status snapshot with masked key prefixes (`sk-or-v1-abc...`) for admin debug.

**Keep because**: already shipped + tested; net reliability uplift (4 keys, 1 cooled → 75% capacity not 0%); touches cash path (OpenRouter-cheapest model class stays cheap when one key rate-limits). **Don't expand now**: cooldown assumes OpenRouter-specific 429 semantics — generalizing to any provider is a real refactor, out of Wave A/B scope.

**Risks**: single-master assumption — if newhub goes active-active, reaper could double-process keys (add leader election before HA). Cooldown tuning (30s→24h) is hardcoded (retune = code change).

## Action items

- [ ] Mention in `epics.md` Epic 7 close-out as delivered reliability feature.
- [ ] When HA work begins: add `IsMasterNode` guard around `reaper.Start()` or wire to leader election.
- [ ] Multi-key for non-OpenRouter providers = new Story under Epic 7 follow-up, not a config flip.

Does NOT generalize multi-key to all providers, change the cooldown algorithm/30s cadence, or expose the pool endpoint to non-admins.
