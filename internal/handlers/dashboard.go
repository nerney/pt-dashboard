package handlers

import (
	"net/http"
	"time"

	"github.com/nerney/pt-dashboard/internal/config"
)

type dashboardData struct {
	Trackers []*config.TrackerEntry
	LastSync *time.Time
	Date     string
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()
	if !cfg.SetupComplete {
		http.Redirect(w, r, "/wizard/1", http.StatusFound)
		return
	}

	var lastSync *time.Time
	for _, t := range cfg.Trackers {
		if t.LastSync != nil {
			if lastSync == nil || t.LastSync.After(*lastSync) {
				ts := *t.LastSync
				lastSync = &ts
			}
		}
	}

	h.render(w, "dashboard", dashboardData{
		Trackers: cfg.Trackers,
		LastSync: lastSync,
		Date:     time.Now().Format("02 Jan 2006"),
	})
}
