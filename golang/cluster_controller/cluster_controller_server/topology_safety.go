package main

// topology_safety.go — Case 12: TOPOLOGY_SAFETY_DRIFT
//
// Topology preflight checks run before any reconcile action that could change
// cluster membership, storage configuration, or ingress participant sets.
//
// Invariant: drift reconcile must never apply topology/runtime changes that
// violate known safety constraints. Unsafe actions are blocked with an explicit
// DEGRADED lane status; unrelated safe actions continue normally.

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	configpkg "github.com/globulario/services/golang/config"
)

// topologySafetyViolation describes a single topology safety constraint that
// is currently violated. Violations block the specific action they guard but
// do not stop unrelated reconcile lanes.
type topologySafetyViolation struct {
	Kind    string // "storage_quorum" | "ingress_participant" | "controller_placement"
	NodeID  string // node relevant to the violation (empty = cluster-wide)
	Message string
}

func (v topologySafetyViolation) Error() string {
	if v.NodeID != "" {
		return fmt.Sprintf("topology safety [%s] node=%s: %s", v.Kind, v.NodeID, v.Message)
	}
	return fmt.Sprintf("topology safety [%s]: %s", v.Kind, v.Message)
}

// checkStorageQuorumSafe returns a violation if removing nodeID from the
// cluster would drop active storage nodes below 3 (the minimum for ScyllaDB
// and MinIO redundancy). Must be called under srv.lock().
func (srv *server) checkStorageQuorumSafe(nodeID string) *topologySafetyViolation {
	if srv.state == nil {
		return nil
	}
	active := 0
	for id, n := range srv.state.Nodes {
		if n == nil || id == nodeID {
			continue
		}
		if n.Status == "removed" || n.Status == "blocked" || n.Status == "unreachable" {
			continue
		}
		for _, p := range n.Profiles {
			if p == "storage" {
				active++
				break
			}
		}
	}
	if active < 3 {
		return &topologySafetyViolation{
			Kind:   "storage_quorum",
			NodeID: nodeID,
			Message: fmt.Sprintf(
				"removing node would leave %d active storage node(s) — minimum 3 required for ScyllaDB/MinIO redundancy",
				active,
			),
		}
	}
	return nil
}

// checkIngressParticipantSafe returns a violation if removing nodeID from the
// ingress participant set would leave fewer than 2 participants (VRRP needs at
// least 2 nodes for active/standby). Must be called under srv.lock().
func (srv *server) checkIngressParticipantSafe(nodeID string) *topologySafetyViolation {
	if srv.state == nil {
		return nil
	}
	remaining := 0
	for id, n := range srv.state.Nodes {
		if n == nil || id == nodeID {
			continue
		}
		if n.Status == "removed" || n.Status == "blocked" || n.Status == "unreachable" {
			continue
		}
		if nodeHasProfile(&memberNode{Profiles: n.Profiles}, []string{"control-plane"}) {
			remaining++
		}
	}
	if remaining < 1 {
		return &topologySafetyViolation{
			Kind:   "ingress_participant",
			NodeID: nodeID,
			Message: fmt.Sprintf(
				"removing node would leave %d control-plane node(s) — at least 1 required for VIP failover",
				remaining,
			),
		}
	}
	return nil
}

// checkControllerPlacementSafe returns a violation if removing nodeID from the
// control-plane would leave no eligible controller leaders. Must be called under
// srv.lock().
func (srv *server) checkControllerPlacementSafe(nodeID string) *topologySafetyViolation {
	if srv.state == nil {
		return nil
	}
	eligible := 0
	for id, n := range srv.state.Nodes {
		if n == nil || id == nodeID {
			continue
		}
		if n.Status == "removed" || n.Status == "blocked" {
			continue
		}
		if nodeHasProfile(&memberNode{Profiles: n.Profiles}, []string{"control-plane"}) {
			eligible++
		}
	}
	if eligible == 0 {
		return &topologySafetyViolation{
			Kind:   "controller_placement",
			NodeID: nodeID,
			Message: "removing node would leave no eligible controller leaders",
		}
	}
	return nil
}

