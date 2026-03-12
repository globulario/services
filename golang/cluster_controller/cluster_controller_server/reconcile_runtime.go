package main

import (
	"context"
	"fmt"
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
		if pkgs, err := installed_state.ListAllNodes(ctx, "SERVICE"); err == nil && len(pkgs) > 0 {
			break
		}
		if pkgs, err := installed_state.ListAllNodes(ctx, "APPLICATION"); err == nil && len(pkgs) > 0 {
			break
		}
		if pkgs, err := installed_state.ListAllNodes(ctx, "INFRASTRUCTURE"); err == nil && len(pkgs) > 0 {
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
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		logger.Warn("startupAutoImport: failed to list desired services", "error", err)
		return
	}
	if len(items) > 0 {
		logger.Info("startupAutoImport: desired state already has entries, skipping auto-import",
			"count", len(items))
		srv.autoImportDone.Store(true)
		return
	}

	// Desired state is empty — attempt import from installed.
	logger.Info("startupAutoImport: desired state is empty, importing from installed services")

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
