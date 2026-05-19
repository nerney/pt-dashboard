package handlers

import (
	"fmt"
	"strings"
	"time"

	"github.com/nerney/ptv/internal/config"
	"github.com/nerney/ptv/internal/prowlarr"
)

const prowlarrMetadataTTL = 5 * time.Minute

type cachedAppProfiles struct {
	key       string
	expiresAt time.Time
	values    []prowlarr.AppProfile
}

type cachedTags struct {
	key       string
	expiresAt time.Time
	values    []prowlarr.Tag
}

// warmProwlarrSchemas fetches the full indexer schema list from Prowlarr
// and stores it in the Handler's in-memory cache, keyed by lowercase name.
// It is a no-op when Prowlarr is not fully configured. Intended to run as
// a goroutine — failures are logged but never propagated.
func (h *Handler) warmProwlarrSchemas() {
	cfg := h.store.Get()
	if !cfg.ProwlarrEnabled || cfg.ProwlarrURL == "" || cfg.ProwlarrAPIKey == "" {
		return
	}
	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	schemas, err := client.GetAllSchemas()
	if err != nil {
		h.log.Err("PROWLARR", "schema cache: "+err.Error())
		return
	}
	byName := make(map[string]prowlarr.IndexerSchema, len(schemas))
	for _, s := range schemas {
		byName[strings.ToLower(s.Name)] = s
	}
	h.pSchemasMu.Lock()
	h.pSchemas = byName
	h.pSchemasMu.Unlock()
	h.log.Info("PROWLARR", fmt.Sprintf("Cached %d indexer schemas", len(schemas)))
}

// prowlarrSchemaByName looks up a schema by name from the in-memory cache.
// On a cache miss it falls back to a live Prowlarr fetch — this covers the
// window before warmProwlarrSchemas has completed on a fresh login.
func (h *Handler) prowlarrSchemaByName(name string) (*prowlarr.IndexerSchema, error) {
	h.pSchemasMu.RLock()
	s, ok := h.pSchemas[strings.ToLower(name)]
	h.pSchemasMu.RUnlock()
	if ok {
		return &s, nil
	}
	cfg := h.store.Get()
	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	return client.SchemaByName(name)
}

func (h *Handler) prowlarrAppProfiles(cfg *config.Config) ([]prowlarr.AppProfile, error) {
	key := prowlarrMetadataKey(cfg)
	now := time.Now()
	h.pMetadataMu.RLock()
	if h.pAppProfiles.key == key && now.Before(h.pAppProfiles.expiresAt) {
		out := append([]prowlarr.AppProfile(nil), h.pAppProfiles.values...)
		h.pMetadataMu.RUnlock()
		return out, nil
	}
	h.pMetadataMu.RUnlock()

	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	profiles, err := client.GetAppProfiles()
	if err != nil {
		return nil, err
	}
	h.pMetadataMu.Lock()
	h.pAppProfiles = cachedAppProfiles{
		key:       key,
		expiresAt: now.Add(prowlarrMetadataTTL),
		values:    append([]prowlarr.AppProfile(nil), profiles...),
	}
	h.pMetadataMu.Unlock()
	return profiles, nil
}

func (h *Handler) prowlarrTags(cfg *config.Config) ([]prowlarr.Tag, error) {
	key := prowlarrMetadataKey(cfg)
	now := time.Now()
	h.pMetadataMu.RLock()
	if h.pTags.key == key && now.Before(h.pTags.expiresAt) {
		out := append([]prowlarr.Tag(nil), h.pTags.values...)
		h.pMetadataMu.RUnlock()
		return out, nil
	}
	h.pMetadataMu.RUnlock()

	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	tags, err := client.GetTags()
	if err != nil {
		return nil, err
	}
	h.pMetadataMu.Lock()
	h.pTags = cachedTags{
		key:       key,
		expiresAt: now.Add(prowlarrMetadataTTL),
		values:    append([]prowlarr.Tag(nil), tags...),
	}
	h.pMetadataMu.Unlock()
	return tags, nil
}

func (h *Handler) invalidateProwlarrMetadataCache() {
	h.pMetadataMu.Lock()
	h.pAppProfiles = cachedAppProfiles{}
	h.pTags = cachedTags{}
	h.pMetadataMu.Unlock()
}

func prowlarrMetadataKey(cfg *config.Config) string {
	return strings.TrimRight(cfg.ProwlarrURL, "/") + "\x00" + cfg.ProwlarrAPIKey
}
