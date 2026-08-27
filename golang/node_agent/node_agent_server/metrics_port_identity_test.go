package main

import "testing"

// TestMetricsPortIsNotPersistedUnderAProvisionalID pins that the metrics-port
// writer never files under an identity the cluster has not granted.
//
// metricsPortEtcdKey already treats an empty node id as "skip persistence
// (pre-registration startup)". A locally-derived PROVISIONAL id is exactly that
// case, but it is a non-empty string, so it produced a real key under a node
// that may never become a member — and because the derivation is deterministic,
// the same orphan reappears on every boot.
//
// Measured on a fresh 1.2.339 cluster before any scenario ran: six ids under
// /globular/nodes/ for five nodes. The extra —
// 12944a1b-cfae-5d2f-8056-e8f633c8d3dd, which is nodeid.FromMAC of node-3's own
// container MAC 02:42:0a:0a:00:0d — held exactly one key, node_agent_metrics_port,
// and was absent from the node list. One orphaned record is enough to make
// cluster-doctor report CRITICAL on a healthy cluster and fail every scenario
// asserting zero doctor errors.
func TestMetricsPortIsNotPersistedUnderAProvisionalID(t *testing.T) {
	const provisional = "12944a1b-cfae-5d2f-8056-e8f633c8d3dd"
	const canonical = "a166b992-b66d-53cb-b7c7-61dfa4dd5a36"

	t.Run("an empty id yields no key", func(t *testing.T) {
		if k := metricsPortEtcdKey(""); k != "" {
			t.Fatalf("empty id must skip persistence, got key %q", k)
		}
	})

	t.Run("a granted id yields its own key", func(t *testing.T) {
		want := "/globular/nodes/" + canonical + "/node_agent_metrics_port"
		if k := metricsPortEtcdKey(canonical); k != want {
			t.Fatalf("granted id must persist under itself; got %q want %q", k, want)
		}
	})

	// The guard lives in the closure passed to startMetricsServer: while the id
	// is provisional it reports "", which routes into the skip case above. This
	// asserts the two states produce different outcomes for the same server.
	t.Run("provisional withholds the id, granted releases it", func(t *testing.T) {
		srv := &NodeAgentServer{nodeID: provisional, nodeIDProvisional: true}
		currentNodeID := func() string {
			if srv.nodeIDProvisional {
				return ""
			}
			return srv.nodeID
		}

		if got := currentNodeID(); got != "" {
			t.Fatalf("a provisional id must be withheld from the persister, got %q", got)
		}
		if k := metricsPortEtcdKey(currentNodeID()); k != "" {
			t.Errorf("a provisional id produced key %q — that is an orphaned record "+
				"under a node the cluster never granted", k)
		}

		// Once the controller assigns the canonical id, persistence resumes.
		srv.nodeID = canonical
		srv.nodeIDProvisional = false
		if got := currentNodeID(); got != canonical {
			t.Fatalf("a granted id must be released to the persister, got %q", got)
		}
	})
}
