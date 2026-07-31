package collector

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func placedNode(id string, workloads string) *cluster_controllerpb.NodeRecord {
	n := &cluster_controllerpb.NodeRecord{NodeId: id, Metadata: map[string]string{}}
	if workloads != "" {
		n.Metadata["desired_workloads"] = workloads
	}
	return n
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// SCAR (2026-07-31): with no per-node release status, every desired service was
// required on EVERY node, so the doctor demanded a runtime proof from nodes that
// were never meant to run the service.
//
// On the first 3-node cluster this produced 8 service.runtime_identity_unproven
// warnings: media, title and torrent (media-server profile — held only by ryzen)
// plus a deliberately-held minio, all charged against nuc and dell while both
// were behaving correctly.
func TestRequiredNodes_SkipsNodesWhosePlacementExcludesTheService(t *testing.T) {
	nodes := []*cluster_controllerpb.NodeRecord{
		placedNode("ryzen", "media,title,torrent,event"), // media-server
		placedNode("nuc", "event,log"),                   // no media-server
		placedNode("dell", "event,log"),
	}
	got := requiredNodesFromStatus(nil, nodes, "media")
	if contains(got, "nuc") || contains(got, "dell") {
		t.Errorf("media required on nodes that do not place it: %v", got)
	}
	if !contains(got, "ryzen") {
		t.Errorf("media must still be required on ryzen, which places it: %v", got)
	}
}

// NEGATIVE CONTROL. Without this the fix is indistinguishable from switching the
// rule off: a service that a node genuinely SHOULD run must still be required
// there, so a real "installed but not running" stays detectable.
func TestRequiredNodes_StillRequiresServiceOnNodesThatPlaceIt(t *testing.T) {
	nodes := []*cluster_controllerpb.NodeRecord{
		placedNode("ryzen", "event,log"),
		placedNode("nuc", "event,log"),
		placedNode("dell", "event,log"),
	}
	got := requiredNodesFromStatus(nil, nodes, "event")
	if len(got) != 3 {
		t.Fatalf("event is placed on all three nodes and must be required on all three, got %v", got)
	}
}

// Absent placement metadata is UNKNOWN, not "not placed here". Concluding
// otherwise would let a controller that has not resolved intent silently
// suppress genuine findings — failure_mode:doctor.rule_silently_suppressed_on_data_source_error.
func TestRequiredNodes_AbsentPlacementMetadataKeepsTheNodeInScope(t *testing.T) {
	nodes := []*cluster_controllerpb.NodeRecord{
		placedNode("ryzen", "event"),
		placedNode("silent", ""), // publishes nothing
	}
	got := requiredNodesFromStatus(nil, nodes, "media")
	if !contains(got, "silent") {
		t.Errorf("a node publishing no placement metadata must stay in scope, got %v", got)
	}
	if contains(got, "ryzen") {
		t.Errorf("ryzen positively states a placement excluding media and must be dropped: %v", got)
	}
}

// Explicit per-node status is the controller's own placement decision and must
// be used verbatim — the doctor does not get to narrow the owner's answer.
func TestRequiredNodes_ExplicitStatusIsUsedVerbatim(t *testing.T) {
	status := []*cluster_controllerpb.NodeReleaseStatus{{NodeID: "nuc"}}
	nodes := []*cluster_controllerpb.NodeRecord{placedNode("nuc", "event,log")}
	got := requiredNodesFromStatus(status, nodes, "media")
	if len(got) != 1 || got[0] != "nuc" {
		t.Errorf("explicit status must be used as-is even when metadata disagrees, got %v", got)
	}
}

func TestNodePlacesService_KnownFlag(t *testing.T) {
	if _, known := nodePlacesService(placedNode("a", ""), "media"); known {
		t.Error("no metadata must report known=false")
	}
	places, known := nodePlacesService(placedNode("a", "media,event"), "media")
	if !known || !places {
		t.Errorf("listed service must report known=true places=true, got known=%v places=%v", known, places)
	}
	places, known = nodePlacesService(placedNode("a", "event"), "media")
	if !known || places {
		t.Errorf("unlisted service on a node that states placement must be known=true places=false, got known=%v places=%v", known, places)
	}
}
