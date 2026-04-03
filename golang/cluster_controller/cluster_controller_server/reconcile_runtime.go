package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
)

const (
	networkReconcileKey = "network/default"
	serviceKeyPrefix    = "service/"
)

func isNetworkKey(key string) bool {
	return key == networkReconcileKey
}

func serviceNameFromKey(key string) string {
	if len(key) <= len(serviceKeyPrefix) || key[:len(serviceKeyPrefix)] != serviceKeyPrefix {
		return ""
	}
	return key[len(serviceKeyPrefix):]
}

// startControllerRuntime starts watch-based reconcile with a small worker pool.
func (srv *server) startControllerRuntime(ctx context.Context, workers int) {
	if workers <= 0 {
		workers = 2
	}
	queue := newWorkQueue(128)

	// initial enqueue
	queue.Enqueue(networkReconcileKey)
	if srv.resources != nil {
		if items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", ""); err == nil {
			for _, obj := range items {
				if sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion); ok && sdv.Meta != nil {
					queue.Enqueue(serviceKeyPrefix + canonicalServiceName(sdv.Meta.Name))
				}
			}
		}
	}

	// watchers
	if srv.resources != nil {
		safeGo("watch-cluster-network", func() {
			ch, err := srv.resources.Watch(ctx, "ClusterNetwork", "default", "")
			if err != nil {
				return
			}
			for evt := range ch {
				_ = evt
				queue.Enqueue(networkReconcileKey)
			}
		})
		safeGo("watch-service-desired", func() {
			ch, err := srv.resources.Watch(ctx, "ServiceDesiredVersion", "", "")
			if err != nil {
				return
			}
			for evt := range ch {
				sdv, ok := evt.Object.(*cluster_controllerpb.ServiceDesiredVersion)
				if !ok || sdv == nil || sdv.Meta == nil {
					continue
				}
				queue.Enqueue(serviceKeyPrefix + canonicalServiceName(sdv.Meta.Name))
			}
		})
		srv.startReleaseReconciler(ctx, queue)
		// Allow ReportNodeStatus to trigger release re-evaluation (e.g. after
		// a node finishes a drift-repair plan and its AppliedServicesHash changes).
		srv.releaseEnqueue = func(releaseName string) {
			queue.Enqueue(releaseKeyPrefix + releaseName)
		}
		srv.infraReleaseEnqueue = func(releaseName string) {
			queue.Enqueue(infraReleaseKeyPrefix + releaseName)
		}
	}
	// Allow SetNodeProfiles to immediately trigger reconciliation.
	srv.enqueueReconcile = func() {
		queue.Enqueue(networkReconcileKey)
	}

	// workers
	for i := 0; i < workers; i++ {
		safeGo(fmt.Sprintf("reconcile-worker-%d", i), func() {
			for {
				key, ok := queue.Get(ctx)
				if !ok {
					return
				}
				if !srv.isLeader() {
					queue.Done(key, fmt.Errorf("not leader"))
					continue
				}
				switch {
				case isNetworkKey(key):
					srv.reconcileNodes(ctx)
				case serviceNameFromKey(key) != "":
					srv.reconcileNodes(ctx)
				case isReleaseKey(key):
					srv.reconcileRelease(ctx, releaseNameFromKey(key))
				case isAppReleaseKey(key):
					srv.reconcileAppRelease(ctx, appReleaseNameFromKey(key))
				case isInfraReleaseKey(key):
					srv.reconcileInfraRelease(ctx, infraReleaseNameFromKey(key))
				default:
					// unknown key, drop
				}
				queue.Done(key, nil)
			}
		})
	}

	// Periodic cluster.reconcile workflow: drives infrastructure health
	// scans (ScyllaDB, MinIO join phases + probes) and detects package
	// drift. Runs every 30s when leader, replacing the old direct calls
	// in reconcileNodes().
	safeGo("periodic-cluster-reconcile", func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				// Run asynchronously so it doesn't block the reconcile queue.
				go func() {
					rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
					defer cancel()
					if _, err := srv.RunClusterReconcileWorkflow(rctx); err != nil {
						logger.Debug("periodic cluster.reconcile failed", "error", err)
					}
				}()
			}
		}
	})

	// Periodic bridge: re-create ServiceRelease objects for desired services
	// that lost their release (e.g. deleted during troubleshooting, or
	// garbage-collected while in REMOVED phase). Without this, a missing
	// release causes the service to stay stuck at "Planned" indefinitely.
	safeGo("periodic-release-bridge", func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if srv.isLeader() {
					srv.ensureServiceReleasesFromDesired(ctx)
				}
			}
		}
	})

	// Trigger A: Auto-import desired state from installed services at startup.
	// Waits for at least one node to report installed versions, then checks if
	// desired state is empty. If so, runs importInstalledToDesired to backfill.
	// This closes the gap where Day-0 seed failed or was skipped.
	safeGo("startup-auto-import", func() {
		srv.startupAutoImport(ctx, queue)
	})
}

