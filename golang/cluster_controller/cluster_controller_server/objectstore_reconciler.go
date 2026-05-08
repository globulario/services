package main

// objectstore_reconciler.go — delete-approval guard for /globular/objectstore/config.
//
// When the objectstore desired-state key is absent, the controller restores it
// from in-memory state UNLESS a valid delete-approval tombstone exists at
// objectstoreDeleteApprovalPrefix. This prevents accidental key deletion from
// silently resetting the MinIO topology contract.
//
// Pattern mirrors ingress_spec_guard.go. The shared approval helpers are in
// delete_approval.go; the approval struct is criticalKeyDeleteApproval.
//
// Invariant: critical_state.deletion_requires_audited_intent

import (
	"context"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
)

const (
	// objectstoreDeleteApprovalPrefix is the etcd key prefix for explicit
	// objectstore desired-state delete approvals.
	// A key at /globular/objectstore/delete_approval/<generation> signals that
	// the operator intentionally deleted the config for that generation.
	// Without an approval, the controller always restores a missing config key.
	objectstoreDeleteApprovalPrefix = "/globular/objectstore/delete_approval/"
)

// startObjectstoreDeleteApprovalGuard starts the background goroutine that
// ensures /globular/objectstore/config is never absent without an explicit
// delete-approval tombstone.
func (srv *server) startObjectstoreDeleteApprovalGuard(ctx context.Context) {
	safeGoTracked("objectstore-delete-approval-guard", 30*time.Second, func(h *globular_service.SubsystemHandle) {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					h.Tick()
					continue
				}
				rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				srv.ensureObjectstoreDesiredState(rctx)
				cancel()
				h.Tick()
			}
		}
	})
}

// ensureObjectstoreDesiredState checks /globular/objectstore/config. If the
// key is absent without a valid delete-approval tombstone, it restores the
// key from the current in-memory cluster state via publishObjectStoreDesiredStateLocked.
func (srv *server) ensureObjectstoreDesiredState(ctx context.Context) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("objectstore-guard: no etcd client: %v", err)
		return
	}

	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	resp, err := cli.Get(rctx, config.EtcdKeyObjectStoreDesired)
	cancel()
	if err != nil {
		log.Printf("objectstore-guard: read %s failed: %v", config.EtcdKeyObjectStoreDesired, err)
		return
	}
	if len(resp.Kvs) > 0 && len(resp.Kvs[0].Value) > 0 {
		return // present and non-empty — nothing to do
	}

	// Key is absent. Check for explicit delete approval before restoring.
	if srv.hasDeleteApproval(ctx, objectstoreDeleteApprovalPrefix) {
		log.Printf("objectstore-guard: %s absent with valid delete approval — honouring operator intent, not restoring",
			config.EtcdKeyObjectStoreDesired)
		return
	}

	log.Printf("objectstore-guard: %s absent without approval — restoring from cluster state",
		config.EtcdKeyObjectStoreDesired)
	srv.mu.Lock()
	srv.publishObjectStoreDesiredStateLocked()
	srv.mu.Unlock()
}
