package enforce_test

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/enforce"
)

// Test 12: RenderAuditMarkdown includes severity counts and finding codes.
func TestRenderAuditMarkdown(t *testing.T) {
	r := &enforce.AuditResult{
		Findings: []enforce.Finding{
			{Code: "MALFORMED_STATE_TRANSITION", Severity: enforce.SeverityError, File: "pkg/foo.go", Message: "bad transition"},
			{Code: "MISSING_HASH_CONSUMER", Severity: enforce.SeverityWarning, Message: "no consumer"},
		},
		ErrorCount:   1,
		WarningCount: 1,
		Pass:         false,
	}

	md := enforce.RenderAuditMarkdown(r)
	if !strings.Contains(md, "FAIL") {
		t.Error("expected FAIL in markdown")
	}
	if !strings.Contains(md, "MALFORMED_STATE_TRANSITION") {
		t.Error("expected MALFORMED_STATE_TRANSITION in markdown")
	}
	if !strings.Contains(md, "MISSING_HASH_CONSUMER") {
		t.Error("expected MISSING_HASH_CONSUMER in markdown")
	}
}

// Test: RenderAuditJSON is valid JSON and includes pass/findings.
func TestRenderAuditJSON(t *testing.T) {
	r := &enforce.AuditResult{
		Findings:     []enforce.Finding{},
		ErrorCount:   0,
		WarningCount: 0,
		Pass:         true,
	}

	jsonStr := enforce.RenderAuditJSON(r)
	if !strings.Contains(jsonStr, `"pass": true`) {
		t.Errorf("expected pass:true in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"findings"`) {
		t.Error("expected findings key in JSON")
	}
}
