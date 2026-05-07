package awarectx_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	awarectx "github.com/globulario/services/golang/awareness/context"
	"github.com/globulario/services/golang/awareness/graph"
)

// openTestGraph opens a temp-file awareness graph pre-populated with test fixtures.
func openTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g, err := graph.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { g.Close() })

	ctx := context.Background()

	// Nodes.
	nodes := []graph.Node{
		{ID: "file:golang/workflow/server.go", Type: graph.NodeTypeSourceFile, Name: "server.go", Path: "golang/workflow/server.go", Summary: "Workflow server implementation"},
		{ID: "sym:Serve", Type: graph.NodeTypeSymbol, Name: "Serve", Path: "golang/workflow/server.go", Summary: "Main serve loop"},
		{ID: "pkg:workflow", Type: graph.NodeTypeGoPackage, Name: "workflow", Summary: "Workflow Go package"},
		{ID: "svc:workflow", Type: graph.NodeTypeGlobularService, Name: "workflow", Summary: "Globular workflow service"},
		{ID: "inv:service.endpoint.reachability", Type: graph.NodeTypeInvariant, Name: "service.endpoint.reachability", Summary: "Etcd address must be reachable"},
		{ID: "fm:service.endpoint.port_squatting", Type: graph.NodeTypeFailureMode, Name: "service.endpoint.port_squatting_cgroup_escape", Summary: "Cgroup-escaped orphan squats port"},
		{ID: "test:TestWorkflowOrphan", Type: graph.NodeTypeTest, Name: "TestWorkflowOrphan", Summary: "Verifies orphan is killed before restart"},
		{ID: "fix:relying_on_restart", Type: graph.NodeTypeForbiddenFix, Name: "relying_on_restart_without_pkill_guard", Summary: "Forbidden: restart without pkill guard"},
		{ID: "unit:globular-workflow.service", Type: graph.NodeTypeSystemdUnit, Name: "globular-workflow.service", Summary: "Workflow systemd unit"},
		{ID: "etcd:/globular/services/workflow/config", Type: graph.NodeTypeEtcdKey, Name: "/globular/services/workflow/config", Summary: "Workflow etcd config"},
	}
	for _, n := range nodes {
		if err := g.AddNode(ctx, n); err != nil {
			t.Fatalf("AddNode %s: %v", n.ID, err)
		}
	}

	// Invariant record.
	if err := g.UpsertInvariant(ctx, graph.Invariant{
		ID:       "service.endpoint.reachability",
		Title:    "Service endpoint must be reachable",
		Summary:  "Etcd address must be probed before reconciliation trusts it",
		Severity: "critical",
		Status:   "active",
	}); err != nil {
		t.Fatalf("UpsertInvariant: %v", err)
	}

	// Failure mode record.
	if err := g.UpsertFailureMode(ctx, graph.FailureMode{
		ID:      "service.endpoint.port_squatting",
		Title:   "Port squatting via cgroup escape",
		Summary: "Orphan process escapes cgroup, squats original port",
	}); err != nil {
		t.Fatalf("UpsertFailureMode: %v", err)
	}

	// Edges.
	edges := []graph.Edge{
		{Src: "svc:workflow", Dst: "file:golang/workflow/server.go", Kind: graph.EdgeOwns, Confidence: 1.0},
		{Src: "file:golang/workflow/server.go", Dst: "sym:Serve", Kind: graph.EdgeDefines, Confidence: 1.0},
		{Src: "file:golang/workflow/server.go", Dst: "pkg:workflow", Kind: graph.EdgeImports, Confidence: 1.0},
		{Src: "inv:service.endpoint.reachability", Dst: "svc:workflow", Kind: graph.EdgeProtects, Confidence: 1.0, Required: true},
		{Src: "inv:service.endpoint.reachability", Dst: "fix:relying_on_restart", Kind: graph.EdgeForbids, Confidence: 1.0},
		{Src: "fm:service.endpoint.port_squatting", Dst: "svc:workflow", Kind: graph.EdgeAffects, Confidence: 1.0},
		{Src: "svc:workflow", Dst: "etcd:/globular/services/workflow/config", Kind: graph.EdgeReads, Confidence: 1.0},
		{Src: "svc:workflow", Dst: "test:TestWorkflowOrphan", Kind: graph.EdgeTestedBy, Confidence: 1.0},
		{Src: "unit:globular-workflow.service", Dst: "svc:workflow", Kind: graph.EdgeRunsAs, Confidence: 1.0},
	}
	for _, e := range edges {
		if err := g.AddEdge(ctx, e); err != nil {
			t.Fatalf("AddEdge %s->%s: %v", e.Src, e.Dst, err)
		}
	}

	return g
}

