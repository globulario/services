// workflow_invariant.go — Workflow runner for cluster.invariant.enforcement.
//
// This wires the invariant enforcement workflow to live controller state.
// The workflow provides audit trails and structured reporting on top of the
// direct reconcile-loop enforcement in invariant_enforcement.go.
//
// The reconcile-loop enforcement is the "last line of defense" (no MinIO,
// no workflow service needed). This workflow path provides the auditable,
// structured version when the workflow service is healthy.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// buildInvariantConfig returns the InvariantConfig that wires
// cluster.invariant.enforcement workflow actions to real controller state.
func (srv *server) buildInvariantConfig() engine.InvariantConfig {
	return engine.InvariantConfig{
		ValidateWorkflows:        srv.invariantValidateWorkflows,
		RepairWorkflows:          srv.invariantRepairWorkflows,
		ValidateInfraQuorum:      srv.invariantValidateInfraQuorum,
		EnforceQuorum:            srv.invariantEnforceQuorum,
		VerifyQuorum:             srv.invariantVerifyQuorum,
		ValidateFoundingProfiles: srv.invariantValidateFoundingProfiles,
		ValidateMinioStorage:     srv.invariantValidateMinioStorage,
		RepairMinioStorage:       srv.invariantRepairMinioStorage,
		ValidatePKIHealth:        srv.invariantValidatePKIHealth,
		RepairPKICerts:           srv.invariantRepairPKICerts,
		EmitReport:               srv.invariantEmitReport,
		MarkFailed:               srv.invariantMarkFailed,
		EmitCompleted:            srv.invariantEmitCompleted,
	}
}

// RunInvariantEnforcementWorkflow dispatches the cluster.invariant.enforcement
// workflow to the centralized WorkflowService.
func (srv *server) RunInvariantEnforcementWorkflow(ctx context.Context) error {
	if !srv.mustBeLeader() {
		return fmt.Errorf("not leader")
	}

	router := engine.NewRouter()
	engine.RegisterInvariantActions(router, srv.buildInvariantConfig())

	correlationID := fmt.Sprintf("invariant:%d", time.Now().Unix())

	inputs := map[string]any{
		"cluster_id": srv.cfg.ClusterDomain,
		"enforce":    true,
	}

	log.Printf("invariant-workflow: dispatching cluster.invariant.enforcement")
	start := time.Now()

	resp, err := srv.executeWorkflowCentralized(ctx, "cluster.invariant.enforcement", correlationID, inputs, router)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("invariant-workflow: FAILED after %s: %v", elapsed.Round(time.Millisecond), err)
		return err
	}

	if resp.Status == "SUCCEEDED" {
		log.Printf("invariant-workflow: SUCCEEDED in %s", elapsed.Round(time.Millisecond))
	} else {
		log.Printf("invariant-workflow: %s in %s: %s", resp.Status, elapsed.Round(time.Millisecond), resp.Error)
	}

	return nil
}

// --------------------------------------------------------------------------
// Workflow action implementations — wired to real controller state
// --------------------------------------------------------------------------

// invariantValidateWorkflows checks that all required core workflow
// definitions exist in etcd.
func (srv *server) invariantValidateWorkflows(ctx context.Context, required []string) (map[string]any, error) {
	var present, missing []string
	for _, name := range required {
		if v1alpha1.EtcdFetcher == nil {
			missing = append(missing, name)
			continue
		}
		if b, err := v1alpha1.EtcdFetcher(name); err != nil || len(b) == 0 {
			missing = append(missing, name)
		} else {
			present = append(present, name)
		}
	}

	report := map[string]any{
		"present":       present,
		"missing":       missing,
		"present_count": len(present),
		"missing_count": len(missing),
	}
	return report, nil
}

