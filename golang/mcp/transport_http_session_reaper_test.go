package main

import (
	"testing"
	"time"
)

// TestReapIdleSessions locks in the AWG re-audit fix
// (meta.write_creates_completion_obligation): createSession writes a session
// entry whose only client-driven removal is an explicit DELETE, so without a
// reaper a dropped client leaks its entry forever. reapIdleSessions must evict
// entries idle longer than sessionIdleTTL and keep recently-seen ones.
func TestReapIdleSessions(t *testing.T) {
	sessionMu.Lock()
	sessionStore = map[string]*mcpSession{}
	sessionMu.Unlock()

	now := time.Now()
	fresh := createSession()

	// A stale session: present but last seen well beyond the idle TTL.
	sessionMu.Lock()
	sessionStore["stale"] = &mcpSession{
		createdAt: now.Add(-2 * sessionIdleTTL),
		lastSeen:  now.Add(-2 * sessionIdleTTL),
	}
	sessionMu.Unlock()

	if n := reapIdleSessions(now); n != 1 {
		t.Fatalf("expected 1 idle session reaped, got %d", n)
	}
	if !touchSession(fresh) {
		t.Error("fresh session must survive reaping")
	}

	sessionMu.RLock()
	_, staleStillThere := sessionStore["stale"]
	sessionMu.RUnlock()
	if staleStillThere {
		t.Error("stale session must be evicted")
	}
}
