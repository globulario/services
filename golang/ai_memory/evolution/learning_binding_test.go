package evolution

import (
	"context"
	"testing"

	behavioral "github.com/globulario/services/golang/ai_memory/behavioral/api"
)

// boundProof is the scenario proof a learning artifact must answer for.
func boundProof() (string, ProofRecord) {
	return "chg-1", ProofRecord{
		Scenario:            "x",
		CandidateRepository: "globulario/services",
		CandidateRevision:   "cand-sha",
		PlanDigest:          "sha256:plan",
		SimulationRevision:  "sim-sha",
		InvocationID:        "inv-1",
		Result:              "FAIL", // matches validLearning()
	}
}

func boundLearning() SimulationLearning {
	l := validLearning()
	l.Invocation = ProofInvocation{ID: "inv-1"}
	return l
}

func TestLearningBoundToItsProofIsAccepted(t *testing.T) {
	changeID, proof := boundProof()
	if err := boundLearning().RequireBoundTo(changeID, proof); err != nil {
		t.Fatalf("correctly bound learning rejected: %v", err)
	}
}

// An unbound artifact must be refused outright on the proof path. It is not
// "legacy" here: the command is reporting it as learning from this proof run.
func TestUnboundLearningIsRefusedOnTheProofPath(t *testing.T) {
	changeID, proof := boundProof()
	for _, tc := range []struct {
		name    string
		mutate  func(*SimulationLearning)
		wantErr string
	}{
		{"no change id", func(l *SimulationLearning) { l.Change.ID = "" }, "change.id"},
		{"no invocation", func(l *SimulationLearning) { l.Invocation = ProofInvocation{} }, "invocation"},
		{"no candidate revision", func(l *SimulationLearning) { l.Change.CandidateRevision = "" }, "candidate_revision"},
		{"no plan digest", func(l *SimulationLearning) { l.Change.PlanDigest = "" }, "plan_digest"},
		{"no simulation revision", func(l *SimulationLearning) { l.Change.SimulationRevision = "" }, "simulation_revision"},
		{"no result", func(l *SimulationLearning) { l.Result = "" }, "result"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := boundLearning()
			tc.mutate(&l)
			err := l.RequireBoundTo(changeID, proof)
			if err == nil {
				t.Fatal("unbound learning accepted on the proof path")
			}
		})
	}
}

func TestMisboundLearningIsRefused(t *testing.T) {
	changeID, proof := boundProof()
	for _, tc := range []struct {
		name   string
		mutate func(*SimulationLearning)
	}{
		{"other change", func(l *SimulationLearning) { l.Change.ID = "chg-other" }},
		{"other scenario", func(l *SimulationLearning) { l.Scenario = "other" }},
		{"other repository", func(l *SimulationLearning) { l.Change.CandidateRepository = "globulario/other" }},
		{"other candidate revision", func(l *SimulationLearning) { l.Change.CandidateRevision = "other-sha" }},
		{"other plan digest", func(l *SimulationLearning) { l.Change.PlanDigest = "sha256:other" }},
		{"other invocation", func(l *SimulationLearning) { l.Invocation.ID = "inv-other" }},
		{"contradicting result", func(l *SimulationLearning) { l.Result = "PASS" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := boundLearning()
			tc.mutate(&l)
			if err := l.RequireBoundTo(changeID, proof); err == nil {
				t.Fatal("learning bound to a different occurrence was accepted")
			}
		})
	}
}

// The consumer must not repair an unidentified artifact by copying values from
// the proof — that would launder it into looking bound.
func TestBindingCheckDoesNotFillMissingValues(t *testing.T) {
	changeID, proof := boundProof()
	l := boundLearning()
	l.Change.CandidateRevision = ""
	_ = l.RequireBoundTo(changeID, proof)
	if l.Change.CandidateRevision != "" {
		t.Fatalf("binding check filled a missing value from the proof: %q", l.Change.CandidateRevision)
	}
}

// --- duplicate required scenario names -------------------------------------

func TestDuplicateRequiredScenarioNamesAreRefusedAtPlanFreeze(t *testing.T) {
	e := NewChangeEnvelope("chg-dup", ChangeSimulationRepair, "repair", "source-sha", RiskCritical)
	e.RequiredScenarios = []ScenarioRequirement{
		{Name: "chaos", Path: "tests/scenarios/a.yaml", Required: true},
		{Name: "chaos", Path: "tests/scenarios/b.yaml", Required: true},
	}
	if err := e.BindCandidate("globulario/services", "candidate-sha"); err == nil {
		t.Fatal("two obligations sharing one name were frozen into the plan")
	}
	if err := e.Validate(); err == nil {
		t.Fatal("duplicate required scenario names passed validation")
	}
}

