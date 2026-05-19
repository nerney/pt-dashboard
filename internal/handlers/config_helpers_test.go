package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/nerney/ptv/internal/config"
	"github.com/nerney/ptv/internal/defs"
)

// ── buildConfigRows ──────────────────────────────────────────────────────────

func TestBuildConfigRowsConfiguredFirst(t *testing.T) {
	trackers := []*config.TrackerEntry{
		{DefinitionName: "TrackerB", Name: "B"},
		{DefinitionName: "TrackerA", Name: "A"},
	}
	allDefs := []defs.TrackerDef{
		{Name: "TrackerA", TypeID: "unit3d", URLs: []string{"https://a.example"}},
		{Name: "TrackerB", TypeID: "unit3d", URLs: []string{"https://b.example"}},
		{Name: "TrackerC", TypeID: "unit3d", URLs: []string{"https://c.example"}},
	}
	rows := buildConfigRows(trackers, allDefs)
	// Configured rows first (sorted by name), then available (sorted by name).
	if len(rows) != 3 {
		t.Fatalf("len = %d, want 3", len(rows))
	}
	if !rows[0].Configured || rows[0].Name != "A" {
		t.Errorf("rows[0] should be configured TrackerA, got %+v", rows[0])
	}
	if !rows[1].Configured || rows[1].Name != "B" {
		t.Errorf("rows[1] should be configured TrackerB, got %+v", rows[1])
	}
	if rows[2].Configured {
		t.Errorf("rows[2] should be available, got configured")
	}
	if rows[2].Name != "TrackerC" {
		t.Errorf("rows[2].Name = %q, want TrackerC", rows[2].Name)
	}
}

func TestBuildConfigRowsURLsFromCatalog(t *testing.T) {
	trackers := []*config.TrackerEntry{
		{DefinitionName: "TrackerA", Name: "A"},
	}
	allDefs := []defs.TrackerDef{
		{Name: "TrackerA", TypeID: "unit3d", URLs: []string{"https://a.example", "https://a2.example"}},
	}
	rows := buildConfigRows(trackers, allDefs)
	if len(rows[0].URLs) != 2 {
		t.Errorf("URLs len = %d, want 2", len(rows[0].URLs))
	}
}

func TestBuildConfigRowsTypeIDFallsBackToCatalog(t *testing.T) {
	trackers := []*config.TrackerEntry{
		{DefinitionName: "TrackerA", Name: "A", TrackerType: ""},
	}
	allDefs := []defs.TrackerDef{
		{Name: "TrackerA", TypeID: "unit3d"},
	}
	rows := buildConfigRows(trackers, allDefs)
	if rows[0].TypeID != "unit3d" {
		t.Errorf("TypeID = %q, want unit3d", rows[0].TypeID)
	}
}

func TestBuildConfigRowsEmptyInputs(t *testing.T) {
	rows := buildConfigRows(nil, nil)
	if len(rows) != 0 {
		t.Errorf("expected empty rows, got %d", len(rows))
	}
}

func TestBuildConfigRowsNoCatalog(t *testing.T) {
	trackers := []*config.TrackerEntry{
		{DefinitionName: "TrackerA", Name: "A"},
	}
	rows := buildConfigRows(trackers, nil)
	if len(rows) != 1 || !rows[0].Configured {
		t.Errorf("expected 1 configured row, got %+v", rows)
	}
}

// ── managedSet ───────────────────────────────────────────────────────────────

func TestManagedSetContainsConfiguredTrackers(t *testing.T) {
	trackers := []*config.TrackerEntry{
		{DefinitionName: "TrackerA"},
		{DefinitionName: "TrackerB"},
	}
	m := managedSet(trackers)
	if !m["trackera"] {
		t.Error("managedSet missing trackera")
	}
	if !m["trackerb"] {
		t.Error("managedSet missing trackerb")
	}
	if m["trackerc"] {
		t.Error("managedSet contains trackerc which was not added")
	}
}

func TestManagedSetCaseInsensitive(t *testing.T) {
	trackers := []*config.TrackerEntry{
		{DefinitionName: "SomeTracker"},
	}
	m := managedSet(trackers)
	if !m["sometracker"] {
		t.Error("managedSet is not case-folding definition names")
	}
}

// ── staleAge ─────────────────────────────────────────────────────────────────

func TestStaleAgeNil(t *testing.T) {
	if got := staleAge(nil); got != "never synced" {
		t.Fatalf("staleAge(nil) = %q, want %q", got, "never synced")
	}
}

func TestStaleAgeSeconds(t *testing.T) {
	ts := time.Now().Add(-30 * time.Second)
	got := staleAge(&ts)
	if !contains(got, "s ago") {
		t.Errorf("staleAge(30s) = %q, want seconds format", got)
	}
}

func TestStaleAgeMinutes(t *testing.T) {
	ts := time.Now().Add(-5 * time.Minute)
	got := staleAge(&ts)
	if !contains(got, "m") {
		t.Errorf("staleAge(5min) = %q, want minutes format", got)
	}
}

func TestStaleAgeHours(t *testing.T) {
	ts := time.Now().Add(-2 * time.Hour)
	got := staleAge(&ts)
	if !contains(got, "h") {
		t.Errorf("staleAge(2h) = %q, want hours format", got)
	}
}

// ── isStale ──────────────────────────────────────────────────────────────────

func TestIsStaleNilNotStale(t *testing.T) {
	if isStale(nil) {
		t.Error("isStale(nil) = true, want false")
	}
}

func TestIsStaleRecentNotStale(t *testing.T) {
	ts := time.Now().Add(-1 * time.Minute)
	if isStale(&ts) {
		t.Error("isStale(1min ago) = true, want false")
	}
}

func TestIsStaleOldIsStale(t *testing.T) {
	ts := time.Now().Add(-20 * time.Minute)
	if !isStale(&ts) {
		t.Error("isStale(20min ago) = false, want true")
	}
}

// ── ratioClassStr ─────────────────────────────────────────────────────────────

func TestRatioClassStr(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"", "ratio-none"},
		{"0", "ratio-none"},
		{"0.3", "ratio-danger"},
		{"0.7", "ratio-warn"},
		{"1.5", "ratio-ok"},
		{"3.0", "ratio-high"},
		{"not-a-number", "ratio-none"},
	}
	for _, c := range cases {
		if got := ratioClassStr(c.input); got != c.want {
			t.Errorf("ratioClassStr(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// ── hasInt ────────────────────────────────────────────────────────────────────

func TestHasInt(t *testing.T) {
	vals := []int{1, 3, 5}
	if !hasInt(vals, 3) {
		t.Error("hasInt did not find 3")
	}
	if hasInt(vals, 4) {
		t.Error("hasInt found 4 which is not present")
	}
	if hasInt(nil, 1) {
		t.Error("hasInt(nil, 1) = true")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
