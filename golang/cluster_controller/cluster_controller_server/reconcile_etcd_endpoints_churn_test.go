package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The endpoint reconciler used to derive the desired endpoint list from node
// CONVERGENCE STATUS (snapshotCoreNodes keeps only Status ready/admitted) while
// measuring drift against etcd's LIVE MEMBERSHIP. Those are two different
// authorities, so on any cluster where a core node was mid-convergence they
// disagreed permanently: drift was true on every tick, the write could never
// make it false, and the cluster-wide endpoint key was rewritten forever.
//
// Observed 2026-08-10 on a converging 5-node cluster — `live=5` every pass while
// the published list was rewritten to a rotating 3-member subset:
//
//	desired=[.12 .14 .15] stale=[globular-etcd node-3]
//	desired=[.11 .14 .15] stale=[node-2 node-3]
//	desired=[.13 .14 .15] stale=[globular-etcd node-2]
//
// Each list omitted two members that were live in etcd, and each labelled those
// live members "stale". Publishing an endpoint list that omits reachable members
// endangers invariant:etcd.endpoint_reachability, and deriving membership from
// anything other than etcd forks the truth (intent:etcd.is_source_of_truth).

// endpointReconcilerForTest wires a reconciler whose core nodes are split into
// "ready" and "all", mirroring a cluster where some nodes are mid-convergence.
func endpointReconcilerForTest(t *testing.T, readyIPs, allIPs []string, memberIPs []string) (*etcdEndpointReconciler, *map[string]string) {
	t.Helper()
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string { return readyIPs }
	r.knownCoreNodes = func() []string { return allIPs }
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		out := make([]memberSnapshot, 0, len(memberIPs))
		for _, ip := range memberIPs {
			out = append(out, memberSnapshot{
				Name:       "etcd-" + ip,
				PeerURLs:   []string{"https://" + ip + ":2380"},
				ClientURLs: []string{"https://" + ip + ":2379"},
			})
		}
		return out, nil
	}
	written := map[string]string{}
	r.writeToEtcd = func(_ context.Context, key, value string) error {
		written[key] = value
		return nil
	}
	r.writeOutcome = func(_ context.Context, _ etcdEndpointReconcileOutcome) error { return nil }
	return r, &written
}

// TestEndpointListKeepsConvergingCoreNodes pins the core of the fix: a node that
// is a live etcd member stays in the published endpoint list even while its
// convergence status is not "ready". Dropping it publishes a list that omits a
// reachable member.
func TestEndpointListKeepsConvergingCoreNodes(t *testing.T) {
	all := []string{"10.10.0.11", "10.10.0.12", "10.10.0.13", "10.10.0.14", "10.10.0.15"}
	// Only two nodes report ready; the other three are mid-convergence.
	ready := []string{"10.10.0.14", "10.10.0.15"}

	r, written := endpointReconcilerForTest(t, ready, all, all)
	r.reconcileOnce(context.Background())

	payload, ok := (*written)[etcdEndpointListKey]
	if !ok {
		// No write is also acceptable — it means the published list already
		// matched. What must never happen is a write that drops members.
		return
	}
	var eps []string
	if err := json.Unmarshal([]byte(payload), &eps); err != nil {
		t.Fatalf("endpoint payload is not a JSON array: %v", err)
	}
	if len(eps) != len(all) {
		t.Fatalf("published %d endpoints, want %d — a converging node must not be dropped from the endpoint list: %v",
			len(eps), len(all), eps)
	}
	for _, ip := range all {
		found := false
		for _, ep := range eps {
			if strings.Contains(ep, ip) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("endpoint list omits live member %s: %v", ip, eps)
		}
	}
}

// TestEndpointListNotRewrittenWhenAlreadyCorrect pins that an already-correct
// published list is left alone. Without this the reconciler cannot distinguish
// "already correct" from "needs correcting" and rewrites a cluster-wide key on
// every tick — the churn itself, independent of which members are listed.
func TestEndpointListNotRewrittenWhenAlreadyCorrect(t *testing.T) {
	all := []string{"10.10.0.11", "10.10.0.12", "10.10.0.13"}
	ready := []string{"10.10.0.11"} // the other two are mid-convergence

	r, written := endpointReconcilerForTest(t, ready, all, all)
	// The published list is already exactly right — note the deliberately
	// different ORDER, which must not count as drift.
	r.readEndpointList = func(_ context.Context) (string, bool, error) {
		return `["https://10.10.0.13:2379","https://10.10.0.11:2379","https://10.10.0.12:2379"]`, true, nil
	}

	r.reconcileOnce(context.Background())

	if payload, ok := (*written)[etcdEndpointListKey]; ok {
		t.Fatalf("rewrote an already-correct endpoint list (order must not count as drift): %s", payload)
	}
}

// TestEndpointListWrittenWhenPublishedIsWrong pins that the idempotence check
// does not silence a real correction.
func TestEndpointListWrittenWhenPublishedIsWrong(t *testing.T) {
	all := []string{"10.10.0.11", "10.10.0.12", "10.10.0.13"}

	r, written := endpointReconcilerForTest(t, all, all, all)
	// Published list is missing a member — a genuine correction is required.
	r.readEndpointList = func(_ context.Context) (string, bool, error) {
		return `["https://10.10.0.11:2379"]`, true, nil
	}

	r.reconcileOnce(context.Background())

	if _, ok := (*written)[etcdEndpointListKey]; !ok {
		t.Fatal("expected a write when the published endpoint list is missing members")
	}
}

// TestPublishedEndpointsMatchIsSetwise pins the comparison semantics directly.
func TestPublishedEndpointsMatchIsSetwise(t *testing.T) {
	desired := []string{"https://10.0.0.1:2379", "https://10.0.0.2:2379"}

	if !publishedEndpointsMatch(`["https://10.0.0.2:2379","https://10.0.0.1:2379"]`, desired) {
		t.Error("reordered but identical set must match")
	}
	if publishedEndpointsMatch(`["https://10.0.0.1:2379"]`, desired) {
		t.Error("a missing endpoint must not match")
	}
	if publishedEndpointsMatch(`["https://10.0.0.1:2379","https://10.0.0.2:2379","https://10.0.0.3:2379"]`, desired) {
		t.Error("an extra endpoint must not match")
	}
	if publishedEndpointsMatch(`not json`, desired) {
		t.Error("unparseable published value must not be treated as matching")
	}
}