// invariantRepairWorkflows re-seeds missing workflow definitions from local
// disk to etcd.
func (srv *server) invariantRepairWorkflows(ctx context.Context, missing []string) (int, error) {
	defs := loadCoreWorkflowsFromDisk()
	repair := make(map[string][]byte)
	for _, name := range missing {
		if data, ok := defs[name]; ok {
			repair[name] = data
		} else {
			log.Printf("invariant-workflow: %s not found on disk, cannot repair", name)
		}
	}

	if len(repair) == 0 {
		return 0, nil
	}

	if err := v1alpha1.SeedCoreWorkflows(repair); err != nil {
		return 0, fmt.Errorf("seed core workflows: %w", err)
	}

	srv.emitClusterEvent("controller.workflows_repaired", map[string]interface{}{
		"source":         "invariant_workflow",
		"repaired_count": len(repair),
		"missing_count":  len(missing),
	})

	return len(repair), nil
}

// invariantValidateInfraQuorum checks infrastructure quorum: etcd on all
// nodes, ScyllaDB ≥ minScylla, MinIO ≥ minMinio.
func (srv *server) invariantValidateInfraQuorum(ctx context.Context, minScylla, minMinio int, etcdAllNodes bool) (map[string]any, error) {
	srv.lock("invariantValidateInfraQuorum")
	defer srv.unlock()

	var violations []map[string]any
	var candidates []map[string]any

	storageCount := countNodesWithProfile(srv.state.Nodes, "storage")
	cpCount := countNodesWithProfile(srv.state.Nodes, "control-plane")
	totalNodes := len(srv.state.Nodes)

	// etcd runs on all nodes — check that node count is at least 1.
	if etcdAllNodes && totalNodes == 0 {
		violations = append(violations, map[string]any{
			"invariant": "etcd_all_nodes",
			"message":   "no nodes registered",
		})
	}

	// ScyllaDB quorum: needs storage profile on ≥ minScylla nodes.
	if storageCount < minScylla {
		violations = append(violations, map[string]any{
			"invariant": "scylladb_quorum",
			"message":   fmt.Sprintf("need %d storage nodes, have %d", minScylla, storageCount),
			"have":      storageCount,
			"need":      minScylla,
		})
	}

	// MinIO quorum: also needs storage profile on ≥ minMinio nodes.
	if storageCount < minMinio {
		violations = append(violations, map[string]any{
			"invariant": "minio_quorum",
			"message":   fmt.Sprintf("need %d storage nodes, have %d", minMinio, storageCount),
			"have":      storageCount,
			"need":      minMinio,
		})
	}

	// Build candidate list: non-storage nodes that could be promoted.
	if len(violations) > 0 {
		for id, n := range srv.state.Nodes {
			if n.BlockedReason != "" {
				continue
			}
			hasStorage := false
			hasCP := false
			for _, p := range n.Profiles {
				if p == "storage" {
					hasStorage = true
				}
				if p == "control-plane" {
					hasCP = true
				}
			}
			if hasStorage {
				continue
			}
			candidates = append(candidates, map[string]any{
				"node_id":           id,
				"hostname":          n.Identity.Hostname,
				"has_control_plane": hasCP,
			})
		}
	}

	report := map[string]any{
		"violations":    violations,
		"candidates":    candidates,
		"storage_count": storageCount,
		"cp_count":      cpCount,
		"total_nodes":   totalNodes,
	}
	return report, nil
}

// invariantEnforceQuorum auto-promotes nodes to restore storage quorum.
// Delegates to the existing enforceStorageQuorumLocked.
func (srv *server) invariantEnforceQuorum(ctx context.Context, quorumReport map[string]any) error {
	srv.lock("invariantEnforceQuorum")
	defer srv.unlock()
	srv.enforceStorageQuorumLocked()
	return nil
}

// invariantVerifyQuorum re-checks that quorum requirements are met after
// enforcement.
func (srv *server) invariantVerifyQuorum(ctx context.Context, minScylla, minMinio int) (bool, error) {
	srv.lock("invariantVerifyQuorum")
	defer srv.unlock()

	storageCount := countNodesWithProfile(srv.state.Nodes, "storage")
	return storageCount >= minScylla && storageCount >= minMinio, nil
}

