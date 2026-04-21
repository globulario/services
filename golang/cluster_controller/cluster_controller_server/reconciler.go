package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// driftReconciler periodically compares desired state against installed state
// on each node and dispatches ApplyPackageRelease for any drift detected.
// Only the leader runs reconciliation.
//
// IMPORTANT: As of the workflow pipeline fix, the cluster.reconcile workflow
// handles drift remediation through proper auditable steps (scan_drift →
// classify_drift → dispatch_remediations). The drift reconciler now ONLY
// detects drift and emits events — it no longer applies packages directly.
// This prevents duplicate applies and ensures all deployments have an
// audit trail through the workflow system.
type driftReconciler struct {
	srv      *server
	interval time.Duration
	timeout  time.Duration // per-apply RPC timeout

	mu          sync.Mutex
	inflight    map[string]time.Time // "nodeID/KIND/name" -> dispatch time
	inflightTTL time.Duration
	sem         chan struct{} // bounds concurrent dispatches
}

func newDriftReconciler(srv *server, interval time.Duration) *driftReconciler {
	return &driftReconciler{
		srv:         srv,
		interval:    interval,
		timeout:     5 * time.Minute,
		inflight:    make(map[string]time.Time),
		inflightTTL: 10 * time.Minute,
		sem:         make(chan struct{}, 2), // max 2 concurrent applies
	}
}

// Start launches the reconciler as a background goroutine.
func (r *driftReconciler) Start(ctx context.Context) {
	safeGo("drift-reconciler", func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if r.srv.isLeader() {
					r.reconcileOnce(ctx)
				}
			}
		}
	})
}

