package enforce

import (
	"context"

	"github.com/globulario/services/golang/awareness/graph"
)

// ValidateRequiredTests checks that every test declared via //globular:tested_by
// exists in the graph as a test node created by the Go test extractor.
//
// A tested_by edge that points to a non-existent test node → ERROR (test missing).
// The test node exists but has no source file path → WARNING (unverified location).
func ValidateRequiredTests(ctx context.Context, g *graph.Graph) []Finding {
	if g == nil {
		return nil
	}

	// Collect all tested_by edges.
	edges, err := g.EdgesByKind(ctx, graph.EdgeTestedBy)
	if err != nil {
		return []Finding{{
			Code:     "TEST_QUERY_ERROR",
			Severity: SeverityError,
			Message:  "failed to query tested_by edges: " + err.Error(),
		}}
	}

	var findings []Finding
	for _, e := range edges {
		testNode, err := g.FindNode(ctx, e.Dst)
		if err != nil {
			findings = append(findings, Finding{
				Code:     "REQUIRED_TEST_LOOKUP_ERROR",
				Severity: SeverityWarning,
				Symbol:   e.Src,
				Message:  "tested_by lookup failed for " + e.Dst + ": " + err.Error(),
			})
			continue
		}
		if testNode == nil {
			findings = append(findings, Finding{
				Code:     CodeRequiredTestMissing,
				Severity: SeverityError,
				Symbol:   e.Src,
				Message:  "tested_by target '" + e.Dst + "' does not exist in the graph — add a test function named " + stripPrefix(e.Dst, "test:"),
			})
			continue
		}
		if testNode.Path == "" {
			findings = append(findings, Finding{
				Code:     "REQUIRED_TEST_NO_PATH",
				Severity: SeverityWarning,
				Symbol:   e.Src,
				Message:  "tested_by target '" + testNode.Name + "' declared but not yet implemented — add func " + testNode.Name + "(t *testing.T) to a *_test.go file",
			})
		}
	}

	return findings
}

// stripPrefix removes a prefix from s. Returns s unchanged if prefix not present.
func stripPrefix(s, prefix string) string {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}
