package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const stateFilename = "state.enc"

type Config struct {
	SetupComplete  bool             `json:"setup_complete"`
	ProwlarrURL    string           `json:"prowlarr_url"`
	ProwlarrAPIKey string           `json:"prowlarr_api_key"`
	Trackers       []*TrackerEntry  `json:"trackers"`
}

type TrackerEntry struct {
	DefinitionName string `json:"definition_name"`
	Name           string `json:"name"`
	TrackerURL     string `json:"tracker_url"`
	APIKey         string `json:"api_key"`
	Username       string `json:"username"`
	ProwlarrID     int    `json:"prowlarr_id"`
	Enabled        bool   `json:"enabled"`
	LastSync       *time.Time `json:"last_sync,omitempty"`
	UserStats      *UserStats `json:"user_stats,omitempty"`
	SyncError      string     `json:"sync_error,omitempty"`
}

type UserStats struct {
	UserID   int     `json:"user_id"`
	Username string  `json:"username"`
	Upload   int64   `json:"upload"`
	Download int64   `json:"download"`
	Ratio    float64 `json:"ratio"`
	Buffer   int64   `json:"buffer"`
	Bonus    float64 `json:"bonus"`
	Seeding  int     `json:"seeding"`
	Leeching int     `json:"leeching"`
	Class    string  `json:"class"`
}

type Store struct {
	mu     sync.RWMutex
	dir    string
	key    []byte
	config *Config
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	key, err := loadOrCreateKey(dir)
	if err != nil {
		return nil, fmt.Errorf("key: %w", err)
	}

	s := &Store{dir: dir, key: key, config: &Config{}}
	_ = s.load()
	return s, nil
}

func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := *s.config
	return cp
}

func (s *Store) Save(cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	enc, err := encrypt(s.key, data)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	path := filepath.Join(s.dir, stateFilename)
	if err := os.WriteFile(path, enc, 0600); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	s.config = cfg
	return nil
}

func (s *Store) load() error {
	path := filepath.Join(s.dir, stateFilename)
	enc, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	data, err := decrypt(s.key, enc)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	s.config = &cfg
	return nil
}
