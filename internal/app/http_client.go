package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/LurusTech/lurus-hub/internal/pkg/common"
	"github.com/LurusTech/lurus-hub/internal/pkg/setting/system_setting"

	"golang.org/x/net/proxy"
)

var (
	httpClient      *http.Client
	proxyClientLock sync.Mutex
	proxyClients    = make(map[string]*http.Client)
)

// maxProxyClients caps the proxy-client cache. Keys are configured proxy URLs
// (from channel config, not attacker-controlled), so this is a safety ceiling
// against unbounded growth — not a hot eviction path. When the cache is full it
// is dropped wholesale (idle connections closed first) and rebuilt on demand;
// entries are cheap to reconstruct, so a coarse reset beats per-key LRU here.
const maxProxyClients = 256

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := system_setting.GetFetchSetting()
	urlStr := req.URL.String()
	if err := common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func InitHttpClient() {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		ForceAttemptHTTP2:   true,
		Proxy:               http.ProxyFromEnvironment, // Support HTTP_PROXY, HTTPS_PROXY, NO_PROXY env vars
	}

	if common.RelayTimeout == 0 {
		httpClient = &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
		}
	} else {
		httpClient = &http.Client{
			Transport:     transport,
			Timeout:       time.Duration(common.RelayTimeout) * time.Second,
			CheckRedirect: checkRedirect,
		}
	}
}

func GetHttpClient() *http.Client {
	return httpClient
}

// GetHttpClientWithProxy returns the default client or a proxy-enabled one when proxyURL is provided.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttpClient(), nil
	}
	return NewProxyHttpClient(proxyURL)
}

// ResetProxyClientCache 清空代理客户端缓存，确保下次使用时重新初始化
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	resetProxyClientsLocked()
}

// resetProxyClientsLocked closes idle connections on every cached client and
// replaces the cache with a fresh empty map. Callers must hold proxyClientLock.
func resetProxyClientsLocked() {
	for _, client := range proxyClients {
		if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
			transport.CloseIdleConnections()
		}
	}
	proxyClients = make(map[string]*http.Client)
}

// storeProxyClient caches client under proxyURL, enforcing maxProxyClients.
// When the cache is already at the bound it is dropped wholesale (idle
// connections closed) before the new entry is inserted, so len(proxyClients)
// never exceeds maxProxyClients.
func storeProxyClient(proxyURL string, client *http.Client) {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	if len(proxyClients) >= maxProxyClients {
		resetProxyClientsLocked()
	}
	proxyClients[proxyURL] = client
}

// NewProxyHttpClient 创建支持代理的 HTTP 客户端
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return http.DefaultClient, nil
	}

	proxyClientLock.Lock()
	if client, ok := proxyClients[proxyURL]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	switch parsedURL.Scheme {
	case "http", "https":
		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        common.RelayMaxIdleConns,
				MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
				ForceAttemptHTTP2:   true,
				Proxy:               http.ProxyURL(parsedURL),
			},
			CheckRedirect: checkRedirect,
		}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		storeProxyClient(proxyURL, client)
		return client, nil

	case "socks5", "socks5h":
		// 获取认证信息
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		// 创建 SOCKS5 代理拨号器
		// proxy.SOCKS5 使用 tcp 参数，所有 TCP 连接包括 DNS 查询都将通过代理进行。行为与 socks5h 相同
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		client := &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        common.RelayMaxIdleConns,
				MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
				ForceAttemptHTTP2:   true,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return dialer.Dial(network, addr)
				},
			},
			CheckRedirect: checkRedirect,
		}
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
		storeProxyClient(proxyURL, client)
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
