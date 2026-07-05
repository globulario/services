package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// TestComputeDesiredEndpoints verifies that endpoint URLs are constructed
// correctly and returned in sorted order regardless of input order.
func TestComputeDesiredEndpoints(t *testing.T) {
	got := computeDesiredEndpoints([]string{"10.0.0.63", "10.0.0.8", "10.0.0.20"})
	want := []string{
		"https://10.0.0.20:2379",
		"https://10.0.0.63:2379",
		"https://10.0.0.8:2379",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d endpoints, want %d: %v", len(got), len(want), got)
	}
	for i, ep := range got {
		if ep != want[i] {
			t.Errorf("[%d] got %q, want %q", i, ep, want[i])
		}
	}
}

// TestDetectStaleEtcdMembers_OneStale verifies that a member whose peer URL
// does not match any core-node IP is surfaced as stale.
func TestDetectStaleEtcdMembers_OneStale(t *testing.T) {
	members := []memberSnapshot{
		{Name: "ryzen", PeerURLs: []string{"https://10.0.0.63:2380"}},
		{Name: "nuc", PeerURLs: []string{"https://10.0.0.8:2380"}},
		{Name: "departed", PeerURLs: []string{"https://10.0.0.99:2380"}},
	}
	coreIPs := []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}
	stale := detectStaleEtcdMembers(members, coreIPs)
	if len(stale) != 1 || stale[0] != "departed" {
		t.Errorf("stale = %v, want [departed]", stale)
	}
}

// TestDetectStaleEtcdMembers_NoneStale verifies that a fully matched
// membership produces an empty stale list.
func TestDetectStaleEtcdMembers_NoneStale(t *testing.T) {
	members := []memberSnapshot{
		{Name: "ryzen", PeerURLs: []string{"https://10.0.0.63:2380"}},
		{Name: "nuc", PeerURLs: []string{"https://10.0.0.8:2380"}},
		{Name: "dell", PeerURLs: []string{"https://10.0.0.20:2380"}},
	}
	coreIPs := []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}
	if got := detectStaleEtcdMembers(members, coreIPs); len(got) != 0 {
		t.Errorf("expected no stale members, got %v", got)
	}
}

// TestDetectStaleEtcdMembers_UnnamedFallback verifies that an unnamed stale
// member is labelled "<unnamed>" rather than producing an empty string.
func TestDetectStaleEtcdMembers_UnnamedFallback(t *testing.T) {
	members := []memberSnapshot{
		{Name: "", PeerURLs: []string{"https://10.0.0.99:2380"}},
	}
	stale := detectStaleEtcdMembers(members, []string{"10.0.0.63"})
	if len(stale) != 1 || stale[0] != "<unnamed>" {
		t.Errorf("stale = %v, want [<unnamed>]", stale)
	}
}

// TestDetectEtcdEndpointDrift_NoDrift verifies that a live membership whose
// client URLs exactly match the desired set reports no drift.
func TestDetectEtcdEndpointDrift_NoDrift(t *testing.T) {
	members := []memberSnapshot{
		{ClientURLs: []string{"https://10.0.0.63:2379"}},
		{ClientURLs: []string{"https://10.0.0.8:2379"}},
		{ClientURLs: []string{"https://10.0.0.20:2379"}},
	}
	desired := []string{
		"https://10.0.0.20:2379",
		"https://10.0.0.63:2379",
		"https://10.0.0.8:2379",
	}
	if detectEtcdEndpointDrift(members, desired) {
		t.Error("no drift expected but detectEtcdEndpointDrift returned true")
	}
}

// TestDetectEtcdEndpointDrift_MissingMember verifies that a live membership
// that is missing one of the desired nodes is reported as drifted.
func TestDetectEtcdEndpointDrift_MissingMember(t *testing.T) {
	members := []memberSnapshot{
		{ClientURLs: []string{"https://10.0.0.63:2379"}},
		{ClientURLs: []string{"https://10.0.0.8:2379"}},
		// 10.0.0.20 missing from live cluster
	}
	desired := []string{
		"https://10.0.0.20:2379",
		"https://10.0.0.63:2379",
		"https://10.0.0.8:2379",
	}
	if !detectEtcdEndpointDrift(members, desired) {
		t.Error("drift expected but detectEtcdEndpointDrift returned false")
	}
}

