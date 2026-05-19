package netacl

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- Allowed() ---

func TestAllowedPreInitPermitsEverything(t *testing.T) {
	a := New() // starts in preInit mode
	cases := []string{"192.168.1.1", "8.8.8.8", "10.0.0.1", "2001:db8::1"}
	for _, ip := range cases {
		if !a.Allowed(ip) {
			t.Errorf("Allowed(%q) = false during preInit, want true", ip)
		}
	}
}

func TestAllowedLoopbackAlwaysAllowed(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	// No user CIDRs configured — only loopback should be allowed.
	cases := []string{"127.0.0.1", "127.0.0.2", "127.255.255.255", "::1"}
	for _, ip := range cases {
		if !a.Allowed(ip) {
			t.Errorf("Allowed(%q) = false, loopback must always be allowed", ip)
		}
	}
}

func TestAllowedBlocksNonLoopbackWhenNoCIDRs(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	cases := []string{"192.168.1.1", "10.0.0.1", "8.8.8.8"}
	for _, ip := range cases {
		if a.Allowed(ip) {
			t.Errorf("Allowed(%q) = true with no CIDRs configured, want false", ip)
		}
	}
}

func TestAllowedPermitsIPInConfiguredCIDR(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	if _, err := a.Reload([]string{"192.168.1.0/24"}, ""); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !a.Allowed("192.168.1.100") {
		t.Error("Allowed(192.168.1.100) = false, should be in 192.168.1.0/24")
	}
	if a.Allowed("192.168.2.1") {
		t.Error("Allowed(192.168.2.1) = true, should be outside 192.168.1.0/24")
	}
}

func TestAllowedBareSingleIP(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	if _, err := a.Reload([]string{"10.0.0.5"}, ""); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if !a.Allowed("10.0.0.5") {
		t.Error("Allowed(10.0.0.5) = false, should match exact configured IP")
	}
	if a.Allowed("10.0.0.6") {
		t.Error("Allowed(10.0.0.6) = true, should not match different IP")
	}
}

func TestAllowedInvalidIPReturnsFalse(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	if a.Allowed("not-an-ip") {
		t.Error("Allowed(not-an-ip) = true, want false")
	}
}

// --- Reload() ---

func TestReloadRejectsMalformedCIDR(t *testing.T) {
	a := New()
	_, err := a.Reload([]string{"not-a-cidr"}, "")
	if err == nil {
		t.Fatal("Reload() with bad CIDR returned nil error, want error")
	}
}

func TestReloadIgnoresEmptyStrings(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	if _, err := a.Reload([]string{"", "  ", "10.0.0.1"}, ""); err != nil {
		t.Fatalf("Reload() with empty entries error = %v", err)
	}
	if !a.Allowed("10.0.0.1") {
		t.Error("Allowed(10.0.0.1) = false, should be in reloaded config")
	}
}

func TestReloadSwapsSnapshotAtomically(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	a.Reload([]string{"10.0.0.1"}, "")
	if !a.Allowed("10.0.0.1") {
		t.Fatal("first reload failed")
	}

	// Second reload replaces the first.
	a.Reload([]string{"10.0.0.2"}, "")
	if a.Allowed("10.0.0.1") {
		t.Error("old IP still allowed after reload")
	}
	if !a.Allowed("10.0.0.2") {
		t.Error("new IP not allowed after reload")
	}
}

func TestReloadLiteralProxyIP(t *testing.T) {
	a := New()
	res, err := a.Reload([]string{}, "192.168.99.1")
	if err != nil {
		t.Fatalf("Reload() with literal proxy IP error = %v", err)
	}
	if len(res.ResolvedProxyIPs) == 0 {
		t.Error("expected resolved proxy IP in result")
	}
}

// --- ClientIP() ---

