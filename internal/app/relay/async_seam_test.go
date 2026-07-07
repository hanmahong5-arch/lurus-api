package relay

import (
	"os"
	"testing"

	"github.com/LurusTech/lurus-hub/internal/app"
)

// TestMain forces app.PostConsumeQuota's async side effects to run inline for
// the relay test binary too. Several relay tests call app.PostConsumeQuota,
// whose fire-and-forget gopool goroutines read package globals (e.g.
// common.RedisEnabled) via the cost-spike and quota-notify paths. Running them
// synchronously keeps those reads from outliving the spawning test and racing a
// later test's global-state teardown under the -race gate. See app.AsyncGo.
func TestMain(m *testing.M) {
	app.AsyncGo = func(f func()) { f() }
	os.Exit(m.Run())
}
