package evolution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	behavioral "github.com/globulario/services/golang/ai_memory/behavioral/api"
)

const SimulationLearningSchemaVersion = 1

var simulationResults = map[string]struct{}{
	"PASS": {}, "FAIL": {}, "PARTIAL": {}, "INFRA_ERROR": {}, "UNSUPPORTED": {},
}

type ChangeBinding struct {
	ID                  string `json:"id,omitempty"`
	EnvelopeRef         string `json:"envelope_ref,omitempty"`
	CandidateRepository string `json:"candidate_repository,omitempty"`
	CandidateRevision   string `json:"candidate_revision,omitempty"`
	PlanDigest          string `json:"plan_digest,omitempty"`
	SimulationRevision  string `json:"simulation_revision,omitempty"`
}

type LearningOrigin struct {
	Type string `json:"type,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

type Determinism struct {
	Replayable bool        `json:"replayable"`
	Seed       interface{} `json:"seed,omitempty"`
}

type SimulationProof struct {
	Claim              string         `json:"claim"`
	Kind               string         `json:"kind"`
	Origin             LearningOrigin `json:"origin"`
	GoverningContracts []string       `json:"governing_contracts"`
	Invariants         []string       `json:"invariants"`
	KnownFailureModes  []string       `json:"known_failure_modes"`
	ForbiddenOutcomes  []string       `json:"forbidden_outcomes"`
	Determinism        Determinism    `json:"determinism"`
}

type FailedObservation struct {
	Section       string      `json:"section,omitempty"`
	ID            string      `json:"id,omitempty"`
	ProbeOrAction string      `json:"probe_or_action,omitempty"`
	Result        interface{} `json:"result,omitempty"`
}

type UnsupportedObservation struct {
	Section string `json:"section,omitempty"`
	ID      string `json:"id,omitempty"`
	Action  string `json:"action,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type SimulationObservations struct {
	FailedItems         []FailedObservation      `json:"failed_items"`
	UnsupportedRequired []UnsupportedObservation `json:"unsupported_required"`
	UnsupportedOptional []UnsupportedObservation `json:"unsupported_optional"`
}

type CandidatePolicy struct {
	LearningEnabled     bool     `json:"learning_enabled"`
	CandidateTypes      []string `json:"candidate_types"`
	MayCreateCandidates bool     `json:"may_create_candidates"`
	MayPromote          bool     `json:"may_promote"`
}

type SimulationAuthority struct {
	ProductionAuthoritative bool   `json:"production_authoritative"`
	PromotionRequired       bool   `json:"promotion_required"`
	Note                    string `json:"note,omitempty"`
}

type SimulationLearning struct {
	LearningSchemaVersion int                    `json:"learning_schema_version"`
	CreatedAt             string                 `json:"created_at"`
	Source                string                 `json:"source"`
	Scenario              string                 `json:"scenario"`
	Suite                 string                 `json:"suite"`
	Result                string                 `json:"result"`
	SourceRevision        string                 `json:"source_revision"`
	Change                ChangeBinding          `json:"change,omitempty"`
	Invocation            ProofInvocation        `json:"invocation,omitempty"`
	Proof                 SimulationProof        `json:"proof"`
	Observations          SimulationObservations `json:"observations"`
	CandidatePolicy       CandidatePolicy        `json:"candidate_policy"`
	Authority             SimulationAuthority    `json:"authority"`
	EvidenceRef           string                 `json:"evidence_ref"`
	ProofRef              string                 `json:"proof_ref"`
}

func ParseSimulationLearning(data []byte) (SimulationLearning, error) {
	var learning SimulationLearning
	if err := json.Unmarshal(data, &learning); err != nil {
		return SimulationLearning{}, fmt.Errorf("decode simulation learning: %w", err)
	}
	if err := learning.Validate(); err != nil {
		return SimulationLearning{}, err
	}
	return learning, nil
}

