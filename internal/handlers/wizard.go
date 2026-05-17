package handlers

import (
	"net/http"
	"strings"

	"github.com/nerney/pt-dashboard/internal/config"
	"github.com/nerney/pt-dashboard/internal/prowlarr"
)

type step1Data struct {
	Error  string
	URL    string
	APIKey string
}

type step2Data struct {
	Schemas []prowlarr.IndexerSchema
	Error   string
}

type step3Data struct {
	Trackers []*config.TrackerEntry
	Error    string
}

func (h *Handler) wizardStep1(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if cfg.SetupComplete {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	h.render(w, "wizard1", step1Data{
		URL:    cfg.ProwlarrURL,
		APIKey: cfg.ProwlarrAPIKey,
	})
}

func (h *Handler) wizardStep1Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.render(w, "wizard1", step1Data{Error: "invalid form data"})
		return
	}

	prowlarrURL := strings.TrimSpace(r.FormValue("url"))
	apiKey := strings.TrimSpace(r.FormValue("api_key"))

	if prowlarrURL == "" || apiKey == "" {
		h.render(w, "wizard1", step1Data{
			Error:  "Both URL and API key are required.",
			URL:    prowlarrURL,
			APIKey: apiKey,
		})
		return
	}

	client := prowlarr.New(prowlarrURL, apiKey)
	if err := client.Ping(); err != nil {
		h.render(w, "wizard1", step1Data{
			Error:  "Cannot reach Prowlarr: " + err.Error(),
			URL:    prowlarrURL,
			APIKey: apiKey,
		})
		return
	}

	cfg := h.store.Get()
	cfg.ProwlarrURL = prowlarrURL
	cfg.ProwlarrAPIKey = apiKey
	if err := h.store.Save(&cfg); err != nil {
		h.render(w, "wizard1", step1Data{
			Error:  "Save failed: " + err.Error(),
			URL:    prowlarrURL,
			APIKey: apiKey,
		})
		return
	}

	http.Redirect(w, r, "/wizard/2", http.StatusFound)
}

func (h *Handler) wizardStep2(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if cfg.SetupComplete {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if cfg.ProwlarrURL == "" {
		http.Redirect(w, r, "/wizard/1", http.StatusFound)
		return
	}

	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
	schemas, err := client.GetUnit3dSchemas()
	if err != nil {
		h.render(w, "wizard2", step2Data{Error: "Failed to fetch tracker list from Prowlarr: " + err.Error()})
		return
	}

	h.render(w, "wizard2", step2Data{Schemas: schemas})
}


func (h *Handler) wizardStep2Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/wizard/2", http.StatusFound)
		return
	}

	selected := r.Form["trackers"]
	if len(selected) == 0 {
		cfg := h.store.Get()
		client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
		schemas, _ := client.GetUnit3dSchemas()
		h.render(w, "wizard2", step2Data{
			Schemas: schemas,
			Error:   "Select at least one tracker to continue.",
		})
		return
	}

	cfg := h.store.Get()

	// Build a map of existing entries by name so we don't duplicate
	existing := make(map[string]bool)
	for _, t := range cfg.Trackers {
		existing[t.DefinitionName] = true
	}

	client := prowlarr.New(cfg.ProwlarrURL, cfg.ProwlarrAPIKey)
	schemas, err := client.GetUnit3dSchemas()
	if err != nil {
		h.render(w, "wizard2", step2Data{Error: "Failed to load schemas: " + err.Error()})
		return
	}

	schemaMap := make(map[string]prowlarr.IndexerSchema)
	for _, s := range schemas {
		schemaMap[s.Name] = s
	}

	for _, name := range selected {
		if existing[name] {
			continue
		}
		entry := &config.TrackerEntry{
			DefinitionName: name,
			Name:           name,
		}
		// Pre-fill URL hint from schema fields if available
		if s, ok := schemaMap[name]; ok {
			for _, f := range s.Fields {
				low := strings.ToLower(f.Name)
				if (low == "baseurl" || low == "sitelink") && f.Value != nil {
					if v, ok := f.Value.(string); ok && v != "" {
						entry.TrackerURL = v
					}
				}
			}
		}
		cfg.Trackers = append(cfg.Trackers, entry)
	}

	if err := h.store.Save(&cfg); err != nil {
		h.render(w, "wizard2", step2Data{Error: "Save failed: " + err.Error()})
		return
	}

	http.Redirect(w, r, "/wizard/3", http.StatusFound)
}

func (h *Handler) wizardStep3(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if cfg.SetupComplete {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Show only trackers without credentials yet
	var pending []*config.TrackerEntry
	for _, t := range cfg.Trackers {
		if t.APIKey == "" {
			pending = append(pending, t)
		}
	}

	if len(pending) == 0 {
		// All credentialed, mark setup complete
		cfg.SetupComplete = true
		h.store.Save(&cfg)
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	h.render(w, "wizard3", step3Data{Trackers: pending})
}

func (h *Handler) wizardStep3Post(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/wizard/3", http.StatusFound)
		return
	}

	cfg := h.store.Get()

	for i, entry := range cfg.Trackers {
		prefix := "t_" + strings.ReplaceAll(entry.DefinitionName, " ", "_") + "_"
		url := strings.TrimSpace(r.FormValue(prefix + "url"))
		apiKey := strings.TrimSpace(r.FormValue(prefix + "api_key"))
		username := strings.TrimSpace(r.FormValue(prefix + "username"))

		if url != "" {
			cfg.Trackers[i].TrackerURL = url
		}
		if apiKey != "" {
			cfg.Trackers[i].APIKey = apiKey
		}
		if username != "" {
			cfg.Trackers[i].Username = username
		}
	}

	// Check any still have no creds - they're optional to skip
	cfg.SetupComplete = true

	if err := h.store.Save(&cfg); err != nil {
		h.render(w, "wizard3", step3Data{
			Trackers: cfg.Trackers,
			Error:    "Save failed: " + err.Error(),
		})
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}
