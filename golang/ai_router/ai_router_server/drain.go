package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// drainManager tracks endpoints being drained and enforces per-class
// grace periods before they can be fully removed.
//
// Stream-heavy services (event.OnEvent, log streams) need extended drain
// periods because existing streams can't be moved. The drain manager
// ensures weight goes to 0 (no new connections) but existing streams
// get time to complete naturally.
type drainManager struct {
	mu     sync.Mutex
	active map[string]*drainState // "service/endpoint" → state
}

type drainState struct {
	Service     string
	Endpoint    string
	Class       ai_routerpb.ServiceClass
	Reason      string
	StartedAt   time.Time
	GracePeriod time.Duration
	Completed   bool
	CompletedAt time.Time
}

func newDrainManager() *drainManager {
	return &drainManager{
		active: make(map[string]*drainState),
	}
}

// graceForClass returns the drain grace period per service class.
func graceForClass(class ai_routerpb.ServiceClass) time.Duration {
	switch class {
	case ai_routerpb.ServiceClass_STREAM_HEAVY:
		return 5 * time.Minute // long-lived streams need time
	case ai_routerpb.ServiceClass_CONTROL_PLANE:
		return 3 * time.Minute // important services, careful drain
	case ai_routerpb.ServiceClass_DEPLOYMENT_SENSITIVE:
		return 1 * time.Minute // moderate drain
	default: // STATELESS_UNARY
		return 30 * time.Second // fast drain, no persistent connections
	}
}

// shouldDrain decides if an endpoint should enter drain based on its score.
// An endpoint enters drain when its weight drops below the drain threshold
// for its service class.
func shouldDrain(weight uint32, class ai_routerpb.ServiceClass) bool {
	switch class {
	case ai_routerpb.ServiceClass_CONTROL_PLANE:
		return false // never drain control plane
	case ai_routerpb.ServiceClass_STREAM_HEAVY:
		return weight <= 10 // very low weight triggers drain
	default:
		return weight <= 5 // near-zero triggers drain
	}
}

// startDrain begins draining an endpoint. Returns true if this is a new drain.
func (dm *drainManager) startDrain(service, endpoint string, class ai_routerpb.ServiceClass, reason string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	key := service + "/" + endpoint
	if existing, ok := dm.active[key]; ok && !existing.Completed {
		return false // already draining
	}

	dm.active[key] = &drainState{
		Service:     service,
		Endpoint:    endpoint,
		Class:       class,
		Reason:      reason,
		StartedAt:   time.Now(),
		GracePeriod: graceForClass(class),
	}

	logger.Info("drain_started",
		"service", service,
		"endpoint", endpoint,
		"class", class.String(),
		"grace", graceForClass(class),
		"reason", reason,
	)

	return true
}

// isDraining returns true if the endpoint is currently in drain state.
func (dm *drainManager) isDraining(service, endpoint string) bool {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	key := service + "/" + endpoint
	state, ok := dm.active[key]
	if !ok {
		return false
	}
	return !state.Completed
}

// cancelDrain removes an endpoint from drain (e.g., it recovered).
func (dm *drainManager) cancelDrain(service, endpoint string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	key := service + "/" + endpoint
	if state, ok := dm.active[key]; ok && !state.Completed {
		logger.Info("drain_cancelled",
			"service", service,
			"endpoint", endpoint,
			"reason", "endpoint recovered",
		)
		delete(dm.active, key)
	}
}

// applyDrains processes active drains: sets weight=0 for draining endpoints,
// completes drains that have exceeded their grace period, and populates
// the policy's drain entries.
func (dm *drainManager) applyDrains(policy *ai_routerpb.RoutingPolicy) []string {
	if policy == nil {
		return nil
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	now := time.Now()
	var events []string

	for key, state := range dm.active {
		if state.Completed {
			// Clean up completed drains after 2x grace period.
			if now.Sub(state.CompletedAt) > 2*state.GracePeriod {
				delete(dm.active, key)
			}
			continue
		}

		sp := policy.Services[state.Service]
		if sp == nil {
			continue
		}

		// Set weight to 0 (no new connections).
		if _, ok := sp.Weights[state.Endpoint]; ok {
			sp.Weights[state.Endpoint] = 0
		}

		// Add drain entry to policy.
		sp.Drain = append(sp.Drain, &ai_routerpb.DrainEntry{
			Endpoint:      state.Endpoint,
			Reason:        state.Reason,
			StartedAtMs:   state.StartedAt.UnixMilli(),
			GracePeriodMs: state.GracePeriod.Milliseconds(),
		})

		// Check if grace period expired → mark complete.
		if now.Sub(state.StartedAt) >= state.GracePeriod {
			state.Completed = true
			state.CompletedAt = now
			events = append(events, fmt.Sprintf(
				"drain_completed: %s/%s after %s",
				state.Service, state.Endpoint,
				state.GracePeriod))

			logger.Info("drain_completed",
				"service", state.Service,
				"endpoint", state.Endpoint,
				"duration", now.Sub(state.StartedAt).Round(time.Second),
			)
		}
	}

	return events
}

// activeDrains returns the count of endpoints currently draining.
func (dm *drainManager) activeDrains() int {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	count := 0
	for _, state := range dm.active {
		if !state.Completed {
			count++
		}
	}
	return count
}

// getDrainEntries returns all active drain states (for GetStatus).
func (dm *drainManager) getDrainEntries() []*ai_routerpb.DrainEntry {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	var entries []*ai_routerpb.DrainEntry
	for _, state := range dm.active {
		if state.Completed {
			continue
		}
		entries = append(entries, &ai_routerpb.DrainEntry{
			Endpoint:      state.Endpoint,
			Reason:        state.Reason,
			StartedAtMs:   state.StartedAt.UnixMilli(),
			GracePeriodMs: state.GracePeriod.Milliseconds(),
		})
	}
	return entries
}
