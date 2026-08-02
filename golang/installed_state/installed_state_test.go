package installed_state

import (
	"context"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestPackageKey(t *testing.T) {
	tests := []struct {
		nodeID, kind, name string
		want               string
	}{
		{"node-1", "SERVICE", "gateway", "/globular/nodes/node-1/packages/SERVICE/gateway"},
		{"node-2", "application", "admin", "/globular/nodes/node-2/packages/APPLICATION/admin"},
		{"node-1", "INFRASTRUCTURE", "etcd", "/globular/nodes/node-1/packages/INFRASTRUCTURE/etcd"},
	}
	for _, tt := range tests {
		got := packageKey(tt.nodeID, tt.kind, tt.name)
		if got != tt.want {
			t.Errorf("packageKey(%q, %q, %q) = %q, want %q", tt.nodeID, tt.kind, tt.name, got, tt.want)
		}
	}
}

func TestNodePackagesPrefix(t *testing.T) {
	got := nodePackagesPrefix("node-1")
	want := "/globular/nodes/node-1/packages/"
	if got != want {
		t.Errorf("nodePackagesPrefix(node-1) = %q, want %q", got, want)
	}
}

func TestNodeKindPrefix(t *testing.T) {
	got := nodeKindPrefix("node-1", "service")
	want := "/globular/nodes/node-1/packages/SERVICE/"
	if got != want {
		t.Errorf("nodeKindPrefix(node-1, service) = %q, want %q", got, want)
	}
}

func TestUnmarshalPackage(t *testing.T) {
	data := []byte(`{"nodeId":"n1","name":"gateway","version":"1.2.3","kind":"SERVICE","status":"installed"}`)
	pkg, err := unmarshalPackage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.GetNodeId() != "n1" {
		t.Errorf("node_id = %q, want n1", pkg.GetNodeId())
	}
	if pkg.GetName() != "gateway" {
		t.Errorf("name = %q, want gateway", pkg.GetName())
	}
	if pkg.GetVersion() != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", pkg.GetVersion())
	}
	if pkg.GetKind() != "SERVICE" {
		t.Errorf("kind = %q, want SERVICE", pkg.GetKind())
	}
}

