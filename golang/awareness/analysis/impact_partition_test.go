package analysis_test

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/graph"
)

// TestImpactByFile_DirectVsInferredPartition reproduces the exact package-edge
// bleed that motivated this change: three sibling files in the same package,
// where ONE of them has a file-specific invariant anchor. Before the
// partition, the impact tool returned essentially identical match sets for all
// three siblings because the broader symbol/package walk reaches every
// invariant the package touches. After the partition, the file-specific
// invariant must appear in DirectInvariants for the anchored file, and must
// NOT appear in DirectInvariants for the unanchored siblings.
//
// Scenario mirrors the real cluster_controller_server/ package on 2026-05-29:
// release_pipeline.go has a release_pipeline.set_fields_routing_must_match
// anchor; server.go and reconcile_nodes.go share the package and its symbols
// but do not anchor that invariant.
func TestImpactByFile_DirectVsInferredPartition(t *testing.T) {
	g := openGraph(t)
	ctx := context.Background()

	// Three sibling source files in the same package.
	anchored := "golang/cluster_controller/cluster_controller_server/release_pipeline.go"
	sibling1 := "golang/cluster_controller/cluster_controller_server/server.go"
	sibling2 := "golang/cluster_controller/cluster_controller_server/reconcile_nodes.go"

	for _, p := range []string{anchored, sibling1, sibling2} {
		_ = g.AddNode(ctx, graph.Node{
			ID:   "source_file:" + p,
			Type: graph.NodeTypeSourceFile,
			Name: p,
			Path: p,
		})
	}

	// Package node that all three siblings belong to. This is the bleed
	// vector: the loader connects each file to the package and the package
	// to invariants/services, so a depth-6 walk from any file reaches every
	// package-level invariant.
	pkgID := "package:cluster_controller_server"
	_ = g.AddNode(ctx, graph.Node{ID: pkgID, Type: graph.NodeTypePackage, Name: "cluster-controller"})
	for _, p := range []string{anchored, sibling1, sibling2} {
		_ = g.AddEdge(ctx, graph.Edge{Src: "source_file:" + p, Kind: graph.EdgeOwns, Dst: pkgID})
	}

	// File-specific invariant: protects.files lists only the anchored file.
	// The loader creates BOTH the protects edge (invariant → file) AND the
	// reverse implements edge (file → invariant) — the latter is what the
	// partition uses to decide directness.
	directInvID := "invariant:release_pipeline.set_fields_routing_must_match_release_kind"
	_ = g.UpsertInvariant(ctx, graph.Invariant{
		ID:    "release_pipeline.set_fields_routing_must_match_release_kind",
		Title: "SetFields routing must match release kind",
	})
	_ = g.AddNode(ctx, graph.Node{ID: directInvID, Type: graph.NodeTypeInvariant, Name: "release_pipeline.set_fields_routing_must_match_release_kind"})
	_ = g.AddEdge(ctx, graph.Edge{Src: directInvID, Kind: graph.EdgeProtects, Dst: "source_file:" + anchored})
	_ = g.AddEdge(ctx, graph.Edge{Src: "source_file:" + anchored, Kind: graph.EdgeImplements, Dst: directInvID})

	// Package-level invariant: connected to the package, NOT to any individual
	// file. All three siblings reach it via file → package → invariant. This
	// is the kind of inferred match that should NOT dominate file-specific
	// output.
	packageInvID := "invariant:reconcile.global_work_must_not_starve_completion"
	_ = g.UpsertInvariant(ctx, graph.Invariant{
		ID:    "reconcile.global_work_must_not_starve_completion",
		Title: "Reconcile must not starve completion",
	})
	_ = g.AddNode(ctx, graph.Node{ID: packageInvID, Type: graph.NodeTypeInvariant, Name: "reconcile.global_work_must_not_starve_completion"})
	_ = g.AddEdge(ctx, graph.Edge{Src: packageInvID, Kind: graph.EdgeProtects, Dst: pkgID})

	// ── Run impact on the anchored file ────────────────────────────────────
	res, err := analysis.ImpactByFile(ctx, g, anchored)
	if err != nil {
		t.Fatalf("ImpactByFile(%s): %v", anchored, err)
	}

	if !containsByID(res.DirectInvariants, directInvID) {
		t.Errorf("anchored file: DirectInvariants missing %q\n  got DirectInvariants: %s", directInvID, idsOf(res.DirectInvariants))
	}
	if containsByID(res.InferredInvariants, directInvID) {
		t.Errorf("anchored file: %q must NOT also appear in InferredInvariants (double-classification)", directInvID)
	}

	// Back-compat: the legacy Invariants slice must list direct items first.
	if len(res.Invariants) == 0 || res.Invariants[0].ID != directInvID {
		t.Errorf("anchored file: legacy Invariants must list DirectInvariants first; got first=%v", firstIDOrEmpty(res.Invariants))
	}

	// ── Run impact on each sibling that does NOT anchor the invariant ──────
	for _, sib := range []string{sibling1, sibling2} {
		sibRes, err := analysis.ImpactByFile(ctx, g, sib)
		if err != nil {
			t.Fatalf("ImpactByFile(%s): %v", sib, err)
		}
		if containsByID(sibRes.DirectInvariants, directInvID) {
			t.Errorf("sibling %s: DirectInvariants must NOT contain release_pipeline-specific %q (file is not in its protects.files)\n  got DirectInvariants: %s",
				sib, directInvID, idsOf(sibRes.DirectInvariants))
		}
	}
}

