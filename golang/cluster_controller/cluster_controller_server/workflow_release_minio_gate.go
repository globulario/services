package main

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	configpkg "github.com/globulario/services/golang/config"
)

// minioTopologyHeldPackages are the packages whose systemd units the node-agent
// topology gate deliberately keeps stopped on non-member nodes. Kept in sync
// with the doctor's nodeIsMinioNonMember waiver in
// cluster_doctor/.../rules/installed_state_runtime_mismatch.go — sidekick is a
// MinIO-auxiliary metrics proxy whose runtime follows minio's authorization
// (intent:infrastructure.sidekick.minio_auxiliary_must_not_be_identity_authority).
var minioTopologyHeldPackages = map[string]bool{
	"minio":    true,
	"sidekick": true,
}

// minioTopologyHeldOnNode reports whether pkgName on nodeID is expected to be
// INACTIVE because the objectstore topology contract holds it, and therefore
// whether a verify_runtime "want active" probe must be waived.
//
// Why this exists: release.apply.package's verify_runtime step probes the unit
// and requires state=active. On a node outside ObjectStoreDesiredState the
// node-agent gate stops globular-minio.service on purpose
// (held_not_in_topology), so the probe can never pass. The step deferred 5/5
// times and permanently abandoned correlation
// InfrastructureRelease/core@globular.io/minio on a healthy 5-node cluster —
// two subsystems each enforcing a correct rule, in direct contradiction.
//
// Fail-CLOSED on uncertainty: if the desired state cannot be read, or the node
// IS an admitted member, this returns false so the normal active-check applies.
// A waiver granted on missing evidence would silently mask a genuinely dead
// MinIO on a real pool member.
func minioTopologyHeldOnNode(ctx context.Context, nodeID, nodeAddr, pkgName string) bool {
	if !minioTopologyHeldPackages[strings.ToLower(strings.TrimSpace(pkgName))] {
		return false
	}
	if strings.TrimSpace(nodeID) == "" {
		return false
	}

	loadCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	state, err := configpkg.LoadObjectStoreDesiredState(loadCtx)
	if err != nil || state == nil {
		// Unknown topology is not a licence to skip the check.
		return false
	}

	// v2 admission: NodeID must appear as an admitted member at the current
	// generation. Mirrors nodeIsTopologyMember in the node-agent, which is the
	// component that actually decides whether the unit may run.
	if state.AuthorizedMembers != nil {
		for _, m := range state.AuthorizedMembers {
			if m.NodeID != nodeID {
				continue
			}
			if m.Admitted && m.IntentGeneration == uint64(state.Generation) {
				return false // admitted member — it really must be active
			}
			log.Printf("verify_runtime: waiving active-check for %s on node %s — held by objectstore topology (admitted=%v node_gen=%d desired_gen=%d)",
				pkgName, nodeID, m.Admitted, m.IntentGeneration, state.Generation)
			return true
		}
		log.Printf("verify_runtime: waiving active-check for %s on node %s — not listed in objectstore AuthorizedMembers (gen=%d)",
			pkgName, nodeID, state.Generation)
		return true
	}

	// Legacy mode: admission is the pool IP list (state.Nodes). Resolve this
	// node's address from the agent endpoint the call site already holds and
	// compare against the pool.
	//
	// This branch previously returned false unconditionally, on the grounds
	// that a node ID could not be mapped to an IP here. That left the waiver
	// dead on every cluster that has not migrated to AuthorizedMembers — which
	// is the common case: a Day-0 standalone objectstore writes
	// {"mode":"standalone","generation":0,"nodes":["10.10.0.11"]} with no
	// authorized_members at all. The result was silence rather than a waiver:
	// zero "waiving active-check" lines while
	// InfrastructureRelease/core@globular.io/minio abandoned 5/5 on a healthy
	// cluster, exactly the contradiction this gate exists to resolve
	// (invariant:minio.is_commodity_not_a_pillar — MinIO must never block
	// another component's convergence).
	//
	// Still fail-CLOSED: an unparseable endpoint or an empty pool means the
	// membership question is unanswered, so the normal active-check stays in
	// force rather than waiving on a guess.
	if nodeAddr == "" || len(state.Nodes) == 0 {
		return false
	}
	host := strings.TrimSpace(nodeAddr)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == "" {
		return false
	}
	for _, poolIP := range state.Nodes {
		if strings.EqualFold(strings.TrimSpace(poolIP), host) {
			return false // in the pool — it really must be active
		}
	}
	log.Printf("verify_runtime: waiving active-check for %s on node %s (%s) — not in objectstore pool %v (mode=%s gen=%d)",
		pkgName, nodeID, host, state.Nodes, state.Mode, state.Generation)
	return true
}
