package rules

// placement_orphaned_install_test.go — pins the orphaned-install finding:
// a package installed on a node whose profiles do not authorize it under the
// component catalog placement map is a terminal, operator-action-required
// orphan. Pure-snapshot, no RPC.

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func orphanFindingsFor(snap *collector.Snapshot) []Finding {
	return (placementInstalledPackageOrphaned{}).Evaluate(snap, testConfig())
}

func hasOrphan(findings []Finding, pkg string) *Finding {
	for i := range findings {
		f := findings[i]
		if f.InvariantID == "placement.installed_package_orphaned" && f.EntityRef != "" &&
			(f.EntityRef == "node-a/"+pkg) {
			return &f
		}
	}
	return nil
}

// torrent (catalog: compute) installed on a control-plane/core/storage node →
// orphaned-install finding. dns (catalog: core) on the same node → no finding.
func TestDoctorRule_OrphanedInstall_TorrentOnNonComputeNode(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "node-a",
				Status:   "ready",
				Profiles: []string{"control-plane", "core", "storage"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"node-a": {
				NodeId: "node-a",
				InstalledVersions: map[string]string{
					"torrent": "1.2.233", // compute-only → orphan here
					"dns":     "1.2.235", // core → legitimately placed
				},
			},
		},
	}
	findings := orphanFindingsFor(snap)

	tf := hasOrphan(findings, "torrent")
	if tf == nil {
		t.Fatalf("expected orphaned-install finding for torrent; got %+v", findings)
	}
	if tf.Severity.String() == "" || tf.InvariantStatus.String() == "" {
		t.Errorf("finding must carry severity+status: %+v", tf)
	}
	if hasOrphan(findings, "dns") != nil {
		t.Errorf("dns is core-placeable on this node — must NOT be flagged as orphan")
	}
}

// torrent installed on a compute node → legitimately placed, no finding.
func TestDoctorRule_OrphanedInstall_TorrentOnComputeNode_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "node-a",
				Status:   "ready",
				Profiles: []string{"compute"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"node-a": {
				NodeId:            "node-a",
				InstalledVersions: map[string]string{"torrent": "1.2.235"},
			},
		},
	}
	if f := orphanFindingsFor(snap); len(f) != 0 {
		t.Errorf("torrent on a compute node is placeable — expected no findings, got %+v", f)
	}
}

// A package with no catalog entry must NOT be classified as a profile orphan
// (that is a distinct, separately-handled condition).
func TestDoctorRule_OrphanedInstall_UnknownPackage_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "node-a",
				Status:   "ready",
				Profiles: []string{"control-plane", "core", "storage"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"node-a": {
				NodeId:            "node-a",
				InstalledVersions: map[string]string{"totally-unknown-pkg": "9.9.9"},
			},
		},
	}
	if f := orphanFindingsFor(snap); len(f) != 0 {
		t.Errorf("unknown-catalog package must not be flagged as a profile orphan, got %+v", f)
	}
}

// Reduced-harvest honesty: if installed-state for the node could not be observed
// (NodeHealths absent), the rule emits nothing rather than fabricating a verdict.
func TestDoctorRule_OrphanedInstall_NoHealth_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{
				NodeId:   "node-a",
				Status:   "ready",
				Profiles: []string{"control-plane", "core", "storage"},
				LastSeen: timestamppb.New(time.Now()),
			},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{}, // not observed
	}
	if f := orphanFindingsFor(snap); len(f) != 0 {
		t.Errorf("no installed-state observed → expected no findings (UNKNOWN, not a verdict), got %+v", f)
	}
}
