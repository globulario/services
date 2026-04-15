package globular_service

import (
	"sync"
	"time"
)

// SubsystemState represents the health state of a background subsystem.
type SubsystemState int

const (
	SubsystemHealthy  SubsystemState = iota // operating normally
	SubsystemDegraded                        // errors occurring but still running
	SubsystemFailed                          // not functioning
	SubsystemStarting                        // initializing
	SubsystemStopped                         // intentionally shut down
)

func (s SubsystemState) String() string {
	switch s {
	case SubsystemHealthy:
		return "healthy"
	case SubsystemDegraded:
		return "degraded"
	case SubsystemFailed:
		return "failed"
	case SubsystemStarting:
		return "starting"
	case SubsystemStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// SubsystemEntry is a point-in-time snapshot of a single subsystem's health.
type SubsystemEntry struct {
	Name         string
	State        SubsystemState
	LastTick     time.Time
	LastError    string
	ErrorCount   int64 // consecutive errors
	Metadata     map[string]string
	RegisteredAt time.Time
	ExpectedTick time.Duration // expected interval between ticks; 0 = unknown
}

// IsStale returns true if the subsystem hasn't ticked within 3x its expected interval.
func (e SubsystemEntry) IsStale() bool {
	if e.ExpectedTick <= 0 || e.LastTick.IsZero() {
		return false
	}
	return time.Since(e.LastTick) > e.ExpectedTick*3
}

// SubsystemHandle is held by a goroutine to report its health.
// All methods are safe for concurrent use.
type SubsystemHandle struct {
	mu           sync.Mutex
	name         string
	state        SubsystemState
	lastTick     time.Time
	lastError    string
	errorCount   int64
	metadata     map[string]string
	registeredAt time.Time
	expectedTick time.Duration
}

// Tick records a successful iteration. Resets error count and state to healthy.
func (h *SubsystemHandle) Tick() {
	h.mu.Lock()
	h.lastTick = time.Now()
	h.errorCount = 0
	h.state = SubsystemHealthy
	h.mu.Unlock()
}

// TickError records a failed iteration. After 3 consecutive errors the state
// transitions to degraded; after 10 it transitions to failed.
func (h *SubsystemHandle) TickError(err error) {
	h.mu.Lock()
	h.lastTick = time.Now()
	h.errorCount++
	if err != nil {
		h.lastError = err.Error()
	}
	switch {
	case h.errorCount >= 10:
		h.state = SubsystemFailed
	case h.errorCount >= 3:
		h.state = SubsystemDegraded
	}
	h.mu.Unlock()
}

// SetState overrides the computed state. Use for subsystems that manage
// their own state machine (e.g., leader election: STARTING → HEALTHY).
func (h *SubsystemHandle) SetState(s SubsystemState) {
	h.mu.Lock()
	h.state = s
	h.mu.Unlock()
}

// SetError records an error message without counting it as a tick.
func (h *SubsystemHandle) SetError(msg string) {
	h.mu.Lock()
	h.lastError = msg
	h.mu.Unlock()
}

// SetMeta sets a metadata key-value pair on this subsystem.
func (h *SubsystemHandle) SetMeta(key, value string) {
	h.mu.Lock()
	if h.metadata == nil {
		h.metadata = make(map[string]string)
	}
	h.metadata[key] = value
	h.mu.Unlock()
}

func (h *SubsystemHandle) snapshot() SubsystemEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	meta := make(map[string]string, len(h.metadata))
	for k, v := range h.metadata {
		meta[k] = v
	}
	return SubsystemEntry{
		Name:         h.name,
		State:        h.state,
		LastTick:     h.lastTick,
		LastError:    h.lastError,
		ErrorCount:   h.errorCount,
		Metadata:     meta,
		RegisteredAt: h.registeredAt,
		ExpectedTick: h.expectedTick,
	}
}

// ─── Global Registry ────────────────────────────────────────────────────────

var globalRegistry = &subsystemRegistry{
	handles: make(map[string]*SubsystemHandle),
}

type subsystemRegistry struct {
	mu      sync.RWMutex
	handles map[string]*SubsystemHandle
}

// RegisterSubsystem registers a named subsystem with an expected tick interval.
// If interval is 0, staleness detection is disabled for this subsystem.
// Calling Register twice with the same name returns the existing handle.
func RegisterSubsystem(name string, expectedInterval time.Duration) *SubsystemHandle {
	return globalRegistry.register(name, expectedInterval)
}

// DeregisterSubsystem removes a subsystem from the registry.
func DeregisterSubsystem(name string) {
	globalRegistry.deregister(name)
}

// SubsystemSnapshot returns a point-in-time copy of all registered subsystems.
// Stale subsystems are automatically marked as failed.
func SubsystemSnapshot() []SubsystemEntry {
	return globalRegistry.snapshot()
}

// SubsystemOverallState returns the worst state across all subsystems.
func SubsystemOverallState() SubsystemState {
	entries := SubsystemSnapshot()
	worst := SubsystemHealthy
	for _, e := range entries {
		if e.State > worst {
			worst = e.State
		}
	}
	return worst
}

func (r *subsystemRegistry) register(name string, interval time.Duration) *SubsystemHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.handles[name]; ok {
		return h
	}
	h := &SubsystemHandle{
		name:         name,
		state:        SubsystemStarting,
		registeredAt: time.Now(),
		expectedTick: interval,
	}
	r.handles[name] = h
	return h
}

func (r *subsystemRegistry) deregister(name string) {
	r.mu.Lock()
	delete(r.handles, name)
	r.mu.Unlock()
}

func (r *subsystemRegistry) snapshot() []SubsystemEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := make([]SubsystemEntry, 0, len(r.handles))
	for _, h := range r.handles {
		e := h.snapshot()
		// Auto-detect stale subsystems.
		if e.IsStale() && e.State < SubsystemFailed {
			e.State = SubsystemFailed
			if e.LastError == "" {
				e.LastError = "subsystem stale: no tick received"
			}
		}
		entries = append(entries, e)
	}
	return entries
}
