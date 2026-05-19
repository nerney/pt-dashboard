// Package autobrrdefs clones and caches autobrr's bundled indexer definition
// files from https://github.com/autobrr/autobrr. Only the definitions
// subdirectory is fetched via git sparse checkout — the rest of the autobrr
// source tree is never downloaded.
//
// The definitions drive PTV's autobrr config UI: each file describes the
// settings fields (label, help text, required flag) that the autobrr API
// expects when adding or updating an indexer.
package autobrrdefs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nerney/ptv/internal/logger"
)

type State int

const (
	StatePending         State = iota
	StateSyncing
	StateOK
	StateStalePullFailed
	StateUnavailable
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateSyncing:
		return "syncing"
	case StateOK:
		return "ok"
	case StateStalePullFailed:
		return "stale"
	case StateUnavailable:
		return "unavailable"
	}
	return "unknown"
}

const (
	repoURL  = "https://github.com/autobrr/autobrr.git"
	defsPath = "internal/indexer/definitions"
)

// Syncer clones (or updates) the autobrr repository using a shallow sparse
// checkout limited to the definitions subdirectory, then parses and caches
// the definitions in memory.
type Syncer struct {
	dir     string // local repo root, e.g. <configDir>/.hidden/autobrr
	defsDir string // <dir>/internal/indexer/definitions
	log     *logger.Logger
	mu      sync.RWMutex
	state   State
	msg     string
	catalog []Def
	ready   chan struct{}
}

func New(configDir string, log *logger.Logger) *Syncer {
	dir := filepath.Join(configDir, ".hidden", "autobrr")
	return &Syncer{
		dir:     dir,
		defsDir: filepath.Join(dir, filepath.FromSlash(defsPath)),
		log:     log,
		state:   StatePending,
		ready:   make(chan struct{}),
	}
}

// Start launches the sync goroutine. Returns immediately.
func (s *Syncer) Start(ctx context.Context) {
	go s.run(ctx)
}

// WaitReady blocks until the first sync attempt completes or ctx expires.
// Returns non-nil only when no definitions are available at all. A stale
// pull result is NOT an error — the catalog is usable.
func (s *Syncer) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
	case <-ctx.Done():
		return ctx.Err()
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.state == StateUnavailable {
		return fmt.Errorf("autobrr definitions unavailable: %s", s.msg)
	}
	return nil
}

// Status returns the current sync state and an optional message.
func (s *Syncer) Status() (State, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state, s.msg
}

// Catalog returns the in-memory definition catalog. Safe to call
// concurrently after WaitReady unblocks.
func (s *Syncer) Catalog() ([]Def, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.catalog == nil {
		return nil, fmt.Errorf("autobrr definitions not available: %s", s.msg)
	}
	return s.catalog, nil
}

// ByIdentifier returns the definition matching identifier (case-insensitive),
// or nil if not found.
func (s *Syncer) ByIdentifier(id string) *Def {
	s.mu.RLock()
	defer s.mu.RUnlock()
	lower := strings.ToLower(id)
	for i := range s.catalog {
		if strings.ToLower(s.catalog[i].Identifier) == lower {
			return &s.catalog[i]
		}
	}
	return nil
}

// ByURL returns the definition whose URLs list contains trackerURL
// (case-insensitive, trailing-slash agnostic). Used during import to match
// a PTV tracker to its autobrr def before an AutobrrIdentifier is stored.
// Returns nil if no def matches.
func (s *Syncer) ByURL(trackerURL string) *Def {
	s.mu.RLock()
	defer s.mu.RUnlock()
	want := normalizeURL(trackerURL)
	for i := range s.catalog {
		for _, u := range s.catalog[i].URLs {
			if normalizeURL(u) == want {
				return &s.catalog[i]
			}
		}
	}
	return nil
}

func normalizeURL(u string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(u)), "/")
}

func (s *Syncer) run(ctx context.Context) {
	defer close(s.ready)
	s.set(StateSyncing, "")
	s.log.Info("AUTOBRR-DEFS", "Starting definitions sync — "+repoURL)

	if _, err := os.Stat(s.dir); os.IsNotExist(err) {
		if err := s.gitClone(ctx); err != nil {
			s.log.Err("AUTOBRR-DEFS", "Clone failed: "+err.Error())
			s.set(StateUnavailable, err.Error())
			return
		}
		s.log.Info("AUTOBRR-DEFS", "Clone complete")
		s.cacheAndSetState(StateOK, "")
		return
	}

	if err := s.gitPull(ctx); err != nil {
		s.log.Err("AUTOBRR-DEFS", "Pull failed — using stale definitions: "+err.Error())
		s.cacheAndSetState(StateStalePullFailed, err.Error())
		return
	}
	s.log.Info("AUTOBRR-DEFS", "Pull complete")
	s.cacheAndSetState(StateOK, "")
}

func (s *Syncer) cacheAndSetState(finalState State, finalMsg string) {
	catalog, err := parseCatalog(s.defsDir)
	if err != nil {
		s.log.Err("AUTOBRR-DEFS", "Catalog load failed: "+err.Error())
		s.set(StateUnavailable, "catalog load failed: "+err.Error())
		return
	}
	s.mu.Lock()
	s.catalog = catalog
	s.state = finalState
	s.msg = finalMsg
	s.mu.Unlock()
	s.log.Info("AUTOBRR-DEFS", fmt.Sprintf("Catalog loaded: %d definitions", len(catalog)))
}

func (s *Syncer) set(state State, msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.msg = msg
}

// gitClone performs a shallow sparse clone, fetching only the definitions
// subdirectory. Two commands are needed: the clone sets up sparse mode,
// and sparse-checkout set specifies the path.
func (s *Syncer) gitClone(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.dir), 0700); err != nil {
		return err
	}
	if err := s.git(ctx, "clone", "--depth=1", "--filter=blob:none", "--sparse", repoURL, s.dir); err != nil {
		return err
	}
	return s.git(ctx, "-C", s.dir, "sparse-checkout", "set", defsPath)
}

func (s *Syncer) gitPull(ctx context.Context) error {
	return s.git(ctx, "-C", s.dir, "pull", "--ff-only")
}

func (s *Syncer) git(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		msg := err.Error()
		if trimmed != "" {
			msg = trimmed
		}
		return fmt.Errorf("%s", msg)
	}
	if trimmed != "" {
		s.log.Info("AUTOBRR-DEFS", "git: "+trimmed)
	}
	return nil
}
