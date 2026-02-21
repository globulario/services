package main

import (
	"context"
	"fmt"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
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
				if sdv, ok := obj.(*clustercontrollerpb.ServiceDesiredVersion); ok && sdv.Meta != nil {
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
				sdv, ok := evt.Object.(*clustercontrollerpb.ServiceDesiredVersion)
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
				default:
					// unknown key, drop
				}
				queue.Done(key, nil)
			}
		})
	}
}
