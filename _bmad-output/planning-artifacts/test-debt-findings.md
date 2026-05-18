# Test Debt Findings — Lane D Hardening Swarm (2026-05-18)

Discovered while attempting to add `go test -short ./...` as PR-blocking gate
per the hardening swarm plan §Lane D. Documenting here so future cleanup work
has explicit scope; does NOT block Lane D core deliverables (audit/governance
test additions + scoped CI gate via `.github/workflows/go-ci.yml`).

## Fixed in this swarm

| Test | File | Root cause | Fix |
|------|------|------------|-----|
| `TestListTokensV2_Pagination` | `internal/adapter/handler/v2_token_test.go` | Commit `23a87f72` renamed `data["tokens"]` → `data["items"]` and query params `page=/page_size=` → `p=/size=` in handler; test was not updated. | Updated 3 occurrences to match new shape. |

## Open — pre-existing, NOT in Lane D scope

| Test(s) | Suspected cause | Suggested next step |
|---------|-----------------|--------------------|
| `TestValidateIDToken_*` (oauth_security_test.go) — `InvalidIssuer`, `InvalidAudience`, `ExpiredToken`, `InvalidNonce`, `FutureIssuedAt` | `validateIDToken` now rejects upfront with "JWKS manager not initialized; is Zitadel enabled?" before checking claim semantics. Tests pre-date that ordering change. | Either (a) initialize a stub JWKS in test setup, or (b) refactor `validateIDToken` to validate claims independently of JWKS availability when signature check is intentionally skipped. |
| `FuzzGenerateOAuthState`, `FuzzOAuthStateRoundTrip` (oauth_fuzz_test.go) | Same root as above (require Zitadel init). | Same path as above; fuzz seeds should run under a stubbed JWKS. |
| Several `*_integration_test.go` (e.g., `internal_api_integration_test.go` lines 833, 849, 1291) still reference `data["tokens"]`/`data["page_size"]` | Same drift as the token test fix above, but in integration suite. | Integration tests appear not to be `-short` gated; verify, then mirror the `items`/`p`/`size` rename. |

## Why CI gate is scoped narrowly

The hardening plan asked for `go test -short ./...` as a PR-blocking gate.
With the OAuth/JWKS test debt still red on `main`, adding that gate today
would block every PR. Instead Lane D ships:

- A *narrow* `go-ci.yml` gate covering compile (`go build ./...`),
  `go vet ./...`, and the Lane D-relevant handler tests
  (audit + governance + the repaired v2 token pagination).
- This finding doc, so the broader gate can be turned on after the open
  items above are resolved.

Per CLAUDE.md §4.1 ⑥: passing 9/9 of the new audit+governance tests is
"markers present" — the underlying *measurement* (full handler-package
green) is not yet meaningful until JWKS test debt is cleared. Calling the
narrow gate sufficient is honest; pretending the broad gate is enabled
would not be.
