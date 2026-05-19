package config

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestTrackerEntryUnmarshalLegacyIntegrationConfig(t *testing.T) {
	data := []byte(`{
		"definition_name": "Yu-Scene",
		"tracker_type": "unit3d",
		"name": "yu-scene [ptv]",
		"tracker_url": "https://yu-scene.test",
		"api_key": "unit3d-key",
		"username": "nern",
		"enabled": true,
		"prowlarr_id": 3,
		"prowlarr_settings": {"minimumSeeders": "1"},
		"prowlarr_name": "yu-scene",
		"prowlarr_app_profile_id": 7,
		"prowlarr_tags": [11, 12],
		"prowlarr_sync_error": "old prowlarr error",
		"autobrr_id": 44,
		"autobrr_identifier": "yu-scene",
		"autobrr_enabled": true,
		"autobrr_settings": {"rsskey": "secret"},
		"autobrr_sync_error": "old autobrr error",
		"favicon_data_uri": "data:image/png;base64,aaa",
		"theme_color": "#112233"
	}`)

	var got TrackerEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Prowlarr == nil {
		t.Fatal("Prowlarr = nil, want migrated config")
	}
	if got.Prowlarr.ID != 3 ||
		got.Prowlarr.Settings["minimumSeeders"] != "1" ||
		got.Prowlarr.Name != "yu-scene" ||
		got.Prowlarr.AppProfileID != 7 ||
		len(got.Prowlarr.Tags) != 2 ||
		got.Prowlarr.SyncError != "old prowlarr error" {
		t.Fatalf("Prowlarr migrated incorrectly: %#v", got.Prowlarr)
	}
	if got.Autobrr == nil {
		t.Fatal("Autobrr = nil, want migrated config")
	}
	if got.Autobrr.ID != 44 ||
		got.Autobrr.Identifier != "yu-scene" ||
		!got.Autobrr.Enabled ||
		got.Autobrr.Settings["rsskey"] != "secret" ||
		got.Autobrr.SyncError != "old autobrr error" {
		t.Fatalf("Autobrr migrated incorrectly: %#v", got.Autobrr)
	}
}

func TestTrackerEntryUnmarshalNestedIntegrationConfig(t *testing.T) {
	data := []byte(`{
		"definition_name": "Yu-Scene",
		"name": "yu-scene [ptv]",
		"tracker_url": "https://yu-scene.test",
		"api_key": "unit3d-key",
		"prowlarr": {
			"id": 3,
			"settings": {"minimumSeeders": "1"},
			"name": "yu-scene",
			"app_profile_id": 7,
			"tags": [11]
		},
		"autobrr": {
			"id": 44,
			"identifier": "yu-scene",
			"enabled": true,
			"settings": {"rsskey": "secret"}
		}
	}`)

	var got TrackerEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Prowlarr == nil || got.Prowlarr.ID != 3 || got.Prowlarr.AppProfileID != 7 {
		t.Fatalf("Prowlarr nested decode = %#v", got.Prowlarr)
	}
	if got.Autobrr == nil || got.Autobrr.ID != 44 || !got.Autobrr.Enabled {
		t.Fatalf("Autobrr nested decode = %#v", got.Autobrr)
	}
}

// newTestStore creates a Store backed by a temp directory and registers
// cleanup automatically.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return s
}

func TestNewStoreCreatesDir(t *testing.T) {
	dir := t.TempDir() + "/nested/path"
	s, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if s == nil {
		t.Fatal("NewStore() returned nil")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("directory not created: %v", err)
	}
}

func TestIsInitializedFalseOnFreshStore(t *testing.T) {
	s := newTestStore(t)
	if s.IsInitialized() {
		t.Fatal("IsInitialized() = true on fresh store, want false")
	}
}

func TestInitCreatesFiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.Init("hunter2", "alice", "10.0.0.0/8"); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if !s.IsInitialized() {
		t.Fatal("IsInitialized() = false after Init")
	}
	if !s.IsUnlocked() {
		t.Fatal("IsUnlocked() = false after Init")
	}
}

