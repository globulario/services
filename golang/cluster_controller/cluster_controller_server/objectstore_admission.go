package main

// objectstore_admission.go — MinIO disk admission and topology contract enforcement.
//
// The controller is the SOLE AUTHORITY for MinIO topology. It enforces the full
// contract on every apply:
//
//   1. startObjectStoreApplyWatcher — watches /globular/objectstore/topology/apply_request
//      via an etcd Watch. When the CLI writes an apply request this goroutine
//      picks it up, enforces the contract, updates in-memory state, publishes
//      new ObjectStoreDesiredState, and triggers the topology workflow.
//
//   2. applyObjectStoreTopologyRequest — enforces the full contract:
//      a. Loads admitted disks from etcd — every (node, path) in the proposal
//         MUST have a matching admitted disk record (admission = operator consent).
//      b. Recomputes destructiveness server-side — ignores proposal.IsDestructive.
//      c. Rejects destructive apply without ForceDestructive.
//      d. Validates the strengthened ValidateTopologyProposal rules.
//      e. REPLACES MinioPoolNodes with the exact proposal node list (not append).
//      f. Writes a TopologyTransition record when destructive, so node-agents
//         can confirm wipe authorization before wiping .minio.sys.
//      g. Persists state and triggers RunObjectStoreTopologyWorkflow.
//
//   3. ValidateTopologyProposal — enforces:
//      - no pool nodes → reject
//      - DrivesPerNode < 1 for distributed → reject
//      - fewer node_paths than nodes → reject
//      - invalid node IP → reject
//      - missing or non-absolute path → reject
//      - path not matching admitted record → reject
//      - root filesystem path without ForceRoot → reject
//
// The controller NEVER reads disk candidate keys; that is the CLI's job.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	configpkg "github.com/globulario/services/golang/config"
)

// ── injectable hooks (overridden in tests) ────────────────────────────────────

// objectstoreApplyTransitionSaver persists a TopologyTransition to etcd.
// Overridable in tests to simulate write failures.
var objectstoreApplyTransitionSaver = func(ctx context.Context, t *configpkg.TopologyTransition) error {
	return configpkg.SaveTopologyTransition(ctx, t)
}

// objectstoreApplyTransitionLoader reads a TopologyTransition from etcd.
// Overridable in tests.
var objectstoreApplyTransitionLoader = func(ctx context.Context, gen int64) (*configpkg.TopologyTransition, error) {
	return configpkg.LoadTopologyTransition(ctx, gen)
}

// objectstoreApplyTransitionDeleter removes a TopologyTransition from etcd.
// Overridable in tests.
var objectstoreApplyTransitionDeleter = func(ctx context.Context, gen int64) error {
	return configpkg.DeleteTopologyTransition(ctx, gen)
}

// objectstoreApplyCandidateLoader loads disk candidates for a node from etcd.
// Overridable in tests to simulate failures or return synthetic candidates.
var objectstoreApplyCandidateLoader = func(ctx context.Context, nodeID string) ([]*configpkg.DiskCandidate, error) {
	return configpkg.LoadDiskCandidates(ctx, nodeID)
}

// startObjectStoreApplyWatcher starts a background goroutine that watches the
// apply_request key and processes topology apply requests from the CLI.
// Only the leader processes requests.
func (srv *server) startObjectStoreApplyWatcher(ctx context.Context) {
	safeGo("objectstore-apply-watcher", func() {
		cli, err := configpkg.GetEtcdClient()
		if err != nil {
			log.Printf("objectstore-apply-watcher: etcd unavailable: %v", err)
			return
		}

		// Drain any pending request on startup (in case of controller restart).
		srv.drainObjectStoreApplyRequest(ctx, cli)

		watchCh := cli.Watch(ctx, configpkg.EtcdKeyObjectStoreApplyRequest)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-watchCh:
				if !ok {
					return
				}
				if ev.Err() != nil {
					log.Printf("objectstore-apply-watcher: watch error: %v", ev.Err())
					continue
				}
				for _, e := range ev.Events {
					if e.Type != clientv3.EventTypePut {
						continue
					}
					if !srv.isLeader() {
						// Non-leader: ignore — the leader will handle it.
						continue
					}
					srv.handleObjectStoreApplyRequest(ctx, cli, e.Kv.Value)
				}
			}
		}
	})
}

