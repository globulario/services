package main

import (
	"testing"
	"time"
)

// TestRekeyMetricsPortWhenIdentitySettles pins that the metrics port follows the
// node's canonical id rather than the id that happened to be current when the
// listener bound.
//
// startMetricsServer persisted the port under a snapshot of srv.nodeID. On a
// node that starts BEFORE it has joined — every wipe-and-rejoin — that snapshot
// is a pre-join id, and nothing rewrote the key once the real id arrived.
// Observed 2026-08-23 in the 5-node sim: node-5's state.json carried
// 35ac3821-6b90-52eb-a800-41130471770b while its node_agent_metrics_port key sat
// under 2da500c8-32d8-5ffc-8452-6d8af5c02038 — node-5's id from an earlier
// incarnation — and that stale id's entire etcd subtree was that single key.
// The four unaffected nodes had theirs under their canonical ids. Anything
// treating the key as the node's registration or scrape target saw the node as
// absent (identity.has_single_canonical_source_and_is_immutable).
func TestRekeyMetricsPortWhenIdentitySettles(t *testing.T) {
	origAttempts, origInterval, origPersist :=
		metricsRekeyAttempts, metricsRekeyInterval, persistMetricsPortFn
	t.Cleanup(func() {
		metricsRekeyAttempts, metricsRekeyInterval, persistMetricsPortFn =
			origAttempts, origInterval, origPersist
	})
	metricsRekeyAttempts, metricsRekeyInterval = 5, time.Millisecond

	t.Run("re-keys under the canonical id once it settles", func(t *testing.T) {
		type call struct {
			id   string
			port int
		}
		var got []call
		persistMetricsPortFn = func(id string, port int) {
			got = append(got, call{id, port})
		}

		// Bound with a pre-join id; the canonical id arrives on the 2nd check.
		checks := 0
		current := func() string {
			checks++
			if checks < 2 {
				return "2da500c8-pre-join"
			}
			return "35ac3821-canonical"
		}

		rekeyMetricsPortWhenIdentitySettles(current, "2da500c8-pre-join", 11001)

		if len(got) != 1 {
			t.Fatalf("expected exactly one re-key, got %d (%v)", len(got), got)
		}
		if got[0].id != "35ac3821-canonical" {
			t.Errorf("port must be re-keyed under the CANONICAL id, got %q", got[0].id)
		}
		if got[0].port != 11001 {
			t.Errorf("re-key must carry the bound port, got %d", got[0].port)
		}
	})

	t.Run("does not re-key a node whose id never changes", func(t *testing.T) {
		var calls int
		persistMetricsPortFn = func(string, int) { calls++ }

		rekeyMetricsPortWhenIdentitySettles(
			func() string { return "stable-id" }, "stable-id", 11001)

		if calls != 0 {
			t.Errorf("a stable id must not be re-keyed; got %d writes", calls)
		}
	})

	t.Run("gives up rather than watching forever", func(t *testing.T) {
		var calls int
		persistMetricsPortFn = func(string, int) { calls++ }

		done := make(chan struct{})
		go func() {
			defer close(done)
			// An id that is always empty never settles.
			rekeyMetricsPortWhenIdentitySettles(
				func() string { return "" }, "bound-id", 11001)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("re-key watcher must be bounded — it did not terminate " +
				"(error_path.no_unbounded_fire_and_forget_goroutine)")
		}
		if calls != 0 {
			t.Errorf("an unsettled id must not be persisted; got %d writes", calls)
		}
	})
}
