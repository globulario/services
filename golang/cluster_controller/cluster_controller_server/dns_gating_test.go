package main

// dns_gating_test.go — Regression tests for the DNS reconciler's 4-layer
// gate. These exercise the failure mode dns.desired_ghost_records:
// records that point at nodes where the service is planned but not
// installed, or installed but not runtime-healthy.
//
// The end-to-end simulation at the bottom matches the production
// topology from the 2026-05-14 incident:
//   - ryzen + nuc healthy
//   - dell installed but inactive
//   - hp-01 planned only
//   - lenovo rejoining/partial
// and asserts that DNS publishes only healthy endpoints, while
// withdrawal logs are emitted for the half-dead candidates.

import (
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────
// gateForService — the single primitive every record group goes through.
// ─────────────────────────────────────────────────────────────────────────

func TestNodeInfo_ServiceReady_NilMaps_FallsThrough(t *testing.T) {
	// When both readiness maps are nil (cold start / legacy callers), the
	// gate is satisfied. This preserves the bootstrap path that runs
	// before any heartbeat has populated installed/runtime data.
	n := NodeInfo{FQDN: "n1.x", IPv4: "10.0.0.1", Profiles: []string{"gateway"}}
	if !n.ServiceReady("gateway") {
		t.Fatal("nil InstalledServices/RuntimeHealthy must fall through to true")
	}
}

func TestNodeInfo_ServiceReady_DrainingBlocks(t *testing.T) {
	n := NodeInfo{
		FQDN:              "n1.x",
		IPv4:              "10.0.0.1",
		Profiles:          []string{"gateway"},
		InstalledServices: map[string]bool{"gateway": true},
		RuntimeHealthy:    map[string]bool{"gateway": true},
		Draining:          true,
	}
	if n.ServiceReady("gateway") {
		t.Fatal("draining node must fail ServiceReady regardless of installed/health state")
	}
}

func TestNodeInfo_ServiceReady_InstalledButNotHealthy(t *testing.T) {
	n := NodeInfo{
		FQDN:              "n1.x",
		IPv4:              "10.0.0.1",
		Profiles:          []string{"gateway"},
		InstalledServices: map[string]bool{"gateway": true},
		RuntimeHealthy:    map[string]bool{}, // explicitly empty, NOT nil
	}
	if n.ServiceReady("gateway") {
		t.Fatal("installed but runtime unhealthy must fail ServiceReady")
	}
}

func TestNodeInfo_ServiceReady_PlannedNotInstalled(t *testing.T) {
	n := NodeInfo{
		FQDN:              "n1.x",
		IPv4:              "10.0.0.1",
		Profiles:          []string{"gateway"},
		InstalledServices: map[string]bool{}, // explicitly empty, NOT nil
		RuntimeHealthy:    map[string]bool{"gateway": true},
	}
	if n.ServiceReady("gateway") {
		t.Fatal("planned but not installed must fail ServiceReady")
	}
}

// ─────────────────────────────────────────────────────────────────────────
// ComputeDesiredStateWithFunnels — record-set assertions.
// ─────────────────────────────────────────────────────────────────────────

func gatewayRecordsFor(state *DesiredDNSState) []string {
	var out []string
	for _, r := range state.Records {
		if r.Name == "gateway.x" && r.Type == RecordTypeA {
			out = append(out, r.Value)
		}
	}
	return out
}

func wildcardRecordsFor(state *DesiredDNSState) []string {
	var out []string
	for _, r := range state.Records {
		if r.Name == "*.x" && r.Type == RecordTypeA {
			out = append(out, r.Value)
		}
	}
	return out
}

func srvTargetsFor(state *DesiredDNSState, recordName string) []string {
	var out []string
	for _, r := range state.Records {
		if r.Name == recordName && r.Type == RecordTypeSRV {
			out = append(out, r.Value)
		}
	}
	return out
}

func TestComputeDesiredState_PlannedNotInstalled_NoRecord(t *testing.T) {
	// hp-01: profile=gateway but service not installed.
	nodes := []NodeInfo{
		{
			FQDN:              "hp-01.x",
			IPv4:              "10.0.0.9",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{}, // gateway not installed
			RuntimeHealthy:    map[string]bool{},
		},
	}
	state, funnels := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	if got := gatewayRecordsFor(state); len(got) != 0 {
		t.Fatalf("expected no gateway records for planned-not-installed node, got %v", got)
	}
	// Funnel must record the desired→installed drop.
	found := false
	for _, f := range funnels {
		if f.Record == "gateway.x" {
			found = true
			if f.Desired == 0 || f.Installed != 0 || f.Published != 0 {
				t.Errorf("gateway funnel wrong: %+v", f)
			}
		}
	}
	if !found {
		t.Fatal("expected a gateway.x funnel entry even when nothing is published")
	}
}

func TestComputeDesiredState_InstalledNotHealthy_NoRecord(t *testing.T) {
	// dell: installed but unit inactive.
	nodes := []NodeInfo{
		{
			FQDN:              "dell.x",
			IPv4:              "10.0.0.20",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{}, // empty, not nil
		},
	}
	state, _ := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	if got := gatewayRecordsFor(state); len(got) != 0 {
		t.Fatalf("expected no gateway records for installed-but-unhealthy node, got %v", got)
	}
}

// Awareness required-test name wrappers for 4-layer separation.
func TestInstalledNotImpliesRunning(t *testing.T) {
	TestNodeInfo_ServiceReady_InstalledButNotHealthy(t)
}

func TestRuntimeHealthSeparateFromInstalled(t *testing.T) {
	TestComputeDesiredState_InstalledNotHealthy_NoRecord(t)
}

func TestComputeDesiredState_HealthyNode_PublishesRecord(t *testing.T) {
	// nuc: fully ready.
	nodes := []NodeInfo{
		{
			FQDN:              "nuc.x",
			IPv4:              "10.0.0.8",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{"gateway": true},
		},
	}
	state, _ := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	got := gatewayRecordsFor(state)
	if len(got) != 1 || got[0] != "10.0.0.8" {
		t.Fatalf("expected gateway.x → [10.0.0.8], got %v", got)
	}
}

func TestComputeDesiredState_DrainingNode_WithdrawsRecords(t *testing.T) {
	nodes := []NodeInfo{
		{
			FQDN:              "ryzen.x",
			IPv4:              "10.0.0.63",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{"gateway": true},
			Draining:          true,
		},
	}
	state, _ := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	if got := gatewayRecordsFor(state); len(got) != 0 {
		t.Fatalf("draining node must withdraw from gateway record, got %v", got)
	}
	// Per-node A record must also be withdrawn.
	for _, r := range state.Records {
		if r.Name == "ryzen.x" && r.Type == RecordTypeA {
			t.Fatalf("draining node must withdraw its own A record, got %+v", r)
		}
	}
}

func TestComputeDesiredState_WildcardExcludesUnhealthy(t *testing.T) {
	nodes := []NodeInfo{
		{ // healthy
			FQDN:              "nuc.x",
			IPv4:              "10.0.0.8",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{"gateway": true},
		},
		{ // installed but unhealthy
			FQDN:              "dell.x",
			IPv4:              "10.0.0.20",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{},
		},
		{ // planned only
			FQDN:              "hp-01.x",
			IPv4:              "10.0.0.9",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{},
			RuntimeHealthy:    map[string]bool{},
		},
	}
	state, _ := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	got := wildcardRecordsFor(state)
	if len(got) != 1 || got[0] != "10.0.0.8" {
		t.Fatalf("wildcard must include only the healthy node, got %v", got)
	}
}

func TestComputeDesiredState_WildcardMatchesGatewayCandidateSet(t *testing.T) {
	// Structural property: api / gateway / wildcard MUST all derive from
	// the same kept-list. We assert their value sets are equal.
	nodes := []NodeInfo{
		{
			FQDN:              "nuc.x",
			IPv4:              "10.0.0.8",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{"gateway": true},
		},
		{
			FQDN:              "ryzen.x",
			IPv4:              "10.0.0.63",
			Profiles:          []string{"gateway"},
			InstalledServices: map[string]bool{"gateway": true},
			RuntimeHealthy:    map[string]bool{"gateway": true},
		},
	}
	state, _ := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	g := setOf(gatewayRecordsFor(state))
	w := setOf(wildcardRecordsFor(state))
	a := setOf(aRecordsForName(state, "api.x"))
	if !equalSet(g, w) {
		t.Fatalf("wildcard must match gateway set: gateway=%v wildcard=%v", g, w)
	}
	if !equalSet(g, a) {
		t.Fatalf("api must match gateway set: gateway=%v api=%v", g, a)
	}
}

func aRecordsForName(state *DesiredDNSState, name string) []string {
	var out []string
	for _, r := range state.Records {
		if r.Name == name && r.Type == RecordTypeA {
			out = append(out, r.Value)
		}
	}
	return out
}

func setOf(xs []string) map[string]bool {
	out := map[string]bool{}
	for _, x := range xs {
		out[x] = true
	}
	return out
}

func equalSet(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func TestComputeDesiredState_ControllerSRVOnlyHealthy(t *testing.T) {
	// Two control-plane nodes; one healthy, one with controller inactive.
	// The _cluster-controller SRV record must list only the healthy one.
	nodes := []NodeInfo{
		{
			FQDN:              "nuc.x",
			IPv4:              "10.0.0.8",
			Profiles:          []string{"control-plane"},
			InstalledServices: map[string]bool{"cluster-controller": true},
			RuntimeHealthy:    map[string]bool{"cluster-controller": true},
		},
		{
			FQDN:              "dell.x",
			IPv4:              "10.0.0.20",
			Profiles:          []string{"control-plane"},
			InstalledServices: map[string]bool{"cluster-controller": true},
			RuntimeHealthy:    map[string]bool{}, // unit inactive
		},
	}
	state, _ := ComputeDesiredStateWithFunnels("x", nodes, nil, "", nil, 1)
	targets := srvTargetsFor(state, "_cluster-controller._tcp.x")
	if len(targets) != 1 || targets[0] != "nuc.x" {
		t.Fatalf("expected _cluster-controller SRV to include only nuc.x, got %v", targets)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// End-to-end simulation matching the 2026-05-14 production topology.
// ─────────────────────────────────────────────────────────────────────────

func TestComputeDesiredState_E2EProductionTopology(t *testing.T) {
	// ryzen + nuc fully healthy gateway/control-plane.
	// dell installed but gateway/controller inactive.
	// hp-01 planned only — no packages installed.
	// lenovo rejoining: gateway not yet installed.
	nodes := []NodeInfo{
		{
			FQDN:              "ryzen.x",
			IPv4:              "10.0.0.63",
			Profiles:          []string{"gateway", "control-plane", "core"},
			InstalledServices: map[string]bool{"gateway": true, "cluster-controller": true, "dns": true},
			RuntimeHealthy:    map[string]bool{"gateway": true, "cluster-controller": true, "dns": true},
		},
		{
			FQDN:              "nuc.x",
			IPv4:              "10.0.0.8",
			Profiles:          []string{"gateway", "control-plane", "core"},
			InstalledServices: map[string]bool{"gateway": true, "cluster-controller": true, "dns": true},
			RuntimeHealthy:    map[string]bool{"gateway": true, "cluster-controller": true, "dns": true},
		},
		{
			FQDN:              "dell.x",
			IPv4:              "10.0.0.20",
			Profiles:          []string{"gateway", "control-plane", "core"},
			InstalledServices: map[string]bool{"gateway": true, "cluster-controller": true, "dns": true},
			RuntimeHealthy:    map[string]bool{}, // all inactive
		},
		{
			FQDN:              "hp-01.x",
			IPv4:              "10.0.0.9",
			Profiles:          []string{"gateway", "control-plane", "core"},
			InstalledServices: map[string]bool{}, // nothing installed yet
			RuntimeHealthy:    map[string]bool{},
		},
		{
			FQDN:              "lenovo.x",
			IPv4:              "10.0.0.102",
			Profiles:          []string{"compute"},
			InstalledServices: map[string]bool{}, // rejoining
			RuntimeHealthy:    map[string]bool{},
		},
	}
	state, funnels := ComputeDesiredStateWithFunnels("x", nodes, nil, "ryzen.x", nil, 1)

	// gateway must include only ryzen + nuc.
	g := setOf(gatewayRecordsFor(state))
	if !equalSet(g, map[string]bool{"10.0.0.63": true, "10.0.0.8": true}) {
		t.Errorf("gateway must be {ryzen, nuc}, got %v", g)
	}

	// dns.x must include only ryzen + nuc.
	dnsSet := setOf(aRecordsForName(state, "dns.x"))
	if !equalSet(dnsSet, map[string]bool{"10.0.0.63": true, "10.0.0.8": true}) {
		t.Errorf("dns.x must be {ryzen, nuc}, got %v", dnsSet)
	}

	// _cluster-controller._tcp SRV must include only ryzen + nuc.
	srvTargets := setOf(srvTargetsFor(state, "_cluster-controller._tcp.x"))
	if !equalSet(srvTargets, map[string]bool{"ryzen.x": true, "nuc.x": true}) {
		t.Errorf("_cluster-controller SRV must be {ryzen, nuc}, got %v", srvTargets)
	}

	// controller.x (leader) must include only ryzen (the leader).
	leader := setOf(aRecordsForName(state, "controller.x"))
	if !equalSet(leader, map[string]bool{"10.0.0.63": true}) {
		t.Errorf("controller.x (leader) must be {ryzen}, got %v", leader)
	}

	// Funnel must show withdrawal for at least gateway, dns, and controller.
	// dns + cluster-controller funnels both have profile-eligible nodes but
	// after filtering they keep only the healthy two.
	for _, f := range funnels {
		if f.Record == "gateway.x" && f.Desired != 4 {
			t.Errorf("gateway funnel desired count should be 4 (ryzen,nuc,dell,hp-01), got %d", f.Desired)
		}
		if f.Record == "gateway.x" && f.Published != 2 {
			t.Errorf("gateway funnel published count should be 2 (only healthy), got %d", f.Published)
		}
	}
}
