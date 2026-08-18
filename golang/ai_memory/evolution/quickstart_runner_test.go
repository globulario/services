package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHarness stands in for globulario/globular-quickstart's globular-test. It
// reproduces the only contract the runner depends on: a proof artifact written
// into the run directory the caller named, stamped with the invocation identity
// the caller minted.
type fakeHarness struct {
	// mode selects the failure being reproduced.
	//   "pass"            — writes a valid PASS proof for this invocation
	//   "fail-silently"   — exits nonzero writing nothing, as a run that dies
	//                       before it can rotate any report pointer does
	//   "wrong-invocation" — writes a proof stamped with someone else's run
	//   "no-invocation"   — writes a proof with no invocation identity at all
	mode string
	// scenario overrides the scenario name the artifact claims to answer.
	scenario string
}

func writeQuickstartHarness(t *testing.T, quickstartDir string, h fakeHarness) {
	t.Helper()
	binDir := filepath.Join(quickstartDir, "tests", "harness", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	scenario := h.scenario
	if scenario == "" {
		scenario = "chaos"
	}
	script := `#!/bin/sh
set -e
MODE="` + h.mode + `"
SCENARIO="` + scenario + `"
if [ "$MODE" = "pass-then-exit-nonzero" ]; then
  PASS_THEN_FAIL=1
else
  PASS_THEN_FAIL=0
fi
if [ "$MODE" = "fail-silently" ]; then
  # Dies before producing anything for this invocation, and before it could
  # rotate any shared pointer.
  exit 1
fi
# Mirror the real harness: source_revision comes from the tree being executed,
# which under the runner is a detached worktree of the exact simulator revision.
SRC_REV="$(git rev-parse HEAD)"
INV="$GLOBULAR_PROOF_INVOCATION_ID"
if [ "$MODE" = "wrong-invocation" ]; then INV="inv-someone-else"; fi
if [ "$MODE" = "no-invocation" ]; then INV=""; fi
mkdir -p "$GLOBULAR_PROOF_RUN_DIR"
cat > "$GLOBULAR_PROOF_RUN_DIR/evidence.json" <<EOF
{"passed": true}
EOF
cat > "$GLOBULAR_PROOF_RUN_DIR/scenario-proof.json" <<EOF
{
  "scenario": "$SCENARIO",
  "suite": "resilience",
  "source_revision": "$SRC_REV",
  "status": "SUPPORTED",
  "change": {
    "id": "$GLOBULAR_CHANGE_ID",
    "candidate_repository": "$GLOBULAR_CANDIDATE_REPOSITORY",
    "candidate_revision": "$GLOBULAR_CANDIDATE_REVISION",
    "plan_digest": "$GLOBULAR_CHANGE_PLAN_DIGEST",
    "simulation_revision": "$SRC_REV"
  },
  "invocation": {"id": "$INV", "run_dir": "$GLOBULAR_PROOF_RUN_DIR"},
  "execution": {"result": "PASS", "proof_eligible": true}
}
EOF
if [ "$PASS_THEN_FAIL" = "1" ]; then
  # Scenario itself passed; teardown/cleanup failed afterwards.
  echo "teardown failed" >&2
  exit 7
fi
`
	path := filepath.Join(binDir, "globular-test")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The simulator now executes from a detached worktree of its own committed
	// revision, so the fixture has to be a real repository and each harness
	// variant has to be committed to take effect.
	scenarioDir := filepath.Join(quickstartDir, "tests", "scenarios")
	if err := os.MkdirAll(scenarioDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scenarioDir, "chaos.yaml"), []byte("name: chaos\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A second committed file that also declares the name "chaos" — the shape a
	// moved or copied scenario leaves behind.
	if err := os.WriteFile(filepath.Join(scenarioDir, "chaos-moved.yaml"), []byte("name: chaos\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(quickstartDir, ".git")); err != nil {
		runGit(t, quickstartDir, "init")
		runGit(t, quickstartDir, "config", "user.email", "evolution-test@example.invalid")
		runGit(t, quickstartDir, "config", "user.name", "Evolution Test")
	}
	runGit(t, quickstartDir, "add", "-A")
	runGit(t, quickstartDir, "commit", "-m", "simulator "+h.mode+" "+scenario, "--allow-empty")
}

func scenarioEnvelope(t *testing.T, id string) (envelopePath string) {
	t.Helper()
	envelopePath = filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope(id, ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.RequiredScenarios = []ScenarioRequirement{{
		Name:       "chaos",
		Repository: "globulario/globular-quickstart",
		Path:       "tests/scenarios/chaos.yaml",
		Required:   true,
	}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	return envelopePath
}

func runScenario(t *testing.T, quickstartDir, envelopePath string) (QuickstartRunResult, error) {
	t.Helper()
	return RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstartDir,
		Scenario:      filepath.Join(quickstartDir, "tests", "scenarios", "chaos.yaml"),
		ScenarioName:  "chaos",
		EnvelopePath:  envelopePath,
	})
}

// plantStaleReport writes a previous successful run and points the shared
// `latest` symlink at it — the exact state that used to let an earlier PASS be
// read back as though it were the current result.
func plantStaleReport(t *testing.T, quickstartDir, changeID, planDigest string) string {
	t.Helper()
	reports := filepath.Join(quickstartDir, "tests", "reports")
	stale := filepath.Join(reports, "20260101T000000-chaos")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	proof := map[string]any{
		"scenario":        "chaos",
		"suite":           "resilience",
		"source_revision": "stale-sim-sha",
		"status":          "SUPPORTED",
		"change": map[string]string{
			"id":                   changeID,
			"candidate_repository": "globulario/services",
			"candidate_revision":   "candidate-sha",
			"plan_digest":          planDigest,
			"simulation_revision":  "stale-sim-sha",
		},
		"invocation": map[string]string{"id": "inv-stale", "run_dir": stale},
		"execution":  map[string]any{"result": "PASS", "proof_eligible": true},
	}
	data, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "scenario-proof.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "evidence.json"), []byte(`{"passed":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("20260101T000000-chaos", filepath.Join(reports, "latest")); err != nil {
		t.Fatal(err)
	}
	return stale
}

func TestScenarioProofFromCurrentInvocationIsAccepted(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	envelopePath := scenarioEnvelope(t, "chg-inv-ok")

	result, err := runScenario(t, quickstart, envelopePath)
	if err != nil {
		t.Fatalf("current-invocation proof rejected: %v", err)
	}
	if !result.MarkedProven || result.Proof.Result != "PASS" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Proof.InvocationID != result.InvocationID || result.InvocationID == "" {
		t.Fatalf("proof is not bound to this invocation: %+v", result)
	}
	// The consumed artifact lives in this invocation's own directory, beside the
	// envelope's other evidence — deliberately outside both checkouts, since the
	// simulator worktree is discarded after the run.
	if filepath.Base(filepath.Dir(result.ProofPath)) != result.InvocationID {
		t.Fatalf("proof was not read from this invocation's run dir: %s", result.ProofPath)
	}
	if strings.HasPrefix(result.ProofPath, quickstart+string(filepath.Separator)) {
		t.Fatalf("proof artifact was written inside the simulator checkout: %s", result.ProofPath)
	}
	// All prior bindings survive.
	if result.Proof.CandidateRevision != "candidate-sha" ||
		result.Proof.Repository != "globulario/globular-quickstart" ||
		result.Proof.Digest == "" {
		t.Fatalf("proof lost a required binding: %+v", result.Proof)
	}
	// The simulation revision is now the exact committed simulator the run
	// executed from, not a value the artifact was free to assert.
	simHead := strings.TrimSpace(mustGitOutput(t, quickstart, "rev-parse", "HEAD"))
	if result.Proof.SimulationRevision != simHead {
		t.Fatalf("proof simulation revision %q is not the simulator revision %q",
			result.Proof.SimulationRevision, simHead)
	}
}

func TestFailingRunCannotReusePreviousPass(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	envelopePath := scenarioEnvelope(t, "chg-inv-rerun")

	first, err := runScenario(t, quickstart, envelopePath)
	if err != nil || !first.MarkedProven {
		t.Fatalf("first run should prove: %+v err=%v", first, err)
	}

	// The rerun dies before producing anything. The previous PASS is still on
	// disk and `latest` still points at it, so nothing but invocation identity
	// stops it from being read back.
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "fail-silently"})
	if _, err := runScenario(t, quickstart, envelopePath); err == nil {
		t.Fatal("a run that produced no proof must not succeed")
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Stage != StageCandidate {
		t.Fatalf("failed rerun left a stale PROVEN claim standing: %s", stored.Stage)
	}
	if len(stored.Proofs) != 0 {
		t.Fatalf("failed rerun kept the previous proof: %+v", stored.Proofs)
	}
}

func TestStaleLatestReportIsNeverConsumed(t *testing.T) {
	quickstart := t.TempDir()
	envelopePath := scenarioEnvelope(t, "chg-inv-stale")
	planned, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	// The stale report is fully valid on every binding the envelope checks —
	// change id, candidate, frozen plan, simulation revision. Only invocation
	// identity distinguishes it from the current run, which is the point.
	stale := plantStaleReport(t, quickstart, "chg-inv-stale", planned.PlanDigest)
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "fail-silently"})

	result, runErr := runScenario(t, quickstart, envelopePath)
	if runErr == nil {
		t.Fatal("expected the stale report to be unreachable, not consumed")
	}
	if strings.HasPrefix(result.ProofPath, stale) {
		t.Fatalf("runner read the stale `latest` report: %s", result.ProofPath)
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Stage == StageProven {
		t.Fatal("a stale report certified the candidate as PROVEN")
	}
}

