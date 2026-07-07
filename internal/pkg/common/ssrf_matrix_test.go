package common

import (
	"net"
	"strings"
	"testing"
)

// TestSSRFPrivateIPMatrix asserts every private / loopback / link-local /
// reserved range is treated as private (denied by default) and public IPs are
// not — the core guard that stops relay traffic from reaching internal
// metadata / cluster endpoints.
func TestSSRFPrivateIPMatrix(t *testing.T) {
	private := []string{
		"10.0.0.1",        // RFC1918 /8
		"172.16.5.5",      // RFC1918 /12
		"192.168.1.1",     // RFC1918 /16
		"127.0.0.1",       // loopback
		"169.254.169.254", // link-local (cloud metadata!)
		"224.0.0.1",       // multicast
		"240.0.0.1",       // reserved
		"::1",             // IPv6 loopback
		"fe80::1",         // IPv6 link-local
		"fc00::1",         // IPv6 unique local
		"fd12::1",         // IPv6 unique local
	}
	for _, s := range private {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("bad test IP %q", s)
		}
		if !isPrivateIP(ip) {
			t.Errorf("%s must be classified private", s)
		}
	}
	public := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2001:4860:4860::8888"}
	for _, s := range public {
		ip := net.ParseIP(s)
		if isPrivateIP(ip) {
			t.Errorf("%s must be classified public", s)
		}
	}
}

