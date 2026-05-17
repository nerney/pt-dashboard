package handlers

import (
	"net/http"
	"time"

	"github.com/nerney/pt-dashboard/internal/config"
	"github.com/nerney/pt-dashboard/internal/unit3d"
)

type syncResult struct {
	Trackers []*config.TrackerEntry
	LastSync *time.Time
}

func (h *Handler) sync(w http.ResponseWriter, r *http.Request) {
	cfg := h.store.Get()

	now := time.Now()
	changed := false

	for i, entry := range cfg.Trackers {
		if entry.APIKey == "" || entry.Username == "" || entry.TrackerURL == "" {
			continue
		}

		client := unit3d.New(entry.TrackerURL, entry.APIKey)
		stats, err := client.FetchStats(entry.Username)

		ts := now
		cfg.Trackers[i].LastSync = &ts

		if err != nil {
			cfg.Trackers[i].SyncError = err.Error()
		} else {
			cfg.Trackers[i].SyncError = ""
			cfg.Trackers[i].UserStats = &config.UserStats{
				UserID:   stats.UserID,
				Username: stats.Username,
				Upload:   stats.Upload,
				Download: stats.Download,
				Ratio:    stats.Ratio,
				Buffer:   stats.Buffer,
				Bonus:    stats.Bonus,
				Seeding:  stats.Seeding,
				Leeching: stats.Leeching,
				Class:    stats.Class,
			}
		}
		changed = true
	}

	if changed {
		h.store.Save(&cfg)
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

	// Return both the tracker cards and sync status as separate OOB swaps
	h.renderPartial(w, "tracker_cards", syncResult{
		Trackers: cfg.Trackers,
		LastSync: lastSync,
	})
}
