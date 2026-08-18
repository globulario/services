package evolution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A scenario obligation names a specific file in the frozen plan. Another file
// declaring the same name — what a moved or copied scenario leaves behind — must
// not be able to discharge it, because the plan digest covers the path.
func TestScenarioPathMustMatchTheFrozenRequirement(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	envelopePath := scenarioEnvelope(t, "chg-path-swap")

	_, err := RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstart,
		Scenario:      filepath.Join(quickstart, "tests", "scenarios", "chaos-moved.yaml"),
		ScenarioName:  "chaos",
		EnvelopePath:  envelopePath,
	})
	if err == nil {
		t.Fatal("a scenario file outside the frozen plan discharged the obligation")
	}
	if !strings.Contains(err.Error(), "frozen") && !strings.Contains(err.Error(), "declared") {
		t.Fatalf("expected a frozen-plan refusal, got %v", err)
	}
	stored, loadErr := LoadChangeEnvelope(envelopePath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if stored.Stage == StageProven {
		t.Fatal("envelope reached PROVEN without executing the plan-digested path")
	}
}

// An unknown scenario name has no obligation to discharge at all.
func TestUndeclaredScenarioNameIsRefused(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	envelopePath := scenarioEnvelope(t, "chg-unknown-name")

	_, err := RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstart,
		Scenario:      filepath.Join(quickstart, "tests", "scenarios", "chaos.yaml"),
		ScenarioName:  "not-in-the-plan",
		EnvelopePath:  envelopePath,
	})
	if err == nil {
		t.Fatal("a scenario name absent from the plan was accepted")
	}
}

// Artifact verification belongs to the transition that sets PROVEN, not to the
// full-plan wrapper. Any durable mutation must re-derive the recorded digests.
func TestDurableTransitionVerifiesArtifactsBeforeProven(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	envelopePath := scenarioEnvelope(t, "chg-artifact-gone")

	proven, err := runScenario(t, quickstart, envelopePath)
	if err != nil || !proven.MarkedProven {
		t.Fatalf("setup run should prove: %+v err=%v", proven, err)
	}

	// The scenario's proof artifact disappears after the fact.
	if err := os.Remove(proven.ProofPath); err != nil {
		t.Fatal(err)
	}

	// A mutation through the durable owner — the shape `evolution-run-test`
	// takes when the last required test is rerun — must not leave PROVEN standing
	// on evidence that can no longer be reproduced.
	current, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	marked, err := MutateEnvelope(envelopePath, current.Identity(), func(e *ChangeEnvelope) {})
	if err != nil {
		t.Fatalf("mutation should demote rather than fail: %v", err)
	}
	if marked {
		t.Fatal("transition reported PROVEN with an unreadable proof artifact")
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Stage == StageProven {
		t.Fatal("PROVEN survived deletion of the proof artifact")
	}
}

// Every failure after the command has run must withdraw the standing claim, not
// only the failures that happen before it launches.
func TestUnwritableEvidenceWithdrawsStandingProof(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; filesystem permissions are not enforced")
	}
	workspace, revision, envelopePath := candidateUnderTest(t, "chg-evidence-unwritable")
	first, err := runCandidateTest(t, workspace, envelopePath)
	if err != nil || !first.MarkedProven {
		t.Fatalf("first run should prove: %+v err=%v", first, err)
	}
	_ = revision

	// The evidence log becomes unwritable between runs, so this invocation can
	// execute the command but cannot persist what it observed.
	if err := os.Chmod(first.Record.EvidenceRef, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(first.Record.EvidenceRef, 0o644) })
	if f, err := os.OpenFile(first.Record.EvidenceRef, os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		_ = f.Close()
		t.Skip("evidence log is still writable; cannot exercise the failure")
	}

	if _, err := runCandidateTest(t, workspace, envelopePath); err == nil {
		t.Fatal("a run that could not persist evidence reported success")
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Stage == StageProven {
		t.Fatal("PROVEN survived a run that produced no durable evidence")
	}
	for _, record := range stored.Tests {
		if record.Name == "echo-contents" && record.Result == "PASS" {
			t.Fatalf("the superseded PASS survived: %+v", record)
		}
	}
}

// The full-plan demotion must go through the same locked transaction as every
// other mutation, so it cannot overwrite a concurrent delta.
func TestPlanDemotionRefusesAMovedEnvelope(t *testing.T) {
	envelopePath := scenarioEnvelope(t, "chg-demote-moved")
	stale, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	staleIdentity := stale.Identity()

	rebound, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := rebound.BindCandidate("globulario/services", "a-different-candidate"); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, rebound); err != nil {
		t.Fatal(err)
	}

	if err := withdrawUnreproducibleProof(envelopePath, staleIdentity); err == nil {
		t.Fatal("demotion was applied to a different candidate")
	}
}
