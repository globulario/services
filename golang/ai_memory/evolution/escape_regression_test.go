package evolution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSymlinkedWorkDirCannotEscapeCandidateTree proves the isolation guarantee
// holds against the candidate's own contents. A commit may legitimately contain
// symlinks, so a declared work_dir that resolves outside the tree would let the
// candidate run its declared command against arbitrary external contents and
// still receive PASS evidence stamped with the candidate revision.
func TestSymlinkedWorkDirCannotEscapeCandidateTree(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "marker.txt"), []byte("OUTSIDE-THE-CANDIDATE\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	workspace := t.TempDir()
	runGit(t, workspace, "init")
	runGit(t, workspace, "config", "user.email", "evolution-test@example.invalid")
	runGit(t, workspace, "config", "user.name", "Evolution Test")
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("candidate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A symlink committed into the candidate itself.
	if err := os.Symlink(outside, filepath.Join(workspace, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	runGit(t, workspace, "add", "-A")
	runGit(t, workspace, "commit", "-m", "candidate with symlink")
	revision := headRevision(t, workspace)

	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-symlink-escape", ChangeFeature, "feature", revision, RiskLow)
	e.RequiredTests = []TestRequirement{{
		Name:     "escaping-test",
		WorkDir:  "escape",
		Command:  []string{"sh", "-c", "cat marker.txt || true"},
		Required: true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}

	result, err := RunDeclaredTest(context.Background(), TestRunOptions{
		EnvelopePath: envelopePath,
		WorkspaceDir: workspace,
		TestName:     "escaping-test",
	})
	if err == nil {
		evidence, readErr := os.ReadFile(result.Record.EvidenceRef)
		if readErr == nil && strings.Contains(string(evidence), "OUTSIDE-THE-CANDIDATE") {
			t.Fatalf("work_dir symlink escaped the candidate tree and produced evidence from %s", outside)
		}
		t.Fatalf("expected a symlinked work_dir escape to be refused, got result %+v", result)
	}
	if !strings.Contains(err.Error(), "escapes candidate workspace") {
		t.Fatalf("expected containment refusal, got %v", err)
	}
}

// TestProofForADifferentScenarioIsRejected proves a rerun cannot be satisfied by
// an artifact that answers a different obligation. The old PASS for the
// requested scenario would otherwise keep closure green while this invocation
// produced no proof for it at all.
func TestProofForADifferentScenarioIsRejected(t *testing.T) {
	quickstart := t.TempDir()
	envelopePath := scenarioEnvelope(t, "chg-scenario-swap")

	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	first, err := runScenario(t, quickstart, envelopePath)
	if err != nil || !first.MarkedProven {
		t.Fatalf("first run should prove: %+v err=%v", first, err)
	}

	// The rerun returns a properly stamped, otherwise valid PASS — for a
	// different scenario than the one requested.
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass", scenario: "some-other-scenario"})
	if _, err := runScenario(t, quickstart, envelopePath); err == nil {
		t.Fatal("a proof for a different scenario must not satisfy this obligation")
	}
	stored, loadErr := LoadChangeEnvelope(envelopePath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if stored.Stage == StageProven {
		t.Fatalf("stale PASS for %q still certifies the candidate", "chaos")
	}
	for _, p := range stored.Proofs {
		if p.Scenario == "chaos" && p.Result == "PASS" && p.ProofEligible {
			t.Fatalf("the superseded proof survived the mismatched rerun: %+v", p)
		}
	}
}

func headRevision(t *testing.T, dir string) string {
	t.Helper()
	out, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(out)
}
