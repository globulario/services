package main

// posture.go: Cluster Posture / Storm-Control Layer.
//
// The posture loop evaluates cluster health signals every 30 seconds and
// maintains a ClusterPosture value. Results are written to etcd and exposed
// via Prometheus.
//
// # Enforcement gates (active)
//
//	Gate 1: ROLLOUT-class workflows suppressed when posture == RECOVERY_ONLY.
//	        release.apply.package and release.remove.package return a transient
//	        error so the release stays RESOLVED and retries on the next cycle.
//
// # Posture states
//
//	Normal       — full operation; all work classes allowed
//	Degraded     — meaningful pressure detected; posture recorded and logged
//	RecoveryOnly — crisis level; ROLLOUT dispatch suppressed
//
// # Trusted triggers
//
//	workflow circuit breaker open
//	reconcile circuit breaker open
//	leader liveness degraded (stale heartbeat from other nodes)
//
// # Observational signals (not yet enforcement triggers)
//
//	unreachable node fraction   — denominator validity unvalidated on live cluster
//	ACC P2 rejection rate       — requires delta tracking, not yet connected
//
// # etcd key
//
//	/globular/system/posture — JSON PostureSnapshot, read by doctor + CLI.
//	Written on transition and as a 5-minute heartbeat.
//	Doctor treats key as stale if age > 10 minutes and falls back to Degraded.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ── constants ─────────────────────────────────────────────────────────────────

const (
	// postureEvalInterval is how often the posture loop runs.
	postureEvalInterval = 30 * time.Second

	// postureHeartbeatInterval is the maximum time between etcd writes even
	// when posture has not changed. Keeps the key fresh for the doctor's
	// stale-age check.
	postureHeartbeatInterval = 5 * time.Minute

	// postureHysteresisTicks is the number of consecutive evaluation ticks
	// at a lower (less severe) posture required before transitioning upward.
	// Downward escalation (Normal→Degraded→RecoveryOnly) is always immediate.
	postureHysteresisTicks = 2

	// postureEtcdKey is the authoritative posture location in etcd.
	postureEtcdKey = "/globular/system/posture"

	// postureEtcdWriteTimeout caps individual etcd write calls.
	postureEtcdWriteTimeout = 5 * time.Second
)

// ── ClusterPosture ────────────────────────────────────────────────────────────

// ClusterPosture is an ordered severity enum. Higher values are more severe.
// It is stored as an atomic int32 on the server struct for zero-copy reads.
type ClusterPosture int32

const (
	PostureNormal      ClusterPosture = 0
	PostureDegraded    ClusterPosture = 1
	PostureRecoveryOnly ClusterPosture = 2
)

func (p ClusterPosture) String() string {
	switch p {
	case PostureNormal:
		return "NORMAL"
	case PostureDegraded:
		return "DEGRADED"
	case PostureRecoveryOnly:
		return "RECOVERY_ONLY"
	default:
		return "UNKNOWN"
	}
}

// ── WorkloadClass ─────────────────────────────────────────────────────────────

// WorkloadClass classifies the kind of work being dispatched.
// Used in phase 2 enforcement gates and in posture snapshot reporting.
type WorkloadClass string

const (
	WorkClassLiveness      WorkloadClass = "LIVENESS"        // health probes, heartbeats, workflow completion
	WorkClassConvergence   WorkloadClass = "CONVERGENCE"     // cluster.reconcile, install of already-desired version
	WorkClassRepairTargeted WorkloadClass = "REPAIR_TARGETED" // node.repair from_repository / from_reference
	WorkClassRepairReseed  WorkloadClass = "REPAIR_RESEED"   // node.repair full_reseed, node.reseed
	WorkClassTopology      WorkloadClass = "TOPOLOGY"        // node.join, node.remove
	WorkClassRollout       WorkloadClass = "ROLLOUT"         // cluster.update, new desired service deployments
	WorkClassBackground    WorkloadClass = "BACKGROUND"      // doctor auto-heal, cache cleanup
)

// ── Workflow classification ────────────────────────────────────────────────────

// mapWorkflowToClass maps a workflow name to its WorkloadClass for posture gating.
// Workflows not explicitly listed are treated as WorkClassBackground (lowest
// priority and most suppressible).
func mapWorkflowToClass(workflowName string) WorkloadClass {
	switch workflowName {
	case "cluster.invariant.enforcement":
		return WorkClassLiveness
	case "node.bootstrap", "node.join", "node.remove":
		return WorkClassTopology
	case "node.recover.full_reseed":
		return WorkClassRepairReseed
	case "node.repair":
		return WorkClassRepairTargeted
	case "cluster.reconcile":
		return WorkClassConvergence
	case "release.apply.package", "release.remove.package":
		return WorkClassRollout
	case "repository.sync.upstream":
		return WorkClassBackground
	default:
		return WorkClassBackground
	}
}