// drainObjectStoreApplyRequest handles any apply_request that was written while
// this controller instance was starting up or while leadership was transitioning.
func (srv *server) drainObjectStoreApplyRequest(ctx context.Context, cli *clientv3.Client) {
	if !srv.isLeader() {
		return
	}
	readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(readCtx, configpkg.EtcdKeyObjectStoreApplyRequest)
	if err != nil || len(resp.Kvs) == 0 {
		return
	}
	srv.handleObjectStoreApplyRequest(ctx, cli, resp.Kvs[0].Value)
}

// handleObjectStoreApplyRequest parses and processes a single apply request.
func (srv *server) handleObjectStoreApplyRequest(ctx context.Context, cli *clientv3.Client, data []byte) {
	var req configpkg.ObjectStoreApplyRequest
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("objectstore-apply: parse request: %v", err)
		srv.writeObjectStoreApplyResult(ctx, cli, &configpkg.ObjectStoreApplyResult{
			Status: "failed",
			Error:  fmt.Sprintf("parse request: %v", err),
		})
		return
	}

	log.Printf("objectstore-apply: processing request %s proposal=%s destructive=%v",
		req.RequestID, req.ProposalID, req.ForceDestructive)

	result := srv.applyObjectStoreTopologyRequest(ctx, &req)
	result.RequestID = req.RequestID
	result.ProposalID = req.ProposalID
	result.ProcessedAt = time.Now().UTC()

	srv.writeObjectStoreApplyResult(ctx, cli, result)

	// Delete the request key regardless of outcome so the CLI doesn't re-process.
	delCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := cli.Delete(delCtx, configpkg.EtcdKeyObjectStoreApplyRequest); err != nil {
		log.Printf("objectstore-apply: delete request key: %v", err)
	}
}

