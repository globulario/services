package rules

// Regression for the drift_stuck / missing-blob misattribution (2026-07-12):
// an out-of-band `pkg publish` left claude/codex/alertmanager with a cluster-wide
// PUBLISHED manifest but no blob in the local POSIX CAS. Nodes could resolve the
// manifest but never fetch the blob, so release.apply.package failed every cycle
// and workflow.drift_stuck escalated to CRITICAL — misattributing a repository
// artifact problem to a "stuck workflow". A missing_package drift whose artifact
// the repository itself reports as non-installable must be reclassified to a
// bounded WARN that names the repository cause, never a CRITICAL.

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

func dsHasSeverity(findings []Finding, sev cluster_doctorpb.Severity) bool {
	for _, f := range findings {
		if f.Severity == sev {
			return true
		}
	}
	return false
}

func TestDriftStuck_RepositoryBlocked_NotCritical(t *testing.T) {
	snap := &collector.Snapshot{
		DriftUnresolved: []*workflowpb.DriftUnresolved{
			{DriftType: "missing_package", EntityRef: "claude@node-a", ConsecutiveCycles: 1293, ChosenWorkflow: "release.apply.package"},
		},
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{Kind: "REPO_FIND_PUBLISHED_MISSING_BLOB", Name: "claude", ArtifactKey: "core@globular.io%claude%2.1.177%linux_amd64%1"},
		},
	}
	got := workflowDriftStuck{}.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("want exactly 1 finding, got %d", len(got))
	}
	if dsHasSeverity(got, cluster_doctorpb.Severity_SEVERITY_CRITICAL) {
		t.Error("repository-blocked missing_package must NOT escalate to CRITICAL drift_stuck")
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("want WARN, got %v", got[0].Severity)
	}
	if !strings.Contains(strings.ToLower(got[0].Summary), "repository") {
		t.Errorf("summary must name the repository cause, got: %q", got[0].Summary)
	}
}

func TestDriftStuck_NoRepositoryFinding_StillCritical(t *testing.T) {
	snap := &collector.Snapshot{
		DriftUnresolved: []*workflowpb.DriftUnresolved{
			{DriftType: "missing_package", EntityRef: "claude@node-a", ConsecutiveCycles: 1293, ChosenWorkflow: "release.apply.package"},
		},
	}
	got := workflowDriftStuck{}.Evaluate(snap, Config{})
	if !dsHasSeverity(got, cluster_doctorpb.Severity_SEVERITY_CRITICAL) {
		t.Error("a genuinely stuck drift with no repository cause must remain CRITICAL")
	}
}

func TestDriftStuck_UnrelatedRepositoryFinding_StillCritical(t *testing.T) {
	// The missing-blob finding is for claude; the stuck drift is mcp. mcp must
	// still be treated as a genuine stuck workflow (CRITICAL) — the reclassify
	// must match on package identity, not blanket-suppress all drift_stuck.
	snap := &collector.Snapshot{
		DriftUnresolved: []*workflowpb.DriftUnresolved{
			{DriftType: "missing_package", EntityRef: "mcp@node-b", ConsecutiveCycles: 1294, ChosenWorkflow: "release.apply.package"},
		},
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{Kind: "REPO_FIND_PUBLISHED_MISSING_BLOB", Name: "claude"},
		},
	}
	got := workflowDriftStuck{}.Evaluate(snap, Config{})
	if !dsHasSeverity(got, cluster_doctorpb.Severity_SEVERITY_CRITICAL) {
		t.Error("mcp drift must stay CRITICAL when only claude has a repository finding")
	}
}

func TestDriftPackageName(t *testing.T) {
	if got := driftPackageName("claude@eb9a2dac-05b0-52ac"); got != "claude" {
		t.Errorf("want claude, got %q", got)
	}
	if got := driftPackageName("no-at-sign"); got != "" {
		t.Errorf("want empty for no @, got %q", got)
	}
}

func TestArtifactKeyPackageName(t *testing.T) {
	// publisher itself contains '@' — must split on '%', not '@'.
	if got := artifactKeyPackageName("core@globular.io%claude%2.1.177%linux_amd64%1"); got != "claude" {
		t.Errorf("want claude, got %q", got)
	}
	if got := artifactKeyPackageName("garbage"); got != "" {
		t.Errorf("want empty for malformed key, got %q", got)
	}
}
