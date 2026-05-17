package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/nerney/pt-dashboard/internal/config"
	"github.com/nerney/pt-dashboard/internal/prowlarr"
)

type configPageData struct {
	Config  config.Config
	Error   string
	Success string
}

func (h *Handler) configPage(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	h.render(w, "config", configPageData{Config: cfg})
}

func (h *Handler) configProwlarrPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "config", configPageData{Error: "invalid form"})
		return
	}

	prowlarrURL := strings.TrimSpace(r.FormValue("url"))
	apiKey := strings.TrimSpace(r.FormValue("api_key"))

	client := prowlarr.New(prowlarrURL, apiKey)
	if err := client.Ping(); err != nil {
		cfg := h.store.Get()
		h.render(w, "config", configPageData{
			Config: cfg,
			Error:  "Cannot reach Prowlarr: " + err.Error(),
		})
		return
	}

	cfg := h.store.Get()
	cfg.ProwlarrURL = prowlarrURL
	cfg.ProwlarrAPIKey = apiKey
	if err := h.store.Save(&cfg); err != nil {
		h.render(w, "config", configPageData{Config: cfg, Error: "Save failed: " + err.Error()})
		return
	}

	h.render(w, "config", configPageData{Config: cfg, Success: "Prowlarr config updated."})
}

func (h *Handler) trackerIndex(r *http.Request) (int, *config.Config, bool) {
	idxStr := chi.URLParam(r, "idx")
	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return 0, nil, false
	}
	cfg := h.store.Get()
	if idx < 0 || idx >= len(cfg.Trackers) {
		return 0, nil, false
	}
	return idx, &cfg, true
}

func (h *Handler) configTrackerUpdate(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		http.Error(w, "invalid tracker index", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", 400)
		return
	}

	if url := strings.TrimSpace(r.FormValue("url")); url != "" {
		cfg.Trackers[idx].TrackerURL = url
	}
	if key := strings.TrimSpace(r.FormValue("api_key")); key != "" {
		cfg.Trackers[idx].APIKey = key
	}
	if username := strings.TrimSpace(r.FormValue("username")); username != "" {
		cfg.Trackers[idx].Username = username
	}
	cfg.Trackers[idx].UserStats = nil
	cfg.Trackers[idx].LastSync = nil

	if err := h.store.Save(cfg); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Save failed: " + err.Error()})
		return
	}
	h.render(w, "config", configPageData{Config: *cfg, Success: cfg.Trackers[idx].Name + " updated."})
}

func (h *Handler) configTrackerProwlarrAdd(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		http.Error(w, "invalid tracker index", 400)
		return
	}

	entry := cfg.Trackers[idx]
	if entry.TrackerURL == "" || entry.APIKey == "" {
		h.render(w, "config", configPageData{
			Config: *cfg,
			Error:  entry.Name + ": set URL and API key first.",
		})
		return
	}

	pclient := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
	schema, err := pclient.SchemaByName(entry.DefinitionName)
	if err != nil {
		h.render(w, "config", configPageData{
			Config: *cfg,
			Error:  "Schema not found in Prowlarr: " + err.Error(),
		})
		return
	}

	added, err := pclient.AddIndexer(*schema, entry.TrackerURL, entry.APIKey)
	if err != nil {
		h.render(w, "config", configPageData{
			Config: *cfg,
			Error:  "Prowlarr add failed: " + err.Error(),
		})
		return
	}

	cfg.Trackers[idx].ProwlarrID = added.ID
	cfg.Trackers[idx].Enabled = added.Enable

	if err := h.store.Save(cfg); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Save failed: " + err.Error()})
		return
	}
	h.render(w, "config", configPageData{Config: *cfg, Success: entry.Name + " added to Prowlarr."})
}

func (h *Handler) configTrackerProwlarrToggle(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		http.Error(w, "invalid tracker index", 400)
		return
	}

	entry := cfg.Trackers[idx]
	if entry.ProwlarrID == 0 {
		h.render(w, "config", configPageData{Config: *cfg, Error: entry.Name + " is not in Prowlarr."})
		return
	}

	pclient := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
	indexer, err := pclient.GetIndexer(entry.ProwlarrID)
	if err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Prowlarr fetch failed: " + err.Error()})
		return
	}

	if err := pclient.SetEnabled(*indexer, !entry.Enabled); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Prowlarr update failed: " + err.Error()})
		return
	}

	cfg.Trackers[idx].Enabled = !entry.Enabled
	if err := h.store.Save(cfg); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Save failed: " + err.Error()})
		return
	}

	status := "disabled"
	if cfg.Trackers[idx].Enabled {
		status = "enabled"
	}
	h.render(w, "config", configPageData{Config: *cfg, Success: entry.Name + " " + status + " in Prowlarr."})
}

func (h *Handler) configTrackerProwlarrRemove(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		http.Error(w, "invalid tracker index", 400)
		return
	}

	entry := cfg.Trackers[idx]
	if entry.ProwlarrID == 0 {
		h.render(w, "config", configPageData{Config: *cfg, Error: entry.Name + " is not in Prowlarr."})
		return
	}

	pclient := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
	if err := pclient.DeleteIndexer(entry.ProwlarrID); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Prowlarr remove failed: " + err.Error()})
		return
	}

	cfg.Trackers[idx].ProwlarrID = 0
	cfg.Trackers[idx].Enabled = false
	if err := h.store.Save(cfg); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Save failed: " + err.Error()})
		return
	}
	h.render(w, "config", configPageData{Config: *cfg, Success: entry.Name + " removed from Prowlarr."})
}

func (h *Handler) configTrackerDelete(w http.ResponseWriter, r *http.Request) {
	idx, cfg, ok := h.trackerIndex(r)
	if !ok {
		http.Error(w, "invalid tracker index", 400)
		return
	}

	name := cfg.Trackers[idx].Name
	cfg.Trackers = append(cfg.Trackers[:idx], cfg.Trackers[idx+1:]...)

	if err := h.store.Save(cfg); err != nil {
		h.render(w, "config", configPageData{Config: *cfg, Error: "Save failed: " + err.Error()})
		return
	}
	h.render(w, "config", configPageData{Config: *cfg, Success: name + " removed from dashboard."})
}
