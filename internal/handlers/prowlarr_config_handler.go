package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nerney/ptv/internal/config"
	"github.com/nerney/ptv/internal/prowlarr"
)

// prowlarr_config_handler covers the /config/integrations/prowlarr settings
// tab and the per-tracker Prowlarr link/enable/remove actions. The push
// and sync flows live in prowlarr_tracker_handler.go and
// prowlarr_sync_handler.go respectively.

type configProwlarrData struct {
	Config       config.Config
	FlashError   string
	FlashSuccess string
	ActiveTab    string // "settings" | "import" | "sync"
	Section      string
}

const pathConfigProwlarr = "/config/integrations/prowlarr"

// ── /config/integrations/prowlarr ─────────────────────────────────────────

// configProwlarrPage renders the Prowlarr connection settings. If the
// URL field is empty we pre-fill a sensible Docker-network default —
// most users have prowlarr on the same docker-compose stack.
func (h *Handler) configProwlarrPage(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if cfg.ProwlarrURL == "" {
		cfg.ProwlarrURL = "http://prowlarr:9696"
	}
	h.render(w, r, "config_prowlarr", configProwlarrData{
		Config:       cfg,
		FlashError:   r.URL.Query().Get("err"),
		FlashSuccess: r.URL.Query().Get("ok"),
		ActiveTab:    "settings",
		Section:      "integrations",
	})
}

// configProwlarrPost saves Prowlarr URL + API key. We Ping() the instance
// before persisting so the user gets immediate feedback on bad credentials,
// rather than discovering the failure on the next import attempt.
//
// Empty API key in the form means "keep existing" — lets the user save just
// a URL change without retyping the key (which the form never echoes back).
func (h *Handler) configProwlarrPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		flash(w, r, pathConfigProwlarr, "", "invalid form")
		return
	}
	prowlarrURL := strings.TrimSpace(r.FormValue("url"))
	apiKey := strings.TrimSpace(r.FormValue("api_key"))
	if prowlarrURL == "" {
		flash(w, r, pathConfigProwlarr, "", "URL is required.")
		return
	}

	cfg := h.store.Get()
	if apiKey == "" {
		if cfg.ProwlarrAPIKey == "" {
			flash(w, r, pathConfigProwlarr, "", "API key is required.")
			return
		}
		apiKey = cfg.ProwlarrAPIKey
	}

	h.log.Info("CONFIG", "Saving Prowlarr settings — testing connection")
	client := prowlarr.New(prowlarrURL, apiKey, h.log)
	if err := client.Ping(); err != nil {
		h.flashError(w, r, pathConfigProwlarr, "CONFIG", "Cannot reach Prowlarr", err)
		return
	}

	cfg.ProwlarrURL = prowlarrURL
	cfg.ProwlarrAPIKey = apiKey
	cfg.ProwlarrEnabled = true
	if err := h.store.Save(&cfg); err != nil {
		h.flashError(w, r, pathConfigProwlarr, "CONFIG", "Save failed", err)
		return
	}
	h.invalidateProwlarrMetadataCache()
	h.log.Info("CONFIG", "Prowlarr settings saved")
	go h.warmProwlarrSchemas()
	flash(w, r, pathConfigProwlarr, "Prowlarr settings saved.", "")
}

// configProwlarrEnable / configProwlarrDisable flip the boolean without
// touching credentials. Disabling preserves URL+key so re-enabling
// doesn't require retyping.
func (h *Handler) configProwlarrEnable(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	cfg.ProwlarrEnabled = true
	if err := h.store.Save(&cfg); err != nil {
		h.flashError(w, r, pathConfigProwlarr, "CONFIG", "Save failed", err)
		return
	}
	h.log.Info("CONFIG", "Prowlarr integration enabled")
	go h.warmProwlarrSchemas()
	flash(w, r, pathConfigProwlarr, "Prowlarr integration enabled.", "")
}

func (h *Handler) configProwlarrDisable(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	cfg.ProwlarrEnabled = false
	if err := h.store.Save(&cfg); err != nil {
		h.flashError(w, r, pathConfigProwlarr, "CONFIG", "Save failed", err)
		return
	}
	h.log.Info("CONFIG", "Prowlarr integration disabled (credentials preserved)")
	flash(w, r, pathConfigProwlarr, "Prowlarr integration disabled.", "")
}

// ── per-tracker Prowlarr link/enable/remove ────────────────────────────────

