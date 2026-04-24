package main

// scylla_reconnect.go — Scylla session lifecycle manager with automatic
// reconnect after transient pool failures.
//
// Invariant: after initial successful connect, transient pool deaths (e.g.
// during Scylla topology changes on node join) must NOT require a process
// restart. This manager detects ping failures, triggers a background reconnect
// with exponential backoff, and swaps the session atomically on success so
// all subsequent queries use the new pool.

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql"
)

const (
	// reconnectFailThreshold is the number of consecutive ping failures that
	// triggers an async session rebuild.
	reconnectFailThreshold = 3

	// maxReconnectBackoff caps the per-attempt sleep between reconnect tries.
	maxReconnectBackoff = 60 * time.Second
)

// reconnectBackoffs is the per-attempt backoff schedule (capped at maxReconnectBackoff).
var reconnectBackoffs = []time.Duration{
	1 * time.Second,
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
}

// scyllaSessionMgr manages the gocql Session lifecycle with automatic reconnect.
//
// Metrics/log fields emitted:
//
//	scylla_session_generation   — incremented on each successful (re)connect
//	scylla_last_success         — time of last successful ping
//	scylla_consecutive_failures — current failure streak
//	scylla_reconnect_attempts   — lifetime reconnect loop iterations
//	scylla_reconnect_last_error — last error from a reconnect attempt
//	workflow.scylla_reconnected — log event emitted on successful reconnect
type scyllaSessionMgr struct {
	mu sync.RWMutex

	// session is the current live gocql session. Written only by set() and
	// reconnectLoop(); read by all callers via get(). Protected by mu.
	session *gocql.Session

	// generation is incremented each time a new session is installed.
	// Exported in stats for observability.
	generation uint64

	// consecutiveFails is the number of consecutive watchdog ping failures
	// since the last success. Written atomically from any goroutine.
	consecutiveFails atomic.Int32

	// reconnecting is true while a background reconnect goroutine is running.
	// Prevents concurrent reconnect storms.
	reconnecting atomic.Bool

	// reconnectAttempts is the lifetime count of reconnect loop iterations.
	// Protected by mu.
	reconnectAttempts int

	// lastSuccessAt is the time of the most recent successful ping.
	// Protected by mu.
	lastSuccessAt time.Time

	// reconnectLastError is the last error seen during a reconnect attempt.
	// Protected by mu.
	reconnectLastError string

	logger    *slog.Logger
	connectFn func() (*gocql.Session, error)
}

func newScyllaSessionMgr(logger *slog.Logger, connectFn func() (*gocql.Session, error)) *scyllaSessionMgr {
	return &scyllaSessionMgr{
		logger:    logger,
		connectFn: connectFn,
	}
}

// set stores the initial session after a successful startup connect.
func (m *scyllaSessionMgr) set(s *gocql.Session) {
	m.mu.Lock()
	m.session = s
	m.generation++
	m.lastSuccessAt = time.Now()
	m.mu.Unlock()
	m.consecutiveFails.Store(0)
}

// get returns the current live session. Returns nil when a reconnect is in
// progress — callers must treat nil as codes.Unavailable.
func (m *scyllaSessionMgr) get() *gocql.Session {
	m.mu.RLock()
	s := m.session
	m.mu.RUnlock()
	return s
}

// onPingSuccess resets the failure counter and updates the last-success timestamp.
func (m *scyllaSessionMgr) onPingSuccess() {
	m.consecutiveFails.Store(0)
	m.mu.Lock()
	m.lastSuccessAt = time.Now()
	m.mu.Unlock()
}

// onPingFailure increments the failure counter. After reconnectFailThreshold
// consecutive failures, it launches a background reconnect goroutine (at most
// one at a time).
func (m *scyllaSessionMgr) onPingFailure(ctx context.Context) {
	n := m.consecutiveFails.Add(1)
	if int(n) < reconnectFailThreshold {
		return
	}
	if m.reconnecting.CompareAndSwap(false, true) {
		go m.reconnectLoop(ctx)
	}
}

// reconnectLoop rebuilds the gocql session with exponential backoff. When
// successful it swaps the session in-place and logs workflow.scylla_reconnected.
func (m *scyllaSessionMgr) reconnectLoop(ctx context.Context) {
	defer m.reconnecting.Store(false)

	// Nil out the current session immediately so that getSession() returns nil
	// and callers receive codes.Unavailable instead of hitting a broken pool.
	m.mu.Lock()
	old := m.session
	m.session = nil
	m.mu.Unlock()

	if old != nil {
		old.Close()
		m.logger.Info("scylla_reconnect: stale session closed — rebuilding pool")
	}

	for attempt := 1; ; attempt++ {
		if ctx.Err() != nil {
			m.logger.Warn("scylla_reconnect: context cancelled, aborting reconnect")
			return
		}

		backoff := maxReconnectBackoff
		if attempt-1 < len(reconnectBackoffs) {
			backoff = reconnectBackoffs[attempt-1]
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		m.mu.Lock()
		m.reconnectAttempts++
		m.mu.Unlock()

		m.logger.Info("scylla_reconnect: attempting reconnect",
			"attempt", attempt, "backoff", backoff)

		newSession, err := m.connectFn()
		if err != nil {
			m.mu.Lock()
			m.reconnectLastError = err.Error()
			m.mu.Unlock()
			m.logger.Warn("scylla_reconnect: attempt failed",
				"attempt", attempt, "err", err)
			continue
		}

		// Success: install new session.
		m.mu.Lock()
		m.session = newSession
		m.generation++
		m.lastSuccessAt = time.Now()
		m.reconnectLastError = ""
		gen := m.generation
		attempts := m.reconnectAttempts
		m.mu.Unlock()

		m.consecutiveFails.Store(0)

		m.logger.Info("workflow.scylla_reconnected",
			"scylla_session_generation", gen,
			"scylla_reconnect_attempts", attempts,
		)
		return
	}
}

// close closes the current session if any. Called on server shutdown.
func (m *scyllaSessionMgr) close() {
	m.mu.Lock()
	s := m.session
	m.session = nil
	m.mu.Unlock()
	if s != nil {
		s.Close()
	}
}

// stats returns diagnostic fields for logging/metrics (Phase 1.6).
func (m *scyllaSessionMgr) stats() (generation uint64, lastSuccess time.Time, consecutiveFails int32, reconnectAttempts int, reconnectLastError string, isReconnecting bool) {
	m.mu.RLock()
	generation = m.generation
	lastSuccess = m.lastSuccessAt
	reconnectAttempts = m.reconnectAttempts
	reconnectLastError = m.reconnectLastError
	m.mu.RUnlock()
	consecutiveFails = m.consecutiveFails.Load()
	isReconnecting = m.reconnecting.Load()
	return
}

// sessionUnavailableError is returned by getSession when the manager has no
// live session (initial connect not done, or reconnect in progress).
func sessionUnavailableError(reconnecting bool) error {
	if reconnecting {
		return fmt.Errorf("WORKFLOW_DEPENDENCY_UNAVAILABLE: dependency=scylla reconnecting=true")
	}
	return fmt.Errorf("WORKFLOW_DEPENDENCY_UNAVAILABLE: dependency=scylla reconnecting=false")
}