// applyObjectStoreTopologyRequest enforces the full topology contract and applies
// the proposal when all checks pass.
//
// For destructive applies the ordering is STRICTLY:
//  1. Write TopologyTransition to etcd (pre-write before any state mutation)
//  2. Only then acquire lock, update in-memory state, and persist desired topology
//  3. Verify transition record still exists after persist (belt-and-suspenders)
//  4. Only trigger workflow after verification passes
//
// If the transition write fails at step 1, the apply is rejected without any
// state change. If persist fails at step 2, the transition record is cleaned up
// and the in-memory counter is rolled back. This ensures the cluster can never
// reach a state where a destructive desired topology exists without a matching
// approved TopologyTransition record.
func (srv *server) applyObjectStoreTopologyRequest(ctx context.Context, req *configpkg.ObjectStoreApplyRequest) *configpkg.ObjectStoreApplyResult {
	if req.Proposal == nil {
		return applyFail("request has nil proposal")
	}
	p := req.Proposal

	// ── Step 1: Load admitted disks — operator consent records ───────────────
	admitCtx, admitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer admitCancel()
	admitted, err := configpkg.LoadAdmittedDisks(admitCtx)
	if err != nil {
		return applyFail("load admitted disks: " + err.Error())
	}
	// Index by nodeIP → path → record (preserves multiple admissions per node).
	admittedByIPPath := buildAdmittedIndex(admitted)

	// ── Step 1b: Validate against live disk candidates (fail closed) ──────────
	// Stale admissions must not pass when current candidate state is unavailable.
	// Uses injectable loader so tests can simulate failures.
	candCtx, candCancel := context.WithTimeout(ctx, 10*time.Second)
	defer candCancel()
	if candErrs := validateAdmissionsAgainstCandidates(candCtx, p, admittedByIPPath, objectstoreApplyCandidateLoader); len(candErrs) > 0 {
		return applyFail("disk candidate validation failed: " + strings.Join(candErrs, "; "))
	}

	// ── Step 2: Full validation (including admission checks) ─────────────────
	if valErrs := ValidateTopologyProposal(p, admittedByIPPath); len(valErrs) > 0 {
		return applyFail("proposal validation failed: " + strings.Join(valErrs, "; "))
	}

	// ── Step 3: Load current desired state and recompute destructiveness ──────
	// CRITICAL: never trust proposal.IsDestructive from the client.
	desiredCtx, desiredCancel := context.WithTimeout(ctx, 10*time.Second)
	defer desiredCancel()
	current, err := configpkg.LoadObjectStoreDesiredState(desiredCtx)
	if err != nil {
		return applyFail("load current desired state: " + err.Error())
	}
	isDestructive, destructiveReasons := ComputeTopologyDestructiveness(p, current)
	if isDestructive && !req.ForceDestructive {
		return applyFail("topology change is destructive (" + strings.Join(destructiveReasons, "; ") +
			") — rerun with --i-understand-data-reset to confirm")
	}

	// ── Step 4: Validate node IPs are known cluster members ──────────────────
	srv.lock("applyObjectStoreTopology:snapshot")
	knownIPs := make(map[string]bool, len(srv.state.Nodes))
	for _, n := range srv.state.Nodes {
		if ip := nodeRoutableIP(n); ip != "" {
			knownIPs[ip] = true
		}
	}
	currentGen := srv.state.ObjectStoreGeneration
	srv.unlock()

	for _, ip := range p.Nodes {
		if !knownIPs[ip] {
			return applyFail(fmt.Sprintf("proposal node %s is not a known cluster node", ip))
		}
	}

	// ── Step 5: Pre-write TopologyTransition BEFORE mutating desired state ────
	// Invariant: a destructive desired topology must NEVER exist in etcd without
	// a matching approved TopologyTransition record. Achieve this by writing
	// the transition first, under the prospective new generation number, then
	// persisting the desired state only if the transition write succeeded.
	//
	// Prospective generation is currentGen+1. If a concurrent apply changes the
	// generation between now and Step 6, the generation-guard at Step 6 fires and
	// the orphaned transition record is cleaned up before returning.
	prospectiveGen := currentGen + 1
	var prewrittenTransition *configpkg.TopologyTransition

	if isDestructive {
		prewrittenTransition = &configpkg.TopologyTransition{
			Generation:    prospectiveGen,
			IsDestructive: true,
			AffectedNodes: append([]string(nil), p.Nodes...),
			AffectedPaths: copyStringMap(p.NodePaths),
			Reasons:       destructiveReasons,
			Approved:      req.ForceDestructive,
			CreatedAt:     time.Now().UTC(),
		}
		tCtx, tCancel := context.WithTimeout(ctx, 5*time.Second)
		defer tCancel()
		if err := objectstoreApplyTransitionSaver(tCtx, prewrittenTransition); err != nil {
			// Hard fail — do not touch desired state without the transition record.
			return applyFail(fmt.Sprintf(
				"failed to write TopologyTransition for gen %d — destructive apply aborted: %v",
				prospectiveGen, err))
		}
		log.Printf("objectstore-apply: pre-wrote transition record gen=%d approved=%v reasons=%v",
			prospectiveGen, prewrittenTransition.Approved, destructiveReasons)
	}

	// cleanupTransition removes the pre-written transition record on failure.
	// Called whenever we bail after writing the transition but before commit.
	cleanupTransition := func(gen int64) {
		if gen <= 0 {
			return
		}
		cCtx, cCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cCancel()
		if err := objectstoreApplyTransitionDeleter(cCtx, gen); err != nil {
			log.Printf("objectstore-apply: WARNING: could not clean up transition gen=%d: %v", gen, err)
		}
	}

	// ── Step 6: Apply under lock — REPLACE pool nodes (not append) ───────────
	srv.lock("applyObjectStoreTopology:apply")

	// Concurrent-apply guard: if another apply ran between Step 4 and now, abort.
	if srv.state.ObjectStoreGeneration != currentGen {
		srv.unlock()
		cleanupTransition(prospectiveGen)
		return applyFail(fmt.Sprintf(
			"concurrent topology apply detected — generation changed from %d, retry the apply",
			currentGen))
	}

	// Replace node paths entirely so stale entries from removed nodes are cleared.
	srv.state.MinioNodePaths = copyStringMap(p.NodePaths)
	if p.DrivesPerNode > 0 {
		srv.state.MinioDrivesPerNode = p.DrivesPerNode
	}
	// Replace pool — not append — so removed nodes are gone.
	srv.state.MinioPoolNodes = append([]string(nil), p.Nodes...)

	srv.state.ObjectStoreGeneration++ // == prospectiveGen
	newGen := srv.state.ObjectStoreGeneration

	if err := srv.persistStateLocked(true); err != nil {
		// Roll back the in-memory generation so the server state matches disk.
		srv.state.ObjectStoreGeneration = currentGen
		srv.unlock()
		cleanupTransition(prospectiveGen)
		return applyFail("persist state: " + err.Error())
	}
	srv.unlock()

	// ── Step 7: Verify transition record survives persist (belt-and-suspenders) ─
	// Guards against etcd TTL expiry or split-brain between pre-write and now.
	if isDestructive {
		verCtx, verCancel := context.WithTimeout(ctx, 5*time.Second)
		defer verCancel()
		verified, verErr := objectstoreApplyTransitionLoader(verCtx, newGen)
		if verErr != nil {
			return applyFail(fmt.Sprintf(
				"transition record unreadable after persist gen=%d: %v — workflow not triggered; re-run apply",
				newGen, verErr))
		}
		if verified == nil {
			return applyFail(fmt.Sprintf(
				"transition record vanished after persist gen=%d — workflow not triggered; re-run apply",
				newGen))
		}
		if !verified.Approved {
			return applyFail(fmt.Sprintf(
				"transition record gen=%d exists but Approved=false — workflow not triggered",
				newGen))
		}
	}

	log.Printf("objectstore-apply: topology committed gen=%d nodes=%v destructive=%v",
		newGen, p.Nodes, isDestructive)

	// ── Step 8: Trigger topology workflow asynchronously ──────────────────────
	capturedGen := newGen
	safeGo("objectstore-apply-workflow", func() {
		wctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if _, err := srv.RunObjectStoreTopologyWorkflow(wctx, capturedGen); err != nil {
			log.Printf("objectstore-apply: topology workflow failed: %v", err)
		}
	})

	return &configpkg.ObjectStoreApplyResult{
		Status:     "accepted",
		Generation: newGen,
	}
}