func TestInitRejectsDoubleInit(t *testing.T) {
	s := newTestStore(t)
	s.Init("hunter2", "alice", "")
	if err := s.Init("hunter2", "alice", ""); !errors.Is(err, ErrAlreadyInit) {
		t.Fatalf("second Init() = %v, want ErrAlreadyInit", err)
	}
}

func TestUnlockRoundtrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.Init("secret-pass", "bob", ""); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	s.Lock()
	if s.IsUnlocked() {
		t.Fatal("IsUnlocked() = true after Lock")
	}
	if err := s.Unlock("secret-pass"); err != nil {
		t.Fatalf("Unlock() error = %v", err)
	}
	if !s.IsUnlocked() {
		t.Fatal("IsUnlocked() = false after correct Unlock")
	}
}

func TestUnlockRejectsBadPassword(t *testing.T) {
	s := newTestStore(t)
	s.Init("correct-pass", "carol", "")
	s.Lock()
	if err := s.Unlock("wrong-pass"); !errors.Is(err, ErrBadPassword) {
		t.Fatalf("Unlock(wrong) = %v, want ErrBadPassword", err)
	}
}

func TestUnlockRejectsUninitializedStore(t *testing.T) {
	s := newTestStore(t)
	if err := s.Unlock("any"); !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Unlock() on fresh store = %v, want ErrNotInitialized", err)
	}
}

func TestSaveGetRoundtrip(t *testing.T) {
	s := newTestStore(t)
	s.Init("pass", "dave", "")

	cfg := s.Get()
	cfg.ProwlarrURL = "https://prowlarr.local"
	cfg.ProwlarrAPIKey = "abc123"
	cfg.Trackers = []*TrackerEntry{{Name: "MyTracker", TrackerURL: "https://tracker.local", APIKey: "key"}}

	if err := s.Save(&cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got := s.Get()
	if got.ProwlarrURL != "https://prowlarr.local" {
		t.Errorf("Get().ProwlarrURL = %q, want %q", got.ProwlarrURL, "https://prowlarr.local")
	}
	if len(got.Trackers) != 1 || got.Trackers[0].Name != "MyTracker" {
		t.Errorf("Get().Trackers = %#v, want 1 tracker", got.Trackers)
	}
}

func TestSavePersistedThroughRelock(t *testing.T) {
	s := newTestStore(t)
	s.Init("pass", "eve", "")

	cfg := s.Get()
	cfg.AutobrrURL = "https://autobrr.local"
	s.Save(&cfg)
	s.Lock()

	s.Unlock("pass")
	got := s.Get()
	if got.AutobrrURL != "https://autobrr.local" {
		t.Errorf("AutobrrURL not persisted across lock/unlock: got %q", got.AutobrrURL)
	}
}

func TestSaveLockedStoreReturnsError(t *testing.T) {
	s := newTestStore(t)
	s.Init("pass", "frank", "")
	s.Lock()
	cfg := Config{}
	if err := s.Save(&cfg); !errors.Is(err, ErrLocked) {
		t.Fatalf("Save() on locked store = %v, want ErrLocked", err)
	}
}

func TestGetNetACLReturnsCopy(t *testing.T) {
	s := newTestStore(t)
	n := &NetACL{AllowedCIDRs: []string{"10.0.0.0/8"}}
	s.SaveNetACL(n)

	got := s.GetNetACL()
	got.AllowedCIDRs[0] = "mutated"

	// Original should be unaffected.
	second := s.GetNetACL()
	if second.AllowedCIDRs[0] == "mutated" {
		t.Fatal("GetNetACL() returned aliased slice, not a copy")
	}
}

func TestSaveNetACLSetsConfirmed(t *testing.T) {
	s := newTestStore(t)
	n := &NetACL{AllowedCIDRs: []string{"192.168.0.0/16"}, Confirmed: false}
	if err := s.SaveNetACL(n); err != nil {
		t.Fatalf("SaveNetACL() error = %v", err)
	}
	if got := s.GetNetACL(); !got.Confirmed {
		t.Fatal("SaveNetACL() did not set Confirmed=true")
	}
}
