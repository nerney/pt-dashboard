package auth

import (
	"testing"
	"time"
)

func TestAllowedTrueForUnknownIP(t *testing.T) {
	r := NewRateLimiter()
	if !r.Allowed("1.2.3.4") {
		t.Fatal("Allowed() = false for unknown IP, want true")
	}
}

func TestRecordFailureLockAfterMaxAttempts(t *testing.T) {
	r := NewRateLimiter()
	ip := "10.0.0.1"
	for i := 0; i < maxAttempts; i++ {
		if !r.Allowed(ip) {
			t.Fatalf("Allowed() = false before maxAttempts (iteration %d)", i)
		}
		r.RecordFailure(ip)
	}
	if r.Allowed(ip) {
		t.Fatal("Allowed() = true after maxAttempts, want false")
	}
}

func TestRecordSuccessResetsState(t *testing.T) {
	r := NewRateLimiter()
	ip := "10.0.0.2"
	for i := 0; i < maxAttempts; i++ {
		r.RecordFailure(ip)
	}
	if r.Allowed(ip) {
		t.Fatal("Allowed() should be false after lock")
	}
	r.RecordSuccess(ip)
	if !r.Allowed(ip) {
		t.Fatal("Allowed() = false after RecordSuccess, want true")
	}
}

func TestLockedIPIsReleasedAfterLockDuration(t *testing.T) {
	r := NewRateLimiter()
	ip := "10.0.0.3"
	for i := 0; i < maxAttempts; i++ {
		r.RecordFailure(ip)
	}
	if r.Allowed(ip) {
		t.Fatal("Allowed() should be false immediately after lock")
	}

	// Push the lockedAt timestamp past lockDuration.
	r.mu.Lock()
	r.records[ip].lockedAt = r.records[ip].lockedAt.Add(-(lockDuration + time.Second))
	r.mu.Unlock()

	if !r.Allowed(ip) {
		t.Fatal("Allowed() = false after lock expired, want true")
	}
	// Record should have been cleaned up.
	r.mu.Lock()
	_, exists := r.records[ip]
	r.mu.Unlock()
	if exists {
		t.Fatal("expired record not cleaned up from records map")
	}
}

func TestOldFailuresTrimmedOutsideWindow(t *testing.T) {
	r := NewRateLimiter()
	ip := "10.0.0.4"

	// Add maxAttempts-1 failures that are older than attemptWindow.
	r.mu.Lock()
	rec := &attemptRecord{}
	old := time.Now().Add(-(attemptWindow + time.Second))
	for i := 0; i < maxAttempts-1; i++ {
		rec.failures = append(rec.failures, old)
	}
	r.records[ip] = rec
	r.mu.Unlock()

	// One fresh failure should not trigger lockout because old ones are trimmed.
	r.RecordFailure(ip)
	if !r.Allowed(ip) {
		t.Fatal("Allowed() = false, but old failures should have been trimmed")
	}
}

func TestDifferentIPsTrackedIndependently(t *testing.T) {
	r := NewRateLimiter()
	ip1 := "10.1.0.1"
	ip2 := "10.1.0.2"

	for i := 0; i < maxAttempts; i++ {
		r.RecordFailure(ip1)
	}
	// ip1 locked, ip2 not.
	if r.Allowed(ip1) {
		t.Fatal("ip1 should be locked")
	}
	if !r.Allowed(ip2) {
		t.Fatal("ip2 should not be locked")
	}
}
