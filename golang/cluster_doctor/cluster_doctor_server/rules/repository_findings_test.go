package rules

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

func TestRepositoryFindings_PublishedMissingBlob(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{
				Kind: "REPO_FIND_PUBLISHED_MISSING_BLOB",
				Severity: "REPO_FIND_CRITICAL",
				ArtifactKey: "core@globular.io%echo%1.0.0%linux_amd64%1",
				PublisherID: "core@globular.io", Name: "echo", Version: "1.0.0",
				Platform: "linux_amd64",
				CurrentState: "PUBLISHED", ExpectedState: "PUBLISHED + blob present",
				Reason: "PUBLISHED row but missing_blob",
				RecommendedCommand: "globular repository repair core@globular.io/echo 1.0.0 --platform linux_amd64",
			},
		},
	}
	out := repositoryFindings{}.Evaluate(snap, Config{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	f := out[0]
	if f.InvariantID != "repository.published_missing_blob" {
		t.Errorf("invariant id: got %s, want repository.published_missing_blob", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("severity: got %s, want ERROR", f.Severity)
	}
	if !strings.Contains(f.Summary, "echo@1.0.0") {
		t.Errorf("summary should include package id, got %q", f.Summary)
	}
	if len(f.Remediation) == 0 || f.Remediation[0].GetCliCommand() == "" {
		t.Error("expected remediation step with CLI command")
	}
}

func TestRepositoryFindings_RevokedInstallable(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{
				Kind: "REPO_FIND_REVOKED_INSTALLABLE",
				Severity: "REPO_FIND_CRITICAL",
				ArtifactKey: "k", PublisherID: "p", Name: "n",
				Version: "v", Platform: "linux_amd64",
				Reason: "REVOKED row has stale publish_state=PUBLISHED",
				RecommendedCommand: "globular repository artifact revoke p/n v --platform linux_amd64",
			},
		},
	}
	out := repositoryFindings{}.Evaluate(snap, Config{})
	if len(out) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out))
	}
	if out[0].InvariantID != "repository.revoked_installable" {
		t.Errorf("invariant id: got %s, want repository.revoked_installable", out[0].InvariantID)
	}
}

func TestRepositoryFindings_PublishedUnsignedRequired(t *testing.T) {
	snap := &collector.Snapshot{
		RepositoryFindings: []*collector.RepositoryFindingSnapshot{
			{
				Kind: "REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED",
				Severity: "REPO_FIND_CRITICAL",
				ArtifactKey: "k", PublisherID: "core@globular.io", Name: "echo",
				Reason: "no signatures registered for artifact",
				RecommendedCommand: "globular repository signature verify core@globular.io/echo 1.0.0",
			},
		},
	}
	out := repositoryFindings{}.Evaluate(snap, Config{})
	if len(out) != 1 || out[0].InvariantID != "repository.published_unsigned_required" {
		t.Fatalf("expected published_unsigned_required, got %v", out)
	}
}

func TestRepositoryFindings_NilSnapshot(t *testing.T) {
	if out := (repositoryFindings{}).Evaluate(nil, Config{}); len(out) != 0 {
		t.Fatalf("nil snapshot must yield 0 findings, got %d", len(out))
	}
}

func TestRepositoryFindings_EmptyFindings(t *testing.T) {
	snap := &collector.Snapshot{}
	if out := (repositoryFindings{}).Evaluate(snap, Config{}); len(out) != 0 {
		t.Fatalf("empty findings must yield 0 doctor findings, got %d", len(out))
	}
}