// TestDetectEtcdEndpointDrift_ExtraMember verifies that a live membership
// containing a node not in the desired set is reported as drifted.
func TestDetectEtcdEndpointDrift_ExtraMember(t *testing.T) {
	members := []memberSnapshot{
		{ClientURLs: []string{"https://10.0.0.63:2379"}},
		{ClientURLs: []string{"https://10.0.0.8:2379"}},
		{ClientURLs: []string{"https://10.0.0.20:2379"}},
		{ClientURLs: []string{"https://10.0.0.77:2379"}}, // unexpected extra
	}
	desired := []string{
		"https://10.0.0.20:2379",
		"https://10.0.0.63:2379",
		"https://10.0.0.8:2379",
	}
	if !detectEtcdEndpointDrift(members, desired) {
		t.Error("drift expected (extra member) but detectEtcdEndpointDrift returned false")
	}
}

// TestExtractEtcdIPFromURL verifies URL-to-IP extraction for both client and
// peer URL formats.
func TestExtractEtcdIPFromURL(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://10.0.0.8:2379", "10.0.0.8"},
		{"https://10.0.0.20:2380", "10.0.0.20"},
		{"http://192.168.1.1:2379", "192.168.1.1"},
		{"https://10.0.0.63:2379", "10.0.0.63"},
	}
	for _, tc := range cases {
		if got := extractEtcdIPFromURL(tc.url); got != tc.want {
			t.Errorf("extractEtcdIPFromURL(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

// TestEtcdEndpointReconciler_PublishesCompleteSmallCluster verifies the buildout
// relaxation: a genuinely small cluster (2 core nodes total, both ready) publishes
// its COMPLETE endpoint list even though it is below the HA quorum minimum, so the
// joined node has a steady-state endpoint-refresh path after bootstrap.
func TestEtcdEndpointReconciler_PublishesCompleteSmallCluster(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string { return []string{"10.0.0.63", "10.0.0.8"} }
	r.snapshotCoreNodeTotal = func() int { return 2 } // complete: 2 ready == 2 total
	memberListCalled := false
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		memberListCalled = true
		return []memberSnapshot{
			{Name: "ryzen", PeerURLs: []string{"https://10.0.0.63:2380"}, ClientURLs: []string{"https://10.0.0.63:2379"}},
		}, nil
	}
	wrote := false
	r.writeToEtcd = func(_ context.Context, key, _ string) error {
		if key == etcdEndpointListKey {
			wrote = true
		}
		return nil
	}
	r.writeOutcome = func(_ context.Context, _ etcdEndpointReconcileOutcome) error { return nil }

	r.reconcileOnce(context.Background())

	if !memberListCalled {
		t.Fatal("a complete 2-node cluster should proceed past the quorum guard (MemberList called)")
	}
	if !wrote {
		t.Fatal("a complete 2-node cluster should publish its endpoint list")
	}
}

// TestEtcdEndpointReconciler_SkipsTruncatedSubset verifies the truncation guard is
// preserved: a larger cluster (5 core nodes total) that transiently sees only 2
// ready must NOT publish a truncated list that would drop live voters.
func TestEtcdEndpointReconciler_SkipsTruncatedSubset(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string { return []string{"10.0.0.63", "10.0.0.8"} }
	r.snapshotCoreNodeTotal = func() int { return 5 } // incomplete: 2 ready of 5 total
	memberListCalled := false
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		memberListCalled = true
		return nil, nil
	}
	r.writeOutcome = func(_ context.Context, _ etcdEndpointReconcileOutcome) error { return nil }
	r.writeToEtcd = func(_ context.Context, _, _ string) error { return nil }

	r.reconcileOnce(context.Background())

	if memberListCalled {
		t.Error("a 2-of-5 subset must be skipped by the truncation guard (no MemberList)")
	}
}

// TestEtcdEndpointReconciler_QuorumGuard verifies that the reconciler does NOT
// call MemberList or write to etcd when fewer than etcdEndpointQuorumMin core
// nodes are ready — preventing a partial endpoint list from being published.
func TestEtcdEndpointReconciler_QuorumGuard(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	memberListCalled := false
	r.snapshotCoreNodes = func() []string {
		return []string{"10.0.0.8", "10.0.0.20"} // 2 < quorumMin(3)
	}
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		memberListCalled = true
		return nil, nil
	}
	r.writeOutcome = func(_ context.Context, _ etcdEndpointReconcileOutcome) error { return nil }
	r.writeToEtcd = func(_ context.Context, _, _ string) error { return nil }

	r.reconcileOnce(context.Background())

	if memberListCalled {
		t.Error("quorum guard should have prevented MemberList from being called")
	}
}