func TestUnmarshalPackage_Invalid(t *testing.T) {
	_, err := unmarshalPackage([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseInstalledPackageKey(t *testing.T) {
	tests := []struct {
		key        string
		wantNodeID string
		wantKind   string
		wantName   string
		wantOK     bool
	}{
		{
			key:        "/globular/nodes/node-1/packages/SERVICE/gateway",
			wantNodeID: "node-1",
			wantKind:   "SERVICE",
			wantName:   "gateway",
			wantOK:     true,
		},
		{
			key:    "/globular/nodes/bootstrap_marker",
			wantOK: false,
		},
		{
			key:    "/globular/nodes/node-1/node_agent_metrics_port",
			wantOK: false,
		},
		{
			key:    "/globular/nodes/node-1/objectstore/rendered_generation",
			wantOK: false,
		},
		{
			key:    "/globular/nodes/node-1/suspended/scylla-manager-agent",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		nodeID, kind, name, ok := parseInstalledPackageKey(tt.key)
		if ok != tt.wantOK || nodeID != tt.wantNodeID || kind != tt.wantKind || name != tt.wantName {
			t.Fatalf("parseInstalledPackageKey(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
				tt.key, nodeID, kind, name, ok, tt.wantNodeID, tt.wantKind, tt.wantName, tt.wantOK)
		}
	}
}

// Constants for the disk-truth cleanup tests below.
const (
	diskShaCurrent = "2662722de60816248feb9896d9543b58aed4f8b36214e9e6bfb41eb5fe18c67c" // what's on disk now
	stalePrevSha   = "fb672006af1e104527bf406b1913e60e89ca8135fb8b98b24d5bf9ef286fb26c" // an older version's binary
)

func TestStaleSiblingKinds_DeletesStaleByDiskTruth(t *testing.T) {
	// Regression for "stale_kind_manifest_poisons_installed_state":
	// cluster-controller has a SERVICE ghost stamped with the OLD binary
	// (fb672006...) AND an INFRA record stamped with the CURRENT binary
	// (2662722d...). After writing the INFRA record again, cleanup must
	// flag the SERVICE record as stale because its proof does not match
	// disk, and leave any record matching disk alone.
	pkgs := []*node_agentpb.InstalledPackage{
		{Name: "cluster-controller", Kind: "SERVICE", Version: "1.2.131",
			Metadata: map[string]string{"entrypoint_checksum": stalePrevSha}},
		{Name: "cluster-controller", Kind: "INFRASTRUCTURE", Version: "1.2.148",
			Metadata: map[string]string{"entrypoint_checksum": diskShaCurrent}},
	}
	got := staleSiblingKinds(pkgs, "INFRASTRUCTURE", "cluster-controller", diskShaCurrent)
	if len(got) != 1 || got[0] != "SERVICE" {
		t.Fatalf("expected [SERVICE], got %v", got)
	}
}

func TestStaleSiblingKinds_RefusesToDeleteWithoutProof(t *testing.T) {
	// The dangerous case: a SERVICE record with NO proof gets written
	// (bare heartbeat). Pre-fix this deleted the INFRA record that had
	// full proof. The new rule: a sibling without a stored
	// entrypoint_checksum cannot be classified as stale and must NOT be
	// returned for deletion.
	pkgs := []*node_agentpb.InstalledPackage{
		{Name: "cluster-controller", Kind: "INFRASTRUCTURE", Version: "1.2.148",
			Metadata: map[string]string{"entrypoint_checksum": diskShaCurrent}},
		{Name: "cluster-controller", Kind: "SERVICE", Version: "1.2.148"}, // no proof
	}
	// Pretend the caller just wrote the SERVICE record.
	got := staleSiblingKinds(pkgs, "SERVICE", "cluster-controller", diskShaCurrent)
	if len(got) != 0 {
		t.Fatalf("must not delete an authoritative INFRA record on behalf of a proofless SERVICE write; got %v", got)
	}
}

func TestStaleSiblingKinds_NeverTouchesKeepKind(t *testing.T) {
	// The kind we just wrote must not be returned even if its proof
	// disagrees with disk — that disagreement is a separate problem
	// surfaced by the verifier.
	pkgs := []*node_agentpb.InstalledPackage{
		{Name: "x", Kind: "INFRASTRUCTURE",
			Metadata: map[string]string{"entrypoint_checksum": stalePrevSha}},
	}
	if got := staleSiblingKinds(pkgs, "INFRASTRUCTURE", "x", diskShaCurrent); len(got) != 0 {
		t.Fatalf("must not delete keepKind, got %v", got)
	}
}

func TestStaleSiblingKinds_KeepsMatchingProofUnderOtherKind(t *testing.T) {
	// If a sibling record under another kind happens to have proof
	// matching disk, it is NOT stale — the duplication is real but
	// consistent. Leave it for a higher-level consolidator.
	pkgs := []*node_agentpb.InstalledPackage{
		{Name: "x", Kind: "SERVICE",
			Metadata: map[string]string{"entrypoint_checksum": diskShaCurrent}},
		{Name: "x", Kind: "INFRASTRUCTURE",
			Metadata: map[string]string{"entrypoint_checksum": diskShaCurrent}},
	}
	if got := staleSiblingKinds(pkgs, "INFRASTRUCTURE", "x", diskShaCurrent); len(got) != 0 {
		t.Fatalf("must not delete sibling whose proof matches disk; got %v", got)
	}
}

func TestStaleSiblingKinds_NormalizesCasingAndPrefix(t *testing.T) {
	// The doctor-side comparison must be tolerant of "sha256:" prefix
	// and case variance, since values flow through multiple writers.
	pkgs := []*node_agentpb.InstalledPackage{
		{Name: "x", Kind: "SERVICE",
			Metadata: map[string]string{"entrypoint_checksum": "sha256:" + strings.ToUpper(stalePrevSha)}},
	}
	got := staleSiblingKinds(pkgs, "INFRASTRUCTURE", "x", "SHA256:"+diskShaCurrent)
	if len(got) != 1 || got[0] != "SERVICE" {
		t.Fatalf("normalized comparison expected [SERVICE], got %v", got)
	}
}

func TestStaleSiblingKinds_GuardsAgainstEmptyArgs(t *testing.T) {
	pkgs := []*node_agentpb.InstalledPackage{
		{Name: "x", Kind: "SERVICE",
			Metadata: map[string]string{"entrypoint_checksum": stalePrevSha}},
	}
	if got := staleSiblingKinds(pkgs, "", "x", diskShaCurrent); got != nil {
		t.Errorf("empty keepKind should return nil, got %v", got)
	}
	if got := staleSiblingKinds(pkgs, "INFRASTRUCTURE", "", diskShaCurrent); got != nil {
		t.Errorf("empty name should return nil, got %v", got)
	}
	if got := staleSiblingKinds(pkgs, "INFRASTRUCTURE", "x", ""); got != nil {
		t.Errorf("empty diskSha256 should return nil, got %v", got)
	}
}

func TestWriteInstalledPackage_ValidatesNodeID(t *testing.T) {
	err := WriteInstalledPackage(context.Background(), &node_agentpb.InstalledPackage{
		NodeId:  "",
		Name:    "workflow",
		Kind:    "SERVICE",
		Version: "1.0.0",
	})
	if err == nil {
		t.Fatal("expected node_id validation error")
	}
}