// --- ResolveNode tests ---

func TestResolveByExactID(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "svc:workflow")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	if r.Exact == nil {
		t.Fatal("expected exact match, got nil")
	}
	if r.Kind != awarectx.RefKindExact {
		t.Errorf("kind = %q, want %q", r.Kind, awarectx.RefKindExact)
	}
	if r.Exact.Name != "workflow" {
		t.Errorf("name = %q, want %q", r.Exact.Name, "workflow")
	}
}

func TestResolveByFilePath(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "golang/workflow/server.go")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	if r.Exact == nil {
		t.Fatal("expected exact match via file path, got nil")
	}
	if r.Kind != awarectx.RefKindFile {
		t.Errorf("kind = %q, want %q", r.Kind, awarectx.RefKindFile)
	}
}

func TestResolveBySymbolName(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "Serve")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	if r.Exact == nil {
		t.Fatal("expected exact match by symbol name, got nil")
	}
	if r.Kind != awarectx.RefKindSymbol {
		t.Errorf("kind = %q, want %q", r.Kind, awarectx.RefKindSymbol)
	}
}

func TestResolveByServiceName(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "workflow")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	if r.Exact == nil {
		t.Fatalf("expected exact match by service name, got nil (candidates: %d)", len(r.Candidates))
	}
	if r.Kind != awarectx.RefKindService {
		t.Errorf("kind = %q, want %q", r.Kind, awarectx.RefKindService)
	}
}

func TestResolveByInvariantID(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "service.endpoint.reachability")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	if r.Exact == nil {
		t.Fatal("expected exact invariant match, got nil")
	}
	if r.Kind != awarectx.RefKindInvariant {
		t.Errorf("kind = %q, want %q", r.Kind, awarectx.RefKindInvariant)
	}
}

func TestResolveByNameLike(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "TestWorkflow")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	// Should find TestWorkflowOrphan via name-like search.
	found := r.Exact != nil || len(r.Candidates) > 0
	if !found {
		t.Error("expected at least one candidate for 'TestWorkflow', got none")
	}
}

func TestResolveEmptyRef(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	r, err := awarectx.ResolveNode(ctx, g, "")
	if err != nil {
		t.Fatalf("ResolveNode: %v", err)
	}
	if r.Exact != nil {
		t.Error("expected nil exact for empty ref")
	}
}

// --- Build tests ---