// copyStringMap returns a shallow copy of a map[string]string.
// Used to snapshot NodePaths so the transition record isn't aliased to the proposal.
func copyStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// applyFail is a convenience constructor for a failed apply result.
func applyFail(msg string) *configpkg.ObjectStoreApplyResult {
	return &configpkg.ObjectStoreApplyResult{Status: "failed", Error: msg}
}

// writeObjectStoreApplyResult writes the result to etcd for the CLI to read.
func (srv *server) writeObjectStoreApplyResult(ctx context.Context, cli *clientv3.Client, result *configpkg.ObjectStoreApplyResult) {
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("objectstore-apply: marshal result: %v", err)
		return
	}
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// TTL of 5 minutes so stale results don't accumulate.
	lease, err := cli.Grant(writeCtx, 300)
	if err != nil {
		// Write without TTL on lease failure.
		cli.Put(writeCtx, configpkg.EtcdKeyObjectStoreApplyResult, string(data)) //nolint:errcheck
		return
	}
	cli.Put(writeCtx, configpkg.EtcdKeyObjectStoreApplyResult, string(data), //nolint:errcheck
		clientv3.WithLease(lease.ID))
}

// ── topology validation (used by CLI and controller) ─────────────────────────

// buildAdmittedIndex converts a flat slice of admitted disks into a
// nodeIP → path → record map. Multiple admitted disks per node are preserved.
func buildAdmittedIndex(admitted []*configpkg.AdmittedDisk) map[string]map[string]*configpkg.AdmittedDisk {
	idx := make(map[string]map[string]*configpkg.AdmittedDisk, len(admitted))
	for _, ad := range admitted {
		if idx[ad.NodeIP] == nil {
			idx[ad.NodeIP] = make(map[string]*configpkg.AdmittedDisk)
		}
		idx[ad.NodeIP][ad.Path] = ad
	}
	return idx
}

