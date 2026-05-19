package handlers

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nerney/ptv/internal/auth"
	"github.com/nerney/ptv/internal/config"
	"github.com/nerney/ptv/internal/logger"
	"github.com/nerney/ptv/internal/netacl"
	"github.com/nerney/ptv/internal/prowlarr"
)

// newAuthTestHandler builds a Handler with full auth fields for HTTP tests.
func newAuthTestHandler(t *testing.T, cfg config.Config) *Handler {
	t.Helper()
	store := newTestStore(t, cfg)
	h := &Handler{
		store:   store,
		log:     logger.New(),
		acl:     netacl.New(),
		limiter: auth.NewRateLimiter(),
		pSchemas: map[string]prowlarr.IndexerSchema{},
	}
	h.sessions = auth.NewManager(func() { store.Lock() })
	h.templates = map[string]*template.Template{
		"login": template.Must(template.New("layout").Funcs(templateFuncs()).Parse(
			`{{define "layout"}}login:{{.Error}}{{end}}`,
		)),
		"setup": template.Must(template.New("layout").Funcs(templateFuncs()).Parse(
			`{{define "layout"}}setup:{{.Error}}{{end}}`,
		)),
		"config_network": template.Must(template.New("layout").Funcs(templateFuncs()).Parse(
			`{{define "layout"}}network{{end}}`,
		)),
	}
	return h
}

// ── validateSetupInput ──────────────────────────────────────────────────────

func TestValidateSetupInput(t *testing.T) {
	cases := []struct {
		name     string
		user     string
		pass     string
		confirm  string
		wantOK   bool
		wantMsg  string
	}{
		{"valid", "alice", "password1", "password1", true, ""},
		{"empty user", "", "password1", "password1", false, "Username and password are required."},
		{"empty password", "alice", "", "", false, "Username and password are required."},
		{"short password", "alice", "short", "short", false, "Password must be at least 8 characters."},
		{"mismatch", "alice", "password1", "password2", false, "Passwords do not match."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg, ok := validateSetupInput(c.user, c.pass, c.confirm)
			if ok != c.wantOK {
				t.Errorf("ok = %v, want %v", ok, c.wantOK)
			}
			if c.wantMsg != "" && msg != c.wantMsg {
				t.Errorf("msg = %q, want %q", msg, c.wantMsg)
			}
			if c.wantOK && msg != "" {
				t.Errorf("ok=true but got non-empty msg %q", msg)
			}
		})
	}
}

// ── /setup ─────────────────────────────────────────────────────────────────

func TestSetupPageRedirectsToLoginWhenInitialized(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	// Store is already initialized by newTestStore.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/setup", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	h.setupPage(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestSetupSubmitRedirectsToLoginWhenInitialized(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/setup", "username=alice&password=hunter12&confirm=hunter12")
	r.RemoteAddr = "127.0.0.1:1234"
	h.setupSubmit(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
}

func TestSetupSubmitValidationErrors(t *testing.T) {
	// Use a fresh uninitialized store.
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	h := &Handler{
		store:   store,
		log:     logger.New(),
		acl:     netacl.New(),
		limiter: auth.NewRateLimiter(),
		pSchemas: map[string]prowlarr.IndexerSchema{},
	}
	h.sessions = auth.NewManager(nil)
	h.templates = map[string]*template.Template{
		"setup": template.Must(template.New("layout").Funcs(templateFuncs()).Parse(
			`{{define "layout"}}err:{{.Error}}{{end}}`,
		)),
	}

	// Empty password → renders error.
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/setup", "username=alice&password=&confirm=")
	r.RemoteAddr = "127.0.0.1:1234"
	h.setupSubmit(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "required") {
		t.Fatalf("expected 'required' in body, got %q", w.Body.String())
	}
}

// ── /login ─────────────────────────────────────────────────────────────────

func TestLoginPageRedirectsToSetupWhenUninitialized(t *testing.T) {
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	h := &Handler{
		store:    store,
		log:      logger.New(),
		pSchemas: map[string]prowlarr.IndexerSchema{},
	}
	h.sessions = auth.NewManager(nil)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	h.loginPage(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/setup" {
		t.Fatalf("Location = %q, want /setup", loc)
	}
}

func TestLoginPageRendersWhenInitialized(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	h.loginPage(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "login:") {
		t.Fatalf("expected login template, got %q", w.Body.String())
	}
}

func TestLoginSubmitRedirectsToSetupWhenUninitialized(t *testing.T) {
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	h := &Handler{
		store:    store,
		log:      logger.New(),
		limiter:  auth.NewRateLimiter(),
		pSchemas: map[string]prowlarr.IndexerSchema{},
	}
	h.sessions = auth.NewManager(nil)
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/login", "username=alice&password=hunter2")
	r.RemoteAddr = "127.0.0.1:1234"
	h.loginSubmit(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/setup" {
		t.Fatalf("Location = %q, want /setup", loc)
	}
}

func TestLoginSubmitRateLimitBlocks(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	// Exhaust rate limiter for this IP.
	for i := 0; i < 5; i++ {
		h.limiter.RecordFailure("10.0.0.1")
	}
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/login", "username=alice&password=hunter2")
	r.RemoteAddr = "10.0.0.1:1234"
	h.loginSubmit(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", w.Code)
	}
}

func TestLoginSubmitEmptyCredentials(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/login", "username=&password=")
	r.RemoteAddr = "127.0.0.1:1234"
	h.loginSubmit(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Incorrect") {
		t.Fatalf("expected error message, got %q", w.Body.String())
	}
}

func TestLoginSubmitRejectsSecondSession(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	// Start a session first.
	h.sessions.Begin()
	w := httptest.NewRecorder()
	r := newFormRequest(http.MethodPost, "/login", "username=tester&password=test-password")
	r.RemoteAddr = "127.0.0.1:1234"
	h.loginSubmit(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for second login", w.Code)
	}
}

// ── /logout ────────────────────────────────────────────────────────────────

func TestLogoutClearsSessionAndRedirects(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	id, err := h.sessions.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	h.logoutSubmit(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("Location = %q, want /login", loc)
	}
	if err := h.sessions.Validate(id); err == nil {
		t.Fatal("session still valid after logout")
	}
}

func TestLogoutIdempotent(t *testing.T) {
	h := newAuthTestHandler(t, config.Config{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/logout", nil)
	r.RemoteAddr = "127.0.0.1:1234"
	h.logoutSubmit(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
}
