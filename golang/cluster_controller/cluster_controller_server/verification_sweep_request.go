// @awareness namespace=globular.platform
// @awareness component=platform_controller.reconciler
// @awareness file_role=verification_sweep_request_dispatch
// @awareness implements=globular.platform:intent.runtime.identity_requires_verification
// @awareness risk=high
package main

// verification_sweep_request.go — targeted sweep request writer.
//
// The controller calls requestVerifierSweep when it detects a persistent
// runtime_identity_unproven finding for a (node, service) pair that has
// passed the Day-0 grace window. The doctor collector reads and clears
// these requests at the start of each sweep cycle so the pair is verified
// on the next pass even if it isn't in the normal scheduled set.
//
// All etcd access goes through config.GetEtcdClient() — never hardcoded
// endpoints, never 127.0.0.1.

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/verifier"
)

// requestVerifierSweep writes a targeted sweep request for one (node, service)
// pair to etcd at EtcdKeyForSweepRequest. The doctor collector reads and
// clears these requests at the start of each sweep, ensuring the pair is
// verified on the next sweep cycle even if it's not in the normal scheduled
// set.
//
// Best-effort: etcd write failures are logged but not propagated — the
// targeted sweep is an optimisation, not a correctness requirement. The
// normal sweep cadence will eventually pick up the pair.
func requestVerifierSweep(ctx context.Context, nodeID, service, reason string) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("verification-sweep-request: etcd unavailable: %v", err)
		return
	}
	req := verifier.SweepRequest{
		NodeID:      nodeID,
		Service:     service,
		Reason:      reason,
		RequestedBy: "cluster-controller",
		RequestedAt: time.Now().UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(req)
	if err != nil {
		log.Printf("verification-sweep-request: marshal failed for %s/%s: %v", nodeID, service, err)
		return
	}
	key := verifier.EtcdKeyForSweepRequest(nodeID, service)
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := cli.Put(wctx, key, string(b)); err != nil {
		log.Printf("verification-sweep-request: write failed for %s/%s: %v", nodeID, service, err)
	}
}