// startupAutoImport waits for nodes to report installed versions, then
// auto-imports into desired state if it is empty. This is the Trigger A
// from the state alignment design doc.
func (srv *server) startupAutoImport(ctx context.Context, queue *workQueue) {
	// Wait up to 90s for at least one node to report installed versions.
	// Nodes typically report within 30-60s of startup.
	const maxWait = 90 * time.Second
	const pollInterval = 5 * time.Second

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}

		if !srv.isLeader() {
			continue
		}

		// Check canonical installed-state registry first (any package kind).
		if pkgs, err := installed_state.ListAllNodes(ctx, "SERVICE", ""); err == nil && len(pkgs) > 0 {
			break
		}
		if pkgs, err := installed_state.ListAllNodes(ctx, "APPLICATION", ""); err == nil && len(pkgs) > 0 {
			break
		}
		if pkgs, err := installed_state.ListAllNodes(ctx, "INFRASTRUCTURE", ""); err == nil && len(pkgs) > 0 {
			break
		}

		// Fallback: check in-memory node state.
		srv.lock("startupAutoImport:check")
		hasNodes := false
		hasInstalled := false
		for _, node := range srv.state.Nodes {
			hasNodes = true
			if len(node.InstalledVersions) > 0 {
				hasInstalled = true
				break
			}
		}
		srv.unlock()

		if hasInstalled {
			break
		}
		if hasNodes {
			// Nodes exist but haven't reported installed versions yet — keep waiting.
			continue
		}
	}

	if !srv.isLeader() {
		return
	}

	// Check if desired state is already populated.
	if srv.resources == nil {
		return
	}
	svcItems, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		logger.Warn("startupAutoImport: failed to list desired services", "error", err)
		return
	}
	infraItems, _, _ := srv.resources.List(ctx, "InfrastructureRelease", "")

	// Always reconcile: remove desired entries for services no longer in
	// the installed-state registry. This cleans up stale entries from
	// packages that were incorrectly marked as installed previously.
	srv.reconcileDesiredWithInstalled(ctx)

	if len(svcItems) > 0 && len(infraItems) > 0 {
		logger.Info("startupAutoImport: desired state already has entries, skipping full import",
			"services", len(svcItems), "infra", len(infraItems))
		srv.autoImportDone.Store(true)
		return
	}

	// Desired state is empty (or missing infra) — attempt import from installed.
	logger.Info("startupAutoImport: importing from installed services")

	// Clean up stale keys first.
	if n := srv.cleanupStaleDesiredKeys(ctx); n > 0 {
		logger.Info("startupAutoImport: cleaned up stale entries", "count", n)
	}

	stats, err := srv.importInstalledToDesired(ctx)
	if err != nil {
		logger.Warn("startupAutoImport: import failed", "error", err)
		return
	}

	srv.autoImportDone.Store(true)
	logger.Info("startupAutoImport: import complete",
		"imported", stats.Imported,
		"updated", stats.Updated,
		"already_present", stats.AlreadyPresent,
		"failed", stats.Failed)

	// Enqueue reconciliation for newly imported services.
	if stats.Imported > 0 || stats.Updated > 0 {
		queue.Enqueue(networkReconcileKey)
	}
}

// reconcileDesiredWithInstalled removes ServiceDesiredVersion entries that
// no longer have a corresponding installed-state record. This cleans up
// desired entries for packages that were never truly installed (e.g. created
// by a previous bug that treated all repo artifacts as installed).
func (srv *server) reconcileDesiredWithInstalled(ctx context.Context) {
	if srv.resources == nil {
		return
	}

	// Collect installed service names from etcd.
	installedNames := make(map[string]bool)
	if pkgs, err := installed_state.ListAllNodes(ctx, "SERVICE", ""); err == nil {
		for _, pkg := range pkgs {
			canon := canonicalServiceName(pkg.GetName())
			if canon != "" {
				installedNames[canon] = true
			}
		}
	}

	// Check each desired entry against installed state.
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return
	}
	removed := 0
	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" && sdv.Meta != nil {
			canon = canonicalServiceName(sdv.Meta.Name)
		}
		if canon == "" {
			continue
		}
		// Remove command-type entries from desired state — commands are
		// one-shot and should not persist as desired services.
		// NOTE: Do NOT remove services that are desired but not yet
		// installed — that is the "Planned" state, and the reconciler
		// needs the desired entry to generate an install plan.
		if strings.HasSuffix(canon, "-cmd") {
			if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", canon); err == nil {
				removed++
			}
		}
	}
	if removed > 0 {
		logger.Info("reconcileDesiredWithInstalled: removed stale desired entries", "count", removed)
	}
}
