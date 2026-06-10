# ADR: Orphan-Feature Backfill #4 — `console_migrate` Investigation (Not Found)

**Status**: Investigation closed — feature does not exist · **Date**: 2026-05-20

The 2026-05-20 plan listed `console_migrate` as a 4th orphan feature. Code search found nothing: `grep -ri 'console_migrate' .` / `grep -ri 'ConsoleMigrate' .` empty; `cmd/` has only `cmd/server/main.go`, no migrate sub-commands.

**Decision**: non-existent investigation artifact — do not implement. There are only **3 real orphan features** (io.net, OpenRouter pool, whitelabel HMAC). The functionality the name suggests (legacy console → v2 data migration) is already covered by `migrations/*.sql` (schema) + `internal/adapter/handler/v2_admin_*.go` (REST admin) + `web/src/pages/v2/*` + the `/console`→v2 redirect (wired 2026-05-12, story-11-2). No data-migration gap is open.

This ADR is the historical record. A future, narrowly-scoped data-migration CLI (e.g. post-cutover newapi→newhub per D1 Option B) would be a separate ADR + Story.