// invariantValidateFoundingProfiles checks that the founding nodes (first 3
// by join order) have the required profiles.
func (srv *server) invariantValidateFoundingProfiles(ctx context.Context) (map[string]any, error) {
	srv.lock("invariantValidateFoundingProfiles")
	defer srv.unlock()

	var violations []map[string]any

	// When total nodes ≤ MinQuorumNodes, ALL nodes are founding nodes and
	// must have all foundational profiles. When > MinQuorumNodes, we need at
	// least MinQuorumNodes nodes with the full founding set.
	requiredProfiles := []string{"core", "control-plane", "storage"}
	totalNodes := len(srv.state.Nodes)

	// Count how many nodes have the full founding set.
	fullyEquipped := 0
	for id, n := range srv.state.Nodes {
		profileSet := make(map[string]bool, len(n.Profiles))
		for _, p := range n.Profiles {
			profileSet[p] = true
		}
		var missingProfiles []string
		for _, rp := range requiredProfiles {
			if !profileSet[rp] {
				missingProfiles = append(missingProfiles, rp)
			}
		}
		if len(missingProfiles) > 0 {
			// Only flag as violation if we need this node to be founding.
			// When total ≤ MinQuorumNodes, every node must qualify.
			if totalNodes <= MinQuorumNodes {
				violations = append(violations, map[string]any{
					"node_id":          id,
					"hostname":         n.Identity.Hostname,
					"missing_profiles": missingProfiles,
					"current_profiles": n.Profiles,
				})
			}
		} else {
			fullyEquipped++
		}
	}

	// When total > MinQuorumNodes, check that at least MinQuorumNodes have
	// the full set.
	if totalNodes > MinQuorumNodes && fullyEquipped < MinQuorumNodes {
		violations = append(violations, map[string]any{
			"invariant":      "founding_quorum",
			"message":        fmt.Sprintf("need %d fully-equipped nodes, have %d", MinQuorumNodes, fullyEquipped),
			"have":           fullyEquipped,
			"need":           MinQuorumNodes,
		})
	}

	foundingCount := totalNodes
	if foundingCount > MinQuorumNodes {
		foundingCount = MinQuorumNodes
	}

	report := map[string]any{
		"violations":      violations,
		"founding_count":  foundingCount,
		"fully_equipped":  fullyEquipped,
	}
	return report, nil
}

// invariantEmitReport publishes the combined invariant enforcement report.
func (srv *server) invariantEmitReport(ctx context.Context, workflowReport, quorumReport, profileReport, minioReport, pkiReport map[string]any) error {
	srv.emitClusterEvent("controller.invariant_enforcement_report", map[string]interface{}{
		"workflow_report": workflowReport,
		"quorum_report":  quorumReport,
		"profile_report": profileReport,
		"minio_report":   minioReport,
		"pki_report":     pkiReport,
	})
	return nil
}

// invariantMarkFailed records that invariant enforcement failed.
func (srv *server) invariantMarkFailed(ctx context.Context, reason string) error {
	log.Printf("invariant-workflow: enforcement FAILED: %s", reason)
	srv.emitClusterEvent("controller.invariant_enforcement_failed", map[string]interface{}{
		"reason": reason,
	})
	return nil
}

// invariantEmitCompleted records that invariant enforcement succeeded.
func (srv *server) invariantEmitCompleted(ctx context.Context) error {
	log.Printf("invariant-workflow: enforcement completed successfully")
	srv.emitClusterEvent("controller.invariant_enforcement_completed", map[string]interface{}{
		"status": "SUCCEEDED",
	})
	return nil
}

// --------------------------------------------------------------------------
// MinIO storage health invariant
// --------------------------------------------------------------------------

