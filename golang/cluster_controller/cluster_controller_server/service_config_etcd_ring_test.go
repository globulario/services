package main

import (
	"strings"
	"testing"
)

// These tests pin meta.storage_is_not_semantic_authority for etcd ring
// membership: etcd's own member list — not the node registry — is the
// authority on who is in the ring, and initial-cluster is validated against
// it by count.
//
// Observed 2026-08-18 on the 5-node simulation: the registry listed five etcd
// nodes, the live ring held four, and renderEtcdConfig emitted a five-member
// initial-cluster. node-5's etcd exited on every boot, and node-4 — which had
// never initialized, so it had no WAL to fall back on — restart-looped 174
// times against a three-member file with
//
//	error validating peerURLs ...: member count is unequal
//
// The ring never regained its fifth member, silently cutting the cluster's
// fault tolerance and failing every catastrophic-suite scenario whose quorum
// arithmetic assumed five.

func ringCtx(ringPeerURLs []string, nodes []memberNode, current memberNode) *serviceConfigContext {
	return &serviceConfigContext{
		Membership:  &clusterMembership{ClusterID: "test-cluster", Nodes: nodes},
		CurrentNode: &current,
		ClusterID:   "test-cluster",
		EtcdState: &etcdMemberState{
			Bootstrapped: len(ringPeerURLs) > 0,
			RingPeerURLs: ringPeerURLs,
		},
	}
}

// TestRenderEtcdConfig_InitialClusterCountMatchesRingNotRegistry is the direct
// regression: the registry knows five etcd nodes, the ring holds four, and the
// rendered initial-cluster must have four entries. A five-entry render is the
// exact config etcd rejects.
func TestRenderEtcdConfig_InitialClusterCountMatchesRingNotRegistry(t *testing.T) {
	nodes := []memberNode{
		{NodeID: "n1", Hostname: "node-1", IP: "10.10.0.11", Profiles: []string{"core"}},
		{NodeID: "n2", Hostname: "node-2", IP: "10.10.0.12", Profiles: []string{"core"}},
		{NodeID: "n3", Hostname: "node-3", IP: "10.10.0.13", Profiles: []string{"core"}},
		{NodeID: "n4", Hostname: "node-4", IP: "10.10.0.14", Profiles: []string{"core"}},
		{NodeID: "n5", Hostname: "node-5", IP: "10.10.0.15", Profiles: []string{"core"}},
	}
	// node-5 has never been MemberAdd'ed — it is registered, not a member.
	ring := []string{
		"https://10.10.0.11:2380",
		"https://10.10.0.12:2380",
		"https://10.10.0.13:2380",
		"https://10.10.0.14:2380",
	}

	config, ok := renderEtcdConfig(ringCtx(ring, nodes, nodes[3]))
	if !ok {
		t.Fatal("renderEtcdConfig() returned false")
	}

	got := initialClusterEntries(t, config)
	if len(got) != len(ring) {
		t.Fatalf("initial-cluster has %d entries, want %d (the live ring)\ngot: %v",
			len(got), len(ring), got)
	}
	for _, entry := range got {
		if strings.Contains(entry, "10.10.0.15") {
			t.Errorf("initial-cluster contains node-5 (%q), which is registered but not an etcd member", entry)
		}
	}
}

// TestRenderEtcdConfig_UnstartedMemberIsCounted covers the member that has been
// added but has never booted. etcd reports it with an empty Name; it still
// counts toward membership, so it must appear in initial-cluster.
func TestRenderEtcdConfig_UnstartedMemberIsCounted(t *testing.T) {
	nodes := []memberNode{
		{NodeID: "n1", Hostname: "node-1", IP: "10.10.0.11", Profiles: []string{"core"}},
		{NodeID: "n2", Hostname: "node-2", IP: "10.10.0.12", Profiles: []string{"core"}},
		{NodeID: "n4", Hostname: "node-4", IP: "10.10.0.14", Profiles: []string{"core"}},
	}
	// node-4 is in the ring but unstarted — no name from etcd, name from registry.
	ring := []string{
		"https://10.10.0.11:2380",
		"https://10.10.0.12:2380",
		"https://10.10.0.14:2380",
	}

	config, ok := renderEtcdConfig(ringCtx(ring, nodes, nodes[2]))
	if !ok {
		t.Fatal("renderEtcdConfig() returned false")
	}

	got := initialClusterEntries(t, config)
	if len(got) != 3 {
		t.Fatalf("initial-cluster has %d entries, want 3\ngot: %v", len(got), got)
	}
	if !strings.Contains(strings.Join(got, ","), "node-4=https://10.10.0.14:2380") {
		t.Errorf("unstarted member node-4 missing or misnamed in initial-cluster: %v", got)
	}
}

// TestRenderEtcdConfig_RingMemberUnknownToRegistryIsKept pins the fallback.
// Dropping a member etcd vouches for would break the count and wedge every
// subsequent join, so an unrecognised member is kept under a derived name.
func TestRenderEtcdConfig_RingMemberUnknownToRegistryIsKept(t *testing.T) {
	nodes := []memberNode{
		{NodeID: "n1", Hostname: "node-1", IP: "10.10.0.11", Profiles: []string{"core"}},
		{NodeID: "n2", Hostname: "node-2", IP: "10.10.0.12", Profiles: []string{"core"}},
	}
	ring := []string{
		"https://10.10.0.11:2380",
		"https://10.10.0.12:2380",
		"https://10.10.0.99:2380", // in the ring, absent from the registry
	}

	config, ok := renderEtcdConfig(ringCtx(ring, nodes, nodes[1]))
	if !ok {
		t.Fatal("renderEtcdConfig() returned false")
	}

	got := initialClusterEntries(t, config)
	if len(got) != 3 {
		t.Fatalf("initial-cluster has %d entries, want 3 — a ring member was dropped\ngot: %v", len(got), got)
	}
	if !strings.Contains(strings.Join(got, ","), "10.10.0.99:2380") {
		t.Errorf("registry-unknown ring member was dropped from initial-cluster: %v", got)
	}
}