// postureGateCheck returns a transient suppression error if the cluster posture
// disallows dispatching the given workflow. Returns nil if dispatch is allowed.
//
// Gate 1 rule: suppress ROLLOUT-class dispatch when posture is RECOVERY_ONLY.
// All other work classes pass through at all posture levels.
//
// The returned error contains "posture gate" so the release pipeline's transient
// error classifier keeps the release in RESOLVED (retryable) rather than FAILED.
func postureGateCheck(posture ClusterPosture, workflowName string) error {
	if posture != PostureRecoveryOnly {
		return nil
	}
	class := mapWorkflowToClass(workflowName)
	if class != WorkClassRollout {
		return nil
	}
	return fmt.Errorf("posture gate: cluster in RECOVERY_ONLY — %s dispatch suppressed (will retry when posture clears)", workflowName)
}

// ── PostureSignals ────────────────────────────────────────────────────────────

// PostureSignals holds the raw values read during a posture evaluation.
// All fields are exported for JSON serialisation into the etcd snapshot.
type PostureSignals struct {
	// Trusted triggers — enforcement-ready after live validation.
	WorkflowCBOpen        bool `json:"workflow_cb_open"`
	ReconcileCBOpen       bool `json:"reconcile_cb_open"`
	LeaderLivenessDegraded bool `json:"leader_liveness_degraded"`

	// Observational — not yet used for posture decisions.
	KnownNodes          int     `json:"known_nodes"`
	UnreachableNodes    int     `json:"unreachable_nodes"`
	UnreachableFraction float64 `json:"unreachable_fraction,omitempty"` // 0 when KnownNodes < 3
}

// ── PostureSnapshot ───────────────────────────────────────────────────────────

// PostureSnapshot is the full point-in-time posture state written to etcd
// and returned to CLI/Prometheus consumers.
type PostureSnapshot struct {
	Posture     ClusterPosture `json:"-"`         // stored as string below
	PostureStr  string         `json:"posture"`
	Reason      string         `json:"reason"`
	Signals     PostureSignals `json:"signals"`
	EvaluatedAt time.Time      `json:"evaluated_at"`
	StableTicks int            `json:"stable_ticks"`
}

// ── evaluatePosture ───────────────────────────────────────────────────────────

// evaluatePosture reads all in-memory signals and returns a PostureSnapshot
// with the raw desired posture (before hysteresis is applied by the loop).
// It acquires srv.mu briefly to count nodes — no other locks are held.
func (srv *server) evaluatePosture() PostureSnapshot {
	sig := PostureSignals{}

	// Signal 1: workflow circuit breaker.
	sig.WorkflowCBOpen = srv.workflowGate.IsOpen()

	// Signal 2: reconcile circuit breaker.
	sig.ReconcileCBOpen = srv.reconcileBreaker.IsOpen()

	// Signal 3: leader liveness.
	// Mirrors the logic in startLeaderLivenessCheck: stale lastHeartbeatProcessed
	// while other nodes exist implies the leader is not hearing from the cluster.
	lastNano := srv.lastHeartbeatProcessed.Load()
	if srv.isLeader() && srv.hasExpectedHeartbeatSources() {
		sig.LeaderLivenessDegraded = lastNano == 0 ||
			time.Since(time.Unix(0, lastNano)) > leaderLivenessThreshold
	}

	// Signal 4: node fraction (observational only).
	sig.KnownNodes, sig.UnreachableNodes = srv.countNodeFraction()
	if sig.KnownNodes >= 3 {
		sig.UnreachableFraction = float64(sig.UnreachableNodes) / float64(sig.KnownNodes)
	}

	// ── Compute raw desired posture ───────────────────────────────────────────

	desired := PostureNormal
	reason := ""

	// RECOVERY_ONLY: both circuit breakers open simultaneously — the workflow
	// backend and the reconcile path are both degraded; only liveness work is safe.
	if sig.WorkflowCBOpen && sig.ReconcileCBOpen {
		desired = PostureRecoveryOnly
		reason = "workflow CB and reconcile CB both open"
	}

	// RECOVERY_ONLY: workflow CB open and leader can't hear the cluster.
	if desired < PostureRecoveryOnly && sig.WorkflowCBOpen && sig.LeaderLivenessDegraded {
		desired = PostureRecoveryOnly
		reason = "workflow circuit breaker open; leader liveness degraded"
	}

	// DEGRADED (any one of these is sufficient, most specific first).
	if desired < PostureDegraded {
		switch {
		case sig.WorkflowCBOpen:
			desired = PostureDegraded
			reason = "workflow circuit breaker open"
		case sig.ReconcileCBOpen:
			desired = PostureDegraded
			reason = "reconcile circuit breaker open"
		case sig.LeaderLivenessDegraded:
			desired = PostureDegraded
			reason = "leader liveness degraded — no recent heartbeat from cluster"
		}
	}

	return PostureSnapshot{
		Posture:     desired,
		PostureStr:  desired.String(),
		Reason:      reason,
		Signals:     sig,
		EvaluatedAt: time.Now(),
	}
}

