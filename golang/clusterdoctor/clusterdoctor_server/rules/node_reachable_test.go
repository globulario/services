package rules

import (
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testConfig() Config {
	return Config{HeartbeatStale: 2 * time.Minute}
}

func TestNodeReachable_FreshHeartbeat(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*clustercontrollerpb.NodeRecord{
			{NodeId: "node-1", LastSeen: timestamppb.New(time.Now().Add(-30 * time.Second)), Status: "ok"},
		},
	}
	findings := (nodeReachable{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for fresh heartbeat, got %d", len(findings))
	}
}

func TestNodeReachable_StaleHeartbeat(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*clustercontrollerpb.NodeRecord{
			{NodeId: "node-1", LastSeen: timestamppb.New(time.Now().Add(-5 * time.Minute)), Status: "ok"},
		},
	}
	findings := (nodeReachable{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for stale heartbeat, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "node.reachable" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
	if f.Severity != clusterdoctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL severity, got %v", f.Severity)
	}
	if f.EntityRef != "node-1" {
		t.Errorf("expected entity_ref=node-1, got %s", f.EntityRef)
	}
	if f.FindingID == "" {
		t.Error("finding_id must not be empty")
	}
}

func TestNodeReachable_UnreachableStatus(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*clustercontrollerpb.NodeRecord{
			{NodeId: "node-2", LastSeen: timestamppb.New(time.Now().Add(-10 * time.Second)), Status: "unreachable"},
		},
	}
	findings := (nodeReachable{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unreachable status, got %d", len(findings))
	}
}

func TestNodeReachable_StableFindingID(t *testing.T) {
	// Same inputs must produce same finding_id.
	stale := timestamppb.New(time.Now().Add(-10 * time.Minute))
	snap := &collector.Snapshot{
		Nodes: []*clustercontrollerpb.NodeRecord{
			{NodeId: "node-3", LastSeen: stale, Status: "ok"},
		},
	}
	f1 := (nodeReachable{}).Evaluate(snap, testConfig())
	f2 := (nodeReachable{}).Evaluate(snap, testConfig())
	if len(f1) == 0 || len(f2) == 0 {
		t.Fatal("expected at least one finding")
	}
	if f1[0].FindingID != f2[0].FindingID {
		t.Errorf("finding_id not stable: %s != %s", f1[0].FindingID, f2[0].FindingID)
	}
}
