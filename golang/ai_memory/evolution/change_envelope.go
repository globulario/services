package evolution

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const ChangeEnvelopeSchemaVersion = 1

type ChangeKind string

const (
	ChangeIncidentRepair        ChangeKind = "incident_repair"
	ChangeSimulationRepair      ChangeKind = "simulation_repair"
	ChangeFeature               ChangeKind = "feature"
	ChangeArchitectureEvolution ChangeKind = "architecture_evolution"
)

type RiskClass string

const (
	RiskLow      RiskClass = "low"
	RiskMedium   RiskClass = "medium"
	RiskHigh     RiskClass = "high"
	RiskCritical RiskClass = "critical"
)

type ChangeStage string

const (
	StageDraft     ChangeStage = "DRAFT"
	StageCandidate ChangeStage = "CANDIDATE"
	StageProven    ChangeStage = "PROVEN"
	StageAdmitted  ChangeStage = "ADMITTED"
	StageReleased  ChangeStage = "RELEASED"
	StageVerified  ChangeStage = "VERIFIED"
	StageLearned   ChangeStage = "LEARNED"
	StageBlocked   ChangeStage = "BLOCKED"
)

type ArtifactRef struct {
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
	Revision   string `json:"revision,omitempty" yaml:"revision,omitempty"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	Digest     string `json:"digest,omitempty" yaml:"digest,omitempty"`
	URI        string `json:"uri,omitempty" yaml:"uri,omitempty"`
}

type ScenarioRequirement struct {
	Name       string `json:"name" yaml:"name"`
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`
	Path       string `json:"path,omitempty" yaml:"path,omitempty"`
	Required   bool   `json:"required" yaml:"required"`
}

type ProofRecord struct {
	Scenario            string `json:"scenario" yaml:"scenario"`
	Repository          string `json:"repository,omitempty" yaml:"repository,omitempty"`
	SimulationRevision  string `json:"simulation_revision,omitempty" yaml:"simulation_revision,omitempty"`
	CandidateRepository string `json:"candidate_repository,omitempty" yaml:"candidate_repository,omitempty"`
	CandidateRevision   string `json:"candidate_revision,omitempty" yaml:"candidate_revision,omitempty"`
	Result              string `json:"result" yaml:"result"`
	ProofEligible       bool   `json:"proof_eligible" yaml:"proof_eligible"`
	ProofRef            string `json:"proof_ref,omitempty" yaml:"proof_ref,omitempty"`
	EvidenceRef         string `json:"evidence_ref,omitempty" yaml:"evidence_ref,omitempty"`
	Digest              string `json:"digest,omitempty" yaml:"digest,omitempty"`
}

type AdmissionRecord struct {
	Status   string `json:"status,omitempty" yaml:"status,omitempty"`
	Revision string `json:"revision,omitempty" yaml:"revision,omitempty"`
	Ref      string `json:"ref,omitempty" yaml:"ref,omitempty"`
	Actor    string `json:"actor,omitempty" yaml:"actor,omitempty"`
	At       string `json:"at,omitempty" yaml:"at,omitempty"`
}