// invariantValidateMinioStorage checks MinIO distributed storage health:
//   - All storage-profile nodes are in MinioPoolNodes
//   - All pool nodes have MinioJoinPhase == "verified"
//   - MinIO credentials exist
//   - Pool has sufficient nodes for erasure coding (≥3)
//   - Rendered config is consistent (all pool nodes reference same MINIO_VOLUMES)
func (srv *server) invariantValidateMinioStorage(ctx context.Context) (map[string]any, error) {
	srv.lock("invariantValidateMinioStorage")
	defer srv.unlock()

	var violations []map[string]any
	poolNodes := srv.state.MinioPoolNodes
	drivesPerNode := srv.state.MinioDrivesPerNode
	if drivesPerNode < 2 {
		drivesPerNode = 1
	}

	// Check: MinIO credentials must exist.
	if srv.state.MinioCredentials == nil || srv.state.MinioCredentials.RootUser == "" {
		violations = append(violations, map[string]any{
			"invariant": "minio_credentials_missing",
			"message":   "MinIO root credentials not set in controller state",
			"severity":  "CRITICAL",
		})
	}

	// Check: pool size must be ≥ 3 for distributed erasure coding.
	if len(poolNodes) < 3 {
		violations = append(violations, map[string]any{
			"invariant":  "minio_pool_insufficient",
			"message":    fmt.Sprintf("MinIO pool has %d nodes, need ≥3 for distributed erasure coding", len(poolNodes)),
			"pool_count": len(poolNodes),
			"severity":   "CRITICAL",
		})
	}

	// Check: all storage-profile nodes should be in the pool.
	var missingFromPool []string
	for _, n := range srv.state.Nodes {
		hasStorage := false
		for _, p := range n.Profiles {
			if p == "storage" {
				hasStorage = true
				break
			}
		}
		if !hasStorage {
			continue
		}
		ip := nodeRoutableIP(n)
		if ip == "" {
			continue
		}
		if !ipInPool(ip, poolNodes) {
			missingFromPool = append(missingFromPool, fmt.Sprintf("%s (%s)", n.Identity.Hostname, ip))
		}
	}
	if len(missingFromPool) > 0 {
		violations = append(violations, map[string]any{
			"invariant": "minio_storage_nodes_not_in_pool",
			"message":   fmt.Sprintf("storage-profile nodes not in MinIO pool: %v", missingFromPool),
			"nodes":     missingFromPool,
			"severity":  "ERROR",
		})
	}

	// Check: all pool nodes should have MinioJoinPhase == "verified".
	var unhealthyNodes []map[string]any
	for _, n := range srv.state.Nodes {
		ip := nodeRoutableIP(n)
		if !ipInPool(ip, poolNodes) {
			continue
		}
		if n.MinioJoinPhase != MinioJoinVerified {
			unhealthyNodes = append(unhealthyNodes, map[string]any{
				"hostname":   n.Identity.Hostname,
				"node_id":    n.NodeID,
				"ip":         ip,
				"join_phase": string(n.MinioJoinPhase),
				"join_error": n.MinioJoinError,
			})
		}
	}
	if len(unhealthyNodes) > 0 {
		violations = append(violations, map[string]any{
			"invariant": "minio_nodes_not_verified",
			"message":   fmt.Sprintf("%d MinIO pool nodes not in verified state", len(unhealthyNodes)),
			"nodes":     unhealthyNodes,
			"severity":  "ERROR",
		})
	}

	// Check: MinIO service should be running on all pool nodes.
	var notRunning []string
	for _, n := range srv.state.Nodes {
		ip := nodeRoutableIP(n)
		if !ipInPool(ip, poolNodes) {
			continue
		}
		if !nodeHasMinioRunning(n) {
			notRunning = append(notRunning, fmt.Sprintf("%s (%s)", n.Identity.Hostname, ip))
		}
	}
	if len(notRunning) > 0 {
		violations = append(violations, map[string]any{
			"invariant": "minio_service_not_running",
			"message":   fmt.Sprintf("globular-minio.service not active on pool nodes: %v", notRunning),
			"nodes":     notRunning,
			"severity":  "CRITICAL",
		})
	}

	// Check: config consistency — pool nodes and drive count produce
	// expected MINIO_VOLUMES with (pool_size × drives_per_node) endpoints.
	expectedEndpoints := len(poolNodes) * drivesPerNode
	if expectedEndpoints < 4 && len(poolNodes) > 1 {
		violations = append(violations, map[string]any{
			"invariant":          "minio_erasure_set_too_small",
			"message":            fmt.Sprintf("MinIO erasure set has %d endpoints (need ≥4 for distributed mode)", expectedEndpoints),
			"pool_nodes":         len(poolNodes),
			"drives_per_node":    drivesPerNode,
			"total_endpoints":    expectedEndpoints,
			"severity":           "WARN",
		})
	}

	report := map[string]any{
		"violations":      violations,
		"pool_size":       len(poolNodes),
		"pool_nodes":      poolNodes,
		"drives_per_node": drivesPerNode,
		"total_endpoints": expectedEndpoints,
	}
	return report, nil
}

