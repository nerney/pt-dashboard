package handlers

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/nerney/ptv/internal/config"
)

func TestProwlarrMetadataCache(t *testing.T) {
	var profileHits atomic.Int32
	var tagHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/appprofile":
			profileHits.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":1,"name":"Default"}]`))
		case "/api/v1/tag":
			tagHits.Add(1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"id":2,"label":"ptv"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	cfg := config.Config{
		ProwlarrURL:    srv.URL,
		ProwlarrAPIKey: "key-a",
	}
	h := newTestHandler(t, cfg)

	if _, err := h.prowlarrAppProfiles(&cfg); err != nil {
		t.Fatalf("first prowlarrAppProfiles() error = %v", err)
	}
	if _, err := h.prowlarrAppProfiles(&cfg); err != nil {
		t.Fatalf("cached prowlarrAppProfiles() error = %v", err)
	}
	if got := profileHits.Load(); got != 1 {
		t.Fatalf("profile hits = %d, want 1", got)
	}

	if _, err := h.prowlarrTags(&cfg); err != nil {
		t.Fatalf("first prowlarrTags() error = %v", err)
	}
	if _, err := h.prowlarrTags(&cfg); err != nil {
		t.Fatalf("cached prowlarrTags() error = %v", err)
	}
	if got := tagHits.Load(); got != 1 {
		t.Fatalf("tag hits = %d, want 1", got)
	}

	h.invalidateProwlarrMetadataCache()
	if _, err := h.prowlarrAppProfiles(&cfg); err != nil {
		t.Fatalf("after invalidate prowlarrAppProfiles() error = %v", err)
	}
	if got := profileHits.Load(); got != 2 {
		t.Fatalf("profile hits after invalidate = %d, want 2", got)
	}
}