type ReleaseRecord struct {
	Status            string        `json:"status,omitempty" yaml:"status,omitempty"`
	CandidateRevision string        `json:"candidate_revision,omitempty" yaml:"candidate_revision,omitempty"`
	ReleaseRevision   string        `json:"release_revision,omitempty" yaml:"release_revision,omitempty"`
	Artifacts         []ArtifactRef `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
	Ref               string        `json:"ref,omitempty" yaml:"ref,omitempty"`
	At                string        `json:"at,omitempty" yaml:"at,omitempty"`
}

type VerificationRecord struct {
	Layer    string      `json:"layer" yaml:"layer"`
	Status   string      `json:"status" yaml:"status"`
	Evidence ArtifactRef `json:"evidence,omitempty" yaml:"evidence,omitempty"`
	At       string      `json:"at,omitempty" yaml:"at,omitempty"`
}

type ChangeEnvelope struct {
	SchemaVersion int         `json:"schema_version" yaml:"schema_version"`
	ID            string      `json:"change_id" yaml:"change_id"`
	Kind          ChangeKind  `json:"kind" yaml:"kind"`
	Stage         ChangeStage `json:"stage" yaml:"stage"`
	Intent        string      `json:"intent" yaml:"intent"`

	SourceRevision      string    `json:"source_revision" yaml:"source_revision"`
	ProductionRevision  string    `json:"production_revision,omitempty" yaml:"production_revision,omitempty"`
	CandidateRepository string    `json:"candidate_repository,omitempty" yaml:"candidate_repository,omitempty"`
	CandidateRevision   string    `json:"candidate_revision,omitempty" yaml:"candidate_revision,omitempty"`
	RiskClass           RiskClass `json:"risk_class" yaml:"risk_class"`

	AuthorityScope         []string              `json:"authority_scope,omitempty" yaml:"authority_scope,omitempty"`
	GoverningContracts     []string              `json:"governing_contracts,omitempty" yaml:"governing_contracts,omitempty"`
	RelevantInvariants     []string              `json:"relevant_invariants,omitempty" yaml:"relevant_invariants,omitempty"`
	KnownFailureModes      []string              `json:"known_failure_modes,omitempty" yaml:"known_failure_modes,omitempty"`
	ForbiddenRepairs       []string              `json:"forbidden_repairs,omitempty" yaml:"forbidden_repairs,omitempty"`
	ProductionEvidence     []ArtifactRef         `json:"production_evidence,omitempty" yaml:"production_evidence,omitempty"`
	RequiredScenarios      []ScenarioRequirement `json:"required_scenarios,omitempty" yaml:"required_scenarios,omitempty"`
	RequiredTests          []string              `json:"required_tests,omitempty" yaml:"required_tests,omitempty"`
	Proofs                 []ProofRecord         `json:"proofs,omitempty" yaml:"proofs,omitempty"`
	Admission              AdmissionRecord       `json:"admission,omitempty" yaml:"admission,omitempty"`
	Release                ReleaseRecord         `json:"release,omitempty" yaml:"release,omitempty"`
	ProductionVerification []VerificationRecord  `json:"production_verification,omitempty" yaml:"production_verification,omitempty"`
	Learning               []ArtifactRef         `json:"learning,omitempty" yaml:"learning,omitempty"`
	BlockedReason          string                `json:"blocked_reason,omitempty" yaml:"blocked_reason,omitempty"`
	CreatedAt              string                `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	UpdatedAt              string                `json:"updated_at,omitempty" yaml:"updated_at,omitempty"`
}

