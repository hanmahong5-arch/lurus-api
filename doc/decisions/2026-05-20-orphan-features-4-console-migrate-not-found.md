# ADR: Orphan-Feature Backfill #4 — `console_migrate` Investigation (Not Found)

**Status**: Investigation closed — feature does not exist in current codebase
**Date**: 2026-05-20
**Source of original mention**: 4-route Sonnet recon report (2026-05-20) listed `console_migrate` as one of 4 orphan features; later code investigation revealed it is not present.
**Code location**: N/A (does not exist)

## Context

The 2026-05-20 plan ("Newhub Evolution: 70% → Industrial-Grade") referenced `console_migrate` as one of four orphan features (along with io.net, OpenRouter pool, whitelabel HMAC). Wave A Squad 1A WA-1.2 budgeted an ADR for each of the four.

During backfill investigation, a thorough search of the repository turned up no matches:

- `grep -ri 'console_migrate' .` → empty
- `grep -ri 'ConsoleMigrate' .` → empty
- `ls cmd/` → only `cmd/server/main.go`; no migrate-style sub-commands

## Findings

1. **`console_migrate` is not in the current lurus-newhub main branch.** It may have:
   - Existed historically in upstream new-api but been removed
   - Existed in a feature branch that was never merged
   - Been confused with another feature during the Sonnet recon (e.g., the legacy console SPA → v2 SPA UI flip, which is handled by `web/` build pipeline, not a Go CLI)
2. **Functionality the name suggests** (CLI command to migrate legacy console data → v2 format) is already accomplished by other means:
   - Migration `migrations/*.sql` files handle DB schema upgrades
   - `internal/adapter/handler/v2_admin_*.go` provides the v2 admin REST surface
   - Web `web/src/pages/v2/*` provides the v2 UI; `/console` redirect to v2 was wired in 2026-05-12 per story-11-2 doc
3. **No data-migration gap is open** that a hypothetical `console_migrate` would close.

## Decision

**Mark `console_migrate` as a non-existent investigation artifact. Do not implement it. Update the Wave A plan to reflect that there are only 3 real orphan features (io.net, OpenRouter pool, whitelabel HMAC), not 4.**

The 2026-05-20 plan reference is retained as historical context — investigations that close without action are a normal part of orphan-feature backfill.

## Consequences

- No code or runtime change.
- Wave A Squad 1A item count drops by 1 (4 ADRs → 3 ADRs + 1 closure record). Time saved: ~0.5h.
- If a future contributor proposes a `console_migrate` CLI: this ADR serves as historical record that the name was considered and the gap it was intended to fill was found to already be covered by SQL migrations + REST admin handlers.

## Action items

- [ ] Update Wave A summary doc to note "3 real orphans + 1 not-found closure".
- [ ] No code action required.

## What this ADR does NOT do

- Does not implement a `console_migrate` CLI.
- Does not preclude a future, properly scoped CLI for some narrowly defined data-migration task (e.g., post-cutover newapi→newhub data migration in the D1 Option B plan). That would be a separate ADR + Story.
