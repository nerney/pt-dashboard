package auth

import (
	"errors"
	"testing"
)

func TestBeginReturnsUniqueTokens(t *testing.T) {
	m := NewManager(nil)
	id1, err := m.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	m.End()
	id2, err := m.Begin()
	if err != nil {
		t.Fatalf("second Begin() error = %v", err)
	}
	if id1 == id2 {
		t.Fatal("Begin() returned same token twice")
	}
}

func TestBeginRejectsSecondActiveSession(t *testing.T) {
	m := NewManager(nil)
	if _, err := m.Begin(); err != nil {
		t.Fatalf("first Begin() error = %v", err)
	}
	_, err := m.Begin()
	if !errors.Is(err, ErrSessionActive) {
		t.Fatalf("second Begin() = %v, want ErrSessionActive", err)
	}
}

func TestValidateRejectsNoSession(t *testing.T) {
	m := NewManager(nil)
	if err := m.Validate("anything"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Validate() = %v, want ErrNoSession", err)
	}
}

func TestValidateRejectsWrongID(t *testing.T) {
	m := NewManager(nil)
	if _, err := m.Begin(); err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := m.Validate("wrong-id"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Validate(wrong) = %v, want ErrNoSession", err)
	}
}

func TestValidateAcceptsCorrectID(t *testing.T) {
	m := NewManager(nil)
	id, err := m.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if err := m.Validate(id); err != nil {
		t.Fatalf("Validate(correct) = %v, want nil", err)
	}
}

func TestEndClearsSession(t *testing.T) {
	m := NewManager(nil)
	id, _ := m.Begin()
	m.End()
	if err := m.Validate(id); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Validate after End = %v, want ErrNoSession", err)
	}
}

func TestEndIsIdempotent(t *testing.T) {
	m := NewManager(nil)
	m.End()
	m.End()
	if _, err := m.Begin(); err != nil {
		t.Fatalf("Begin after double End = %v", err)
	}
}

func TestEndFiresOnExpire(t *testing.T) {
	fired := 0
	m := NewManager(func() { fired++ })
	m.Begin()
	m.End()
	if fired != 1 {
		t.Fatalf("onExpire fired %d times, want 1", fired)
	}
}

func TestEndDoesNotFireOnExpireWhenNoSession(t *testing.T) {
	fired := 0
	m := NewManager(func() { fired++ })
	m.End()
	if fired != 0 {
		t.Fatalf("onExpire fired %d times, want 0", fired)
	}
}

func TestHasSessionFalseWhenEmpty(t *testing.T) {
	m := NewManager(nil)
	if m.HasSession() {
		t.Fatal("HasSession() = true, want false on empty manager")
	}
}

func TestHasSessionTrueAfterBegin(t *testing.T) {
	m := NewManager(nil)
	m.Begin()
	if !m.HasSession() {
		t.Fatal("HasSession() = false, want true after Begin")
	}
}

func TestHasSessionFalseAfterEnd(t *testing.T) {
	m := NewManager(nil)
	m.Begin()
	m.End()
	if m.HasSession() {
		t.Fatal("HasSession() = true, want false after End")
	}
}

func TestBeginSweepsExpiredSession(t *testing.T) {
	// Construct a Manager whose existing session is already expired by
	// manually forcing lastActivity into the past (white-box via unexported
	// fields since we own the package).
	fired := 0
	m := NewManager(func() { fired++ })
	id, _ := m.Begin()

	// Force expiry by pushing lastActivity behind InactivityTimeout.
	m.mu.Lock()
	m.current.lastActivity = m.current.lastActivity.Add(-(InactivityTimeout + 1))
	m.mu.Unlock()

	// Begin should sweep the stale session and start a fresh one.
	newID, err := m.Begin()
	if err != nil {
		t.Fatalf("Begin after expiry = %v, want nil", err)
	}
	if newID == id {
		t.Fatal("Begin returned same ID for swept session")
	}
	// onExpire must have fired once (for the sweep) not twice.
	if fired != 1 {
		t.Fatalf("onExpire fired %d times, want 1", fired)
	}
}

func TestValidateReturnsExpiredWhenInactivityElapsed(t *testing.T) {
	fired := 0
	m := NewManager(func() { fired++ })
	id, _ := m.Begin()

	m.mu.Lock()
	m.current.lastActivity = m.current.lastActivity.Add(-(InactivityTimeout + 1))
	m.mu.Unlock()

	if err := m.Validate(id); !errors.Is(err, ErrExpired) {
		t.Fatalf("Validate() = %v, want ErrExpired", err)
	}
	if fired != 1 {
		t.Fatalf("onExpire fired %d times after expiry, want 1", fired)
	}
}

func TestValidateReturnsExpiredWhenAbsoluteTimeoutElapsed(t *testing.T) {
	m := NewManager(nil)
	id, _ := m.Begin()

	m.mu.Lock()
	m.current.createdAt = m.current.createdAt.Add(-(AbsoluteTimeout + 1))
	m.mu.Unlock()

	if err := m.Validate(id); !errors.Is(err, ErrExpired) {
		t.Fatalf("Validate() = %v, want ErrExpired (absolute timeout)", err)
	}
}

func TestGenerateIDLength(t *testing.T) {
	id, err := generateID()
	if err != nil {
		t.Fatalf("generateID() error = %v", err)
	}
	// base64url(32 bytes) with no padding = 43 chars.
	if len(id) != 43 {
		t.Fatalf("generateID() len = %d, want 43", len(id))
	}
}