func NewChangeEnvelope(id string, kind ChangeKind, intent, sourceRevision string, risk RiskClass) ChangeEnvelope {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return ChangeEnvelope{
		SchemaVersion:  ChangeEnvelopeSchemaVersion,
		ID:             id,
		Kind:           kind,
		Stage:          StageDraft,
		Intent:         intent,
		SourceRevision: sourceRevision,
		RiskClass:      risk,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (e ChangeEnvelope) Validate() error {
	if e.SchemaVersion != ChangeEnvelopeSchemaVersion {
		return fmt.Errorf("unsupported change envelope schema version %d", e.SchemaVersion)
	}
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("change_id is required")
	}
	if !validChangeKind(e.Kind) {
		return fmt.Errorf("invalid change kind %q", e.Kind)
	}
	if !validStage(e.Stage) {
		return fmt.Errorf("invalid change stage %q", e.Stage)
	}
	if strings.TrimSpace(e.Intent) == "" {
		return fmt.Errorf("intent is required")
	}
	if strings.TrimSpace(e.SourceRevision) == "" {
		return fmt.Errorf("source_revision is required")
	}
	if !validRisk(e.RiskClass) {
		return fmt.Errorf("invalid risk_class %q", e.RiskClass)
	}
	if err := validateUnique("authority_scope", e.AuthorityScope); err != nil {
		return err
	}
	if err := validateUnique("governing_contracts", e.GoverningContracts); err != nil {
		return err
	}
	if err := validateUnique("relevant_invariants", e.RelevantInvariants); err != nil {
		return err
	}
	if err := validateUnique("known_failure_modes", e.KnownFailureModes); err != nil {
		return err
	}
	if err := validateUnique("forbidden_repairs", e.ForbiddenRepairs); err != nil {
		return err
	}

	if stageAtLeast(e.Stage, StageCandidate) {
		if strings.TrimSpace(e.CandidateRepository) == "" || strings.TrimSpace(e.CandidateRevision) == "" {
			return fmt.Errorf("candidate_repository and candidate_revision are required at stage %s", e.Stage)
		}
	}
	if stageAtLeast(e.Stage, StageProven) {
		if err := e.validateProofClosure(); err != nil {
			return err
		}
	}
	if stageAtLeast(e.Stage, StageAdmitted) {
		if e.Admission.Status != "ACCEPT" {
			return fmt.Errorf("stage %s requires admission status ACCEPT", e.Stage)
		}
		if e.Admission.Revision != e.CandidateRevision {
			return fmt.Errorf("admission revision %q does not match candidate revision %q", e.Admission.Revision, e.CandidateRevision)
		}
	}
	if stageAtLeast(e.Stage, StageReleased) {
		if e.Release.Status != "RELEASED" {
			return fmt.Errorf("stage %s requires release status RELEASED", e.Stage)
		}
		if e.Release.CandidateRevision != e.CandidateRevision {
			return fmt.Errorf("release candidate revision %q does not match envelope candidate revision %q", e.Release.CandidateRevision, e.CandidateRevision)
		}
		if len(e.Release.Artifacts) == 0 {
			return fmt.Errorf("released change requires at least one immutable artifact reference")
		}
	}
	if stageAtLeast(e.Stage, StageVerified) {
		if len(e.ProductionVerification) == 0 {
			return fmt.Errorf("verified change requires production verification evidence")
		}
		for _, v := range e.ProductionVerification {
			if v.Status != "PASS" {
				return fmt.Errorf("production verification layer %q is %q, expected PASS", v.Layer, v.Status)
			}
		}
	}
	if e.Stage == StageLearned && len(e.Learning) == 0 {
		return fmt.Errorf("learned change requires at least one learning artifact")
	}
	if e.Stage == StageBlocked && strings.TrimSpace(e.BlockedReason) == "" {
		return fmt.Errorf("blocked change requires blocked_reason")
	}
	return nil
}

func (e ChangeEnvelope) validateProofClosure() error {
	required := map[string]bool{}
	for _, s := range e.RequiredScenarios {
		if strings.TrimSpace(s.Name) == "" {
			return fmt.Errorf("required scenario name is empty")
		}
		if s.Required {
			required[s.Name] = false
		}
	}
	for _, p := range e.Proofs {
		if p.CandidateRepository != "" && p.CandidateRepository != e.CandidateRepository {
			return fmt.Errorf("proof %q candidate repository %q does not match %q", p.Scenario, p.CandidateRepository, e.CandidateRepository)
		}
		if p.CandidateRevision != e.CandidateRevision {
			return fmt.Errorf("proof %q candidate revision %q does not match %q", p.Scenario, p.CandidateRevision, e.CandidateRevision)
		}
		if p.Result == "PASS" && p.ProofEligible {
			if _, ok := required[p.Scenario]; ok {
				required[p.Scenario] = true
			}
		}
	}
	for name, ok := range required {
		if !ok {
			return fmt.Errorf("required scenario %q has no PASS proof eligible record for candidate revision %s", name, e.CandidateRevision)
		}
	}
	return nil
}

func (e ChangeEnvelope) IdentityDigest() (string, error) {
	identity := struct {
		SchemaVersion      int        `json:"schema_version"`
		ID                 string     `json:"change_id"`
		Kind               ChangeKind `json:"kind"`
		Intent             string     `json:"intent"`
		SourceRevision     string     `json:"source_revision"`
		ProductionRevision string     `json:"production_revision,omitempty"`
		RiskClass          RiskClass  `json:"risk_class"`
		AuthorityScope     []string   `json:"authority_scope,omitempty"`
		GoverningContracts []string   `json:"governing_contracts,omitempty"`
		RelevantInvariants []string   `json:"relevant_invariants,omitempty"`
		KnownFailureModes  []string   `json:"known_failure_modes,omitempty"`
		ForbiddenRepairs   []string   `json:"forbidden_repairs,omitempty"`
	}{
		SchemaVersion: e.SchemaVersion, ID: e.ID, Kind: e.Kind, Intent: e.Intent,
		SourceRevision: e.SourceRevision, ProductionRevision: e.ProductionRevision,
		RiskClass: e.RiskClass, AuthorityScope: sorted(e.AuthorityScope),
		GoverningContracts: sorted(e.GoverningContracts), RelevantInvariants: sorted(e.RelevantInvariants),
		KnownFailureModes: sorted(e.KnownFailureModes), ForbiddenRepairs: sorted(e.ForbiddenRepairs),
	}
	b, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func validChangeKind(k ChangeKind) bool {
	switch k {
	case ChangeIncidentRepair, ChangeSimulationRepair, ChangeFeature, ChangeArchitectureEvolution:
		return true
	}
	return false
}
func validRisk(r RiskClass) bool {
	switch r {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return true
	}
	return false
}
func validStage(s ChangeStage) bool {
	switch s {
	case StageDraft, StageCandidate, StageProven, StageAdmitted, StageReleased, StageVerified, StageLearned, StageBlocked:
		return true
	}
	return false
}

var stageRank = map[ChangeStage]int{StageDraft: 0, StageCandidate: 1, StageProven: 2, StageAdmitted: 3, StageReleased: 4, StageVerified: 5, StageLearned: 6}

func stageAtLeast(got, want ChangeStage) bool {
	if got == StageBlocked {
		return false
	}
	return stageRank[got] >= stageRank[want]
}

func validateUnique(name string, vals []string) error {
	seen := map[string]struct{}{}
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
		if _, ok := seen[v]; ok {
			return fmt.Errorf("%s contains duplicate %q", name, v)
		}
		seen[v] = struct{}{}
	}
	return nil
}

func sorted(in []string) []string { out := append([]string(nil), in...); sort.Strings(out); return out }
