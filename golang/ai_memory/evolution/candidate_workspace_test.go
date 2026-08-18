package evolution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// candidateUnderTest builds a repo plus an envelope whose one required test
// writes the content of a tracked file into its evidence. That makes the
// evidence say which contents were actually executed, which is the property
// every case below is really about.
func candidateUnderTest(t *testing.T, name string) (workspace, revision, envelopePath string) {
	t.Helper()
	workspace, revision = initTestRepo(t)
	envelopePath = filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope(name, ChangeFeature, "feature", revision, RiskLow)
	e.RequiredTests = []TestRequirement{{
		Name:     "echo-contents",
		Command:  []string{"sh", "-c", "cat README.md"},
		Required: true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	return workspace, revision, envelopePath
}

func runCandidateTest(t *testing.T, workspace, envelopePath string) (TestRunResult, error) {
	t.Helper()
	return RunDeclaredTest(context.Background(), TestRunOptions{
		EnvelopePath: envelopePath,
		WorkspaceDir: workspace,
		TestName:     "echo-contents",
	})
}

func TestExactCleanCandidateIsAccepted(t *testing.T) {
	workspace, revision, envelopePath := candidateUnderTest(t, "chg-clean")
	result, err := runCandidateTest(t, workspace, envelopePath)
	if err != nil {
		t.Fatalf("pristine candidate rejected: %v", err)
	}
	if !result.MarkedProven || result.Record.Result != "PASS" {
		t.Fatalf("unexpected result: %+v", result)
	}
	// Evidence stays bound to the exact revision that was certified.
	if result.Record.CandidateRevision != revision {
		t.Fatalf("evidence revision %q is not the candidate %q", result.Record.CandidateRevision, revision)
	}
	evidence, err := os.ReadFile(result.Record.EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(evidence), "candidate") {
		t.Fatalf("evidence did not capture committed contents: %q", evidence)
	}
}

func TestModifiedTrackedFileIsRejectedBeforeProofExecution(t *testing.T) {
	workspace, revision, envelopePath := candidateUnderTest(t, "chg-dirty-tracked")
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCandidateTest(t, workspace, envelopePath)
	assertContaminationRejected(t, err, revision, "modified: README.md")
	assertNoEvidenceRecorded(t, envelopePath)
}

func TestStagedModificationIsRejectedBeforeProofExecution(t *testing.T) {
	workspace, revision, envelopePath := candidateUnderTest(t, "chg-dirty-staged")
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workspace, "add", "README.md")
	_, err := runCandidateTest(t, workspace, envelopePath)
	assertContaminationRejected(t, err, revision, "staged: README.md")
	assertNoEvidenceRecorded(t, envelopePath)
}

func TestUntrackedBuildAffectingFileCannotBeSilentlyCertified(t *testing.T) {
	// An untracked source file is not part of the candidate revision, so it must
	// never be certified as though it were. HEAD alone cannot see this.
	workspace, revision, envelopePath := candidateUnderTest(t, "chg-dirty-untracked")
	if err := os.WriteFile(filepath.Join(workspace, "injected.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCandidateTest(t, workspace, envelopePath)
	assertContaminationRejected(t, err, revision, "untracked: injected.go")
	assertNoEvidenceRecorded(t, envelopePath)
}

func TestProofExecutesCandidateContentsNotWorkspaceContents(t *testing.T) {
	// The clean-workspace guard rejects contamination, and the detached worktree
	// makes the executed contents exactly the candidate commit. This proves the
	// second half: after committing a later revision, proving the *earlier*
	// candidate must still see the earlier contents.
	workspace, firstRevision, envelopePath := candidateUnderTest(t, "chg-worktree")
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("second revision\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, workspace, "add", "README.md")
	runGit(t, workspace, "commit", "-m", "second")

	// HEAD has moved, so the pristine check must refuse before anything runs.
	if _, err := runCandidateTest(t, workspace, envelopePath); err == nil ||
		!strings.Contains(err.Error(), "does not match candidate revision") {
		t.Fatalf("expected revision mismatch, got %v", err)
	}

	// Return HEAD to the candidate; the workspace is clean again.
	runGit(t, workspace, "checkout", firstRevision)
	result, err := runCandidateTest(t, workspace, envelopePath)
	if err != nil {
		t.Fatalf("clean candidate rejected: %v", err)
	}
	evidence, err := os.ReadFile(result.Record.EvidenceRef)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(evidence), "second revision") {
		t.Fatalf("proof executed contents from another revision: %q", evidence)
	}
}

func TestCandidateWorktreeDoesNotDisturbTheOperatorCheckout(t *testing.T) {
	// Obtaining a clean environment must never be done by cleaning the user's
	// checkout. After a proof run the workspace must be byte-for-byte unchanged.
	workspace, _, envelopePath := candidateUnderTest(t, "chg-nondestructive")
	before, err := os.ReadFile(filepath.Join(workspace, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runCandidateTest(t, workspace, envelopePath); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(workspace, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("proof run modified the operator checkout")
	}
	// And it leaves no worktree registration behind.
	entries, err := os.ReadDir(filepath.Join(workspace, ".git", "worktrees"))
	if err == nil && len(entries) != 0 {
		t.Fatalf("candidate worktree was not released: %v", entries)
	}
}

func assertContaminationRejected(t *testing.T, err error, revision, want string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a contaminated workspace to be rejected before proof execution")
	}
	if !strings.Contains(err.Error(), "is not exactly revision "+revision) {
		t.Fatalf("expected exact-revision refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error naming %q, got %v", want, err)
	}
}

func assertNoEvidenceRecorded(t *testing.T, envelopePath string) {
	t.Helper()
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Tests) != 0 {
		t.Fatalf("a rejected run recorded evidence anyway: %+v", stored.Tests)
	}
	if stored.Stage != StageCandidate {
		t.Fatalf("expected stage CANDIDATE, got %s", stored.Stage)
	}
}