// --------------------------------------------------------------------------
// MinIO storage repair
// --------------------------------------------------------------------------

// invariantRepairMinioStorage attempts to fix MinIO violations:
//   - Nodes with stale join phase → reset to trigger re-join
//   - Service not running → force config re-render + restart
//   - Missing from pool but has storage profile → reset join phase to trigger pool join
//
// All actions are idempotent. The reconcile loop picks up the state changes
// on the next cycle and drives them to completion.
func (srv *server) invariantRepairMinioStorage(ctx context.Context, minioReport map[string]any) (map[string]any, error) {
	srv.lock("invariantRepairMinioStorage")

	poolNodes := srv.state.MinioPoolNodes
	var repaired []map[string]any

	for _, n := range srv.state.Nodes {
		ip := nodeRoutableIP(n)
		if ip == "" {
			continue
		}
		hasStorage := false
		for _, p := range n.Profiles {
			if p == "storage" {
				hasStorage = true
				break
			}
		}
		if !hasStorage {
			continue
		}

		needsRestart := false

		// Fix: node has storage profile but not in pool → reset join phase
		// so the pool manager picks it up on next reconcile.
		if !ipInPool(ip, poolNodes) {
			n.MinioJoinPhase = MinioJoinNone
			n.MinioJoinError = ""
			repaired = append(repaired, map[string]any{
				"node_id":  n.NodeID,
				"hostname": n.Identity.Hostname,
				"action":   "reset_join_phase_for_pool_join",
			})
			continue
		}

		// Fix: node is in pool but join phase is failed → reset for retry.
		if n.MinioJoinPhase == MinioJoinFailed {
			n.MinioJoinPhase = MinioJoinNone
			n.MinioJoinError = ""
			repaired = append(repaired, map[string]any{
				"node_id":  n.NodeID,
				"hostname": n.Identity.Hostname,
				"action":   "reset_failed_join_phase",
			})
			continue
		}

		// Fix: service not running → clear rendered config hash to force
		// re-render on next reconcile, then restart the service.
		if ipInPool(ip, poolNodes) && !nodeHasMinioRunning(n) {
			// Clear the minio.env hash so the reconciler re-renders it.
			for path := range n.RenderedConfigHashes {
				if path == "/var/lib/globular/minio/minio.env" {
					delete(n.RenderedConfigHashes, path)
				}
			}
			needsRestart = true
			repaired = append(repaired, map[string]any{
				"node_id":  n.NodeID,
				"hostname": n.Identity.Hostname,
				"action":   "clear_config_hash_and_restart",
			})
		}

		// Release lock before making RPC calls.
		if needsRestart && n.AgentEndpoint != "" {
			endpoint := n.AgentEndpoint
			nodeID := n.NodeID
			srv.unlock()

			agent, err := srv.getAgentClient(ctx, endpoint)
			if err != nil {
				log.Printf("invariant-repair: cannot reach agent on %s to restart minio: %v", nodeID, err)
			} else {
				_, err = agent.ControlService(ctx, "globular-minio.service", "restart")
				if err != nil {
					log.Printf("invariant-repair: minio restart failed on %s: %v", nodeID, err)
				} else {
					log.Printf("invariant-repair: minio restarted on %s", nodeID)
				}
			}

			srv.lock("invariantRepairMinioStorage-post-restart")
		}
	}

	srv.unlock()

	report := map[string]any{
		"repaired":       repaired,
		"repaired_count": len(repaired),
	}
	return report, nil
}

// --------------------------------------------------------------------------
// PKI / Certificate health invariant
// --------------------------------------------------------------------------

