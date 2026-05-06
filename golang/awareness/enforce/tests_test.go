package enforce_test

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/awareness/enforce"
	"github.com/globulario/services/golang/awareness/graph"
)

// Test 8: tested_by pointing to existing test node → no findings.
func TestValidateRequiredTestsPresent(t *testing.T) {
	ctx := context.Background()
	g := openTestGraph(t)

	_ = g.AddNode(ctx, graph.Node{ID: "symbol:pkg.DoWork", Type: graph.NodeTypeSymbol, Name: "DoWork"})
	_ = g.AddNode(ctx, graph.Node{ID: "test:TestDoWorkBehavior", Type: graph.NodeTypeTest, Name: "TestDoWorkBehavior", Path: "pkg/work_test.go"})
	_ = g.AddEdge(ctx, graph.Edge{Src: "symbol:pkg.DoWork", Kind: graph.EdgeTestedBy, Dst: "test:TestDoWorkBehavior"})

	findings := enforce.ValidateRequiredTests(ctx, g)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got: %v", findings)
	}
}

// Test 9: tested_by pointing to non-existent test node → ERROR.
func TestValidateRequiredTestsMissing(t *testing.T) {
	ctx := context.Background()
	g := openTestGraph(t)

	_ = g.AddNode(ctx, graph.Node{ID: "symbol:pkg.DoWork", Type: graph.NodeTypeSymbol, Name: "DoWork"})
	// Add the edge but NOT the test node.
	_ = g.AddEdge(ctx, graph.Edge{Src: "symbol:pkg.DoWork", Kind: graph.EdgeTestedBy, Dst: "test:TestDoWorkBehavior"})

	findings := enforce.ValidateRequiredTests(ctx, g)
	found := false
	for _, f := range findings {
		if f.Code == "REQUIRED_TEST_MISSING" && f.Severity == enforce.SeverityError {
			found = true
		}
	}
	if !found {
		t.Errorf("expected REQUIRED_TEST_MISSING ERROR, got: %v", findings)
	}
}
