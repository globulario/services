package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestRuntimeVersionIdentityLane_NoMatchingOverride_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"dns": "1.2.43+local.ryzen.1"}},
		},
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{},
		NodePackageKinds: map[string]map[string]string{
			"n1": {"dns": "SERVICE"},
		},
	}

	findings := (runtimeVersionIdentityLane{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.InvariantID != "service.runtime_version_identity_lane" {
		t.Fatalf("wrong invariant id: %s", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("expected WARN, got %v", f.Severity)
	}
}

func TestRuntimeVersionIdentityLane_MatchingOverride_Silent(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"dns": "1.2.43+local.ryzen.1"}},
		},
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"dns": {ServiceName: "dns", Version: "1.2.43+local.ryzen.1"},
		},
		NodePackageKinds: map[string]map[string]string{
			"n1": {"dns": "SERVICE"},
		},
	}

	findings := (runtimeVersionIdentityLane{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %+v", len(findings), findings)
	}
}

func TestRuntimeVersionIdentityLane_OverrideReadFailed_DegradeSilent(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"dns": "1.2.43+local.ryzen.1"}},
		},
		ActiveLocalOverrides: nil, // read failure sentinel
		NodePackageKinds: map[string]map[string]string{
			"n1": {"dns": "SERVICE"},
		},
	}

	findings := (runtimeVersionIdentityLane{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Fatalf("expected no findings when overrides are unavailable, got %d: %+v", len(findings), findings)
	}
}

func TestRuntimeVersionOverrideDivergence_LocalVersionMismatch_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {
				NodeId:           "n1",
				InstalledVersions: map[string]string{"dns": "1.2.43+local.ryzen.2"},
				InstalledBuildIds: map[string]string{"dns": "bid-222"},
			},
		},
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"dns": {ServiceName: "dns", Version: "1.2.43+local.ryzen.1", BuildID: "bid-111"},
		},
		NodePackageKinds: map[string]map[string]string{
			"n1": {"dns": "SERVICE"},
		},
	}

	findings := (runtimeVersionOverrideDivergence{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].InvariantID != "service.runtime_version_override_divergence" {
		t.Fatalf("wrong invariant id: %s", findings[0].InvariantID)
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("expected WARN, got %v", findings[0].Severity)
	}
}

func TestRuntimeVersionOverrideDivergence_BuildIDMismatch_WarnFires(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {
				NodeId:           "n1",
				InstalledVersions: map[string]string{"dns": "1.2.43+local.ryzen.1"},
				InstalledBuildIds: map[string]string{"dns": "bid-222"},
			},
		},
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"dns": {ServiceName: "dns", Version: "1.2.43+local.ryzen.1", BuildID: "bid-111"},
		},
		NodePackageKinds: map[string]map[string]string{
			"n1": {"dns": "SERVICE"},
		},
	}

	findings := (runtimeVersionOverrideDivergence{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
}

func TestRuntimeVersionOverrideDivergence_MatchingOverride_Silent(t *testing.T) {
	snap := &collector.Snapshot{
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {
				NodeId:           "n1",
				InstalledVersions: map[string]string{"dns": "1.2.43+local.ryzen.1"},
				InstalledBuildIds: map[string]string{"dns": "bid-111"},
			},
		},
		ActiveLocalOverrides: map[string]*cluster_controllerpb.LocalOverride{
			"dns": {ServiceName: "dns", Version: "1.2.43+local.ryzen.1", BuildID: "bid-111"},
		},
		NodePackageKinds: map[string]map[string]string{
			"n1": {"dns": "SERVICE"},
		},
	}

	findings := (runtimeVersionOverrideDivergence{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %+v", len(findings), findings)
	}
}