func TestMismatchedInvocationIsRejected(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "wrong-invocation"})
	envelopePath := scenarioEnvelope(t, "chg-inv-mismatch")

	_, err := runScenario(t, quickstart, envelopePath)
	if err == nil || !strings.Contains(err.Error(), "does not belong to this run") {
		t.Fatalf("expected invocation mismatch rejection, got %v", err)
	}
}

func TestProofWithoutInvocationIdentityIsRejected(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "no-invocation"})
	envelopePath := scenarioEnvelope(t, "chg-inv-absent")

	_, err := runScenario(t, quickstart, envelopePath)
	if err == nil || !strings.Contains(err.Error(), "no invocation identity") {
		t.Fatalf("expected unstamped proof to be rejected, got %v", err)
	}
}

func TestSiblingRunArtifactCannotSatisfyThisInvocation(t *testing.T) {
	// A concurrent run for the same change writes a complete, genuinely valid
	// PASS proof — into its own directory. This invocation must not be able to
	// reach it, whatever the shared pointers say.
	quickstart := t.TempDir()
	envelopePath := scenarioEnvelope(t, "chg-inv-sibling")
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	sibling, err := runScenario(t, quickstart, envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	siblingDir := filepath.Dir(sibling.ProofPath)

	// Now a run that produces nothing of its own.
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "fail-silently"})
	result, err := runScenario(t, quickstart, envelopePath)
	if err == nil {
		t.Fatal("expected the barren invocation to fail")
	}
	if filepath.Dir(result.ProofPath) == siblingDir {
		t.Fatal("invocation consumed a sibling run's proof")
	}
	if _, statErr := os.Stat(filepath.Join(siblingDir, "scenario-proof.json")); statErr != nil {
		t.Fatalf("sibling artifact should be untouched: %v", statErr)
	}
}

