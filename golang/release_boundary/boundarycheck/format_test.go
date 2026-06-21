package boundarycheck

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/release_boundary"
)

// Exit-code semantics: PROVEN → 0, everything else → non-zero. This is the
// contract the CLI relies on (PROVEN is the only success).
func TestExitCode(t *testing.T) {
	cases := []struct {
		v        release_boundary.Verdict
		wantZero bool
	}{
		{release_boundary.VerdictProven, true},
		{release_boundary.VerdictFailed, false},
		{release_boundary.VerdictIndeterminate, false},
		{release_boundary.VerdictNotApplicable, false},
	}
	for _, c := range cases {
		got := ExitCode(c.v)
		if (got == 0) != c.wantZero {
			t.Errorf("ExitCode(%s) = %d, wantZero=%v", c.v, got, c.wantZero)
		}
	}
}

// FormatReport must expose every assertion (A0..A4) and the verdict.
func TestFormatReport_IncludesAllAssertions(t *testing.T) {
	report := release_boundary.Evaluate(MapInputs("globular/echo", "globule-ryzen", validEvidence()))
	out := FormatReport(report, nil)

	if !strings.Contains(out, "PROVEN") {
		t.Errorf("output missing overall verdict:\n%s", out)
	}
	for _, id := range []string{"A0", "A1", "A2", "A3", "A4"} {
		if !strings.Contains(out, id) {
			t.Errorf("output missing assertion %s:\n%s", id, out)
		}
	}
}

// FormatReport must surface collection errors (connection errors not absorbed).
func TestFormatReport_IncludesCollectionErrors(t *testing.T) {
	ev := validEvidence()
	ev.addErr("runtime", "node agent unreachable")
	report := release_boundary.Evaluate(MapInputs("globular/echo", "globule-ryzen", ev))
	out := FormatReport(report, ev)

	if !strings.Contains(out, "collection_errors") || !strings.Contains(out, "node agent unreachable") {
		t.Errorf("output missing collection errors:\n%s", out)
	}
}

// ReportToMap carries provenance git SHA and collection errors into the envelope.
func TestReportToMap_Envelope(t *testing.T) {
	ev := validEvidence()
	ev.Manifest.Provenance = nil // no provenance → key omitted
	ev.addErr("verify", "boom")
	report := release_boundary.Evaluate(MapInputs("globular/echo", "n1", ev))
	m := ReportToMap(report, ev)

	if m["verdict"] == "" {
		t.Error("envelope missing verdict")
	}
	if _, ok := m["collection_errors"]; !ok {
		t.Error("envelope missing collection_errors")
	}
	if _, ok := m["assertions"]; !ok {
		t.Error("envelope missing assertions")
	}
}