func TestBuildNodeContext_ServiceNode(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nc, err := awarectx.Build(ctx, g, "svc:workflow", awarectx.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if nc.NodeID != "svc:workflow" {
		t.Errorf("NodeID = %q", nc.NodeID)
	}
	if nc.NodeType != graph.NodeTypeGlobularService {
		t.Errorf("NodeType = %q", nc.NodeType)
	}
	// Invariant should be collected from incoming EdgeProtects.
	if len(nc.RelatedInvariants) == 0 {
		t.Error("expected at least one related invariant via EdgeProtects")
	}
	// Etcd read should be captured.
	if len(nc.StateReads) == 0 {
		t.Error("expected StateReads to include etcd config key")
	}
	// Required test.
	if len(nc.RequiredTests) == 0 {
		t.Error("expected RequiredTests to include TestWorkflowOrphan")
	}
	// Edit warnings should mention workflow invariant/service note.
	if len(nc.EditWarnings) == 0 {
		t.Error("expected EditWarnings for service node")
	}
}

func TestBuildNodeContext_InvariantNode(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nc, err := awarectx.Build(ctx, g, "inv:service.endpoint.reachability", awarectx.Options{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if nc.SourceLabel != awarectx.ConfidenceExplicit {
		t.Errorf("SourceLabel = %q, want explicit", nc.SourceLabel)
	}
	// Outgoing EdgeForbids → forbidden fix should be captured.
	if len(nc.ForbiddenFixes) == 0 {
		t.Error("expected ForbiddenFixes from EdgeForbids")
	}
}

func TestBuildNodeContext_NotFound(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	_, err := awarectx.Build(ctx, g, "nonexistent:node", awarectx.Options{})
	if err == nil {
		t.Error("expected error for non-existent node, got nil")
	}
}

// --- Neighborhood tests ---

func TestNeighborhood_Depth1(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nr, err := awarectx.Neighborhood(ctx, g, "svc:workflow", 1)
	if err != nil {
		t.Fatalf("Neighborhood: %v", err)
	}
	if nr.Center == nil || nr.Center.ID != "svc:workflow" {
		t.Error("center node mismatch")
	}
	if len(nr.Nodes) < 2 {
		t.Errorf("expected more than 1 node at depth 1, got %d", len(nr.Nodes))
	}
	// At depth 1 from svc:workflow we should see file, etcd key, test, invariant neighbor.
	if len(nr.Edges) == 0 {
		t.Error("expected edges in result")
	}
}

func TestNeighborhood_Depth2(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nr, err := awarectx.Neighborhood(ctx, g, "svc:workflow", 2)
	if err != nil {
		t.Fatalf("Neighborhood: %v", err)
	}
	// At depth 2 we should reach symbol via file.
	if len(nr.Nodes) < 3 {
		t.Errorf("expected at least 3 nodes at depth 2, got %d", len(nr.Nodes))
	}
}

// --- ExplainNode tests ---

func TestExplainNode_Service(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	ex, err := awarectx.ExplainNode(ctx, g, "svc:workflow", awarectx.Options{})
	if err != nil {
		t.Fatalf("ExplainNode: %v", err)
	}
	if ex.NodeID != "svc:workflow" {
		t.Errorf("NodeID = %q", ex.NodeID)
	}
	if !strings.Contains(ex.Role, "workflow") {
		t.Errorf("Role does not mention 'workflow': %q", ex.Role)
	}
	// Risks should mention the failure mode.
	foundRisk := false
	for _, r := range ex.Risks {
		if strings.Contains(r, "failure mode") || strings.Contains(r, "port") || strings.Contains(r, "squatting") {
			foundRisk = true
		}
	}
	if !foundRisk {
		t.Errorf("expected failure mode in Risks, got: %v", ex.Risks)
	}
	if len(ex.Warnings) == 0 {
		t.Error("expected at least one warning for service node")
	}
}

// --- Format tests ---

func TestFormatNodeContext_Markdown(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nc, _ := awarectx.Build(ctx, g, "svc:workflow", awarectx.Options{})
	out := awarectx.FormatNodeContext(nc, "markdown")
	if !strings.Contains(out, "## Node Context") {
		t.Error("markdown output missing '## Node Context' header")
	}
	if !strings.Contains(out, "workflow") {
		t.Error("markdown output missing service name")
	}
}

func TestFormatNodeContext_JSON(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nc, _ := awarectx.Build(ctx, g, "svc:workflow", awarectx.Options{})
	out := awarectx.FormatNodeContext(nc, "json")
	if !strings.Contains(out, `"node_id"`) {
		t.Error("json output missing node_id field")
	}
}

func TestFormatNodeContext_Agent(t *testing.T) {
	g := openTestGraph(t)
	ctx := context.Background()
	nc, _ := awarectx.Build(ctx, g, "svc:workflow", awarectx.Options{})
	out := awarectx.FormatNodeContext(nc, "agent")
	if !strings.Contains(out, "node_id:") {
		t.Error("agent output missing 'node_id:' key")
	}
}
