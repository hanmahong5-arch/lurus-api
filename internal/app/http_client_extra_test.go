package app

// http_client_extra_test.go — remaining NewProxyHttpClient branches: the
// cache-hit fast path, socks5 with embedded credentials, and the
// unsupported-scheme error. No network: SOCKS5 dialers are lazy.

import (
	"testing"
)

func TestNewProxyHttpClient_CacheHit(t *testing.T) {
	ResetProxyClientCache()
	const u = "http://proxy.example.com:8080"

	c1, err := NewProxyHttpClient(u)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	c2, err := NewProxyHttpClient(u)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	// Second call must return the cached instance, not a new one.
	if c1 != c2 {
		t.Error("expected the cached client to be returned on the second call")
	}
	ResetProxyClientCache()
}

func TestNewProxyHttpClient_SOCKS5WithAuth(t *testing.T) {
	ResetProxyClientCache()
	c, err := NewProxyHttpClient("socks5://user:pass@127.0.0.1:1080")
	if err != nil {
		t.Fatalf("socks5 with auth: %v", err)
	}
	if c == nil {
		t.Fatal("expected a client for socks5 proxy with auth")
	}
	ResetProxyClientCache()
}

func TestNewProxyHttpClient_UnsupportedScheme(t *testing.T) {
	if _, err := NewProxyHttpClient("ftp://proxy.example.com:21"); err == nil {
		t.Fatal("expected error for unsupported proxy scheme")
	}
}

func TestGetHttpClientWithProxy_EmptyReturnsDefault(t *testing.T) {
	if GetHttpClient() == nil {
		InitHttpClient()
	}
	c, err := GetHttpClientWithProxy("")
	if err != nil {
		t.Fatalf("empty proxy: %v", err)
	}
	if c != GetHttpClient() {
		t.Error("empty proxy URL must return the default shared client")
	}
}
