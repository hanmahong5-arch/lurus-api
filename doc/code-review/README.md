# Code Review Archive

> Epic 6 (Code Review & Security Hardening) complete — all P0/P1/P2 issues fixed and verified. The full reports (`2026-02-13-adversarial-code-review.md`, `P0-1-git-history-cleanup-guide.md`, and the `archive/2026-02-11-*` set) were distilled into this stub on doc-compaction; reconstruct from git history if needed.

## 2026-02-13 adversarial review — 8 issues, all resolved

| ID | Location | Issue | Resolution |
|----|----------|-------|------------|
| P0-1 | `deploy/k8s/secrets.yaml` (git history) | DB password / SESSION_SECRET leaked in 5 commits | git history cleaned 2026-02-13 via `git-filter-repo` v2.47.0 (5019 commits rewritten, secrets.yaml re-added as PLACEHOLDER template). Backup branches: `backup-before-filter-repo-20260213` (delete from remote after verify). |
| P0-2 | `internal/adapter/handler/alipay_test.go` | Single-file `go test alipay_test.go` build-fails (missing pkg context) | Documented: always test at package level (`go test ./internal/adapter/handler/`), never single file. |
| P1-1 | `internal/app/release_service.go:28` | MinIO bucket hardcoded `"lurus-releases"` | Externalized → `MINIO_RELEASES_BUCKET` env (fallback `lurus-releases`). |
| P1-2 | `internal/adapter/middleware/cors.go:12` | CORS origins hardcoded | Externalized → `ALLOWED_ORIGINS` env (comma-separated). |
| P1-3 | `internal/adapter/handler/alipay.go:150` | `"alipay_"` username prefix hardcoded | Extracted to constant in `internal/pkg/constant/oauth.go`. |
| P2-1 | `internal/` (34 files) | 22 non-test uses of `context.Background()` | Cleaned in 15 files; handlers inherit `c.Request.Context()`, goroutines use `context.WithTimeout`. |
| P2-2 | `internal/app/release_service.go` | Unimplemented TODO (MinIO presigned URL, GeoIP) | Tracked; not a ship blocker. |
| P2-3 | `.gitignore` | `secrets.prod.yaml` not ignored | Added `**/secrets.prod.yaml` / `**/secrets-*.yaml` / `*.prod.yaml`. |

Security checks passed: SQLi (GORM), XSS (React escape), CSRF (SameSite+CORS), auth (session + Zitadel), RBAC, rate-limit (`middleware/rate-limit.go`), input validation. Outstanding 2FA / password-reentry TODOs in `sensitive_action.go:65,71` (roadmap).

Test commands: see `TESTING.md`. Git-history-cleanup tool reference: `git-filter-repo` / BFG.