func (l SimulationLearning) Validate() error {
	if l.LearningSchemaVersion != SimulationLearningSchemaVersion {
		return fmt.Errorf("unsupported learning schema version %d", l.LearningSchemaVersion)
	}
	if l.Source != "globular-quickstart-simulation" {
		return fmt.Errorf("untrusted simulation source %q", l.Source)
	}
	if strings.TrimSpace(l.Scenario) == "" {
		return fmt.Errorf("scenario is required")
	}
	if _, ok := simulationResults[l.Result]; !ok {
		return fmt.Errorf("invalid simulation result %q", l.Result)
	}
	if l.Result == "PASS" && strings.TrimSpace(l.SourceRevision) == "" {
		return fmt.Errorf("PASS learning requires source_revision")
	}
	// An observation with no readable time is not an observation with a default
	// time. Stamping "now" onto an artifact whose age is unknown persists a stale
	// result as a fresh one, and freshness is what later contradiction and
	// promotion decisions weigh.
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(l.CreatedAt)); err != nil {
		return fmt.Errorf(
			"learning artifact has no readable created_at (%q): an observation of unknown age cannot be recorded as a fresh one",
			l.CreatedAt,
		)
	}
	if l.Authority.ProductionAuthoritative {
		return fmt.Errorf("simulation learning cannot be production authoritative")
	}
	if !l.Authority.PromotionRequired {
		return fmt.Errorf("simulation learning must require governed promotion")
	}
	if l.CandidatePolicy.MayPromote {
		return fmt.Errorf("simulation learning may not promote behavioral knowledge")
	}
	if l.Proof.Determinism.Replayable && l.Proof.Determinism.Seed == nil {
		return fmt.Errorf("replayable simulation requires deterministic seed")
	}
	if l.Change.ID != "" {
		if strings.TrimSpace(l.Change.CandidateRepository) == "" || strings.TrimSpace(l.Change.CandidateRevision) == "" {
			return fmt.Errorf("change-bound learning requires candidate_repository and candidate_revision")
		}
		if strings.TrimSpace(l.Change.PlanDigest) == "" {
			return fmt.Errorf("change-bound learning requires plan_digest")
		}
		if l.Change.SimulationRevision != "" && l.Change.SimulationRevision != l.SourceRevision {
			return fmt.Errorf(
				"change simulation_revision %q does not match source_revision %q",
				l.Change.SimulationRevision,
				l.SourceRevision,
			)
		}
	}
	return nil
}

// RequireOccurrenceBinding refuses an artifact that asserts a result without
// naming the occurrence that produced it.
//
// A standalone file with result PASS, a plausible source revision and timestamp,
// and empty proof/evidence refs previously validated — and was then recorded as
// a confidence-1 runtime fact with no evidence IDs. A self-authored JSON file
// could become durable Behavioral Memory that later contradiction and promotion
// decisions weigh.
//
// Ingestion is a live mutation of the behavioral store, so the artifact must at
// minimum say which proof occurrence it describes. Verifying that occurrence
// against the executed proof is stronger still and is what RequireBoundTo does
// on the proof path; this is the floor beneath it for the standalone tool.
func (l SimulationLearning) RequireOccurrenceBinding() error {
	for _, field := range []struct{ name, value string }{
		{"proof_ref", l.ProofRef},
		{"evidence_ref", l.EvidenceRef},
		{"change.id", l.Change.ID},
		{"change.candidate_revision", l.Change.CandidateRevision},
		{"change.plan_digest", l.Change.PlanDigest},
		{"invocation.id", l.Invocation.ID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf(
				"learning artifact names no %s; an observation that cannot say which run produced it "+
					"must not become durable behavioral memory",
				field.name,
			)
		}
	}
	return nil
}