// TestEtcdEndpointReconciler_NoOpWhenNoDrift verifies that no write occurs
// to etcdEndpointListKey when the live membership matches the desired set.
func TestEtcdEndpointReconciler_NoOpWhenNoDrift(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string {
		return []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}
	}
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		return []memberSnapshot{
			{Name: "ryzen", PeerURLs: []string{"https://10.0.0.63:2380"}, ClientURLs: []string{"https://10.0.0.63:2379"}},
			{Name: "nuc", PeerURLs: []string{"https://10.0.0.8:2380"}, ClientURLs: []string{"https://10.0.0.8:2379"}},
			{Name: "dell", PeerURLs: []string{"https://10.0.0.20:2380"}, ClientURLs: []string{"https://10.0.0.20:2379"}},
		}, nil
	}
	var writtenKey string
	r.writeToEtcd = func(_ context.Context, key, _ string) error {
		writtenKey = key
		return nil
	}
	var capturedOutcome etcdEndpointReconcileOutcome
	r.writeOutcome = func(_ context.Context, out etcdEndpointReconcileOutcome) error {
		capturedOutcome = out
		return nil
	}

	r.reconcileOnce(context.Background())

	if writtenKey == etcdEndpointListKey {
		t.Error("should not write endpoint list when there is no drift")
	}
	if capturedOutcome.Outcome != "ok" {
		t.Errorf("outcome = %q, want \"ok\"", capturedOutcome.Outcome)
	}
	if capturedOutcome.Drift {
		t.Error("outcome.Drift should be false when membership matches")
	}
}

// TestEtcdEndpointReconciler_WriteOnDrift verifies that when drift is detected
// the corrected endpoint list is written to etcdEndpointListKey and the outcome
// record reflects the correction.
func TestEtcdEndpointReconciler_WriteOnDrift(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string {
		return []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}
	}
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		// Only 2 live members — 10.0.0.20 is missing (drift).
		return []memberSnapshot{
			{Name: "ryzen", PeerURLs: []string{"https://10.0.0.63:2380"}, ClientURLs: []string{"https://10.0.0.63:2379"}},
			{Name: "nuc", PeerURLs: []string{"https://10.0.0.8:2380"}, ClientURLs: []string{"https://10.0.0.8:2379"}},
		}, nil
	}
	written := map[string]string{}
	r.writeToEtcd = func(_ context.Context, key, value string) error {
		written[key] = value
		return nil
	}
	var capturedOutcome etcdEndpointReconcileOutcome
	r.writeOutcome = func(_ context.Context, out etcdEndpointReconcileOutcome) error {
		capturedOutcome = out
		return nil
	}

	r.reconcileOnce(context.Background())

	if _, ok := written[etcdEndpointListKey]; !ok {
		t.Fatal("expected write to etcdEndpointListKey on drift, but no write occurred")
	}
	if capturedOutcome.Outcome != "drift_corrected" {
		t.Errorf("outcome = %q, want \"drift_corrected\"", capturedOutcome.Outcome)
	}
	if !capturedOutcome.Drift {
		t.Error("outcome.Drift should be true when endpoints were corrected")
	}
	if len(capturedOutcome.DesiredEndpoints) != 3 {
		t.Errorf("desired_endpoints count = %d, want 3", len(capturedOutcome.DesiredEndpoints))
	}
	if payload, ok := written[etcdEndpointListKey]; ok {
		var arr []string
		if err := json.Unmarshal([]byte(payload), &arr); err != nil {
			t.Fatalf("endpoint payload is not JSON array: %v payload=%q", err, payload)
		}
		if len(arr) != 3 {
			t.Fatalf("endpoint payload length=%d want 3", len(arr))
		}
	}
}

