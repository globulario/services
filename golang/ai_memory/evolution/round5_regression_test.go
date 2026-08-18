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

	// Each invocation now owns its own evidence directory, so a previous log
	// cannot block a later run — that collision is gone by construction. The law
	// still has to hold though: make the directory those invocation dirs are
	// created under unwritable, so this run executes the command but cannot
	// persist what it observed.
	testsRoot := filepath.Dir(filepath.Dir(first.Record.EvidenceRef))
	if err := os.Chmod(testsRoot, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(testsRoot, 0o755) })
	if err := os.Mkdir(filepath.Join(testsRoot, "probe"), 0o755); err == nil {
		_ = os.Remove(filepath.Join(testsRoot, "probe"))
		t.Skip("evidence root is still writable; cannot exercise the failure")
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
	t.Run("unobserved identity is surfaced, not failed", func(t *testing.T) {
		// The simulator repository is observed, not asserted, so it can be
		// genuinely unknowable — a mirror or airgapped clone has no remote. That
		// is an unproven identity for admission to weigh, not a validation error.
		e := provenFixture(t)
		e.Proofs[0].Repository = ""
		if err := e.Validate(); err != nil {
			t.Fatalf("an unobservable simulator identity was treated as invalid: %v", err)
		}
		unproven := e.SimulatorIdentityUnproven()
		if len(unproven) != 1 || unproven[0] != "chaos" {
			t.Fatalf("unproven simulator identity was not surfaced: %v", unproven)
		}
		if len(e.ProofStatus().SimulatorIdentityUnproven) != 1 {
			t.Fatal("ProofStatus does not surface the unproven identity to admission")
		}
	})
	t.Run("agreement is accepted but never counts as proof", func(t *testing.T) {
		// A remote URL lives in unversioned .git/config and is settable by
		// whoever supplies the checkout, so agreement with the plan is consistency,
		// not established identity. It validates, and it stays unproven.
		e := provenFixture(t)
		e.Proofs[0].Repository = CanonicalSimulationRepository
		if err := e.Validate(); err != nil {
			t.Fatalf("a consistent simulator claim was rejected: %v", err)
		}
		if len(e.SimulatorIdentityUnproven()) != 1 {
			t.Fatal("a caller-settable claim was treated as established identity")
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

// One invocation, one artifact — on the local-test leg too. Two runs of the same
// required test must not share an evidence path, or one can overwrite the file
// between the other's write and its digest.
func TestConcurrentTestRunsDoNotShareAnEvidencePath(t *testing.T) {
	workspace, _, envelopePath := candidateUnderTest(t, "chg-evidence-collision")

	first, err := runCandidateTest(t, workspace, envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runCandidateTest(t, workspace, envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if first.InvocationID == "" || first.InvocationID == second.InvocationID {
		t.Fatalf("test runs share an invocation identity: %q / %q", first.InvocationID, second.InvocationID)
	}
	if first.Record.EvidenceRef == second.Record.EvidenceRef {
		t.Fatalf("two invocations wrote the same evidence path: %s", first.Record.EvidenceRef)
	}
	// The earlier artifact is still intact and still hashes to what it recorded,
	// so a concurrent sibling could not have rewritten it.
	digest, err := DigestFiles(first.Record.EvidenceRef)
	if err != nil {
		t.Fatalf("earlier evidence no longer readable: %v", err)
	}
	if digest != first.Record.Digest {
		t.Fatal("a later invocation overwrote an earlier invocation's evidence")
	}
	if second.Record.InvocationID != second.InvocationID {
		t.Fatalf("record does not name the invocation that produced it: %+v", second.Record)
	}
}

// The test leg must carry the same occurrence identity the scenario leg does.
func TestTestRecordWithoutInvocationIsNotEvidence(t *testing.T) {
	e := provenFixture(t)
	e.Tests[0].InvocationID = ""
	if err := e.Validate(); err == nil {
		t.Fatal("a test record naming no invocation was accepted as evidence")
	}
}

// The harness runs with cmd.Dir set to the detached simulator tree, so a run
// directory derived from a relative envelope path must still be handed over
// absolute — otherwise the harness writes beneath the simulator while the parent
// reads elsewhere, and a passing scenario would be read as producing no proof.
func TestInvocationRunDirIsAbsoluteFromARelativeEnvelopePath(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	runDir, err := createInvocationRunDir("change.yaml", "chg-relative", "inv-relative")
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(runDir) {
		t.Fatalf("run dir handed to the harness is relative: %s", runDir)
	}
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("run dir does not exist at the absolute path given to the harness: %v", err)
	}
	// And it resolves under the envelope's directory, not the process cwd by accident.
	if !strings.HasPrefix(runDir, mustEvalSymlinks(t, dir)) && !strings.HasPrefix(mustEvalSymlinks(t, runDir), mustEvalSymlinks(t, dir)) {
		t.Fatalf("run dir %s is not beneath the envelope directory %s", runDir, dir)
	}
}

func mustEvalSymlinks(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return p
	}
	return resolved
}

// The simulator identity recorded on a proof is the one the checkout actually
// declares — not a constant the runner supplies on its behalf.
func TestSimulatorIdentityIsObservedFromTheCheckout(t *testing.T) {
	quickstart := t.TempDir()
	writeQuickstartHarness(t, quickstart, fakeHarness{mode: "pass"})
	runGit(t, quickstart, "remote", "add", "origin", "git@github.com:globulario/globular-quickstart.git")

	envelopePath := scenarioEnvelope(t, "chg-observed-identity")
	result, err := runScenario(t, quickstart, envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Proof.Repository != "globulario/globular-quickstart" {
		t.Fatalf("simulator identity was not observed from the checkout: %q", result.Proof.Repository)
	}

	// A fork carrying the same harness must be recorded as the fork it is.
	other := t.TempDir()
	writeQuickstartHarness(t, other, fakeHarness{mode: "pass"})
	runGit(t, other, "remote", "add", "origin", "https://github.com/someone-else/globular-quickstart.git")
	forkEnvelope := scenarioEnvelope(t, "chg-fork-identity")
	forkResult, err := runScenario(t, other, forkEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if forkResult.Proof.Repository != "someone-else/globular-quickstart" {
		t.Fatalf("a fork was recorded as something else: %q", forkResult.Proof.Repository)
	}
}

func TestRepositoryURLNormalisation(t *testing.T) {
	// One repository must not read as several identities because git accepts
	// several spellings of it.
	for _, raw := range []string{
		"git@github.com:globulario/globular-quickstart.git",
		"https://github.com/globulario/globular-quickstart.git",
		"https://user@github.com/globulario/globular-quickstart",
		"ssh://git@github.com/globulario/globular-quickstart.git",
	} {
		if got := normalizeRepositoryURL(raw); got != "globulario/globular-quickstart" {
			t.Fatalf("%s normalised to %q", raw, got)
		}
	}
	for _, raw := range []string{"", "not-a-url"} {
		if got := normalizeRepositoryURL(raw); got != "" {
			t.Fatalf("%q should be unobservable, got %q", raw, got)
		}
	}
}

// Risk-scoped plan adequacy: the obligation binds where the central law bites.
func TestSimulationObligationIsRiskScoped(t *testing.T) {
	bind := func(t *testing.T, kind ChangeKind, risk RiskClass, withScenario bool) error {
		t.Helper()
		e := NewChangeEnvelope("chg-adequacy", kind, "intent", "source-sha", risk)
		e.RequiredTests = []TestRequirement{{
			Name: "unit", Command: []string{"go", "test"}, Required: true,
		}}
		if withScenario {
			e.RequiredScenarios = []ScenarioRequirement{{
				Name: "chaos", Path: "tests/scenarios/chaos.yaml", Required: true,
			}}
		}
		return e.BindCandidate("globulario/services", "candidate-sha")
	}

	t.Run("must declare a scenario", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			kind ChangeKind
			risk RiskClass
		}{
			{"simulation repair", ChangeSimulationRepair, RiskLow},
			{"architecture evolution", ChangeArchitectureEvolution, RiskLow},
			{"high risk feature", ChangeFeature, RiskHigh},
			{"critical incident repair", ChangeIncidentRepair, RiskCritical},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if err := bind(t, tc.kind, tc.risk, false); err == nil {
					t.Fatal("a plan with no simulation obligation was frozen")
				}
				if err := bind(t, tc.kind, tc.risk, true); err != nil {
					t.Fatalf("declaring a scenario should satisfy the rule: %v", err)
				}
			})
		}
	})

	t.Run("may prove locally", func(t *testing.T) {
		// A low-risk feature whose whole proof is a unit test should not have to
		// stand up a cluster to say so.
		for _, risk := range []RiskClass{RiskLow, RiskMedium} {
			if err := bind(t, ChangeFeature, risk, false); err != nil {
				t.Fatalf("a %s feature was forced to declare a scenario: %v", risk, err)
			}
		}
	})
}

// A fork can point its origin at the canonical URL, because that lives in
// unversioned .git/config. Agreement must therefore never establish identity.
func TestSpoofedOriginCannotEstablishSimulatorIdentity(t *testing.T) {
	fork := t.TempDir()
	writeQuickstartHarness(t, fork, fakeHarness{mode: "pass"})
	runGit(t, fork, "remote", "add", "origin", "git@github.com:globulario/globular-quickstart.git")

	envelopePath := scenarioEnvelope(t, "chg-spoofed-origin")
	result, err := runScenario(t, fork, envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		t.Fatal(err)
	}
	// The claim is recorded, and it is consistent, so the run is not refused...
	if result.Proof.Repository != CanonicalSimulationRepository {
		t.Fatalf("the checkout's claim was not recorded: %q", result.Proof.Repository)
	}
	// ...but admission is told the identity behind it was never established.
	unproven := stored.SimulatorIdentityUnproven()
	if len(unproven) != 1 || unproven[0] != "chaos" {
		t.Fatalf("a spoofable origin was presented to admission as proven identity: %v", unproven)
	}
	if len(stored.ProofStatus().SimulatorIdentityUnproven) != 1 {
		t.Fatal("ProofStatus hid the unproven identity from admission")
	}
}

// An omitted requirement repository means the canonical simulator, not any
// simulator — otherwise a foreign checkout certifies with nothing to contradict.
func TestForeignSimulatorRefusedWhenRequirementOmitsRepository(t *testing.T) {
	e := provenFixture(t)
	if e.RequiredScenarios[0].Repository != "" {
		t.Fatal("fixture should omit the requirement repository")
	}
	e.Proofs[0].Repository = "someone-else/globular-quickstart"
	err := e.Validate()
	if err == nil {
		t.Fatal("a foreign simulator certified against a requirement that named none")
	}
	if !strings.Contains(err.Error(), "someone-else") {
		t.Fatalf("expected the foreign claim to be named, got %v", err)
	}
}

// A required obligation that names no file is not executable, so nothing can
// discharge it — portable validation must say so, not only the CLIs.
func TestRequiredScenarioMustNameAPath(t *testing.T) {
	e := NewChangeEnvelope("chg-pathless", ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.RequiredScenarios = []ScenarioRequirement{{Name: "chaos", Required: true}}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err == nil {
		t.Fatal("an obligation naming no scenario file was frozen into the plan")
	}
}

// One obligation, one occurrence. Two records for the same scenario and
// candidate let certification and the uncertainty signal describe different
// proofs.
func TestDuplicateProofRecordsForOneObligationAreRefused(t *testing.T) {
	e := provenFixture(t)
	stale := e.Proofs[0]
	stale.Result = "FAIL"
	stale.ProofEligible = false
	stale.Repository = CanonicalSimulationRepository
	e.Proofs = append([]ProofRecord{stale}, e.Proofs...)

	if err := e.Validate(); err == nil {
		t.Fatal("two proof records for one obligation were accepted")
	}
}

// The CLI's exit decision must be judged against the frozen obligation, not
// against the record being judged. Building the expectation out of the proof
// itself always agrees, so a foreign checkout whose contradiction with the plan
// already stopped it certifying would still exit zero.
func TestCLIExitJudgesAgainstTheFrozenRequirement(t *testing.T) {
	foreign := ProofRecord{
		Scenario: "chaos", Result: "PASS", ProofEligible: true,
		CandidateRepository: "globulario/services",
		Repository:          "someone-else/globular-quickstart",
		SimulationRevision:  "sim-sha", InvocationID: "inv-1",
		ProofRef: "p.json", EvidenceRef: "e.json", Digest: "sha256:x",
	}

	// Requirement omits the repository, so the canonical simulator is expected.
	contradicting := QuickstartRunResult{
		Requirement: ScenarioRequirement{Name: "chaos", Path: "tests/scenarios/chaos.yaml", Required: true},
		Proof:       foreign,
	}
	if err := contradicting.CertifiesRequirement("chaos"); err == nil {
		t.Fatal("a run contradicting the frozen plan reported success to automation")
	}

	// Same record, judged against a plan that actually froze that repository.
	agreeing := QuickstartRunResult{
		Requirement: ScenarioRequirement{
			Name: "chaos", Path: "tests/scenarios/chaos.yaml",
			Repository: "someone-else/globular-quickstart", Required: true,
		},
		Proof: foreign,
	}
	if err := agreeing.CertifiesRequirement("chaos"); err != nil {
		t.Fatalf("a run consistent with its frozen plan was rejected: %v", err)
	}
}