// configTrackerProwlarrAdd creates a new indexer in Prowlarr from the
// tracker's stored credentials. The Prowlarr ID returned is then persisted
// so subsequent toggle/remove operations can target it.
func (h *Handler) configTrackerProwlarrAdd(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		flash(w, r, "/", "", "invalid tracker index")
		return
	}
	basePath := trackerConfigPath(idx)
	entry := cfg.Trackers[idx]
	if entry.TrackerURL == "" || entry.APIKey == "" {
		flash(w, r, basePath, "", entry.Name+": set URL and API key first.")
		return
	}

	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	schema, err := client.SchemaByName(entry.DefinitionName)
	if err != nil {
		h.flashError(w, r, basePath, "PROWLARR", "Schema not found in Prowlarr", err)
		return
	}
	if entry.ProwlarrSettings() == nil {
		cfg.Trackers[idx].EnsureProwlarr().Settings = prowlarr.MergeSettings(*schema, nil, nil)
	}
	if err := h.pushTrackerProwlarrConfig(cfg, idx, *schema); err != nil {
		h.flashError(w, r, basePath, "PROWLARR", "Prowlarr add failed", err)
		return
	}

	if err := h.store.Save(cfg); err != nil {
		h.flashError(w, r, basePath, "CONFIG", "Save failed", err)
		return
	}
	h.log.Info("CONFIG", fmt.Sprintf("Added %q to Prowlarr (id=%d)", entry.Name, cfg.Trackers[idx].ProwlarrID()))
	flash(w, r, basePath, entry.Name+" added to Prowlarr.", "")
}

// configTrackerProwlarrToggle flips Prowlarr's enable flag for this
// indexer. The dashboard mirrors the resulting state.
func (h *Handler) configTrackerProwlarrToggle(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		flash(w, r, "/", "", "invalid tracker index")
		return
	}
	basePath := trackerConfigPath(idx)
	entry := cfg.Trackers[idx]
	if entry.ProwlarrID() == 0 {
		flash(w, r, basePath, "", entry.Name+" is not in Prowlarr.")
		return
	}

	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	indexer, err := client.GetIndexer(entry.ProwlarrID())
	if err != nil {
		h.flashError(w, r, basePath, "PROWLARR", "Prowlarr fetch failed", err)
		return
	}
	if err := client.SetEnabled(*indexer, !entry.Enabled); err != nil {
		h.flashError(w, r, basePath, "PROWLARR", "Prowlarr update failed", err)
		return
	}

	cfg.Trackers[idx].Enabled = !entry.Enabled
	if err := h.store.Save(cfg); err != nil {
		h.flashError(w, r, basePath, "CONFIG", "Save failed", err)
		return
	}
	status := "disabled"
	if cfg.Trackers[idx].Enabled {
		status = "enabled"
	}
	h.log.Info("CONFIG", fmt.Sprintf("%s %s in Prowlarr", entry.Name, status))
	flash(w, r, basePath, entry.Name+" "+status+" in Prowlarr.", "")
}

// configTrackerProwlarrRemove deletes the indexer in Prowlarr and clears the
// local ProwlarrID. We do NOT delete the tracker from the dashboard — only
// the Prowlarr linkage is broken.
func (h *Handler) configTrackerProwlarrRemove(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		flash(w, r, "/", "", "invalid tracker index")
		return
	}
	basePath := trackerConfigPath(idx)
	entry := cfg.Trackers[idx]
	if entry.ProwlarrID() == 0 {
		flash(w, r, basePath, "", entry.Name+" is not in Prowlarr.")
		return
	}
	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey, h.log)
	if err := client.DeleteIndexer(entry.ProwlarrID()); err != nil {
		h.flashError(w, r, basePath, "PROWLARR", "Prowlarr remove failed", err)
		return
	}

	cfg.Trackers[idx].Enabled = false
	prowlarrCfg := cfg.Trackers[idx].EnsureProwlarr()
	prowlarrCfg.ID = 0
	prowlarrCfg.Name = ""
	prowlarrCfg.AppProfileID = 0
	prowlarrCfg.Tags = nil
	if err := h.store.Save(cfg); err != nil {
		h.flashError(w, r, basePath, "CONFIG", "Save failed", err)
		return
	}
	h.log.Info("CONFIG", fmt.Sprintf("Removed %q from Prowlarr", entry.Name))
	flash(w, r, basePath, entry.Name+" removed from Prowlarr.", "")
}