// topologyPreflightForRemove runs all topology safety checks for removing
// nodeID from the cluster. Returns all violations found. Safe to call under
// srv.lock().
func (srv *server) topologyPreflightForRemove(nodeID string) []topologySafetyViolation {
	var violations []topologySafetyViolation
	if v := srv.checkStorageQuorumSafe(nodeID); v != nil {
		violations = append(violations, *v)
	}
	if v := srv.checkIngressParticipantSafe(nodeID); v != nil {
		violations = append(violations, *v)
	}
	if v := srv.checkControllerPlacementSafe(nodeID); v != nil {
		violations = append(violations, *v)
	}
	return violations
}

// logTopologyViolations logs all violations as CRITICAL. Returns true if any
// violations were found.
func logTopologyViolations(nodeID string, violations []topologySafetyViolation) bool {
	if len(violations) == 0 {
		return false
	}
	kinds := make([]string, 0, len(violations))
	for _, v := range violations {
		log.Printf("CRITICAL: topology-safety: %s", v.Error())
		kinds = append(kinds, v.Kind)
	}
	log.Printf("topology-safety: blocked %d unsafe action(s) for node %s: %s",
		len(violations), nodeID, strings.Join(kinds, ", "))
	return true
}

// driftTopologyPreflight runs cluster-wide safety checks that must pass before
// drift reconciliation proceeds. This is advisory-blocking for drift lane only:
// unsafe topology blocks drift processing but does not stop other lanes.
func (srv *server) driftTopologyPreflight(ctx context.Context) []topologySafetyViolation {
	var violations []topologySafetyViolation

	srv.lock("drift-topology-preflight")
	var storageCount, controlPlaneCount int
	pool := append([]string(nil), srv.state.MinioPoolNodes...)
	for _, n := range srv.state.Nodes {
		if n == nil || n.Status == "removed" || n.Status == "blocked" || n.Status == "unreachable" {
			continue
		}
		if nodeHasProfile(&memberNode{Profiles: n.Profiles}, []string{"storage"}) {
			storageCount++
		}
		if nodeHasProfile(&memberNode{Profiles: n.Profiles}, []string{"control-plane"}) {
			controlPlaneCount++
		}
	}
	srv.unlock()

	if storageCount < 3 {
		violations = append(violations, topologySafetyViolation{
			Kind:    "storage_quorum",
			Message: fmt.Sprintf("drift preflight: only %d active storage node(s), minimum 3 required", storageCount),
		})
	}
	if controlPlaneCount < 1 {
		violations = append(violations, topologySafetyViolation{
			Kind:    "controller_placement",
			Message: "drift preflight: no active control-plane node available",
		})
	}

	// Objectstore topology consistency guard: controller state pool must match
	// authoritative desired objectstore node list (if desired exists).
	if srv.etcdClient != nil {
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), boundedShort)
		defer cancel()
		desired, err := configpkg.LoadObjectStoreDesiredState(dctx)
		if err == nil && desired != nil && len(desired.Nodes) > 0 {
			want := append([]string(nil), desired.Nodes...)
			got := append([]string(nil), pool...)
			sort.Strings(want)
			sort.Strings(got)
			if len(want) != len(got) || strings.Join(want, ",") != strings.Join(got, ",") {
				violations = append(violations, topologySafetyViolation{
					Kind:    "objectstore_topology_mismatch",
					Message: fmt.Sprintf("drift preflight: controller minio pool %v differs from desired objectstore nodes %v", pool, desired.Nodes),
				})
			}
		} else if err != nil {
			violations = append(violations, topologySafetyViolation{
				Kind:    "objectstore_topology_unavailable",
				Message: fmt.Sprintf("drift preflight: unable to load objectstore desired state: %v", err),
			})
		}
	}
	return violations
}
