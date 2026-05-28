package evidencedigest

import (
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestOfIsDeterministicRegardlessOfInputOrder proves the digest is stable
// across permutations of evidence entries, key/value pairs within an
// entry, and equivalent timestamps. This is the property the approval-
// token mint contract relies on: operator-side CLI and server-side audit
// MUST produce identical bytes for identical evidence, even if one side
// happens to iterate maps in a different order.
func TestOfIsDeterministicRegardlessOfInputOrder(t *testing.T) {
	ts := timestamppb.Now()
	a := []*cluster_doctorpb.Evidence{
		{
			SourceService: "cluster_controller",
			SourceRpc:     "GetClusterHealthV1",
			KeyValues:     map[string]string{"node": "node-1", "unit": "echo.service"},
			Timestamp:     ts,
		},
		{
			SourceService: "node_agent",
			SourceRpc:     "SearchLogs",
			KeyValues:     map[string]string{"q": "panic"},
			Timestamp:     ts,
		},
	}
	// b: same evidence, reversed order, key/values swapped.
	b := []*cluster_doctorpb.Evidence{
		{
			SourceService: "node_agent",
			SourceRpc:     "SearchLogs",
			KeyValues:     map[string]string{"q": "panic"},
			Timestamp:     ts,
		},
		{
			SourceService: "cluster_controller",
			SourceRpc:     "GetClusterHealthV1",
			KeyValues:     map[string]string{"unit": "echo.service", "node": "node-1"},
			Timestamp:     ts,
		},
	}
	if Of(a) != Of(b) {
		t.Fatalf("digest must be deterministic across input order:\n a=%s\n b=%s", Of(a), Of(b))
	}

	// Different evidence must produce a different digest.
	c := append([]*cluster_doctorpb.Evidence(nil), a...)
	c = append(c, &cluster_doctorpb.Evidence{
		SourceService: "verifier",
		SourceRpc:     "Attest",
		Timestamp:     ts,
	})
	if Of(a) == Of(c) {
		t.Fatal("different evidence must produce different digests")
	}

	// Empty/nil returns "" — callers can pass either.
	if Of(nil) != "" {
		t.Fatalf("nil evidence: got %q, want empty", Of(nil))
	}
	if Of([]*cluster_doctorpb.Evidence{}) != "" {
		t.Fatalf("empty evidence: got %q, want empty", Of([]*cluster_doctorpb.Evidence{}))
	}
}
