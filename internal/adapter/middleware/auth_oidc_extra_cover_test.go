package middleware

import (
	"context"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// -------------------- WssAuth --------------------

// WssAuth is a no-op passthrough today; exercising it guards against a future
// change silently aborting websocket-upgrade requests.
func TestWssAuth_Passthrough(t *testing.T) {
	c, w := newTestContext(http.MethodGet, "/realtime", "", "")
	c.Request.Header.Set("Connection", "Upgrade")
	c.Request.Header.Set("Upgrade", "websocket")
	c.Request.Header.Set("Sec-WebSocket-Protocol", "realtime, openai-insecure-api-key.sk-abc")

	WssAuth(c)

	if c.IsAborted() {
		t.Errorf("WssAuth must not abort the request")
	}
	if w.Code != http.StatusOK {
		t.Errorf("WssAuth wrote status %d, want untouched 200", w.Code)
	}
}

// -------------------- InitOIDCAuth (enabled paths) --------------------

// saveOIDCGlobals snapshots and restores every package-level OIDC global that
// InitOIDCAuth mutates, plus the relevant env vars, so the enabled-path tests
// stay -count=1 safe and never leak state into the rest of the suite.
func saveOIDCGlobals(t *testing.T) func() {
	t.Helper()
	prevEnabled := oidcEnabled
	prevIssuers := oidcIssuers
	prevJwks := oidcJwksURI
	prevClient := oidcClientID
	prevAud := oidcAllowedAudiences
	prevMgr := jwksManager
	prevKeys := oidcClaimKeys

	envKeys := []string{"OIDC_ENABLED", "OIDC_ISSUER", "OIDC_JWKS_URI", "OIDC_CLIENT_ID", "OIDC_ALLOWED_AUDIENCES"}
	prevEnv := map[string]*string{}
	for _, k := range envKeys {
		if v, ok := os.LookupEnv(k); ok {
			vv := v
			prevEnv[k] = &vv
		} else {
			prevEnv[k] = nil
		}
	}

	return func() {
		oidcEnabled = prevEnabled
		oidcIssuers = prevIssuers
		oidcJwksURI = prevJwks
		oidcClientID = prevClient
		oidcAllowedAudiences = prevAud
		jwksManager = prevMgr
		oidcClaimKeys = prevKeys
		jwksManagerOnce = sync.Once{}
		for k, v := range prevEnv {
			if v == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *v)
			}
		}
	}
}

// resetForInit clears the accumulating slice globals so a fresh InitOIDCAuth
// call observes only the env we set (InitOIDCAuth appends to oidcIssuers).
func resetForInit() {
	oidcIssuers = nil
	oidcAllowedAudiences = nil
	jwksManager = nil
	jwksManagerOnce = sync.Once{}
}

func TestInitOIDCAuth_MissingIssuer(t *testing.T) {
	defer saveOIDCGlobals(t)()
	resetForInit()
	_ = os.Setenv("OIDC_ENABLED", "true")
	_ = os.Setenv("OIDC_ISSUER", "  ,  ") // all-whitespace entries dropped → none
	_ = os.Setenv("OIDC_JWKS_URI", "https://idp.example/jwks")
	_ = os.Setenv("OIDC_CLIENT_ID", "cid")

	if err := InitOIDCAuth(); err == nil {
		t.Errorf("expected error when no issuer resolves")
	}
}

func TestInitOIDCAuth_MissingJwksURI(t *testing.T) {
	defer saveOIDCGlobals(t)()
	resetForInit()
	_ = os.Setenv("OIDC_ENABLED", "true")
	_ = os.Setenv("OIDC_ISSUER", "https://idp.example")
	_ = os.Unsetenv("OIDC_JWKS_URI")
	_ = os.Setenv("OIDC_CLIENT_ID", "cid")

	if err := InitOIDCAuth(); err == nil {
		t.Errorf("expected error when OIDC_JWKS_URI unset")
	}
}

func TestInitOIDCAuth_MissingClientID(t *testing.T) {
	defer saveOIDCGlobals(t)()
	resetForInit()
	_ = os.Setenv("OIDC_ENABLED", "true")
	_ = os.Setenv("OIDC_ISSUER", "https://idp.example")
	_ = os.Setenv("OIDC_JWKS_URI", "https://idp.example/jwks")
	_ = os.Unsetenv("OIDC_CLIENT_ID")

	if err := InitOIDCAuth(); err == nil {
		t.Errorf("expected error when OIDC_CLIENT_ID unset")
	}
}