// RequireBoundTo refuses any learning artifact that is not this exact proof
// occurrence. It compares only; it never copies a missing value across from the
// proof, because filling a gap here would launder an unidentified artifact into
// looking bound and make the consumer the authority for an identity it cannot
// actually vouch for.
//
// Absence is not a legacy allowance on this path. The caller is about to record
// these observations as learning from this proof run, so an artifact that cannot
// say which run it came from must be refused, not adopted.
func (l SimulationLearning) RequireBoundTo(changeID string, p ProofRecord) error {
	for _, field := range []struct{ name, got, want string }{
		{"change.id", l.Change.ID, changeID},
		{"scenario", l.Scenario, p.Scenario},
		{"change.candidate_repository", l.Change.CandidateRepository, p.CandidateRepository},
		{"change.candidate_revision", l.Change.CandidateRevision, p.CandidateRevision},
		{"change.plan_digest", l.Change.PlanDigest, p.PlanDigest},
		{"change.simulation_revision", l.Change.SimulationRevision, p.SimulationRevision},
		{"invocation.id", l.Invocation.ID, p.InvocationID},
		// The verdict is part of the occurrence, not commentary on it. A
		// learning artifact claiming FAIL for a PASS proof would record a
		// failure observation and promotion-candidate hints against a candidate
		// that actually passed, and the reverse would learn a failure as success.
		{"result", l.Result, p.Result},
	} {
		got := strings.TrimSpace(field.got)
		want := strings.TrimSpace(field.want)
		if got == "" {
			return fmt.Errorf(
				"learning artifact has no %s; it cannot be attributed to this proof run",
				field.name,
			)
		}
		if want == "" {
			return fmt.Errorf(
				"scenario proof has no %s to bind the learning artifact against",
				field.name,
			)
		}
		if got != want {
			return fmt.Errorf(
				"learning %s %q does not match this proof %q",
				field.name,
				got,
				want,
			)
		}
	}
	return nil
}

// occurrenceIdentity is the immutable identity of one simulation execution. It
// is what makes a retry after a partial write idempotent: the same execution
// re-derives the same record IDs, while a genuinely new execution — a new
// invocation of the scenario — derives different ones and is correctly counted
// as a second observation.
//
// It returns "" when the artifact is not fully identified. Idempotence is a
// property of having an identity; without one the server allocates, and the
// proof path forbids that case outright via RequireBoundTo.
func (l SimulationLearning) occurrenceIdentity() string {
	parts := []string{
		l.Change.ID,
		l.Change.CandidateRevision,
		l.Change.PlanDigest,
		l.Scenario,
		l.Invocation.ID,
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return ""
		}
	}
	return strings.Join(parts, "\x00")
}

// recordID derives one stable record identity within an occurrence. kind and
// index keep the signal, each evidence row, and the outcome distinct.
func (l SimulationLearning) recordID(kind string, index int) string {
	occurrence := l.occurrenceIdentity()
	if occurrence == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", occurrence, kind, index)))
	return fmt.Sprintf("sim-%s-%s", kind, hex.EncodeToString(sum[:16]))
}

// requireAllocatedAs guards against silently accepting a collision. When we
// supplied an identity, the store must answer with that identity; a different
// one means the record we asked for is not the record that exists.
func requireAllocatedAs(kind, requested, returned string) error {
	// Only unbound ingestion, which requests no identity, may be answered with
	// whatever the store allocates. Once we have asked for a specific identity,
	// an empty answer is as much a failure to confirm it as a different one:
	// an empty id cannot be attached to an outcome or point at a stored record.
	if requested == "" {
		return nil
	}
	if requested == returned {
		return nil
	}
	if returned == "" {
		return fmt.Errorf(
			"%s identity unconfirmed: requested %q but behavioral memory returned no id",
			kind,
			requested,
		)
	}
	return fmt.Errorf(
		"%s identity mismatch: requested %q but behavioral memory returned %q; refusing to treat this as a replay",
		kind,
		requested,
		returned,
	)
}