// --- idempotent ingestion ---------------------------------------------------

type idCapturingRecorder struct {
	signalIDs   []string
	evidenceIDs []string
	outcomeIDs  []string
}

func (r *idCapturingRecorder) RecordSignal(_ context.Context, req *behavioral.RecordSignalRequest) (*behavioral.RecordSignalResponse, error) {
	r.signalIDs = append(r.signalIDs, req.Signal.ID)
	return &behavioral.RecordSignalResponse{SignalID: req.Signal.ID}, nil
}
func (r *idCapturingRecorder) RecordEvidence(_ context.Context, req *behavioral.RecordEvidenceRequest) (*behavioral.RecordEvidenceResponse, error) {
	r.evidenceIDs = append(r.evidenceIDs, req.Evidence.ID)
	return &behavioral.RecordEvidenceResponse{EvidenceID: req.Evidence.ID}, nil
}
func (r *idCapturingRecorder) RecordOutcome(_ context.Context, req *behavioral.RecordOutcomeRequest) (*behavioral.RecordOutcomeResponse, error) {
	r.outcomeIDs = append(r.outcomeIDs, req.Outcome.ID)
	return &behavioral.RecordOutcomeResponse{OutcomeID: req.Outcome.ID}, nil
}

func ingestOnce(t *testing.T, l SimulationLearning) *idCapturingRecorder {
	t.Helper()
	rec := &idCapturingRecorder{}
	if _, err := (SimulationIngestor{
		Recorder: rec, Project: "globular",
		Domain:  behavioral.DomainRef("cluster_operator"),
		AgentID: "test",
	}).Ingest(context.Background(), l); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	return rec
}

// A retry after a partial write must not manufacture a second observation of
// the same event.
func TestRetryingTheSameIngestionReusesIdentity(t *testing.T) {
	l := boundLearning()
	first := ingestOnce(t, l)
	second := ingestOnce(t, l)

	if first.signalIDs[0] == "" {
		t.Fatal("ingestion allocated no deterministic signal id, so a retry cannot be idempotent")
	}
	if first.signalIDs[0] != second.signalIDs[0] {
		t.Fatalf("retry produced a new signal id: %q vs %q", first.signalIDs[0], second.signalIDs[0])
	}
	if len(first.evidenceIDs) != len(second.evidenceIDs) {
		t.Fatalf("evidence count changed across retry: %d vs %d", len(first.evidenceIDs), len(second.evidenceIDs))
	}
	for i := range first.evidenceIDs {
		if first.evidenceIDs[i] == "" || first.evidenceIDs[i] != second.evidenceIDs[i] {
			t.Fatalf("evidence id %d not stable: %q vs %q", i, first.evidenceIDs[i], second.evidenceIDs[i])
		}
	}
	if first.outcomeIDs[0] == "" || first.outcomeIDs[0] != second.outcomeIDs[0] {
		t.Fatalf("outcome id not stable: %q vs %q", first.outcomeIDs[0], second.outcomeIDs[0])
	}
}

// A genuinely new execution is legitimately a second observation.
func TestNewInvocationProducesNewIdentity(t *testing.T) {
	first := ingestOnce(t, boundLearning())
	next := boundLearning()
	next.Invocation.ID = "inv-2"
	second := ingestOnce(t, next)

	if first.signalIDs[0] == second.signalIDs[0] {
		t.Fatal("a second scenario execution reused the first execution's identity")
	}
	if first.outcomeIDs[0] == second.outcomeIDs[0] {
		t.Fatal("a second execution reused the first outcome identity")
	}
}

// Each record kind and index must be distinct within one occurrence.
func TestIdentitiesAreDistinctWithinOneOccurrence(t *testing.T) {
	rec := ingestOnce(t, boundLearning())
	seen := map[string]bool{}
	all := append(append(append([]string{}, rec.signalIDs...), rec.evidenceIDs...), rec.outcomeIDs...)
	for _, id := range all {
		if id == "" {
			t.Fatal("empty identity within a bound occurrence")
		}
		if seen[id] {
			t.Fatalf("two records in one occurrence share identity %q", id)
		}
		seen[id] = true
	}
	if len(all) < 3 {
		t.Fatalf("expected signal + evidence + outcome identities, got %d", len(all))
	}
}