func (r *driftReconciler) reconcileOnce(ctx context.Context) {
	r.expireStale()

	desired := r.srv.collectDesiredVersions(ctx)
	if len(desired) == 0 {
		return
	}

	repo := resolveRepositoryInfo()

	// Snapshot node list under lock.
	r.srv.lock("drift-reconciler:snapshot")
	nodes := make(map[string]*nodeState, len(r.srv.state.Nodes))
	for id, n := range r.srv.state.Nodes {
		nodes[id] = n
	}
	r.srv.unlock()

	for nodeID, node := range nodes {
		if node.Status != "ready" {
			continue
		}
		if node.AgentEndpoint == "" {
			continue
		}

		// Load suspended services for this node. Services suspended by
		// crash-loop suppression should not be re-dispatched until the
		// operator or doctor clears the marker.
		suspendedSet := r.loadSuspendedServices(ctx, nodeID)

		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, "")
		if err != nil {
			log.Printf("drift-reconciler: list installed for node %s: %v", nodeID, err)
			continue
		}

		// Index installed by "KIND/name".
		installedMap := make(map[string]installedInfo, len(pkgs))
		for _, pkg := range pkgs {
			key := strings.ToUpper(pkg.GetKind()) + "/" + canonicalServiceName(pkg.GetName())
			installedMap[key] = installedInfo{
				version:     pkg.GetVersion(),
				buildNumber: pkg.GetBuildNumber(),
				status:      pkg.GetStatus(),
				checksum:    pkg.GetChecksum(),
			}
		}

		for desiredKey, dv := range desired {
			pkg, found := installedMap[desiredKey]

			// Already aligned.
			if found && versionutil.EqualFull(dv.version, dv.buildNumber, pkg.version, pkg.buildNumber) {
				continue
			}
			// Transition-era installs: if the installed record has no build_id (0 = not tracked),
			// version equality is sufficient. We cannot determine which specific build was
			// installed, so we do not report drift against a known version match.
			if found && pkg.buildNumber == 0 && versionutil.Equal(dv.version, pkg.version) {
				continue
			}
			// Node-agent is already applying this package.
			if found && pkg.status == "updating" {
				continue
			}

			// Skip services suspended by crash-loop suppression.
			parts0 := strings.SplitN(desiredKey, "/", 2)
			if len(parts0) == 2 && suspendedSet[parts0[1]] {
				log.Printf("drift-reconciler: skipping %s on node %s (crash-loop suspended)", parts0[1], nodeID)
				continue
			}

			inflightKey := nodeID + "/" + desiredKey
			if r.isInflight(inflightKey) {
				continue
			}
			// Apply-loop detection: skip if this package/node is quarantined
			// due to repeated applies without convergence.
			if r.srv.applyLoopDet != nil && r.srv.applyLoopDet.IsQuarantined(inflightKey) {
				log.Printf("drift-reconciler: skipped %s — quarantined (apply loop detected)", inflightKey)
				continue
			}
			// Cross-path dedup: check if the release pipeline already
			// has an active workflow for this package on this node.
			if r.srv.dispatchReg != nil && !r.srv.dispatchReg.TryAcquire(inflightKey, "drift-reconciler") {
				continue
			}
			// The drift-reconciler is observation-only — it emits events
			// but does not dispatch workflows. Release the dispatch slot
			// when done so the release pipeline can dispatch corrections.
			// Without this, slots are held until 15m TTL, permanently
			// blocking convergence.
			releaseDedupSlot := func() {
				if r.srv.dispatchReg != nil {
					r.srv.dispatchReg.Release(inflightKey)
				}
			}
			defer releaseDedupSlot()

			parts := strings.SplitN(desiredKey, "/", 2)
			if len(parts) != 2 {
				continue
			}
			kind, name := parts[0], parts[1]

			// Only reconcile SERVICE packages; infrastructure is managed by bootstrap.
			if kind != "SERVICE" {
				continue
			}

			// Resolve the desired artifact via ReleaseResolver. This:
			//   1. Resolves build_number=0 to the latest PUBLISHED build for the
			//      pinned version (so `services desired set <svc> <ver>` without
			//      --build-number picks up the current build at dispatch time).
			//   2. Implicitly validates the artifact is PUBLISHED (via
			//      getLatestPublished filtering) and exists in the repository.
			// Maintains the truth chain: Repository → Desired → Installed → Runtime.
			resolver := &ReleaseResolver{
				RepositoryAddr: repo.Address,
				ArtifactKind:   repositorypb.ArtifactKind_SERVICE,
			}
			resolved, err := resolver.Resolve(ctx, &cluster_controllerpb.ServiceReleaseSpec{
				PublisherID: defaultPublisherID(),
				ServiceName: name,
				Version:     dv.version,
				BuildNumber: dv.buildNumber,
				Platform:    r.srv.getNodePlatform(nodeID),
			})
			if err != nil {
				log.Printf("drift-reconciler: skipping node=%s pkg=%s@%s-b%d — resolve failed: %v",
					nodeID, name, dv.version, dv.buildNumber, err)
				continue
			}
			// Part 3: Validate desired kind matches repository manifest kind.
			// A mismatch (e.g. INFRASTRUCTURE in repo but SERVICE in desired)
			// creates an infinite apply loop — block dispatch and emit a finding.
			if resolved.RepoKind != repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
				repoKindStr := strings.ToUpper(resolved.RepoKind.String())
				if repoKindStr != kind {
					log.Printf("drift-reconciler: BLOCKED node=%s pkg=%s — desired kind mismatch: %s (desired) vs %s (repo); dispatch suppressed",
						node.Identity.Hostname, name, kind, repoKindStr)
					driftKindMismatchTotal.Inc()
					r.srv.emitClusterEvent("desired.kind_mismatch", map[string]interface{}{
						"severity":     "WARNING",
						"node_id":      nodeID,
						"package":      name,
						"desired_kind": kind,
						"repo_kind":    repoKindStr,
						"message":      fmt.Sprintf("desired kind %s does not match repo artifact kind %s — dispatch blocked", kind, repoKindStr),
					})
					continue
				}
			}

			resolvedBuild := resolved.BuildNumber
			if resolvedBuild == 0 {
				resolvedBuild = dv.buildNumber
			}

			// Checksum-based convergence: if the installed artifact content matches
			// the desired artifact digest, suppress the drift event even if build_id
			// metadata is stale. This handles transition-era installs that lack a
			// build_id record but are otherwise identical to the desired artifact.
			if found && resolved.Digest != "" && pkg.checksum == resolved.Digest {
				continue
			}

			installedVer := "<none>"
			if found {
				installedVer = fmt.Sprintf("%s-b%d", pkg.version, pkg.buildNumber)
			}
			log.Printf("drift-reconciler: node=%s pkg=%s desired=%s-b%d (resolved build=%d) installed=%s — dispatching",
				node.Identity.Hostname, name, dv.version, dv.buildNumber, resolvedBuild, installedVer)

			// Drift detected — emit event for observability. Actual remediation
			// is handled by the cluster.reconcile workflow (scan_drift →
			// classify_drift → dispatch_remediations). No direct apply here.
			r.srv.emitClusterEvent("cluster.drift_detected", map[string]interface{}{
				"node_id":           nodeID,
				"hostname":          node.Identity.Hostname,
				"package":           name,
				"kind":              kind,
				"desired_version":   fmt.Sprintf("%s-b%d", dv.version, resolvedBuild),
				"installed_version": installedVer,
			})
		}
	}
}

type installedInfo struct {
	version     string
	buildNumber int64
	status      string
	checksum    string
}

func (r *driftReconciler) isInflight(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.inflight[key]
	return ok
}

func (r *driftReconciler) markInflight(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inflight[key] = time.Now()
}

func (r *driftReconciler) clearInflight(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inflight, key)
}

func (r *driftReconciler) expireStale() {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-r.inflightTTL)
	for key, t := range r.inflight {
		if t.Before(cutoff) {
			delete(r.inflight, key)
		}
	}
}

// loadSuspendedServices reads the /globular/nodes/{nodeID}/suspended/ prefix
// from etcd and returns a set of service names that are crash-loop suspended.
// The controller skips these during drift reconciliation. Operators clear
// suspension by deleting the etcd key and re-enabling the systemd unit.
func (r *driftReconciler) loadSuspendedServices(ctx context.Context, nodeID string) map[string]bool {
	prefix := fmt.Sprintf("/globular/nodes/%s/suspended/", nodeID)
	cli := r.srv.etcdClient
	if cli == nil {
		return nil
	}
	getCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := cli.Get(getCtx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil || resp == nil {
		return nil
	}
	if len(resp.Kvs) == 0 {
		return nil
	}
	set := make(map[string]bool, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		svcName := strings.TrimPrefix(string(kv.Key), prefix)
		if svcName != "" {
			set[svcName] = true
		}
	}
	return set
}