// BehavioralRecorder is intentionally narrower than behavioral.Core. The simulation
// ingestion path can record observations/evidence/outcomes, but it has no method
// capable of promoting a principle. This makes the authority boundary structural.
type BehavioralRecorder interface {
	RecordSignal(context.Context, *behavioral.RecordSignalRequest) (*behavioral.RecordSignalResponse, error)
	RecordEvidence(context.Context, *behavioral.RecordEvidenceRequest) (*behavioral.RecordEvidenceResponse, error)
	RecordOutcome(context.Context, *behavioral.RecordOutcomeRequest) (*behavioral.RecordOutcomeResponse, error)
}

type SimulationIngestor struct {
	Recorder  BehavioralRecorder
	Project   string
	Domain    behavioral.DomainRef
	AgentID   string
	ClusterID string
}

type SimulationIngestResult struct {
	SignalID       string   `json:"signal_id"`
	EvidenceIDs    []string `json:"evidence_ids"`
	OutcomeID      string   `json:"outcome_id"`
	CandidateHints []string `json:"candidate_hints"`
}

func (i SimulationIngestor) Ingest(ctx context.Context, l SimulationLearning) (SimulationIngestResult, error) {
	if i.Recorder == nil {
		return SimulationIngestResult{}, fmt.Errorf("behavioral recorder is required")
	}
	if err := l.Validate(); err != nil {
		return SimulationIngestResult{}, err
	}
	project := i.Project
	if project == "" {
		project = "globular"
	}
	domain := i.Domain
	if domain == "" {
		domain = behavioral.DomainRef("cluster_operator")
	}
	agent := i.AgentID
	if agent == "" {
		agent = "simulation_learning_ingestor"
	}
	observedAt := parseRFC3339Unix(l.CreatedAt)
	payload, _ := json.Marshal(l)
	metadata := map[string]string{
		"simulation":               "true",
		"scenario":                 l.Scenario,
		"suite":                    l.Suite,
		"result":                   l.Result,
		"source_revision":          l.SourceRevision,
		"production_authoritative": "false",
		"promotion_required":       "true",
	}
	if l.Change.ID != "" {
		metadata["change_id"] = l.Change.ID
		metadata["change_envelope_ref"] = l.Change.EnvelopeRef
		metadata["candidate_repository"] = l.Change.CandidateRepository
		metadata["candidate_revision"] = l.Change.CandidateRevision
		metadata["plan_digest"] = l.Change.PlanDigest
	}
	if l.Invocation.ID != "" {
		metadata["proof_invocation_id"] = l.Invocation.ID
	}
	if len(l.CandidatePolicy.CandidateTypes) > 0 {
		metadata["candidate_types"] = strings.Join(l.CandidatePolicy.CandidateTypes, ",")
	}

	signalID := l.recordID("signal", 0)
	sigRes, err := i.Recorder.RecordSignal(ctx, &behavioral.RecordSignalRequest{Signal: behavioral.Signal{
		ID:             signalID,
		Project:        project,
		Domain:         domain,
		Kind:           behavioral.SignalObservedRuntimeFact,
		SourceKind:     "test",
		SourceRef:      l.ProofRef,
		EntityRef:      l.Scenario,
		ClusterID:      i.ClusterID,
		Severity:       simulationSeverity(l.Result),
		AuthorityLevel: behavioral.ObservationAuthorityDerived,
		ObservedAt:     observedAt,
		Payload:        string(payload),
		Confidence:     simulationConfidence(l.Result),
		Status:         behavioral.StatusRawSignal,
		Provenance:     behavioral.Provenance{AgentID: agent, SourceRef: l.ProofRef, CreatedAt: observedAt, UpdatedAt: observedAt},
		Metadata:       metadata,
	}})
	if err != nil {
		return SimulationIngestResult{}, fmt.Errorf("record simulation signal: %w", err)
	}
	if err := requireAllocatedAs("signal", signalID, sigRes.SignalID); err != nil {
		return SimulationIngestResult{}, err
	}

	result := SimulationIngestResult{SignalID: sigRes.SignalID}
	for idx, ref := range []struct{ kind, path string }{{"simulation_proof", l.ProofRef}, {"simulation_evidence", l.EvidenceRef}} {
		if strings.TrimSpace(ref.path) == "" {
			continue
		}
		evidenceID := l.recordID("evidence", idx)
		evRes, err := i.Recorder.RecordEvidence(ctx, &behavioral.RecordEvidenceRequest{Evidence: behavioral.Evidence{
			ID:             evidenceID,
			Project:        project,
			Domain:         domain,
			TargetKind:     "signal",
			TargetID:       sigRes.SignalID,
			Kind:           "test_result",
			Lane:           behavioral.LaneRuntimeRequired,
			Result:         evidenceResult(l.Result),
			SourceKind:     ref.kind,
			SourceRef:      ref.path,
			EntityRef:      l.Scenario,
			ClusterID:      i.ClusterID,
			Severity:       simulationSeverity(l.Result),
			AuthorityLevel: behavioral.ObservationAuthorityDerived,
			ObservedAt:     observedAt,
			Payload:        l.Proof.Claim,
			Provenance:     behavioral.Provenance{AgentID: agent, SourceRef: ref.path, CreatedAt: observedAt, UpdatedAt: observedAt},
			ObservedFrom:   sigRes.SignalID,
			Metadata:       metadata,
		}})
		if err != nil {
			return result, fmt.Errorf("record %s: %w", ref.kind, err)
		}
		if err := requireAllocatedAs("evidence", evidenceID, evRes.EvidenceID); err != nil {
			return result, err
		}
		result.EvidenceIDs = append(result.EvidenceIDs, evRes.EvidenceID)
	}

	outcomeID := l.recordID("outcome", 0)
	outRes, err := i.Recorder.RecordOutcome(ctx, &behavioral.RecordOutcomeRequest{Outcome: behavioral.Outcome{
		ID:          outcomeID,
		Project:     project,
		Domain:      domain,
		EvidenceIDs: append([]string(nil), result.EvidenceIDs...),
		Status:      outcomeStatus(l.Result),
		Severe:      l.Result == "FAIL" || l.Result == "PARTIAL",
		Theme:       "simulation." + l.Scenario,
		Note:        fmt.Sprintf("simulation %s: %s", l.Result, strings.TrimSpace(l.Proof.Claim)),
		AgentID:     agent,
		CreatedAt:   observedAt,
		Metadata:    metadata,
	}})
	if err != nil {
		return result, fmt.Errorf("record simulation outcome: %w", err)
	}
	if err := requireAllocatedAs("outcome", outcomeID, outRes.OutcomeID); err != nil {
		return result, err
	}
	result.OutcomeID = outRes.OutcomeID

	if l.CandidatePolicy.LearningEnabled && l.CandidatePolicy.MayCreateCandidates {
		result.CandidateHints = append([]string(nil), l.CandidatePolicy.CandidateTypes...)
	}
	return result, nil
}

// parseRFC3339Unix is only reached for artifacts that already passed Validate,
// which requires a readable created_at. The zero fallback exists so a caller
// bypassing validation cannot silently acquire the current time as an
// observation timestamp.
func parseRFC3339Unix(v string) int64 {
	if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v)); err == nil {
		return t.Unix()
	}
	return 0
}
func simulationSeverity(result string) string {
	if result == "FAIL" || result == "PARTIAL" {
		return "high"
	}
	if result == "UNSUPPORTED" || result == "INFRA_ERROR" {
		return "warn"
	}
	return "info"
}
func simulationConfidence(result string) float32 {
	if result == "PASS" || result == "FAIL" {
		return 1.0
	}
	return 0.7
}
func evidenceResult(result string) string {
	switch result {
	case "PASS":
		return "pass"
	case "FAIL", "PARTIAL":
		return "fail"
	default:
		return "unknown"
	}
}
func outcomeStatus(result string) string {
	switch result {
	case "PASS":
		return "success"
	case "FAIL", "PARTIAL":
		return "failure"
	default:
		return "blocked"
	}
}
