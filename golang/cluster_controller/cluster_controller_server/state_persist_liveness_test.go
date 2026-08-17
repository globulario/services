package main

// state_persist_liveness_test.go
//
// Replaces state_persist_dedup_test.go and state_persist_observation_test.go.
//
// Those files pinned the WRONG property. They asserted that saveToEtcd SKIPS
// writes — several tests, all green, all confirming the optimization worked.
// Not one of them asserted that anything still functioned afterwards. The dedup
// they protected shipped as cluster-controller@1.2.317 on 2026-08-16 and took
// the cluster down: last_seen was excluded from the write-trigger hash and
// bounded by a 5-minute floor, so the persisted blob went up to 300s stale and
// readers judging node liveness from it saw the cluster as unreachable while
// every node-agent was healthy and serving.
//
// The lesson those tests encode by counter-example: a test that asserts a write
// was suppressed proves the suppression works, never that suppression was safe.
// The property worth pinning is the one the system depends on — liveness
// observations reach etcd — so that is what these tests assert.
//
// Covered invariant: controller.persisted_liveness_must_track_heartbeat.
// Failure mode closed: controller.state_dedup_suppressed_liveness_field.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type putRecorder struct {
	calls  int
	keys   []string
	values []string
	err    error
}

func withStubPut(t *testing.T) *putRecorder {
	t.Helper()
	rec := &putRecorder{}
	prev := saveToEtcdPutFunc
	saveToEtcdPutFunc = func(ctx context.Context, cli *clientv3.Client, key, value string) error {
		rec.calls++
		rec.keys = append(rec.keys, key)
		rec.values = append(rec.values, value)
		return rec.err
	}
	t.Cleanup(func() { saveToEtcdPutFunc = prev })
	return rec
}

// fakeEtcdClient is a non-nil sentinel: saveToEtcd's first guard is
// `if cli == nil`. The stub Put never dereferences it.
func fakeEtcdClient() *clientv3.Client { return &clientv3.Client{} }

func freshState() *controllerState {
	s := newControllerState()
	s.ClusterId = "test-cluster"
	s.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	return s
}

// persistedLastSeen pulls a node's last_seen out of the value actually handed
// to etcd. Reading the serialized payload rather than the in-memory struct is
// the whole point: the regression was that memory was fresh and etcd was stale,
// and only the bytes that reached etcd can tell those two apart.
func persistedLastSeen(t *testing.T, value, node string) time.Time {
	t.Helper()
	var doc struct {
		Nodes map[string]struct {
			LastSeen time.Time `json:"last_seen"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(value), &doc); err != nil {
		t.Fatalf("unmarshal persisted state: %v", err)
	}
	n, ok := doc.Nodes[node]
	if !ok {
		t.Fatalf("node %q absent from persisted state", node)
	}
	return n.LastSeen
}

// A heartbeating node's last_seen must reach etcd on EVERY heartbeat.
//
// This is the test that would have caught 1.2.317. Under the shipped dedup it
// fails on the second heartbeat: the write is suppressed, so the value in etcd
// still carries the first heartbeat's timestamp.
func TestSaveToEtcd_HeartbeatReachesEtcdEveryCycle(t *testing.T) {
	rec := withStubPut(t)
	s := freshState()
	s.Nodes["node-1"] = &nodeState{}

	base := time.Unix(1_700_000_000, 0).UTC()
	const heartbeats = 6
	for i := 0; i < heartbeats; i++ {
		// Only liveness moves — no semantic state change whatsoever. This is
		// precisely the case the dedup classified as "nothing happened".
		s.Nodes["node-1"].LastSeen = base.Add(time.Duration(i) * 30 * time.Second)
		if err := s.saveToEtcd(fakeEtcdClient()); err != nil {
			t.Fatalf("heartbeat %d: %v", i, err)
		}
	}

	if rec.calls != heartbeats {
		t.Fatalf("etcd writes = %d, want %d — a heartbeat that does not reach "+
			"etcd makes every liveness reader see a stale node; this is the "+
			"1.2.317 outage", rec.calls, heartbeats)
	}

	// The last write must carry the last heartbeat, not an older sample.
	want := base.Add((heartbeats - 1) * 30 * time.Second)
	if got := persistedLastSeen(t, rec.values[len(rec.values)-1], "node-1"); !got.Equal(want) {
		t.Errorf("persisted last_seen = %s, want %s", got, want)
	}
}

// The staleness bound, stated directly: after any heartbeat, what etcd holds
// must match what the controller observed. No floor, no grace window.
func TestSaveToEtcd_PersistedLivenessIsNeverStale(t *testing.T) {
	rec := withStubPut(t)
	s := freshState()
	s.Nodes["node-1"] = &nodeState{}

	base := time.Unix(1_700_000_000, 0).UTC()
	for i := 0; i < 4; i++ {
		observed := base.Add(time.Duration(i) * time.Minute)
		s.Nodes["node-1"].LastSeen = observed
		if err := s.saveToEtcd(fakeEtcdClient()); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
		got := persistedLastSeen(t, rec.values[len(rec.values)-1], "node-1")
		if staleBy := observed.Sub(got); staleBy != 0 {
			t.Fatalf("after heartbeat %d the persisted last_seen lags the observed "+
				"one by %s — liveness in etcd must equal liveness in memory", i, staleBy)
		}
	}
}

// A Put failure must not be recorded as success in any form: the next call
// still has to attempt the write. Under the old code a success-only cache made
// this subtle; unconditional writes make it structural, and this pins it.
func TestSaveToEtcd_RetriesAfterPutFailure(t *testing.T) {
	rec := withStubPut(t)
	s := freshState()
	s.Nodes["node-1"] = &nodeState{LastSeen: time.Unix(1_700_000_000, 0).UTC()}

	rec.err = context.DeadlineExceeded
	if err := s.saveToEtcd(fakeEtcdClient()); err == nil {
		t.Fatal("expected the Put error to propagate")
	}
	rec.err = nil
	if err := s.saveToEtcd(fakeEtcdClient()); err != nil {
		t.Fatalf("retry after failure: %v", err)
	}
	if rec.calls != 2 {
		t.Errorf("calls = %d, want 2 — a failed persist must be retried, not "+
			"treated as already-written", rec.calls)
	}
}

// Guard against the regression returning in a new costume. Any future
// rate-limit, floor, or content-hash dedup over this record is only admissible
// once last_seen no longer lives in it. If someone reintroduces suppression the
// heartbeat tests above fail — this one names why, so the next reader does not
// have to re-derive the outage from a diff.
func TestSaveToEtcd_NoSuppressionWhileLivenessLivesInThisRecord(t *testing.T) {
	rec := withStubPut(t)
	s := freshState()
	s.Nodes["node-1"] = &nodeState{}

	// Byte-identical state, twice. Even here the write must go through: the
	// only safe dedup is one that cannot possibly hide a liveness update, and
	// while last_seen is a field of this blob no such dedup exists.
	s.Nodes["node-1"].LastSeen = time.Unix(1_700_000_000, 0).UTC()
	for i := 0; i < 2; i++ {
		if err := s.saveToEtcd(fakeEtcdClient()); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if rec.calls != 2 {
		t.Errorf("calls = %d, want 2 — suppression reintroduced. Split Layer-4 "+
			"heartbeat into a per-node key first (infra.heartbeat_not_desired_"+
			"authority); only then may this record be deduped", rec.calls)
	}
}