// hasExpectedHeartbeatSources returns true if at least one non-self, non-blocked
// node exists that should be sending heartbeats (i.e., has reported at least once
// and is not in an early bootstrap phase). Safe to call without srv.mu held
// only if the caller holds no lock that could deadlock with srv.mu.
func (srv *server) hasExpectedHeartbeatSources() bool {
	selfID := srv.findSelfNodeID()
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.state == nil {
		return false
	}
	for id, node := range srv.state.Nodes {
		if id == selfID {
			continue
		}
		if node.Status == "blocked" {
			continue
		}
		if node.BootstrapPhase == BootstrapAdmitted {
			continue
		}
		if node.LastSeen.IsZero() {
			continue // never reported
		}
		return true
	}
	return false
}

// countNodeFraction counts "known" nodes and how many are unreachable.
// "Known" means: has reported at least once, not blocked, not in early bootstrap.
// Minimum cluster size 3 is required before the fraction is meaningful —
// callers should check KnownNodes before using UnreachableFraction.
func (srv *server) countNodeFraction() (known, unreachable int) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.state == nil {
		return
	}
	for _, node := range srv.state.Nodes {
		if node.LastSeen.IsZero() {
			continue // never reported
		}
		if node.Status == "blocked" {
			continue
		}
		if node.BootstrapPhase == BootstrapAdmitted {
			continue
		}
		known++
		if time.Since(node.LastSeen) > heartbeatStaleThreshold {
			unreachable++
		}
	}
	return
}

// ── postureLoop ───────────────────────────────────────────────────────────────

// startPostureLoop launches the posture evaluation goroutine. It must be called
// after the server is fully initialised (workflowGate, reconcileBreaker, etcdClient
// must all be non-nil). Mirrors the launch pattern of startLeaderLivenessCheck.
func (srv *server) startPostureLoop(ctx context.Context) {
	safeGo("posture-loop", func() {
		srv.runPostureLoop(ctx)
	})
}

