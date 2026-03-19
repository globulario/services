package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/globulario/services/golang/event/eventpb"
)

// clusterContext tracks deployment and recovery state from cluster controller
// events, so the scorer can distinguish expected disruption from real problems.
type clusterContext struct {
	mu sync.RWMutex

	// Services currently being deployed/updated.
	deploying map[string]*deployState

	// Nodes currently recovering (recently marked healthy after unhealthy).
	recovering map[string]*recoverState

	// Services with active plan execution.
	planActive map[string]*planState
}

type deployState struct {
	Service   string
	Phase     string    // APPLYING, RESOLVING, etc.
	StartedAt time.Time
}

type recoverState struct {
	NodeID      string
	RecoveredAt time.Time
	WarmupUntil time.Time // ramp-up period after recovery
}

type planState struct {
	NodeID    string
	PlanType  string // "network" or "service"
	StartedAt time.Time
}

const (
	deployTimeout  = 10 * time.Minute // deployment context expires after 10 min
	warmupDuration = 30 * time.Second // ramp-up period after node recovery
)

func newClusterContext() *clusterContext {
	return &clusterContext{
		deploying:  make(map[string]*deployState),
		recovering: make(map[string]*recoverState),
		planActive: make(map[string]*planState),
	}
}

// handleClusterEvent processes cluster controller events to track
// deployment and recovery state.
func (cc *clusterContext) handleClusterEvent(evt *eventpb.Event) {
	if evt == nil {
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(evt.Data, &payload); err != nil {
		return
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()

	switch evt.Name {
	case "service.phase_changed":
		service, _ := payload["service"].(string)
		toPhase, _ := payload["to_phase"].(string)
		if service == "" {
			return
		}

		switch toPhase {
		case "APPLYING", "PENDING", "RESOLVED":
			// Deployment in progress.
			cc.deploying[service] = &deployState{
				Service:   service,
				Phase:     toPhase,
				StartedAt: now,
			}
		case "AVAILABLE":
			// Deployment complete — remove from deploying.
			delete(cc.deploying, service)
		case "FAILED", "ROLLED_BACK", "DEGRADED":
			// Deployment failed — keep context briefly for scoring.
			if d, ok := cc.deploying[service]; ok {
				d.Phase = toPhase
			}
		}

	case "cluster.health.recovered":
		nodeID, _ := payload["node_id"].(string)
		if nodeID == "" {
			return
		}
		cc.recovering[nodeID] = &recoverState{
			NodeID:      nodeID,
			RecoveredAt: now,
			WarmupUntil: now.Add(warmupDuration),
		}

	case "cluster.health.degraded":
		nodeID, _ := payload["node_id"].(string)
		if nodeID != "" {
			// Node went unhealthy — remove from recovering.
			delete(cc.recovering, nodeID)
		}

	case "plan_apply_started":
		nodeID, _ := payload["node_id"].(string)
		planType, _ := payload["plan_type"].(string)
		if nodeID != "" {
			cc.planActive[nodeID] = &planState{
				NodeID:    nodeID,
				PlanType:  planType,
				StartedAt: now,
			}
		}

	case "plan_apply_succeeded", "plan_apply_failed":
		nodeID, _ := payload["node_id"].(string)
		if nodeID != "" {
			delete(cc.planActive, nodeID)
		}
	}
}

// isDeploying returns true if a service is currently being deployed.
func (cc *clusterContext) isDeploying(service string) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	d, ok := cc.deploying[service]
	if !ok {
		return false
	}
	// Expire stale deployment context.
	if time.Since(d.StartedAt) > deployTimeout {
		return false
	}
	return true
}

// isWarming returns true if a node is in warm-up period after recovery.
func (cc *clusterContext) isWarming(nodeID string) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	r, ok := cc.recovering[nodeID]
	if !ok {
		return false
	}
	return time.Now().Before(r.WarmupUntil)
}

// hasPlanActive returns true if a node has an active plan being applied.
func (cc *clusterContext) hasPlanActive(nodeID string) bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	p, ok := cc.planActive[nodeID]
	if !ok {
		return false
	}
	// Expire stale plan context.
	return time.Since(p.StartedAt) < deployTimeout
}

// getDeploymentModifier returns a score modifier for deployment context.
// During deployment: scorer should be more tolerant (higher threshold before
// reducing weight, because the service is expected to be disrupted).
// During warm-up: reduce weight initially, ramp up gradually.
func (cc *clusterContext) getDeploymentModifier(service string) float64 {
	if cc.isDeploying(service) {
		return -0.15 // reduce score by 0.15 (makes endpoint look healthier)
	}
	return 0
}

// getWarmupWeight returns the weight cap for a warming-up node.
// Returns 0 if the node is not warming up (no cap).
func (cc *clusterContext) getWarmupWeight(nodeID string) uint32 {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	r, ok := cc.recovering[nodeID]
	if !ok {
		return 0
	}

	elapsed := time.Since(r.RecoveredAt)
	if elapsed >= warmupDuration {
		return 0 // warm-up complete, no cap
	}

	// Linear ramp: 25% → 100% over warmupDuration.
	progress := float64(elapsed) / float64(warmupDuration)
	return uint32(25 + 75*progress) // 25 at start, 100 at end
}

// cleanup removes expired entries.
func (cc *clusterContext) cleanup() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	for k, d := range cc.deploying {
		if now.Sub(d.StartedAt) > deployTimeout {
			delete(cc.deploying, k)
		}
	}
	for k, r := range cc.recovering {
		if now.After(r.WarmupUntil) {
			delete(cc.recovering, k)
		}
	}
	for k, p := range cc.planActive {
		if now.Sub(p.StartedAt) > deployTimeout {
			delete(cc.planActive, k)
		}
	}
}