// validateAdmissionsAgainstCandidates cross-checks each (nodeIP, path) pair in
// the proposal against the current disk candidate facts from etcd. Admission
// records may be stale — this catches disk replacements, path changes, and
// eligibility regressions since the operator last admitted the disk.
//
// Fail-closed: if the live candidate list cannot be loaded for any proposal node,
// the apply is rejected. Stale admissions must not pass when current candidate
// state is unavailable.
//
// Physical identity checks (StableID, Device, SizeBytes) detect silent disk
// replacement behind the same mount path since the operator admitted the disk.
func validateAdmissionsAgainstCandidates(
	ctx context.Context,
	p *configpkg.TopologyProposal,
	admittedByIPPath map[string]map[string]*configpkg.AdmittedDisk,
	loadCandidates func(ctx context.Context, nodeID string) ([]*configpkg.DiskCandidate, error),
) []string {
	if len(admittedByIPPath) == 0 {
		return nil
	}

	var errs []string
	for _, ip := range p.Nodes {
		path, ok := p.NodePaths[ip]
		if !ok || path == "" {
			continue // path validation handled by ValidateTopologyProposal
		}
		pathMap, hasNode := admittedByIPPath[ip]
		if !hasNode {
			continue // admission gap reported by ValidateTopologyProposal
		}
		ad, hasAd := pathMap[path]
		if !hasAd {
			continue // path mismatch reported by ValidateTopologyProposal
		}

		// Load disk candidates for this node — FAIL CLOSED on error.
		// An etcd transient is not an acceptable reason to skip identity validation.
		candidates, err := loadCandidates(ctx, ad.NodeID)
		if err != nil {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: could not load disk candidates (nodeID=%s): %v — re-scan and re-admit",
				ip, path, ad.NodeID, err))
			continue
		}

		// Find candidate whose MountPath matches the admitted path.
		var found *configpkg.DiskCandidate
		for _, c := range candidates {
			if c.MountPath == path {
				found = c
				break
			}
		}
		if found == nil {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: no current disk candidate found (disk may have been removed or re-mounted) — re-scan and re-admit",
				ip, path))
			continue
		}

		// ── Physical identity checks (Item 3: disk replacement detection) ────

		// StableID: if both admission record and live candidate have a StableID,
		// they must match. A difference means the physical disk was replaced.
		if ad.StableID != "" && found.StableID != "" && ad.StableID != found.StableID {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: StableID mismatch (admitted=%q current=%q) — disk replaced, re-admit",
				ip, path, ad.StableID, found.StableID))
			continue
		}

		// Device: if both have a Device, they must match. Changing block device
		// behind the same mount point means a new disk was mounted there.
		if ad.Device != "" && found.Device != "" && ad.Device != found.Device {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: block device changed (admitted=%q current=%q) — disk may have been replaced, re-admit",
				ip, path, ad.Device, found.Device))
			continue
		}

		// SizeBytes: reject if capacity changed by more than 20%. A shrink
		// indicates a smaller disk was silently substituted; a large growth
		// indicates a different disk entirely.
		if ad.SizeBytesAtAdmission > 0 && found.SizeBytes > 0 {
			delta := found.SizeBytes - ad.SizeBytesAtAdmission
			if delta < 0 {
				delta = -delta
			}
			if float64(delta)/float64(ad.SizeBytesAtAdmission) > 0.20 {
				errs = append(errs, fmt.Sprintf(
					"node %s path %q: disk size changed by >20%% (admitted=%d bytes current=%d bytes) — verify disk identity, re-admit",
					ip, path, ad.SizeBytesAtAdmission, found.SizeBytes))
				continue
			}
		}

		// ── Eligibility and guard checks ──────────────────────────────────────

		// Eligibility check.
		if !found.Eligible && !ad.ForceExistingData && !ad.ForceRoot {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: disk is no longer eligible (%v) — re-scan and re-admit",
				ip, path, found.Reasons))
			continue
		}

		// Root filesystem guard.
		if found.IsRoot && !ad.ForceRoot {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: candidate reports IsRoot=true but admission lacks --force-root",
				ip, path))
		}

		// Existing-data guard.
		if found.HasExistingData && !found.HasMinioSys && !ad.ForceExistingData {
			errs = append(errs, fmt.Sprintf(
				"node %s path %q: candidate has non-MinIO existing data but admission lacks --force-existing-data",
				ip, path))
		}
	}
	return errs
}

