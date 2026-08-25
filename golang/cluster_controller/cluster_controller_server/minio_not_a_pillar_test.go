package main

import (
	"strings"
	"testing"
	"time"
)

// storageJoiningNode builds a node parked in storage_joining long enough that
// the phase has already timed out — the moment the gate must decide whether a
// waiting dependency costs this node its membership.
//
// Profiles select the dependencies: "core" maps to MinIO only, "scylla" to
// ScyllaDB only, "storage" to both (profilesForMinio / profilesForScyllaDB).
func storageJoiningNode(hostname string, profiles []string) *nodeState {
	return &nodeState{
		NodeID:             "n-" + hostname,
		Identity:           storedIdentity{Hostname: hostname, Ips: []string{"10.0.0.5"}},
		Profiles:           profiles,
		BootstrapPhase:     BootstrapStorageJoining,
		BootstrapStartedAt: time.Now().Add(-bootstrapPhaseTimeout - time.Minute),
		// The node is alive and reporting — this phase's runtime gate needs a
		// heartbeat to classify infra at all. The scenario under test is a live
		// node whose object store will not converge, not a dead one.
		LastSeen: time.Now(),
	}
}

// TestMinioMayNotBlockNodeConvergence pins the difference between a pillar and a
// commodity tier at the bootstrap storage gate.
//
// storage_joining used to treat MinIO and ScyllaDB identically: either one not
// reaching its verified phase held the node, and on timeout the node's bootstrap
// FAILED. For ScyllaDB that is right — it holds cluster state and the workflow
// lease table, so a node that cannot join the ring has not joined the cluster.
//
// For MinIO it violated invariant minio.is_commodity_not_a_pillar — "a commodity
// object-store tier, never a pillar — it must not gate a primary service's health
// nor block node convergence". Blocking turned an object-store problem into a
// cluster-membership problem, which is the recorded failure mode
// bootstrap.held_minio_nonmember_stranded_at_none: a held non-member stranded at
// MinioJoinNone wedges this phase.
func TestMinioMayNotBlockNodeConvergence(t *testing.T) {
	t.Run("MinIO alone degrades, never fails", func(t *testing.T) {
		node := storageJoiningNode("minio-only", []string{"core"})
		node.MinioJoinPhase = MinioJoinNone
		if !nodeHasMinioProfile(node) || nodeHasScyllaProfile(node) {
			t.Fatalf("test setup: %q must select MinIO and not ScyllaDB", "core")
		}

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase == BootstrapFailed {
			t.Fatalf("a commodity tier cost the node its membership: phase=%s err=%q",
				node.BootstrapPhase, node.BootstrapError)
		}
		if node.BootstrapPhase != BootstrapWorkloadReady {
			t.Fatalf("expected advance to %s despite unconverged MinIO, got %s",
				BootstrapWorkloadReady, node.BootstrapPhase)
		}
		// Advancing degraded must not fabricate a converged objectstore: the
		// join phase stays as observed so cluster-doctor still sees the truth.
		if node.MinioJoinPhase != MinioJoinNone {
			t.Fatalf("degraded advance rewrote the observed MinIO phase to %s",
				node.MinioJoinPhase)
		}
	})

	t.Run("ScyllaDB alone still fails", func(t *testing.T) {
		node := storageJoiningNode("scylla-only", []string{"scylla"})
		node.ScyllaJoinPhase = ScyllaJoinStarted
		if !nodeHasScyllaProfile(node) || nodeHasMinioProfile(node) {
			t.Fatalf("test setup: %q must select ScyllaDB and not MinIO", "scylla")
		}

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase != BootstrapFailed {
			t.Fatalf("ScyllaDB is a pillar — a node that cannot join the ring has not "+
				"joined the cluster; expected %s, got %s",
				BootstrapFailed, node.BootstrapPhase)
		}
	})

	t.Run("both waiting fails, because ScyllaDB does", func(t *testing.T) {
		node := storageJoiningNode("both", []string{"storage"})
		node.MinioJoinPhase = MinioJoinNone
		node.ScyllaJoinPhase = ScyllaJoinStarted
		if !nodeHasMinioProfile(node) || !nodeHasScyllaProfile(node) {
			t.Fatalf("test setup: %q must select both tiers", "storage")
		}

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase != BootstrapFailed {
			t.Fatalf("the pillar decides when both are waiting; expected %s, got %s",
				BootstrapFailed, node.BootstrapPhase)
		}
	})

	t.Run("MinIO non-member advances without waiting for the timeout", func(t *testing.T) {
		// A node held out of the pool is a lawful terminal state, not a stall —
		// it must converge immediately, not sit out the 5-minute phase budget.
		node := storageJoiningNode("non-member", []string{"core"})
		node.BootstrapStartedAt = time.Now()
		node.MinioJoinPhase = MinioJoinNonMember

		if !reconcileBootstrapPhases([]*nodeState{node}, nil, &mockEmitter{}) {
			t.Fatal("expected the phase decision to mark state dirty")
		}
		if node.BootstrapPhase != BootstrapWorkloadReady {
			t.Fatalf("expected %s for a lawful non-member, got %s",
				BootstrapWorkloadReady, node.BootstrapPhase)
		}
	})
}