// TestRenderEtcdConfig_FreshBootstrapStillUsesRegistry confirms the fix does not
// break Day-0: with no ring in existence, desired placement is the only
// membership there is, and the state must stay "new".
func TestRenderEtcdConfig_FreshBootstrapStillUsesRegistry(t *testing.T) {
	nodes := []memberNode{
		{NodeID: "n1", Hostname: "node-1", IP: "10.10.0.11", Profiles: []string{"core"}},
	}
	ctx := &serviceConfigContext{
		Membership:  &clusterMembership{ClusterID: "test-cluster", Nodes: nodes},
		CurrentNode: &nodes[0],
		ClusterID:   "test-cluster",
		EtcdState:   nil, // no etcd anywhere yet
	}

	config, ok := renderEtcdConfig(ctx)
	if !ok {
		t.Fatal("renderEtcdConfig() returned false")
	}
	if !strings.Contains(config, `initial-cluster-state: "new"`) {
		t.Error(`fresh bootstrap must render initial-cluster-state: "new"`)
	}
	got := initialClusterEntries(t, config)
	if len(got) != 1 || !strings.Contains(got[0], "10.10.0.11:2380") {
		t.Errorf("fresh bootstrap initial-cluster = %v, want the single registry node", got)
	}
}

// TestRenderEtcdConfig_NonMemberNodeIsNotRendered pins the join-window rule: a
// node the ring does not contain gets no etcd.yaml from the controller at all.
// Rendering one would either omit the node's own entry (etcd: "couldn't find
// local name in initial cluster") or include it and break the count — and it
// would clobber the join script's file, which was written from the
// authoritative MemberAdd output.
func TestRenderEtcdConfig_NonMemberNodeIsNotRendered(t *testing.T) {
	nodes := []memberNode{
		{NodeID: "n1", Hostname: "node-1", IP: "10.10.0.11", Profiles: []string{"core"}},
		{NodeID: "n5", Hostname: "node-5", IP: "10.10.0.15", Profiles: []string{"core"}},
	}
	ring := []string{"https://10.10.0.11:2380"} // node-5 has not been MemberAdd'ed

	if config, ok := renderEtcdConfig(ringCtx(ring, nodes, nodes[1])); ok {
		t.Errorf("renderEtcdConfig() rendered a config for a node that is not an etcd member:\n%s", config)
	}

	// The node that IS in the ring still renders normally.
	if _, ok := renderEtcdConfig(ringCtx(ring, nodes, nodes[0])); !ok {
		t.Error("renderEtcdConfig() refused to render for a node that IS in the ring")
	}
}

// TestRenderEtcdConfig_EtcdMemberNameOutranksRegistryHostname pins the naming
// rule. The founding member is named "globular-etcd" in the ring while the
// registry calls that node "node-1"; renaming a started member in
// initial-cluster would make the rendered config describe a cluster that does
// not exist. etcd's name wins wherever etcd has one.
func TestRenderEtcdConfig_EtcdMemberNameOutranksRegistryHostname(t *testing.T) {
	nodes := []memberNode{
		{NodeID: "n1", Hostname: "node-1", IP: "10.10.0.11", Profiles: []string{"core"}},
		{NodeID: "n2", Hostname: "node-2", IP: "10.10.0.12", Profiles: []string{"core"}},
	}
	ctx := ringCtx([]string{
		"https://10.10.0.11:2380",
		"https://10.10.0.12:2380",
	}, nodes, nodes[1])
	// etcd knows .11 as "globular-etcd", not "node-1".
	ctx.EtcdState.MemberPeerURLs = map[string]string{
		"globular-etcd": "https://10.10.0.11:2380",
		"node-2":        "https://10.10.0.12:2380",
	}

	config, ok := renderEtcdConfig(ctx)
	if !ok {
		t.Fatal("renderEtcdConfig() returned false")
	}
	joined := strings.Join(initialClusterEntries(t, config), ",")
	if !strings.Contains(joined, "globular-etcd=https://10.10.0.11:2380") {
		t.Errorf("initial-cluster renamed the founding member away from etcd's own name: %s", joined)
	}
	if strings.Contains(joined, "node-1=") {
		t.Errorf("initial-cluster used the registry hostname over etcd's member name: %s", joined)
	}
}

func TestPeerHostFromURL(t *testing.T) {
	cases := map[string]string{
		"https://10.10.0.14:2380": "10.10.0.14",
		"http://10.0.0.1:2380":    "10.0.0.1",
		"10.10.0.14:2380":         "10.10.0.14",
	}
	for in, want := range cases {
		if got := peerHostFromURL(in); got != want {
			t.Errorf("peerHostFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// initialClusterEntries pulls the comma-separated initial-cluster value out of
// a rendered etcd.yaml.
func initialClusterEntries(t *testing.T, config string) []string {
	t.Helper()
	for _, line := range strings.Split(config, "\n") {
		if !strings.HasPrefix(line, "initial-cluster:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "initial-cluster:"))
		value = strings.Trim(value, `"`)
		if value == "" {
			return nil
		}
		return strings.Split(value, ",")
	}
	t.Fatalf("rendered config has no initial-cluster line:\n%s", config)
	return nil
}
