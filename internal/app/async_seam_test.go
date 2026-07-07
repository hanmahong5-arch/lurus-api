package app

import (
	"os"
	"testing"
)

// TestMain forces PostConsumeQuota's async side effects (see AsyncGo in
// quota.go) to run inline for the whole app test binary. Under the -race gate
// the production gopool goroutines read package globals such as
// common.RedisEnabled; running them synchronously guarantees those reads finish
// before the spawning test returns, so they cannot race a later test's
// setup/teardown that restores those globals. Production behaviour is unchanged
// (AsyncGo stays gopool.Go outside tests).
func TestMain(m *testing.M) {
	AsyncGo = func(f func()) { f() }
	os.Exit(m.Run())
}