func makeRequest(remoteAddr, xff string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

func TestClientIPDirectConnection(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	a.Reload([]string{}, "")

	req := makeRequest("10.0.0.1:1234", "")
	ip, ok := a.ClientIP(req)
	if !ok {
		t.Fatal("ClientIP() ok = false, want true")
	}
	if ip != "10.0.0.1" {
		t.Fatalf("ClientIP() = %q, want %q", ip, "10.0.0.1")
	}
}

func TestClientIPTrustedProxyUsesXFF(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	// Configure 10.0.0.2 as a trusted proxy.
	a.Reload([]string{}, "10.0.0.2")

	req := makeRequest("10.0.0.2:5678", "203.0.113.1, 10.0.0.2")
	ip, ok := a.ClientIP(req)
	if !ok {
		t.Fatal("ClientIP() ok = false with trusted proxy, want true")
	}
	// Should return left-most XFF entry (the real client IP).
	if ip != "203.0.113.1" {
		t.Fatalf("ClientIP() = %q, want %q", ip, "203.0.113.1")
	}
}

func TestClientIPTrustedProxyMissingXFFReturnsFalse(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	a.Reload([]string{}, "10.0.0.2")

	req := makeRequest("10.0.0.2:5678", "")
	_, ok := a.ClientIP(req)
	if ok {
		t.Fatal("ClientIP() ok = true with trusted proxy and no XFF, want false")
	}
}

func TestClientIPInvalidXFFReturnsFalse(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	a.Reload([]string{}, "10.0.0.2")

	req := makeRequest("10.0.0.2:5678", "not-an-ip")
	_, ok := a.ClientIP(req)
	if ok {
		t.Fatal("ClientIP() ok = true with invalid XFF value, want false")
	}
}

func TestClientIPMalformedRemoteAddrReturnsFalse(t *testing.T) {
	a := New()
	req := makeRequest("not-a-valid-addr", "")
	_, ok := a.ClientIP(req)
	if ok {
		t.Fatal("ClientIP() ok = true with malformed RemoteAddr, want false")
	}
}

func TestClientIPBareIPRemoteAddr(t *testing.T) {
	a := New()
	a.SetPreInit(false)
	// RemoteAddr without port (shouldn't happen in practice but worth testing).
	req := makeRequest("10.0.0.1", "")
	ip, ok := a.ClientIP(req)
	if !ok {
		t.Fatal("ClientIP() ok = false for bare IP RemoteAddr, want true")
	}
	if ip != "10.0.0.1" {
		t.Fatalf("ClientIP() = %q, want %q", ip, "10.0.0.1")
	}
}

// --- parseCIDR() ---

func TestParseCIDRAcceptsCIDRNotation(t *testing.T) {
	n, err := parseCIDR("192.168.0.0/16")
	if err != nil {
		t.Fatalf("parseCIDR() error = %v", err)
	}
	if !n.Contains(net4("192.168.5.1")) {
		t.Error("parsed CIDR did not contain expected IP")
	}
}

func TestParseCIDRAcceptsBareIPv4(t *testing.T) {
	n, err := parseCIDR("1.2.3.4")
	if err != nil {
		t.Fatalf("parseCIDR() error = %v", err)
	}
	if !n.Contains(net4("1.2.3.4")) {
		t.Error("bare IPv4 not contained in resulting /32")
	}
	if n.Contains(net4("1.2.3.5")) {
		t.Error("/32 should not contain adjacent IP")
	}
}

func TestParseCIDRAcceptsBareIPv6(t *testing.T) {
	n, err := parseCIDR("2001:db8::1")
	if err != nil {
		t.Fatalf("parseCIDR() error = %v", err)
	}
	ones, bits := n.Mask.Size()
	if ones != 128 || bits != 128 {
		t.Fatalf("bare IPv6 should produce /128, got /%d", ones)
	}
}

func TestParseCIDRRejectsGarbage(t *testing.T) {
	if _, err := parseCIDR("not-an-ip"); err == nil {
		t.Fatal("parseCIDR() with garbage input returned nil error")
	}
}

// net4 returns a net.IP from a dotted-decimal string.
func net4(s string) net.IP {
	return net.ParseIP(s)
}