func (srv *server) runPostureLoop(ctx context.Context) {
	ticker := time.NewTicker(postureEvalInterval)
	defer ticker.Stop()

	var (
		currentPosture ClusterPosture = PostureNormal
		stableTicks    int            = 0
		lastWritten    ClusterPosture = -1    // sentinel: never written
		lastWriteAt    time.Time
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		if !srv.isLeader() {
			// Non-leaders do not compute or write posture.
			// Reset state so we start fresh if we later become leader.
			currentPosture = PostureNormal
			stableTicks = 0
			lastWritten = -1
			srv.posture.Store(int32(PostureNormal))
			continue
		}

		snap := srv.evaluatePosture()
		desired := snap.Posture

		// Hysteresis: downward escalation is immediate; upward recovery
		// requires postureHysteresisTicks consecutive ticks at the lower level.
		prev := currentPosture
		if desired > currentPosture {
			// Escalation — apply immediately.
			log.Printf("posture: %s → %s (%s)", currentPosture, desired, snap.Reason)
			currentPosture = desired
			stableTicks = 1
		} else if desired < currentPosture {
			// Recovery — require sustained lower reading.
			stableTicks++
			if stableTicks >= postureHysteresisTicks {
				log.Printf("posture: %s → %s (stable for %d ticks)",
					currentPosture, desired, stableTicks)
				currentPosture = desired
				stableTicks = 1
			}
		} else {
			stableTicks++
		}
		if currentPosture != prev {
			postureTransitionsTotal.WithLabelValues(prev.String(), currentPosture.String()).Inc()
		}

		// Update the fast-path atomic (consumed by enforcement gates in phase 2).
		srv.posture.Store(int32(currentPosture))

		// Build the stable snapshot (hysteresis-adjusted).
		snap.Posture = currentPosture
		snap.PostureStr = currentPosture.String()
		snap.StableTicks = stableTicks

		// Store for in-process readers (e.g. GetClusterHealth).
		srv.postureSnap.Store(&snap)

		// Write to etcd on transition or heartbeat tick.
		shouldWrite := currentPosture != lastWritten ||
			time.Since(lastWriteAt) >= postureHeartbeatInterval

		if shouldWrite {
			writeCtx, cancel := context.WithTimeout(context.Background(), postureEtcdWriteTimeout)
			if err := srv.writePostureToEtcd(writeCtx, snap); err != nil {
				log.Printf("posture: etcd write failed: %v", err)
			} else {
				lastWritten = currentPosture
				lastWriteAt = time.Now()
			}
			cancel()
		}

		// Emit posture state gauges (exactly one will be 1).
		postureStateGauge.WithLabelValues(PostureNormal.String()).Set(boolToFloat(currentPosture == PostureNormal))
		postureStateGauge.WithLabelValues(PostureDegraded.String()).Set(boolToFloat(currentPosture == PostureDegraded))
		postureStateGauge.WithLabelValues(PostureRecoveryOnly.String()).Set(boolToFloat(currentPosture == PostureRecoveryOnly))

		// Emit individual signal gauges for dashboards.
		postureSignalActive.WithLabelValues("workflow_cb_open").Set(boolToFloat(snap.Signals.WorkflowCBOpen))
		postureSignalActive.WithLabelValues("reconcile_cb_open").Set(boolToFloat(snap.Signals.ReconcileCBOpen))
		postureSignalActive.WithLabelValues("leader_liveness_degraded").Set(boolToFloat(snap.Signals.LeaderLivenessDegraded))

		// Emit node fraction gauges (observational).
		postureNodeFraction.WithLabelValues("known").Set(float64(snap.Signals.KnownNodes))
		postureNodeFraction.WithLabelValues("unreachable").Set(float64(snap.Signals.UnreachableNodes))
	}
}

// ── getters ───────────────────────────────────────────────────────────────────

// getPosture returns the current cluster posture. Zero-copy atomic read.
// Returns PostureNormal if the posture loop has not yet run.
func (srv *server) getPosture() ClusterPosture {
	return ClusterPosture(srv.posture.Load())
}

// getPostureSnapshot returns the last computed PostureSnapshot or nil if the
// posture loop has not yet run.
func (srv *server) getPostureSnapshot() *PostureSnapshot {
	v := srv.postureSnap.Load()
	if v == nil {
		return nil
	}
	return v.(*PostureSnapshot)
}

// ── etcd persistence ──────────────────────────────────────────────────────────

func (srv *server) writePostureToEtcd(ctx context.Context, snap PostureSnapshot) error {
	if srv.etcdClient == nil {
		return nil // etcd not connected yet; skip silently
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = srv.etcdClient.Put(ctx, postureEtcdKey, string(data))
	return err
}

// ReadClusterPosture reads the posture from etcd and returns:
//   - the posture value
//   - the age of the snapshot (time since EvaluatedAt)
//   - any read error
//
// Falls back to PostureDegraded if the key is missing, unreadable, or stale
// (age > 10 minutes). PostureDegraded is the safe fallback: conservative enough
// to suppress risky healing, not so severe that it paralyses a healthy cluster
// with a transient etcd connectivity issue.
//
// This function is exported for use by the doctor and the CLI.
func ReadClusterPosture(ctx context.Context, cli *clientv3.Client) (ClusterPosture, time.Duration, error) {
	const staleAge = 10 * time.Minute

	resp, err := cli.Get(ctx, postureEtcdKey)
	if err != nil {
		return PostureDegraded, 0, err
	}
	if len(resp.Kvs) == 0 {
		// Key absent — leader has never written posture or was just elected.
		// Treat as normal: the leader will write within one eval tick (30s).
		// Use Normal here rather than Degraded to avoid false alarms during
		// fresh cluster startup.
		return PostureNormal, 0, nil
	}

	var snap PostureSnapshot
	if err := json.Unmarshal(resp.Kvs[0].Value, &snap); err != nil {
		return PostureDegraded, 0, err
	}

	age := time.Since(snap.EvaluatedAt)
	if age > staleAge {
		// Key is stale — leader may have crashed. Fall back to Degraded.
		return PostureDegraded, age, nil
	}

	// Re-parse posture string to enum.
	switch snap.PostureStr {
	case "DEGRADED":
		return PostureDegraded, age, nil
	case "RECOVERY_ONLY":
		return PostureRecoveryOnly, age, nil
	default:
		return PostureNormal, age, nil
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

