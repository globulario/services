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

// Rebinding is a durable read-modify-write like any other, so it must not be the
// one path that writes a snapshot read before the change.
func TestRebindRefusesAStaleSnapshot(t *testing.T) {
	envelopePath := scenarioEnvelope(t, "chg-rebind-stale")
	stale, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	staleIdentity := stale.Identity()

	// Something else moves the envelope first.
	moved, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RebindCandidate(envelopePath, moved.Identity(), "globulario/services", "revision-two"); err != nil {
		t.Fatal(err)
	}

	// The holder of the pre-move snapshot must be refused, not merged.
	if _, err := RebindCandidate(envelopePath, staleIdentity, "globulario/services", "revision-three"); err == nil {
		t.Fatal("a rebind was applied on top of a snapshot that no longer described the envelope")
	}
	current, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if current.CandidateRevision != "revision-two" {
		t.Fatalf("stale rebind overwrote the newer candidate: %s", current.CandidateRevision)
	}
}

// A rebind still clears candidate-derived evidence, under the lock.
func TestRebindThroughTheTransactionStillClearsEvidence(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	envelopePath := scenarioEnvelope(t, "chg-rebind-clears")

	proven, err := runScenario(t, quickstart, envelopePath)
	if err != nil || !proven.MarkedProven {
		t.Fatalf("setup run should prove: %+v err=%v", proven, err)
	}
	before, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	rebound, err := RebindCandidate(envelopePath, before.Identity(), "globulario/services", "a-new-candidate")
	if err != nil {
		t.Fatal(err)
	}
	if len(rebound.Proofs) != 0 || rebound.Stage != StageCandidate {
		t.Fatalf("proof for the old candidate survived the rebind: %+v", rebound)
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Proofs) != 0 {
		t.Fatalf("durable envelope kept superseded proof: %+v", stored.Proofs)
	}
}

// The local-proof gate must ask the question it claims to ask, before anything
// destructive starts — not after the simulation has already run.
func TestUnreproducibleLocalEvidenceBlocksSimulationLaunch(t *testing.T) {
	workspace, revision, _ := candidateUnderTest(t, "chg-local-gate")
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})

	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-local-gate", ChangeSimulationRepair, "repair", revision, RiskCritical)
	e.RequiredTests = []TestRequirement{{
		Name:     "echo-contents",
		Command:  []string{"sh", "-c", "cat README.md"},
		Required: true,
	}}
	e.RequiredScenarios = []ScenarioRequirement{{
		Name: "chaos", Repository: "globulario/globular-quickstart",
		Path: "tests/scenarios/chaos.yaml", Required: true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	local, err := RunDeclaredTest(context.Background(), TestRunOptions{
		EnvelopePath: envelopePath, WorkspaceDir: workspace, TestName: "echo-contents",
	})
	if err != nil || local.Record.Result != "PASS" {
		t.Fatalf("local test should pass: %+v err=%v", local, err)
	}

	// The artifact behind the standing local PASS is altered afterwards.
	if err := os.WriteFile(local.Record.EvidenceRef, []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(quickstart, "harness-was-launched")
	_, err = RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstart,
		Scenario:      filepath.Join(quickstart, "tests", "scenarios", "chaos.yaml"),
		ScenarioName:  "chaos",
		EnvelopePath:  envelopePath,
	})
	if err == nil {
		t.Fatal("simulation ran on local evidence that could not be reproduced")
	}
	if !strings.Contains(err.Error(), "local proof gate") {
		t.Fatalf("expected the local gate to refuse, got %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Fatal("the harness was launched despite the gate")
	}
}

// A committed scenario path may itself be a symlink out of the tree. Following
// it would execute mutable external contents under a frozen simulator revision.
func TestScenarioSymlinkCannotEscapeTheSimulatorRevision(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "external.yaml"), []byte("name: chaos\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})

	link := filepath.Join(quickstart, "tests", "scenarios", "chaos.yaml")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "external.yaml"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	runGit(t, quickstart, "add", "-A")
	runGit(t, quickstart, "commit", "-m", "scenario is a symlink out of the tree")

	envelopePath := scenarioEnvelope(t, "chg-scenario-symlink")
	_, err := RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstart,
		Scenario:      link,
		ScenarioName:  "chaos",
		EnvelopePath:  envelopePath,
	})
	if err == nil {
		t.Fatal("a scenario symlinked outside the simulator revision was executed")
	}
	if !strings.Contains(err.Error(), "outside the simulator revision") {
		t.Fatalf("expected an escape refusal, got %v", err)
	}
}

