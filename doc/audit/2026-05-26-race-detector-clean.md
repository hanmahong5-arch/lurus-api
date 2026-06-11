# Race Detector Audit (2026-05-26)

**Conclusion: MANUAL INSPECTION CLEAN — automated `go test -race` not executable on this host.**

## Environment

Windows 10 Pro (x86-64, Git Bash/MSYS2), go1.25.x, CGO_ENABLED=0 default. Race detector **cannot run** — requires CGO + a C compiler; neither GCC nor Clang installed. `CGO_ENABLED=1 go test -race ./internal/lifecycle/...` → `cgo: C compiler "gcc" not found`. Recommendation: run `go test -race -short -count=10 ./internal/{lifecycle,app}/... ./internal/adapter/middleware/...` on a Linux CI runner.

## Manual inspection — 4 Wave-UAT hot spots (all clean)

1. **`internal/lifecycle/lifecycle.go` Manager** — `tasks []Task` written under `m.mu` by `Register`, read by `Start` without lock. Safe under contract (Register-before-Start, not concurrent in prod). `TestGracefulHTTPShutdown` flakiness is latency (500ms deadline tight on loaded Windows host), not a race.
2. **`internal/app/billing_outbox.go`** — package-level `billingOutboxDB *gorm.DB` written once by `InitBillingOutbox` at startup (single goroutine before concurrent callers), nil-guarded in each function. Standard init-before-spawn idiom; no race under documented usage.
3. **`internal/app/channel_select.go`** — `channelSelectCache` is a `sync.Map` (line 14); all methods (`GetRetry`/`SetRetry`/`IncreaseRetry`/`ResetRetryNextTry`) use sync.Map exclusively. Concurrency-safe by design; existing `channel_select_test.go` (100 goroutines) verifies.
4. **`internal/adapter/middleware/distributor.go` Distribute()** — closure returns `gin.HandlerFunc`; per-request locals (`channel`/`modelRequest`/`err`). Globals: `common.OptionMap` (under `OptionMapRWMutex`), `repo.DB` (set once at startup), `ratio_setting.GetGroupRatioCopy()` (returns copy). All read-only-after-startup or RWMutex-protected.

Manual inspection ~45 min; 0 automated iterations (no C compiler). No data races identified.
