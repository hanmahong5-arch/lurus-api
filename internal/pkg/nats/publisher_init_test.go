package nats

import (
	"net"
	"strings"
	"sync"
	"testing"
)

// resetInitGlobals swaps the package-level once/global/err state for the
// duration of the test so Init's guarded-by-sync.Once body actually runs, and
// restores the previous state afterwards. Reaching into the unexported globals
// is why this lives in-package.
func resetInitGlobals(t *testing.T) {
	t.Helper()
	// sync.Once cannot be copied (copylocks); restore to a fresh Once, which is
	// the clean "Init not yet run" baseline, instead of snapshotting the value.
	prevGlobal := global
	prevErr := globalErr
	globalOnce = sync.Once{}
	global = nil
	globalErr = nil
	t.Cleanup(func() {
		globalOnce = sync.Once{}
		global = prevGlobal
		globalErr = prevErr
	})
}

// freeRefusedAddr returns a host:port that is guaranteed to refuse TCP
// connections: it binds an ephemeral port, records it, then closes the
// listener so the port is free but nothing is listening.
func freeRefusedAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}
	return addr
}

// TestInit_ConnectErrorReturnsError proves the business contract: when NATS is
// enabled but the broker is unreachable, Init must surface a non-nil error
// (naming the URL) rather than silently returning nil — so the caller can
// degrade instead of believing the publisher is live. It also asserts the
// global stays nil on failure, so Get() keeps reporting "not initialised".
func TestInit_ConnectErrorReturnsError(t *testing.T) {
	resetInitGlobals(t)

	refused := "nats://" + freeRefusedAddr(t)
	t.Setenv("LLM_QUOTA_NATS_ENABLED", "true")
	t.Setenv("NATS_URL", refused)

	err := Init()
	if err == nil {
		t.Fatal("Init() with an unreachable broker must return a non-nil error, got nil (silent success would hide the outage from the caller)")
	}
	if !strings.Contains(err.Error(), "nats connect") {
		t.Errorf("error should identify the connect failure, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), refused) {
		t.Errorf("error should name the URL that failed (%s), got %q", refused, err.Error())
	}
	if global != nil {
		t.Error("global publisher must remain nil after a failed connect")
	}
	if Get() != nil {
		t.Error("Get() must report nil (not initialised) after a failed connect")
	}

	// sync.Once semantics: a second Init() call returns the same cached error
	// without re-dialing, so the caller sees a stable failure state.
	if err2 := Init(); err2 == nil || err2.Error() != err.Error() {
		t.Errorf("second Init() must return the same cached error; got %v", err2)
	}
}