func TestEachInvocationGetsItsOwnRunDirectory(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		envelopePath := scenarioEnvelope(t, fmt.Sprintf("chg-inv-unique-%d", i))
		result, err := runScenario(t, quickstart, envelopePath)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Dir(result.ProofPath)
		if seen[dir] {
			t.Fatalf("two invocations shared a run directory: %s", dir)
		}
		seen[dir] = true
	}
}

func mustGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitOutput(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A harness can emit a fully valid, correctly stamped PASS artifact and then
// exit nonzero because teardown or cleanup failed. The run did not succeed, so
// it must not certify — and it must not leave a previous PROVEN standing.
func TestPassArtifactWithNonzeroExitDoesNotCertify(t *testing.T) {
	quickstart := t.TempDir()
	envelopePath := scenarioEnvelope(t, "chg-teardown-fail")

	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	first, err := runScenario(t, quickstart, envelopePath)
	if err != nil || !first.MarkedProven {
		t.Fatalf("first run should prove: %+v err=%v", first, err)
	}

	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass-then-exit-nonzero"})
	result, err := runScenario(t, quickstart, envelopePath)
	if err == nil {
		t.Fatal("a run that exited nonzero certified the candidate anyway")
	}
	if result.ExitCode == 0 {
		t.Fatalf("expected the harness exit code to be reported, got %+v", result)
	}
	stored, loadErr := LoadChangeEnvelope(envelopePath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if stored.Stage == StageProven {
		t.Fatal("durable envelope stayed PROVEN after a failed run")
	}
	for _, p := range stored.Proofs {
		if p.Scenario == "chaos" && p.Result == "PASS" && p.ProofEligible {
			t.Fatalf("the superseded PASS survived a nonzero-exit rerun: %+v", p)
		}
	}
}

// A concurrent success must not write back a snapshot that predates another
// invocation's withdrawal, which would resurrect the proof that was invalidated.
func TestConcurrentSuccessCannotResurrectWithdrawnProof(t *testing.T) {
	quickstart := t.TempDir()
	envelopePath := scenarioEnvelope(t, "chg-concurrent")
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})

	proven, err := runScenario(t, quickstart, envelopePath)
	if err != nil || !proven.MarkedProven {
		t.Fatalf("setup run should prove: %+v err=%v", proven, err)
	}
	before, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if before.Stage != StageProven {
		t.Fatalf("expected PROVEN setup, got %s", before.Stage)
	}
	identity := before.Identity()

	// A concurrent invocation withdraws the standing proof.
	if err := invalidateScenarioProof(envelopePath, identity, "chaos"); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn.Stage == StageProven {
		t.Fatal("withdrawal did not demote the envelope")
	}

	// An invocation holding the pre-withdrawal snapshot now commits an unrelated
	// delta. Going through the one owner, it reloads current state first, so the
	// withdrawal survives instead of being overwritten.
	if _, err := MutateEnvelope(envelopePath, identity, func(e *ChangeEnvelope) {
		e.Learning = append(e.Learning, ArtifactRef{Kind: "note", URI: "concurrent"})
	}); err != nil {
		t.Fatal(err)
	}
	after, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stage == StageProven {
		t.Fatal("a concurrent write resurrected the withdrawn proof")
	}
	for _, p := range after.Proofs {
		if p.Scenario == "chaos" {
			t.Fatalf("withdrawn proof came back: %+v", p)
		}
	}
}

// A mutation must not land on an envelope that has moved to a different
// candidate or plan while the invocation was running.
func TestMutationRefusesAMovedEnvelope(t *testing.T) {
	envelopePath := scenarioEnvelope(t, "chg-moved")
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

	if _, err := MutateEnvelope(envelopePath, staleIdentity, func(e *ChangeEnvelope) {
		e.Proofs = append(e.Proofs, ProofRecord{Scenario: "chaos", Result: "PASS"})
	}); err == nil {
		t.Fatal("a result was applied to a different candidate")
	}
}
