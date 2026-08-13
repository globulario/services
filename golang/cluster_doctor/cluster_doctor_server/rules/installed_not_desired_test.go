package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// node.join's install_workloads_compute places seven workload packages
// (sql, catalog, ldap, mail, blog, conversation, echo) on every compute-profile
// node. None of them appear in the cluster's desired-service set, so they sit
// installed-and-disabled indefinitely.
//
// Both runtime rules used to read "installed" as "expected active" and reported
// each one twice — installed_state_runtime_mismatch plus
// node.systemd.units_running. On an otherwise healthy 5-node cluster
// (2026-08-10) that was 29 of 31 findings, permanent and unclearable: no
// convergence pass will ever start a service the cluster never desired, and no
// remediation can clear it.
//
// Desired state is the authority for what should run
// (intent:desired_state.is_authority, intent:runtime_observation_must_not_mutate_desired).
// These tests pin that, and pin the fail-safe: when desired state is UNKNOWN the
// findings must stay.

func desiredTargets(names ...string) map[string]*collector.DesiredServiceTarget {
	m := make(map[string]*collector.DesiredServiceTarget, len(names))
	for _, n := range names {
		m[n] = &collector.DesiredServiceTarget{Service: n}
	}
	return m
}

func snapshotForDesiredTest(pkg, unitName, unitState string, desired map[string]*collector.DesiredServiceTarget) *collector.Snapshot {
	return &collector.Snapshot{
		Nodes:                 []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths:           map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{pkg: "1.2.303"})},
		Inventories:           map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit(unitName, unitState))},
		DesiredServiceTargets: desired,
	}
}

func TestInstalledButNotDesired_NoFindingFromEitherRule(t *testing.T) {
	// blog is installed and inactive, but the cluster desires only rbac/dns.
	snap := snapshotForDesiredTest("blog", "globular-blog.service", "inactive", desiredTargets("rbac", "dns"))

	if f := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Errorf("installed-but-not-desired package must not be convergence drift, got %d: %+v", len(f), f)
	}
	if f := (nodeUnitsRunning{}).Evaluate(snap, testConfig()); len(f) != 0 {
		t.Errorf("unit of an installed-but-not-desired package must not be expected-active, got %d: %+v", len(f), f)
	}
}

func TestInstalledAndDesired_StillReported(t *testing.T) {
	// rbac IS desired and is not running — that is real drift and must surface.
	snap := snapshotForDesiredTest("rbac", "globular-rbac.service", "inactive", desiredTargets("rbac", "dns"))

	if f := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig()); len(f) == 0 {
		t.Error("a DESIRED package that is not running must still be reported")
	}
	if f := (nodeUnitsRunning{}).Evaluate(snap, testConfig()); len(f) == 0 {
		t.Error("a DESIRED package's inactive unit must still be reported")
	}
}

// The fail-safe. An empty/nil desired set means the collector could not read
// desired state, not that nothing is desired — suppressing then would silence
// every real mismatch at once
// (fm doctor.rule_silently_suppressed_on_data_source_error).
func TestDesiredStateUnknown_DoesNotSuppress(t *testing.T) {
	snap := snapshotForDesiredTest("rbac", "globular-rbac.service", "inactive", nil)

	if f := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig()); len(f) == 0 {
		t.Error("with desired state UNKNOWN the mismatch must still be reported — absence of evidence is not evidence of 'not desired'")
	}
	if f := (nodeUnitsRunning{}).Evaluate(snap, testConfig()); len(f) == 0 {
		t.Error("with desired state UNKNOWN the inactive unit must still be reported")
	}
}

func TestUnitInstalledButNotDesired_ScopeIsGlobularUnitsOnly(t *testing.T) {
	snap := &collector.Snapshot{DesiredServiceTargets: desiredTargets("rbac")}

	// OS units are not named by desired state; they keep their own waivers
	// (ingress-disabled keepalived, storage-plane scylla) and must not be
	// swept up by this one.
	for _, u := range []string{"keepalived.service", "scylla-server.service"} {
		if unitInstalledButNotDesired(snap, u) {
			t.Errorf("%s is an OS unit not named by desired state and must not be waived here", u)
		}
	}
	if !unitInstalledButNotDesired(snap, "globular-blog.service") {
		t.Error("globular-blog.service is not desired and should be waived")
	}
	if unitInstalledButNotDesired(snap, "globular-rbac.service") {
		t.Error("globular-rbac.service IS desired and must not be waived")
	}
	// Unknown desired state must never waive.
	if unitInstalledButNotDesired(&collector.Snapshot{}, "globular-blog.service") {
		t.Error("with desired state unknown nothing may be waived")
	}
}
