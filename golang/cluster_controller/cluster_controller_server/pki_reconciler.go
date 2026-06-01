// @awareness namespace=globular.platform
// @awareness component=platform_controller.pki
// @awareness file_role=pki_certificate_and_ca_reconciler
// @awareness implements=globular.platform:intent.dns_pki.explicit_identity_over_convenient_routing
// @awareness risk=high
package main

// pki_reconciler.go — delete-approval guard for /globular/pki/ca.
//
// When the PKI CA metadata key is absent, the controller restores it from the
// on-disk CA certificate UNLESS a valid delete-approval tombstone exists at
// pkiCADeleteApprovalPrefix. A missing CA metadata record prevents node agents
// from detecting CA rotation and blocks certificate health checks.
//
// Pattern mirrors ingress_spec_guard.go. The shared approval helpers are in
// delete_approval.go; the approval struct is criticalKeyDeleteApproval.
//
// Note: the restore path calls publishCAMetadataLocked(), which reads
// /var/lib/globular/pki/ca.crt from disk. On nodes that do not hold the CA
// private key this function is a safe no-op — it detects the missing key and
// publishes a placeholder. The guard therefore runs on all leader nodes.
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
	// pkiCADeleteApprovalPrefix is the etcd key prefix for explicit PKI CA
	// metadata delete approvals.
	// A key at /globular/pki/ca_delete_approval/<generation> signals that the
	// operator intentionally deleted the CA metadata record for that generation.
	// Without an approval, the controller always restores a missing CA record.
	pkiCADeleteApprovalPrefix = "/globular/pki/ca_delete_approval/"
)

// startPKICADeleteApprovalGuard starts the background goroutine that ensures
// /globular/pki/ca is never absent without an explicit delete-approval tombstone.
func (srv *server) startPKICADeleteApprovalGuard(ctx context.Context) {
	safeGoTracked("pki-ca-delete-approval-guard", 30*time.Second, func(h *globular_service.SubsystemHandle) {
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
				srv.ensurePKICAMetadata(rctx)
				cancel()
				h.Tick()
			}
		}
	})
}

// ensurePKICAMetadata checks /globular/pki/ca. If the key is absent without a
// valid delete-approval tombstone, it restores CA metadata via
// publishCAMetadataLocked (reads the on-disk CA cert and writes to etcd).
func (srv *server) ensurePKICAMetadata(ctx context.Context) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("pki-ca-guard: no etcd client: %v", err)
		return
	}

	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	resp, err := cli.Get(rctx, config.EtcdKeyCAMetadata)
	cancel()
	if err != nil {
		log.Printf("pki-ca-guard: read %s failed: %v", config.EtcdKeyCAMetadata, err)
		return
	}
	if len(resp.Kvs) > 0 && len(resp.Kvs[0].Value) > 0 {
		return // present and non-empty — nothing to do
	}

	// Key is absent. Check for explicit delete approval before restoring.
	if srv.hasDeleteApproval(ctx, pkiCADeleteApprovalPrefix) {
		log.Printf("pki-ca-guard: %s absent with valid delete approval — honouring operator intent, not restoring",
			config.EtcdKeyCAMetadata)
		return
	}

	log.Printf("pki-ca-guard: %s absent without approval — restoring CA metadata", config.EtcdKeyCAMetadata)
	srv.mu.Lock()
	srv.publishCAMetadataLocked()
	srv.mu.Unlock()
}