// A failed run is not proof, but it is an occurrence — its identity must survive
// so the failure that stops the plan can still be learned from.
func TestFailedRunKeepsItsOccurrenceIdentityForLearning(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass-then-exit-nonzero"})
	envelopePath := scenarioEnvelope(t, "chg-failed-learning")

	result, err := RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstart,
		Scenario:      filepath.Join(quickstart, "tests", "scenarios", "chaos.yaml"),
		ScenarioName:  "chaos",
		EnvelopePath:  envelopePath,
	})
	if err == nil {
		t.Fatal("a nonzero exit must not certify")
	}
	if result.Proof.InvocationID != result.InvocationID || result.Proof.InvocationID == "" {
		t.Fatalf("failed run lost its occurrence identity: %+v", result.Proof)
	}
	if result.Proof.ProofEligible {
		t.Fatal("a failed run was marked proof eligible")
	}
	if result.Proof.Scenario != "chaos" || result.Proof.CandidateRevision != "candidate-sha" {
		t.Fatalf("failed run identity incomplete: %+v", result.Proof)
	}
	// And it is never recorded into the envelope.
	stored, loadErr := LoadChangeEnvelope(envelopePath)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(stored.Proofs) != 0 {
		t.Fatalf("a non-certifying run was recorded as proof: %+v", stored.Proofs)
	}
}

// Refusing to launch is not enough on its own: an envelope already at PROVEN
// would keep reporting PROVEN from metadata while its evidence is gone.
func TestLocalGateFailureAlsoWithdrawsTheProvenClaim(t *testing.T) {
	workspace, revision, _ := candidateUnderTest(t, "chg-gate-demote")
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})

	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-gate-demote", ChangeSimulationRepair, "repair", revision, RiskCritical)
	e.RequiredTests = []TestRequirement{{
		Name: "echo-contents", Command: []string{"sh", "-c", "cat README.md"}, Required: true,
	}}
	e.RequiredScenarios = []ScenarioRequirement{{
		Name: "chaos", Repository: "globulario/globular-quickstart",
		Path: "tests/scenarios/chaos.yaml", Required: true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	local, err := RunDeclaredTest(context.Background(), TestRunOptions{
		EnvelopePath: envelopePath, WorkspaceDir: workspace, TestName: "echo-contents",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runScenario(t, quickstart, envelopePath); err != nil {
		t.Fatalf("setup scenario should pass: %v", err)
	}
	proven, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if proven.Stage != StageProven {
		t.Fatalf("expected PROVEN setup, got %s", proven.Stage)
	}

	// The local evidence behind the standing PROVEN is altered.
	if err := os.WriteFile(local.Record.EvidenceRef, []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runScenario(t, quickstart, envelopePath); err == nil {
		t.Fatal("gate did not refuse")
	}
	after, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if after.Stage == StageProven {
		t.Fatal("PROVEN survived a local gate failure; the claim was refused but never withdrawn")
	}
}

// Two concurrent creations must not both succeed: the second would replace the
// first envelope's durable history.
func TestConcurrentCreateCannotReplaceAnEnvelope(t *testing.T) {
	path := filepath.Join(t.TempDir(), "change.yaml")
	first := NewChangeEnvelope("chg-create-a", ChangeFeature, "first", "sha-a", RiskLow)
	second := NewChangeEnvelope("chg-create-b", ChangeFeature, "second", "sha-b", RiskLow)

	if err := CreateChangeEnvelope(path, first); err != nil {
		t.Fatal(err)
	}
	if err := CreateChangeEnvelope(path, second); err == nil {
		t.Fatal("a second creation replaced an existing envelope")
	}
	stored, err := LoadChangeEnvelope(path)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != "chg-create-a" {
		t.Fatalf("existing envelope was overwritten: %s", stored.ID)
	}
}

// The failed-occurrence path must apply the same identity checks as the
// certifying path, or a learning artifact could be ingested under a repository
// or simulator revision that never ran.
func TestFailedOccurrenceRejectsForeignIdentity(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass-then-exit-nonzero", repository: "globulario/somewhere-else"})
	envelopePath := scenarioEnvelope(t, "chg-failed-foreign")

	result, err := RunQuickstartScenario(context.Background(), QuickstartRunOptions{
		QuickstartDir: quickstart,
		Scenario:      filepath.Join(quickstart, "tests", "scenarios", "chaos.yaml"),
		ScenarioName:  "chaos",
		EnvelopePath:  envelopePath,
	})
	if err == nil {
		t.Fatal("a nonzero exit must not certify")
	}
	if result.Proof.CandidateRepository != "" || result.Proof.InvocationID != "" {
		t.Fatalf("a foreign-identity artifact was preserved as an occurrence: %+v", result.Proof)
	}
}

// An envelope arriving for independent review or admission must not be able to
// present a PASS that cannot be tied back to the repository, the simulator
// revision, or the run that produced it.
func TestIncompleteOccurrenceIdentityIsNotEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		strip  func(*ProofRecord)
		expect string
	}{
		{"candidate repository", func(p *ProofRecord) { p.CandidateRepository = "" }, "candidate repository"},
		{"simulation repository", func(p *ProofRecord) { p.Repository = "" }, "simulation repository"},
		{"simulation revision", func(p *ProofRecord) { p.SimulationRevision = "" }, "simulation revision"},
		{"invocation id", func(p *ProofRecord) { p.InvocationID = "" }, "invocation id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := provenFixture(t)
			tc.strip(&e.Proofs[0])
			err := e.Validate()
			if err == nil {
				t.Fatal("a proof missing part of its occurrence identity was accepted as evidence")
			}
			if !strings.Contains(err.Error(), tc.expect) {
				t.Fatalf("expected an error naming %q, got %v", tc.expect, err)
			}
			// And it cannot be reached by reconciliation either.
			reconciled := e
			if reconciled.MarkProvenIfComplete() {
				t.Fatal("reconciliation restored PROVEN on incomplete identity")
			}
		})
	}
}

// A test record must also name the repository it was proven against.
func TestTestRecordWithoutCandidateRepositoryIsNotEvidence(t *testing.T) {
	e := provenFixture(t)
	e.Tests[0].CandidateRepository = ""
	if err := e.Validate(); err == nil {
		t.Fatal("a test record naming no candidate repository was accepted as evidence")
	}
}

// A crash mid-create must not leave a corrupt file at the published path where
// every retry is then refused as already-existing.
func TestCreatePublishesNothingPartial(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "change.yaml")
	e := NewChangeEnvelope("chg-atomic-create", ChangeFeature, "intent", "sha", RiskLow)
	if err := CreateChangeEnvelope(path, e); err != nil {
		t.Fatal(err)
	}
	// Nothing but the published envelope is left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "change.yaml" {
		names := []string{}
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("staging artifacts survived publication: %v", names)
	}
	// What is published is complete and loadable.
	loaded, err := LoadChangeEnvelope(path)
	if err != nil {
		t.Fatalf("published envelope is not loadable: %v", err)
	}
	if loaded.ID != "chg-atomic-create" {
		t.Fatalf("published envelope is not the one written: %+v", loaded)
	}
}