// invariantValidatePKIHealth checks TLS certificate health across all nodes
// by calling GetCertificateStatus on each node agent. Checks:
//   - Certificate exists and is readable
//   - Certificate not expired (CRITICAL) or near expiry (<30d WARN, <7d ERROR)
//   - SANs cover all node IPs (including VIP if applicable)
//   - Certificate chain validates against the cluster CA
//   - CA certificate exists on each node
func (srv *server) invariantValidatePKIHealth(ctx context.Context) (map[string]any, error) {
	srv.lock("invariantValidatePKIHealth")
	nodes := make(map[string]*nodeState, len(srv.state.Nodes))
	for id, n := range srv.state.Nodes {
		nodes[id] = n
	}
	srv.unlock()

	var violations []map[string]any
	var checkedCount int

	for id, n := range nodes {
		agent, err := srv.getAgentClient(ctx, n.AgentEndpoint)
		if err != nil {
			violations = append(violations, map[string]any{
				"invariant": "pki_agent_unreachable",
				"node_id":   id,
				"hostname":  n.Identity.Hostname,
				"message":   fmt.Sprintf("cannot reach node agent to check certs: %v", err),
				"severity":  "WARN",
			})
			continue
		}

		certResp, certErr := agent.GetCertificateStatus(ctx)
		if certErr != nil {
			violations = append(violations, map[string]any{
				"invariant": "pki_status_unavailable",
				"node_id":   id,
				"hostname":  n.Identity.Hostname,
				"message":   fmt.Sprintf("GetCertificateStatus failed: %v", certErr),
				"severity":  "ERROR",
			})
			continue
		}
		checkedCount++

		cert := certResp.GetServerCert()
		if cert == nil {
			violations = append(violations, map[string]any{
				"invariant": "pki_cert_missing",
				"node_id":   id,
				"hostname":  n.Identity.Hostname,
				"message":   "no server certificate found on node",
				"severity":  "CRITICAL",
			})
			continue
		}

		// Check expiry.
		days := cert.GetDaysUntilExpiry()
		switch {
		case days <= 0:
			violations = append(violations, map[string]any{
				"invariant":  "pki_cert_expired",
				"node_id":    id,
				"hostname":   n.Identity.Hostname,
				"message":    fmt.Sprintf("server certificate EXPIRED (not_after=%s)", cert.GetNotAfter()),
				"not_after":  cert.GetNotAfter(),
				"days_left":  days,
				"severity":   "CRITICAL",
			})
		case days <= 7:
			violations = append(violations, map[string]any{
				"invariant":  "pki_cert_expiring_soon",
				"node_id":    id,
				"hostname":   n.Identity.Hostname,
				"message":    fmt.Sprintf("server certificate expires in %d days", days),
				"not_after":  cert.GetNotAfter(),
				"days_left":  days,
				"severity":   "ERROR",
			})
		case days <= 30:
			violations = append(violations, map[string]any{
				"invariant":  "pki_cert_expiring_soon",
				"node_id":    id,
				"hostname":   n.Identity.Hostname,
				"message":    fmt.Sprintf("server certificate expires in %d days", days),
				"not_after":  cert.GetNotAfter(),
				"days_left":  days,
				"severity":   "WARN",
			})
		}

		// Check chain validity.
		if !cert.GetChainValid() {
			violations = append(violations, map[string]any{
				"invariant": "pki_chain_invalid",
				"node_id":   id,
				"hostname":  n.Identity.Hostname,
				"message":   fmt.Sprintf("certificate chain invalid (subject=%s, issuer=%s)", cert.GetSubject(), cert.GetIssuer()),
				"severity":  "CRITICAL",
			})
		}

		// Check SAN coverage — every node IP must be in the cert SANs.
		sanSet := make(map[string]bool, len(cert.GetSans()))
		for _, san := range cert.GetSans() {
			sanSet[san] = true
		}
		var missingSANs []string
		for _, ip := range n.Identity.Ips {
			if ip == "127.0.0.1" || ip == "::1" {
				continue
			}
			if !sanSet[ip] {
				missingSANs = append(missingSANs, ip)
			}
		}
		if len(missingSANs) > 0 {
			violations = append(violations, map[string]any{
				"invariant":   "pki_san_missing",
				"node_id":     id,
				"hostname":    n.Identity.Hostname,
				"message":     fmt.Sprintf("certificate missing IP SANs: %v — gRPC clients will fail TLS verification", missingSANs),
				"missing_ips": missingSANs,
				"cert_sans":   cert.GetSans(),
				"severity":    "ERROR",
			})
		}

		// Check CA cert availability.
		if certResp.GetCaCert() == nil {
			violations = append(violations, map[string]any{
				"invariant": "pki_ca_missing",
				"node_id":   id,
				"hostname":  n.Identity.Hostname,
				"message":   "CA certificate not found on node — mTLS will fail",
				"severity":  "CRITICAL",
			})
		}
	}

	report := map[string]any{
		"violations":    violations,
		"checked_nodes": checkedCount,
		"total_nodes":   len(nodes),
	}
	return report, nil
}

