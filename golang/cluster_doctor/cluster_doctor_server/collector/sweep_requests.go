package collector

// sweep_requests.go — targeted sweep request consumer.
//
// The controller writes sweep requests to etcd under
// /globular/verification/requests/<nodeID>/<service> when it detects a
// persistent runtime_identity_unproven finding past the Day-0 grace window.
// sweepRequestedPairs reads those requests, clears them, and returns the
// (nodeID, service) pairs so runVerification can inject them into the
// current sweep ahead of the normal scheduled set.
//
// All etcd access goes through config.GetEtcdClient() — never hardcoded
// endpoints, never 127.0.0.1.

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/verifier"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// sweepRequestedPair is the minimal resolved form of a targeted sweep request.
type sweepRequestedPair struct {
	NodeID  string
	Service string
}

// sweepRequestedPairs reads all pending targeted sweep requests from etcd,
// clears them, and returns the (nodeID, service) pairs. The pairs are
// injected into the verification sweep ahead of the normal sweep set so
// the controller doesn't have to wait for the next scheduled sweep cycle.
//
// Best-effort: an etcd read failure returns nil (empty slice) — the normal
// sweep cadence remains the correctness backstop.
func sweepRequestedPairs(ctx context.Context) []sweepRequestedPair {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil
	}
	rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := cli.Get(rctx, verifier.EtcdSweepRequestPrefix, clientv3.WithPrefix())
	if err != nil || resp == nil || len(resp.Kvs) == 0 {
		return nil
	}

	var pairs []sweepRequestedPair
	for _, kv := range resp.Kvs {
		var req verifier.SweepRequest
		parseErr := json.Unmarshal(kv.Value, &req)
		if parseErr != nil || req.NodeID == "" || req.Service == "" {
			// Fallback: derive (node, service) from the key itself.
			// Key shape: /globular/verification/requests/<nodeID>/<service>
			keyStr := strings.TrimPrefix(string(kv.Key), verifier.EtcdSweepRequestPrefix)
			parts := strings.SplitN(keyStr, "/", 2)
			if len(parts) == 2 {
				req.NodeID = parts[0]
				req.Service = parts[1]
			}
		}
		if req.NodeID == "" || req.Service == "" {
			continue
		}
		pairs = append(pairs, sweepRequestedPair{NodeID: req.NodeID, Service: req.Service})
		log.Printf("verification: processing targeted sweep request: node=%s service=%s reason=%s",
			req.NodeID, req.Service, req.Reason)

		// Clear the request — best-effort; a lingering key just causes a
		// duplicate (harmless) sweep on the next cycle.
		dctx, dcancel := context.WithTimeout(ctx, 2*time.Second)
		_, _ = cli.Delete(dctx, string(kv.Key))
		dcancel()
	}
	return pairs
}