// ValidateTopologyProposal validates a topology proposal and returns validation
// errors. Called by the controller on every apply (with admitted disk records)
// and optionally by the CLI pre-apply (admittedByIPPath may be nil for basic checks).
//
// Rejects:
//   - empty node list
//   - DrivesPerNode < 1 for multi-node (distributed) topology
//   - fewer node_paths entries than nodes
//   - invalid node IPs
//   - missing or non-absolute paths
//   - path not matching an admitted disk record (admission = operator consent)
//   - root filesystem path without ForceRoot in the admitted record
func ValidateTopologyProposal(p *configpkg.TopologyProposal, admittedByIPPath map[string]map[string]*configpkg.AdmittedDisk) []string {
	var errs []string

	if len(p.Nodes) == 0 {
		return append(errs, "no pool nodes specified")
	}

	// DrivesPerNode must be ≥ 1 for distributed (multi-node) topology.
	if len(p.Nodes) >= 2 && p.DrivesPerNode < 1 {
		errs = append(errs, "drives_per_node must be ≥ 1 for distributed topology")
	}

	// Every node in the pool must have a path.
	if len(p.NodePaths) < len(p.Nodes) {
		errs = append(errs, fmt.Sprintf(
			"missing node paths: %d nodes but only %d paths — all pool nodes must have a path",
			len(p.Nodes), len(p.NodePaths)))
	}

	// Validate each node.
	for _, ip := range p.Nodes {
		if net.ParseIP(ip) == nil {
			errs = append(errs, fmt.Sprintf("node %q is not a valid IP address", ip))
		}

		path, ok := p.NodePaths[ip]
		if !ok || path == "" {
			errs = append(errs, fmt.Sprintf("node %s has no path in node_paths", ip))
			continue
		}
		if !strings.HasPrefix(path, "/") {
			errs = append(errs, fmt.Sprintf("node %s path %q is not absolute", ip, path))
			continue
		}

		// Admission record checks (only when records are available).
		if admittedByIPPath == nil {
			continue
		}
		pathMap, hasNode := admittedByIPPath[ip]
		if !hasNode {
			errs = append(errs, fmt.Sprintf("node %s has no admitted disk record — run 'globular objectstore disk approve'", ip))
			continue
		}
		ad, hasPath := pathMap[path]
		if !hasPath {
			errs = append(errs, fmt.Sprintf("node %s path %q is not in admitted records — run 'globular objectstore disk approve' for this path", ip, path))
			continue
		}
		// Root filesystem guard.
		if ad.Path == "/" && !ad.ForceRoot {
			errs = append(errs, fmt.Sprintf("node %s: path %q is the root filesystem — re-admit with --force-root", ip, path))
		}
	}

	return errs
}

// ComputeTopologyDestructiveness returns (isDestructive, reasons) by checking
// whether applying this proposal would require wiping .minio.sys.
// The current desired state fingerprint is compared to the proposal's fingerprint.
func ComputeTopologyDestructiveness(
	proposal *configpkg.TopologyProposal,
	current *configpkg.ObjectStoreDesiredState,
) (bool, []string) {
	if current == nil {
		// No current desired state: standalone → distributed.
		if len(proposal.Nodes) >= 2 {
			return true, []string{"first distributed topology: will wipe standalone .minio.sys on all pool nodes"}
		}
		return false, nil
	}

	var reasons []string

	// Mode transition: standalone → distributed.
	if current.Mode == configpkg.ObjectStoreModeStandalone && len(proposal.Nodes) >= 2 {
		reasons = append(reasons, fmt.Sprintf(
			"standalone → distributed transition on %d nodes: .minio.sys will be wiped on all pool nodes",
			len(proposal.Nodes)))
	}

	// Node path changes: if a pool node's base path changes, its .minio.sys must be wiped.
	for ip, newPath := range proposal.NodePaths {
		if oldPath, ok := current.NodePaths[ip]; ok && oldPath != newPath {
			reasons = append(reasons, fmt.Sprintf(
				"node %s path change %q → %q: .minio.sys will be wiped", ip, oldPath, newPath))
		}
	}

	// Fingerprint change with applied topology: if the workflow already applied
	// a generation, changing topology wipes erasure sets.
	if current.Mode == configpkg.ObjectStoreModeDistributed {
		currentFP := configpkg.RenderStateFingerprint(current)
		// Build a tentative desired state from the proposal to compute its fingerprint.
		tentative := &configpkg.ObjectStoreDesiredState{
			Mode:          configpkg.ObjectStoreModeDistributed,
			Generation:    current.Generation, // same generation — compare topology only, not the bump
			Nodes:         proposal.Nodes,
			DrivesPerNode: proposal.DrivesPerNode,
		}
		// Build volumes hash from proposed paths.
		nodeVols := make(map[string]string, len(proposal.NodePaths))
		for ip, path := range proposal.NodePaths {
			nodeVols[ip] = path
		}
		tentative.VolumesHash = configpkg.ComputeVolumesHash(nodeVols)
		proposalFP := configpkg.RenderStateFingerprint(tentative)
		if proposalFP != currentFP {
			reasons = append(reasons, fmt.Sprintf(
				"topology fingerprint change: current=%s→ proposed=%s (drives or pool changed)",
				currentFP[:8], proposalFP[:8]))
		}
	}

	return len(reasons) > 0, reasons
}