// TestStorageGateSatisfiedIsOneRule covers the shared predicate directly — the
// bootstrap workflow's storage_verified condition resolves to it, and that path
// is the one that can mark a node failed via the workflow's onFailure handler.
//
// maybe_wait_storage gates mark_workload_ready in node.bootstrap.yaml, so before
// this rule was shared, the workflow could still fail a node over MinIO even
// though the reconciler had stopped doing so.
func TestStorageGateSatisfiedIsOneRule(t *testing.T) {
	timedOut := func(n *nodeState) *nodeState {
		n.BootstrapStartedAt = time.Now().Add(-bootstrapPhaseTimeout - time.Minute)
		return n
	}
	fresh := func(n *nodeState) *nodeState {
		n.BootstrapStartedAt = time.Now()
		return n
	}

	cases := []struct {
		name string
		node *nodeState
		want bool
	}{
		{
			// The bounded wait: MinIO still has budget, so hold.
			name: "MinIO unconverged within its budget holds",
			node: fresh(storageJoiningNode("a", []string{"core"})),
			want: false,
		},
		{
			// The budget is spent. A commodity tier does not get to hold forever.
			name: "MinIO unconverged past its budget releases",
			node: timedOut(storageJoiningNode("b", []string{"core"})),
			want: true,
		},
		{
			// ScyllaDB has no deadline — the phase budget must not release it.
			name: "ScyllaDB unconverged past its budget still holds",
			node: timedOut(storageJoiningNode("c", []string{"scylla"})),
			want: false,
		},
		{
			name: "verified MinIO releases immediately",
			node: func() *nodeState {
				n := fresh(storageJoiningNode("d", []string{"core"}))
				n.MinioJoinPhase = MinioJoinVerified
				return n
			}(),
			want: true,
		},
		{
			name: "non-member MinIO releases immediately",
			node: func() *nodeState {
				n := fresh(storageJoiningNode("e", []string{"core"}))
				n.MinioJoinPhase = MinioJoinNonMember
				return n
			}(),
			want: true,
		},
		{
			name: "verified ScyllaDB releases",
			node: func() *nodeState {
				n := fresh(storageJoiningNode("f", []string{"scylla"}))
				n.ScyllaJoinPhase = ScyllaJoinVerified
				return n
			}(),
			want: true,
		},
		{
			// A node with neither profile has nothing to wait on.
			name: "no storage profile releases",
			node: fresh(storageJoiningNode("g", []string{"gateway"})),
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := storageGateSatisfied(tc.node, time.Now()); got != tc.want {
				t.Errorf("storageGateSatisfied = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestStorageGateNeedsALiveDeadline guards the quiet way this fix could fail
// open in the other direction: phaseTimedOut returns false for a zero
// BootstrapStartedAt, so if the workflow ever reached this gate without a phase
// start recorded, MinIO would hold the node forever and the release valve would
// never open. Today mark_awareness_ready stamps it via setBootstrapPhase
// immediately before maybe_wait_storage runs.
func TestStorageGateNeedsALiveDeadline(t *testing.T) {
	node := storageJoiningNode("no-deadline", []string{"core"})
	node.BootstrapStartedAt = time.Time{}

	if phaseTimedOut(node, time.Now()) {
		t.Fatal("precondition: a zero start time must not read as timed out")
	}
	if storageGateSatisfied(node, time.Now()) {
		t.Fatal("unexpected release without a recorded phase start")
	}
	// Documenting the dependency: the gate's MinIO valve is only reachable
	// because some earlier step stamped BootstrapStartedAt.
	node.BootstrapStartedAt = time.Now().Add(-bootstrapPhaseTimeout - time.Minute)
	if !storageGateSatisfied(node, time.Now()) {
		t.Fatal("with a recorded phase start the MinIO budget must expire")
	}
}

// poolMemberUnits reports every unit the node's own plan requires as active,
// except the ones the caller names. Deriving it from the plan rather than
// hand-listing keeps the test honest when the catalog changes.
func poolMemberUnits(t *testing.T, srv *server, node *nodeState, down ...string) []unitStatusRecord {
	t.Helper()
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	if len(required) == 0 {
		t.Fatal("test setup: node plan requires no units")
	}
	downSet := make(map[string]bool, len(down))
	for _, d := range down {
		if _, ok := required[d]; !ok {
			t.Fatalf("test setup: %q is not in this node's required set", d)
		}
		downSet[d] = true
	}
	units := make([]unitStatusRecord, 0, len(required))
	for name := range required {
		if downSet[name] {
			continue
		}
		units = append(units, unitStatusRecord{Name: name, State: "active"})
	}
	return units
}

func poolMemberNode() *nodeState {
	return &nodeState{
		NodeID:         "member",
		Status:         "ready",
		BootstrapPhase: BootstrapWorkloadReady,
		MinioJoinPhase: MinioJoinVerified,
		LastSeen:       time.Now(),
		ReportedAt:     time.Now(),
		Profiles:       []string{"core", "storage", "control-plane", "gateway"},
		Identity:       storedIdentity{Hostname: "globule-member", Ips: []string{"10.0.0.63"}},
	}
}

// newMemberServer returns a server holding one healthy pool-member node.
func newMemberServer(t *testing.T) (*server, *nodeState) {
	t.Helper()
	node := poolMemberNode()
	return newTestServer(t, &controllerState{Nodes: map[string]*nodeState{"member": node}}), node
}

// settle records the health verdict the reconciler would later read, exactly as
// the live path does (handlers_status.go writes node.Status from
// evaluateNodeStatus).
func settle(t *testing.T, srv *server, node *nodeState, units []unitStatusRecord) {
	t.Helper()
	node.Status, _ = srv.evaluateNodeStatus(node, units)
	node.Units = units
}

// TestObjectstoreOutageDoesNotFreezeServiceConvergence pins the split between
// "is this node healthy" and "may services converge here".
//
// They used to be one verdict. evaluateNodeStatus marks a pool member non-ready
// when globular-minio.service is down (deliberately — see
// TestEvaluateNodeStatus_MinioMember_RequiresMinio), and the drift reconciler
// skipped every node whose Status != "ready". So a MinIO outage silently stopped
// convergence for every unrelated service on that node — a commodity tier gating
// primary work, which invariant:minio.is_commodity_not_a_pillar forbids.
func TestObjectstoreOutageDoesNotFreezeServiceConvergence(t *testing.T) {
	t.Run("health still reports the object store honestly", func(t *testing.T) {
		srv, node := newMemberServer(t)
		units := poolMemberUnits(t, srv, node, "globular-minio.service")

		status, reason := srv.evaluateNodeStatus(node, units)
		if status == "ready" {
			t.Fatal("a pool member with MinIO down must not report ready — that " +
				"reporting is deliberate and must survive this change")
		}
		if !strings.Contains(reason, "minio") {
			t.Errorf("expected the reason to name minio, got %q", reason)
		}
	})

	t.Run("convergence proceeds anyway", func(t *testing.T) {
		srv, node := newMemberServer(t)
		settle(t, srv, node, poolMemberUnits(t, srv, node, "globular-minio.service"))

		if !srv.nodeReadyForServiceConvergence(node) {
			t.Fatalf("a MinIO outage froze convergence for every unrelated service "+
				"on this node (node.Status=%q)", node.Status)
		}
	})

	t.Run("sidekick down alone also does not freeze convergence", func(t *testing.T) {
		// sidekick is MinIO's front door and declares it as a runtime dependency,
		// so it goes down with MinIO. It belongs to the same commodity tier.
		srv, node := newMemberServer(t)
		settle(t, srv, node, poolMemberUnits(t, srv, node, "globular-sidekick.service"))

		if !srv.nodeReadyForServiceConvergence(node) {
			t.Fatalf("sidekick is commodity tier too (node.Status=%q)", node.Status)
		}
	})

	t.Run("a real pillar still withholds convergence", func(t *testing.T) {
		// The gate must stay narrow. etcd down is not an object-store problem.
		srv, node := newMemberServer(t)
		settle(t, srv, node, poolMemberUnits(t, srv, node, "globular-etcd.service"))

		if srv.nodeReadyForServiceConvergence(node) {
			t.Fatal("etcd down must still withhold convergence — this gate is " +
				"about the object store, not about ignoring unhealthy nodes")
		}
	})

	t.Run("an unreachable node still withholds convergence", func(t *testing.T) {
		srv, node := newMemberServer(t)
		node.Units = poolMemberUnits(t, srv, node)
		node.Status = "unreachable"
		node.LastSeen = time.Now().Add(-24 * time.Hour)

		if srv.nodeReadyForServiceConvergence(node) {
			t.Fatal("a node with no heartbeat must not be converged onto")
		}
	})
}
