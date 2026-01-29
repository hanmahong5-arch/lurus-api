package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/lurus-api/internal/pkg/common"
)

func TestHttpClient_Init(t *testing.T) {
	common.RelayTimeout = 0
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	InitHttpClient()

	client := GetHttpClient()
	if client == nil {
		t.Fatal("expected non-nil http client after InitHttpClient()")
	}
}

func TestHttpClient_ProxyFromEnv(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:8888")

	common.RelayTimeout = 0
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	// Should not panic even with an unreachable proxy
	InitHttpClient()

	client := GetHttpClient()
	if client == nil {
		t.Fatal("expected non-nil http client with proxy env set")
	}
}

func TestHttpClient_NoProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")

	common.RelayTimeout = 0
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	InitHttpClient()

	client := GetHttpClient()
	if client == nil {
		t.Fatal("expected non-nil http client without proxy env")
	}
}

func TestHttpClient_Timeout(t *testing.T) {
	common.RelayTimeout = 30
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	InitHttpClient()

	client := GetHttpClient()
	if client == nil {
		t.Fatal("expected non-nil http client")
	}
	expected := 30 * time.Second
	if client.Timeout != expected {
		t.Fatalf("expected timeout %v, got %v", expected, client.Timeout)
	}
}

func TestHttpClient_NewProxyClient_HTTP(t *testing.T) {
	common.RelayTimeout = 10
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	ResetProxyClientCache()

	client, err := NewProxyHttpClient("http://proxy:8080")
	if err != nil {
		t.Fatalf("unexpected error for http proxy: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client for http proxy")
	}
}

func TestHttpClient_NewProxyClient_SOCKS5(t *testing.T) {
	common.RelayTimeout = 10
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	ResetProxyClientCache()

	client, err := NewProxyHttpClient("socks5://proxy:1080")
	if err != nil {
		t.Fatalf("unexpected error for socks5 proxy: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client for socks5 proxy")
	}
}

func TestHttpClient_NewProxyClient_Invalid(t *testing.T) {
	common.RelayTimeout = 10
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	ResetProxyClientCache()

	_, err := NewProxyHttpClient("ftp://proxy:21")
	if err == nil {
		t.Fatal("expected error for unsupported proxy scheme ftp")
	}
}

func TestHttpClient_NewProxyClient_Empty(t *testing.T) {
	ResetProxyClientCache()

	client, err := NewProxyHttpClient("")
	if err != nil {
		t.Fatalf("unexpected error for empty proxy URL: %v", err)
	}
	if client != http.DefaultClient {
		t.Fatal("expected default http client for empty proxy URL")
	}
}

func TestHttpClient_GetHttpClientWithProxy_Empty(t *testing.T) {
	common.RelayTimeout = 0
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	InitHttpClient()

	client, err := GetHttpClientWithProxy("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != GetHttpClient() {
		t.Fatal("expected GetHttpClient() result when proxy URL is empty")
	}
}

func TestHttpClient_GetHttpClientWithProxy_WithURL(t *testing.T) {
	common.RelayTimeout = 10
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 50

	ResetProxyClientCache()

	client, err := GetHttpClientWithProxy("http://proxy:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client for proxy URL")
	}
}
