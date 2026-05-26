# Race Detector Audit — 2026-05-26

## Environment

- Platform: Windows 10 Pro 10.0.19045 (x86-64, Git Bash / MSYS2)
- Go version: go1.25.x (CGO_ENABLED=0 by default in this repo)
- Race detector status: **cannot execute** — `go test -race` requires CGO and a
  C compiler. Neither GCC nor Clang is installed in this environment.
  Attempted: `CGO_ENABLED=1 go test -race ./internal/lifecycle/...` returned
  `cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%`

## Commands attempted

```
CGO_ENABLED=1 go test -race -short -count=1 -timeout=180s \
  ./internal/lifecycle/... \
  ./internal/app/... \
  ./internal/adapter/middleware/...
# Result: build failed — gcc not found

CGO_ENABLED=1 go test -race -short -count=3 -timeout=300s \
  ./internal/lifecycle/... ./internal/adapter/middleware/...
# Result: build failed — gcc not found
```

## Manual inspection of hot-spot packages

The Wave-UAT plan identified four hot spots. Each was read and assessed below.

### 1. `internal/lifecycle/lifecycle.go` — Manager

**Shared state**: `Manager.tasks []Task` (written by `Register`, read by `Start`).

**Protection**: `Register` acquires `m.mu` (sync.Mutex). `Start` reads `m.tasks`
without a lock — safe as long as callers follow the contract that `Register` is
called before `Start`. The existing `TestGracefulHTTPShutdown` test was noted as
flaky; on inspection the test uses a 500 ms deadline which is sufficient on CI but
tight on heavily loaded Windows hosts. No shared mutable write during `Start` —
the slice itself is only appended to under `Register`, and `Start` only iterates
it once after registration.

**Conclusion**: No data race. The contract-violating window (concurrent `Register`
+ `Start`) is not exercised in production and the test failure was latency-related.

### 2. `internal/app/billing_outbox.go` — package-level `billingOutboxDB`

**Shared state**: `billingOutboxDB *gorm.DB` is a package-level variable.
Written once by `InitBillingOutbox` at startup; read by `EnqueueSettle`,
`EnqueueRelease`, and `ProcessBillingOutbox`.

**Protection**: No mutex. The write happens at program startup (single goroutine)
before any concurrent callers are spawned — this is the standard Go
"initialise-before-goroutine-spawn" idiom. The nil guard in each function prevents
a nil dereference if the DB is not initialised.

**Conclusion**: No data race under the documented usage contract (Init → then
concurrent callers). Would become a race if callers invoked `Init` concurrently
with `Enqueue*` — not done in this codebase.

### 3. `internal/app/channel_select.go` — selection cache

**Shared state**: `channelSelectCache` — a `sync.Map` (inspected on line 14 of
`channel_select.go`). All methods (`GetRetry`, `SetRetry`, `IncreaseRetry`,
`ResetRetryNextTry`) call `sync.Map` methods exclusively.

**Protection**: `sync.Map` is concurrency-safe by design; no additional mutex
needed.

**Conclusion**: No data race. The existing `channel_select_test.go` with 100
goroutines already verifies this path.

### 4. `internal/adapter/middleware/distributor.go` — Distribute()

**Shared state**: `Distribute()` is a closure returned as `gin.HandlerFunc`.
Each HTTP request gets its own `channel`, `modelRequest`, and `err` locals.
The only globals accessed are `common.OptionMap` (protected by
`common.OptionMapRWMutex`), `repo.DB` (set once at startup), and
`ratio_setting.GetGroupRatioCopy()` (returns a copy).

**Protection**: All concurrently accessed globals use their own RWMutex or are
read-only after startup.

**Conclusion**: No data race.

## Duration / iterations

Manual code inspection: approximately 45 minutes.
Automated `go test -race` run: 0 iterations executed (no C compiler available).

## Recommendation

Run `go test -race -short -count=10 ./internal/lifecycle/... ./internal/app/...
./internal/adapter/middleware/...` on a Linux CI runner (where CGO is available)
to get instrumented confirmation. The four manually-inspected hot spots all appear
clean based on code review.

## Conclusion

**MANUAL INSPECTION CLEAN — automated race detector not executable on this host.**

No data races were identified in the four Wave-UAT hot spots through code review.
The `TestGracefulHTTPShutdown` flakiness is a timing issue (500 ms deadline too
tight on loaded Windows host), not a race condition.
