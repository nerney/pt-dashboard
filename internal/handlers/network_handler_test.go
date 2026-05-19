package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nerney/ptv/internal/config"
	"github.com/nerney/ptv/internal/netacl"
)

// ── parseCIDRTextarea ────────────────────────────────────────────────────────

func TestParseCIDRTextarea(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single", "10.0.0.0/8", []string{"10.0.0.0/8"}},
		{"multiple lines", "10.0.0.0/8\n192.168.0.0/16", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"blank lines stripped", "10.0.0.0/8\n\n192.168.0.0/16\n", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"whitespace trimmed", "  10.0.0.0/8  \n  192.168.0.0/16  ", []string{"10.0.0.0/8", "192.168.0.0/16"}},
		{"all blank", "  \n  \n  ", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseCIDRTextarea(c.input)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

// ── buildNetworkFlashMsg ─────────────────────────────────────────────────────

func TestBuildNetworkFlashMsg(t *testing.T) {
	t.Run("no proxy note", func(t *testing.T) {
		got := buildNetworkFlashMsg(netacl.ReloadResult{})
		if got != "Network config saved." {
			t.Fatalf("got %q, want %q", got, "Network config saved.")
		}
	})
	t.Run("with proxy note", func(t *testing.T) {
		got := buildNetworkFlashMsg(netacl.ReloadResult{ProxyNote: "resolve failed: no such host"})
		if !strings.Contains(got, "Proxy:") {
			t.Fatalf("expected proxy note in message, got %q", got)
		}
	})
}

// ── networkPage ──────────────────────────────────────────────────────────────

func TestNetworkPageRenders(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	// Seed the netacl so the page has something to show.
	n := &config.NetACL{AllowedCIDRs: []string{"10.0.0.0/8"}, Confirmed: true}
	if err := h.store.SaveNetACL(n); err != nil {
		t.Fatalf("SaveNetACL() error = %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/config/app/network", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	h.networkPage(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "network") {
		t.Fatalf("expected network template content, got %q", w.Body.String())
	}
}

// ── networkSubmit ─────────────────────────────────────────────────────────────

func TestNetworkSubmitInvalidCIDRFlashesError(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	form := "cidrs=not-a-cidr&proxy_host="
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/config/app/network", form)
	r.RemoteAddr = "127.0.0.1:1234"
	h.networkSubmit(w, r)
	// Error path: flash redirects with ?err=...
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "err=") {
		t.Fatalf("Location = %q, want err param", loc)
	}
}

func TestNetworkSubmitValidCIDRSaves(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	// First save to set Confirmed=true so we know the redirect target.
	h.store.SaveNetACL(&config.NetACL{AllowedCIDRs: []string{"127.0.0.1/32"}, Confirmed: true})

	form := "cidrs=10.0.0.0%2F8&proxy_host="
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/config/app/network", form)
	r.RemoteAddr = "127.0.0.1:1234"
	h.networkSubmit(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	loc := w.Header().Get("Location")
	if strings.Contains(loc, "err=") {
		t.Fatalf("unexpected error redirect: %q", loc)
	}
	// ACL should now contain the new CIDR.
	if !h.acl.Allowed("10.5.5.5") {
		t.Error("ACL did not reload with new CIDR")
	}
}

func TestNetworkSubmitFirstRunRedirectsToConfig(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	// Confirmed is false on a fresh store — first save should redirect to /config.
	form := "cidrs=10.0.0.0%2F8&proxy_host="
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/config/app/network", form)
	r.RemoteAddr = "127.0.0.1:1234"
	h.networkSubmit(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.HasPrefix(loc, "/config") {
		t.Fatalf("first-run redirect = %q, want /config...", loc)
	}
}