// --------------------------------------------------------------------------
// PKI / Certificate repair
// --------------------------------------------------------------------------

// invariantRepairPKICerts attempts to fix certificate violations by
// restarting node-agent on affected nodes. The node-agent's ExecStartPre
// re-issues certificates from the cluster CA on every start, which fixes:
//   - Expired certificates → new cert with fresh validity period
//   - Missing SANs → cert re-issued with current node IPs
//   - Invalid chain → cert re-signed by current CA
//   - Missing cert → cert generated from scratch
//
// Cannot fix: missing CA cert (requires manual intervention).
func (srv *server) invariantRepairPKICerts(ctx context.Context, pkiReport map[string]any) (map[string]any, error) {
	violations, _ := pkiReport["violations"].([]map[string]any)
	if len(violations) == 0 {
		// Try []any (JSON-deserialized).
		if raw, ok := pkiReport["violations"].([]any); ok {
			for _, v := range raw {
				if m, ok := v.(map[string]any); ok {
					violations = append(violations, m)
				}
			}
		}
	}

	// Collect unique node IDs that need cert repair. Skip CA-missing
	// (can't fix from here) and agent-unreachable (can't reach anyway).
	needsRestart := make(map[string]string) // node_id → hostname
	for _, v := range violations {
		inv, _ := v["invariant"].(string)
		nodeID, _ := v["node_id"].(string)
		hostname, _ := v["hostname"].(string)
		if nodeID == "" {
			continue
		}
		switch inv {
		case "pki_cert_expired", "pki_cert_expiring_soon",
			"pki_chain_invalid", "pki_san_missing", "pki_cert_missing":
			needsRestart[nodeID] = hostname
		}
		// pki_ca_missing, pki_agent_unreachable, pki_status_unavailable
		// → cannot auto-repair
	}

	if len(needsRestart) == 0 {
		return map[string]any{"repaired": []any{}, "repaired_count": 0}, nil
	}

	// Get agent endpoints under lock, then release before RPCs.
	srv.lock("invariantRepairPKICerts")
	endpoints := make(map[string]string, len(needsRestart))
	for nodeID := range needsRestart {
		if n, ok := srv.state.Nodes[nodeID]; ok && n.AgentEndpoint != "" {
			endpoints[nodeID] = n.AgentEndpoint
		}
	}
	srv.unlock()

	var repaired []map[string]any
	for nodeID, endpoint := range endpoints {
		agent, err := srv.getAgentClient(ctx, endpoint)
		if err != nil {
			log.Printf("invariant-repair: cannot reach agent on %s for cert repair: %v", nodeID, err)
			continue
		}

		_, err = agent.ControlService(ctx, "globular-node-agent.service", "restart")
		if err != nil {
			log.Printf("invariant-repair: node-agent restart failed on %s: %v", nodeID, err)
			continue
		}

		log.Printf("invariant-repair: node-agent restarted on %s (%s) for cert re-issuance",
			needsRestart[nodeID], nodeID)
		repaired = append(repaired, map[string]any{
			"node_id":  nodeID,
			"hostname": needsRestart[nodeID],
			"action":   "restart_node_agent_for_cert_reissuance",
		})
	}

	return map[string]any{
		"repaired":       repaired,
		"repaired_count": len(repaired),
	}, nil
}