func TestParsePortRanges(t *testing.T) {
	ports, err := parsePortRanges([]string{"80", "443", "8000-8002", "  ", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int]bool{80: true, 443: true, 8000: true, 8001: true, 8002: true}
	if len(ports) != len(want) {
		t.Errorf("got %v", ports)
	}
	for _, p := range ports {
		if !want[p] {
			t.Errorf("unexpected port %d", p)
		}
	}

	bad := [][]string{
		{"notaport"},
		{"0"},
		{"70000"},
		{"100-50"},  // start > end
		{"1-2-3"},   // malformed range
		{"abc-100"}, // bad start
		{"100-xyz"}, // bad end
		{"1-70000"}, // out of range end
	}
	for _, b := range bad {
		if _, err := parsePortRanges(b); err == nil {
			t.Errorf("parsePortRanges(%v) should error", b)
		}
	}
}

func TestIsAllowedPort(t *testing.T) {
	open := &SSRFProtection{AllowedPorts: []int{}}
	if !open.isAllowedPort(12345) {
		t.Error("empty allow list means all ports allowed")
	}
	restricted := &SSRFProtection{AllowedPorts: []int{443}}
	if !restricted.isAllowedPort(443) {
		t.Error("443 should be allowed")
	}
	if restricted.isAllowedPort(22) {
		t.Error("22 should not be allowed")
	}
}

func TestIsDomainListedAndAllowed(t *testing.T) {
	list := []string{"example.com", "*.trusted.org", ""}
	if !isDomainListed("example.com", list) {
		t.Error("exact match failed")
	}
	if !isDomainListed("EXAMPLE.COM", list) {
		t.Error("match should be case-insensitive")
	}
	if !isDomainListed("api.trusted.org", list) {
		t.Error("wildcard subdomain match failed")
	}
	if !isDomainListed("trusted.org", list) {
		t.Error("wildcard should also match apex")
	}
	if isDomainListed("evil.com", list) {
		t.Error("unlisted domain must not match")
	}
	if isDomainListed("x", nil) {
		t.Error("empty list matches nothing")
	}

	// Whitelist mode: only listed allowed.
	wl := &SSRFProtection{DomainFilterMode: true, DomainList: []string{"example.com"}}
	if !wl.isDomainAllowed("example.com") || wl.isDomainAllowed("evil.com") {
		t.Error("whitelist domain allow logic wrong")
	}
	// Blacklist mode: listed denied.
	bl := &SSRFProtection{DomainFilterMode: false, DomainList: []string{"evil.com"}}
	if bl.isDomainAllowed("evil.com") || !bl.isDomainAllowed("good.com") {
		t.Error("blacklist domain allow logic wrong")
	}
}

func TestIsIPAccessAllowed(t *testing.T) {
	// Private IP denied unless explicitly allowed.
	deny := &SSRFProtection{AllowPrivateIp: false, IpFilterMode: false}
	if deny.IsIPAccessAllowed(net.ParseIP("10.0.0.1")) {
		t.Error("private IP must be denied by default")
	}
	allowPriv := &SSRFProtection{AllowPrivateIp: true, IpFilterMode: false, IpList: []string{}}
	if !allowPriv.IsIPAccessAllowed(net.ParseIP("10.0.0.1")) {
		t.Error("private IP should pass when AllowPrivateIp and blacklist-empty")
	}
	// Whitelist mode: only listed public IP allowed.
	wl := &SSRFProtection{IpFilterMode: true, IpList: []string{"8.8.8.8"}}
	if !wl.IsIPAccessAllowed(net.ParseIP("8.8.8.8")) {
		t.Error("whitelisted public IP should be allowed")
	}
	if wl.IsIPAccessAllowed(net.ParseIP("1.1.1.1")) {
		t.Error("non-whitelisted public IP must be denied")
	}
}

// TestValidateURL_Matrix covers the ValidateURL branches without any DNS: bad
// scheme, disallowed port, private-IP host (metadata guard), and a public-IP
// host in blacklist mode (ApplyIPFilterForDomain off so no LookupIP).
func TestValidateURL_Matrix(t *testing.T) {
	p := &SSRFProtection{
		AllowPrivateIp:   false,
		DomainFilterMode: false, // blacklist (empty) so public domains pass
		IpFilterMode:     false, // blacklist (empty) so public IPs pass
		AllowedPorts:     []int{80, 443},
	}

	if err := p.ValidateURL("ftp://example.com"); err == nil ||
		!strings.Contains(err.Error(), "unsupported protocol") {
		t.Errorf("ftp scheme should be rejected: %v", err)
	}
	if err := p.ValidateURL("://bad url with spaces"); err == nil {
		t.Error("malformed URL should error")
	}
	if err := p.ValidateURL("http://example.com:22"); err == nil ||
		!strings.Contains(err.Error(), "port 22 is not allowed") {
		t.Errorf("disallowed port should be rejected: %v", err)
	}
	// Private IP host — the SSRF-critical case.
	if err := p.ValidateURL("http://169.254.169.254/latest/meta-data/"); err == nil ||
		!strings.Contains(err.Error(), "private IP") {
		t.Errorf("metadata endpoint must be blocked: %v", err)
	}
	if err := p.ValidateURL("http://127.0.0.1"); err == nil {
		t.Error("loopback host must be blocked")
	}
	// Public IP host with permissive blacklist passes without DNS.
	if err := p.ValidateURL("http://8.8.8.8:80"); err != nil {
		t.Errorf("public IP on allowed port should pass: %v", err)
	}
	// Public domain host, ApplyIPFilterForDomain off -> no DNS, passes.
	if err := p.ValidateURL("https://example.com"); err != nil {
		t.Errorf("public domain should pass: %v", err)
	}

	// Whitelist domain mode: unlisted domain rejected.
	wl := &SSRFProtection{
		DomainFilterMode: true,
		DomainList:       []string{"allowed.com"},
		IpFilterMode:     false,
		AllowedPorts:     []int{443},
	}
	if err := wl.ValidateURL("https://blocked.com"); err == nil ||
		!strings.Contains(err.Error(), "not in whitelist") {
		t.Errorf("unlisted domain should be rejected: %v", err)
	}
	if err := wl.ValidateURL("https://allowed.com"); err != nil {
		t.Errorf("whitelisted domain should pass: %v", err)
	}
}

func TestValidateURLWithFetchSetting(t *testing.T) {
	// Protection disabled: always nil, even for a metadata endpoint.
	if err := ValidateURLWithFetchSetting("http://169.254.169.254/", false,
		false, false, false, nil, nil, nil, false); err != nil {
		t.Errorf("disabled protection should pass everything: %v", err)
	}
	// Enabled, bad port config bubbles up.
	if err := ValidateURLWithFetchSetting("http://example.com", true,
		false, false, false, nil, nil, []string{"bad"}, false); err == nil ||
		!strings.Contains(err.Error(), "invalid port configuration") {
		t.Errorf("bad port config should error: %v", err)
	}
	// Enabled, blocks private IP host.
	if err := ValidateURLWithFetchSetting("http://10.0.0.1", true,
		false, false, false, nil, nil, []string{"80"}, false); err == nil {
		t.Error("enabled protection should block private IP")
	}
	// Enabled, allows a public IP with permissive lists.
	if err := ValidateURLWithFetchSetting("http://8.8.8.8:80", true,
		false, false, false, nil, nil, []string{"80"}, false); err != nil {
		t.Errorf("public IP should pass: %v", err)
	}
}