// Requiring a field to be present is not the same as requiring it to be right.
// A proof labelled with an unrelated simulator repository must not discharge an
// obligation frozen against the canonical one.
func TestProofRepositoryMustMatchTheFrozenScenario(t *testing.T) {
	t.Run("explicit requirement", func(t *testing.T) {
		e := provenFixture(t, func(e *ChangeEnvelope) {
			e.RequiredScenarios[0].Repository = CanonicalSimulationRepository
		})
		e.Proofs[0].Repository = "globulario/somewhere-else"
		err := e.Validate()
		if err == nil {
			t.Fatal("a proof from an unrelated simulator repository was accepted")
		}
		if !strings.Contains(err.Error(), "somewhere-else") {
			t.Fatalf("expected the claimed repository to be named, got %v", err)
		}
	})
	t.Run("requirement omits the repository", func(t *testing.T) {
		// An omitted requirement repository means the canonical simulator, not
		// "any simulator".
		e := provenFixture(t)
		e.Proofs[0].Repository = "globulario/somewhere-else"
		if err := e.Validate(); err == nil {
			t.Fatal("an implicit requirement accepted a foreign simulator repository")
		}
	})
	t.Run("canonical repository still validates", func(t *testing.T) {
		e := provenFixture(t)
		e.Proofs[0].Repository = CanonicalSimulationRepository
		if err := e.Validate(); err != nil {
			t.Fatalf("the canonical simulator was rejected: %v", err)
		}
	})
}

// A PASS that is missing its mandatory evidence artifact can never certify, so
// the standalone CLI must not report success for it.
func TestRunResultWithoutEvidenceDoesNotCertify(t *testing.T) {
	complete := QuickstartRunResult{Proof: ProofRecord{
		Scenario: "chaos", Result: "PASS", ProofEligible: true,
		CandidateRepository: "globulario/services",
		Repository:          CanonicalSimulationRepository,
		SimulationRevision:  "sim-sha", InvocationID: "inv-1",
		ProofRef: "p.json", EvidenceRef: "e.json", Digest: "sha256:x",
	}}
	if err := complete.CertifiesRequirement("chaos"); err != nil {
		t.Fatalf("a complete record was rejected: %v", err)
	}
	for _, tc := range []struct {
		name  string
		strip func(*ProofRecord)
	}{
		{"no evidence_ref", func(p *ProofRecord) { p.EvidenceRef = "" }},
		{"no proof_ref", func(p *ProofRecord) { p.ProofRef = "" }},
		{"no digest", func(p *ProofRecord) { p.Digest = "" }},
		{"not eligible", func(p *ProofRecord) { p.ProofEligible = false }},
		{"not PASS", func(p *ProofRecord) { p.Result = "UNSUPPORTED" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := complete
			tc.strip(&r.Proof)
			if err := r.CertifiesRequirement("chaos"); err == nil {
				t.Fatal("a record that can never certify reported success")
			}
		})
	}
}