// TestImpactByFile_NoAnchorMeansEmptyDirect proves the bleed-free guarantee:
// a file with zero file-specific anchors must return an EMPTY DirectInvariants
// slice — not a silently-populated one because of package walks.
//
// Note: this test only asserts that DirectInvariants is empty. Verifying that
// the package-level invariant remains reachable as Inferred requires a
// realistic bleed vector (symbol cross-references, ownership chains) that's
// awkward to construct in a unit test. The build-verification step exercises
// the live 45k-node graph and asserts that real package-level invariants
// still surface as Inferred matches there — see the report after the
// `globular awareness build --clean` step.
func TestImpactByFile_NoAnchorMeansEmptyDirect(t *testing.T) {
	g := openGraph(t)
	ctx := context.Background()

	filePath := "golang/cluster_controller/cluster_controller_server/server.go"
	fileID := "source_file:" + filePath
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: filePath, Path: filePath})

	// Package + a package-level invariant. We connect it via protects→pkg
	// to verify it doesn't accidentally land in DirectInvariants (only
	// 1-hop file→implements/enforces/configures/observes→invariant qualifies).
	pkgID := "package:cluster_controller_server"
	_ = g.AddNode(ctx, graph.Node{ID: pkgID, Type: graph.NodeTypePackage, Name: "cluster-controller"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeOwns, Dst: pkgID})

	pkgInvID := "invariant:some.package.invariant"
	_ = g.AddNode(ctx, graph.Node{ID: pkgInvID, Type: graph.NodeTypeInvariant, Name: "some.package.invariant"})
	_ = g.AddEdge(ctx, graph.Edge{Src: pkgInvID, Kind: graph.EdgeProtects, Dst: pkgID})

	res, err := analysis.ImpactByFile(ctx, g, filePath)
	if err != nil {
		t.Fatalf("ImpactByFile: %v", err)
	}

	if len(res.DirectInvariants) != 0 {
		t.Errorf("file with no protects.files anchor must have empty DirectInvariants; got %s", idsOf(res.DirectInvariants))
	}
}

// TestImpactByFile_DirectFailureModesFollowDirectInvariants pins the 2-hop
// rule for failure_modes: a failure_mode is Direct iff at least one
// DirectInvariant lists it in related_failure_modes (which the loader emits
// as invariant→affects→failure_mode). This keeps the file-specific signal
// honest without expanding the surface artificially.
func TestImpactByFile_DirectFailureModesFollowDirectInvariants(t *testing.T) {
	g := openGraph(t)
	ctx := context.Background()

	filePath := "golang/cluster_controller/cluster_controller_server/release_pipeline.go"
	fileID := "source_file:" + filePath
	_ = g.AddNode(ctx, graph.Node{ID: fileID, Type: graph.NodeTypeSourceFile, Name: filePath, Path: filePath})

	directInvID := "invariant:release_pipeline.set_fields_routing_must_match_release_kind"
	_ = g.AddNode(ctx, graph.Node{ID: directInvID, Type: graph.NodeTypeInvariant, Name: "release_pipeline.set_fields_routing_must_match_release_kind"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeImplements, Dst: directInvID})

	directFMID := "failure_mode:release_pipeline.set_fields_silent_noop_on_wrong_kind"
	_ = g.AddNode(ctx, graph.Node{ID: directFMID, Type: graph.NodeTypeFailureMode, Name: "release_pipeline.set_fields_silent_noop_on_wrong_kind"})
	_ = g.AddEdge(ctx, graph.Edge{Src: directInvID, Kind: graph.EdgeAffects, Dst: directFMID})

	// An unrelated failure_mode reached only via the package — should be inferred.
	pkgID := "package:cluster_controller_server"
	_ = g.AddNode(ctx, graph.Node{ID: pkgID, Type: graph.NodeTypePackage, Name: "cluster-controller"})
	_ = g.AddEdge(ctx, graph.Edge{Src: fileID, Kind: graph.EdgeOwns, Dst: pkgID})

	inferredFMID := "failure_mode:reconcile.lane_starvation"
	_ = g.AddNode(ctx, graph.Node{ID: inferredFMID, Type: graph.NodeTypeFailureMode, Name: "reconcile.lane_starvation"})
	_ = g.AddEdge(ctx, graph.Edge{Src: pkgID, Kind: graph.EdgeAffects, Dst: inferredFMID})

	res, err := analysis.ImpactByFile(ctx, g, filePath)
	if err != nil {
		t.Fatalf("ImpactByFile: %v", err)
	}

	if !containsByID(res.DirectFailureModes, directFMID) {
		t.Errorf("DirectFailureModes must include %q (reached via DirectInvariant→affects→failure_mode); got Direct=%s",
			directFMID, idsOf(res.DirectFailureModes))
	}
	if containsByID(res.DirectFailureModes, inferredFMID) {
		t.Errorf("DirectFailureModes must NOT include %q (reached only via package); got Direct=%s",
			inferredFMID, idsOf(res.DirectFailureModes))
	}
}

// helpers — keep them out of the test bodies so the assertions read clean.

func containsByID(nodes []*graph.Node, id string) bool {
	for _, n := range nodes {
		if n != nil && n.ID == id {
			return true
		}
	}
	return false
}

func idsOf(nodes []*graph.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out = append(out, n.ID)
		}
	}
	return out
}

func firstIDOrEmpty(nodes []*graph.Node) string {
	if len(nodes) == 0 || nodes[0] == nil {
		return ""
	}
	return nodes[0].ID
}