// The full success path: two comma-separated issuers + an aud allow-list, with
// OIDC_JWKS_URI pointing at a live test JWKS server. Asserts the parsed globals
// and that the JWKS manager was constructed (jwksManagerOnce.Do ran).
func TestInitOIDCAuth_Success(t *testing.T) {
	defer saveOIDCGlobals(t)()
	resetForInit()

	_, pub := generateTestRSAKeyPair(t)
	jwks := JWKSet{Keys: []JWK{rsaPublicKeyToJWK(pub, "init-kid")}}
	srv := createTestJWKSServer(t, jwks)

	// Pre-build the JWKS manager as a plain struct — deliberately NOT via
	// NewJWKSManagerWithContext — so it spawns no background auto-refresh
	// goroutine. That goroutine reads the package global jwksRefreshInterval at
	// startup, and (SafeGoWithContext offers no join, so cancel only signals) it
	// can outlive this test and race the very next test, which mutates
	// jwksRefreshInterval. getKeyWithRefresh below fetches synchronously, which
	// is all the success-path assertion needs. Consuming jwksManagerOnce makes
	// InitOIDCAuth's own manager construction a no-op.
	jwksManager = &JWKSManager{
		jwksURI:            srv.URL,
		publicKeys:         make(map[string]*rsa.PublicKey),
		minRefreshInterval: 30 * time.Second,
	}
	jwksManagerOnce.Do(func() {})

	_ = os.Setenv("OIDC_ENABLED", "true")
	_ = os.Setenv("OIDC_ISSUER", "https://old.idp , https://new.idp")
	_ = os.Setenv("OIDC_JWKS_URI", srv.URL)
	_ = os.Setenv("OIDC_CLIENT_ID", "my-client")
	_ = os.Setenv("OIDC_ALLOWED_AUDIENCES", "aud-a, aud-b")

	if err := InitOIDCAuth(); err != nil {
		t.Fatalf("InitOIDCAuth success path returned error: %v", err)
	}
	if !oidcEnabled {
		t.Errorf("oidcEnabled must be true")
	}
	if len(oidcIssuers) != 2 || oidcIssuers[0] != "https://old.idp" || oidcIssuers[1] != "https://new.idp" {
		t.Errorf("oidcIssuers = %v, want two trimmed issuers", oidcIssuers)
	}
	if len(oidcAllowedAudiences) != 2 {
		t.Errorf("oidcAllowedAudiences = %v, want two entries", oidcAllowedAudiences)
	}
	if oidcClientID != "my-client" {
		t.Errorf("oidcClientID = %q, want my-client", oidcClientID)
	}
	if jwksManager == nil {
		t.Errorf("jwksManager must be constructed on the success path")
	} else if key, err := jwksManager.getKeyWithRefresh("init-kid"); err != nil || key == nil {
		t.Errorf("manager should have fetched init-kid: err=%v key=%v", err, key)
	}
}

// -------------------- JWKS auto-refresh --------------------

// autoRefreshWithContext must actually re-fetch on each tick. Drive it with a
// short interval and a hit-counting server, then cancel to stop the goroutine.
func TestAutoRefreshWithContext_TickReFetches(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`)) // empty is fine; we only count fetches
	}))
	defer srv.Close()

	prevInterval := jwksRefreshInterval
	jwksRefreshInterval = 15 * time.Millisecond
	defer func() { jwksRefreshInterval = prevInterval }()

	m := &JWKSManager{
		jwksURI:            srv.URL,
		publicKeys:         make(map[string]*rsa.PublicKey),
		minRefreshInterval: 30 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.autoRefreshWithContext(ctx)
		close(done)
	}()

	// Wait for at least two ticks worth of re-fetches.
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt64(&hits) < 2 {
		select {
		case <-deadline:
			t.Fatalf("auto-refresh only fetched %d times, want >=2", atomic.LoadInt64(&hits))
		case <-time.After(5 * time.Millisecond):
		}
	}

	// Cancel must drive the loop to return promptly (context-done branch).
	cancel()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("autoRefreshWithContext did not stop after cancel")
	}
}