// TestEtcdEndpointReconciler_StaleLoggedButNotRemoved verifies that stale
// members appear in the outcome record but no removal is triggered.
func TestEtcdEndpointReconciler_StaleLoggedButNotRemoved(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string {
		return []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}
	}
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		return []memberSnapshot{
			{Name: "ryzen", PeerURLs: []string{"https://10.0.0.63:2380"}, ClientURLs: []string{"https://10.0.0.63:2379"}},
			{Name: "nuc", PeerURLs: []string{"https://10.0.0.8:2380"}, ClientURLs: []string{"https://10.0.0.8:2379"}},
			{Name: "dell", PeerURLs: []string{"https://10.0.0.20:2380"}, ClientURLs: []string{"https://10.0.0.20:2379"}},
			// stale: was removed from cluster, still in member list.
			{Name: "old-node", PeerURLs: []string{"https://10.0.0.77:2380"}, ClientURLs: []string{"https://10.0.0.77:2379"}},
		}, nil
	}
	written := map[string]string{}
	r.writeToEtcd = func(_ context.Context, key, value string) error {
		written[key] = value
		return nil
	}
	var capturedOutcome etcdEndpointReconcileOutcome
	r.writeOutcome = func(_ context.Context, out etcdEndpointReconcileOutcome) error {
		capturedOutcome = out
		return nil
	}

	r.reconcileOnce(context.Background())

	// The extra member causes drift (live has 4, desired has 3).
	if capturedOutcome.Outcome != "drift_corrected" {
		t.Errorf("outcome = %q, want \"drift_corrected\"", capturedOutcome.Outcome)
	}
	if len(capturedOutcome.StaleMembers) == 0 {
		t.Error("expected stale member \"old-node\" in outcome, got none")
	}
	if capturedOutcome.StaleMembers[0] != "old-node" {
		t.Errorf("stale member = %q, want \"old-node\"", capturedOutcome.StaleMembers[0])
	}
}

// TestEtcdEndpointReconciler_OutcomeJSON verifies that the outcome record
// serialises cleanly to JSON (no field name typos or marshal panics).
func TestEtcdEndpointReconciler_OutcomeJSON(t *testing.T) {
	out := etcdEndpointReconcileOutcome{
		TimestampUnix:    time.Now().Unix(),
		Outcome:          "drift_corrected",
		DesiredEndpoints: []string{"https://10.0.0.8:2379"},
		LiveMemberCount:  2,
		StaleMembers:     []string{"old-node"},
		Drift:            true,
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var roundtrip etcdEndpointReconcileOutcome
	if err := json.Unmarshal(b, &roundtrip); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if roundtrip.Outcome != out.Outcome {
		t.Errorf("Outcome = %q, want %q", roundtrip.Outcome, out.Outcome)
	}
}

// TestEtcdEndpointReconciler_MemberListError verifies that a MemberList
// failure writes an error outcome and does not panic.
func TestEtcdEndpointReconciler_MemberListError(t *testing.T) {
	r := &etcdEndpointReconciler{
		srv:      newTestServer(t, &controllerState{}),
		interval: etcdEndpointReconcileInterval,
		now:      time.Now,
	}
	r.snapshotCoreNodes = func() []string {
		return []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}
	}
	r.listMembers = func(_ context.Context) ([]memberSnapshot, error) {
		return nil, fmt.Errorf("network timeout")
	}
	r.writeToEtcd = func(_ context.Context, _, _ string) error { return nil }
	var capturedOutcome etcdEndpointReconcileOutcome
	r.writeOutcome = func(_ context.Context, out etcdEndpointReconcileOutcome) error {
		capturedOutcome = out
		return nil
	}

	r.reconcileOnce(context.Background())

	if capturedOutcome.Outcome != "error" {
		t.Errorf("outcome = %q, want \"error\"", capturedOutcome.Outcome)
	}
}
