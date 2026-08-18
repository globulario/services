package evolution

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func LoadChangeEnvelope(path string) (ChangeEnvelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ChangeEnvelope{}, fmt.Errorf("read change envelope: %w", err)
	}
	var envelope ChangeEnvelope
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		err = json.Unmarshal(data, &envelope)
	default:
		err = yaml.Unmarshal(data, &envelope)
	}
	if err != nil {
		return ChangeEnvelope{}, fmt.Errorf("decode change envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		// PROVEN is a derived claim, never an independently stored fact. When the
		// stored evidence no longer substantiates it, the honest reading of the
		// file is CANDIDATE — the claim is dropped, never carried forward. Any
		// other validation failure is a hard refusal: this only ever demotes.
		if envelope.Stage == StageProven {
			demoted := envelope
			demoted.Stage = StageCandidate
			if demoted.Validate() == nil {
				return demoted, nil
			}
		}
		return ChangeEnvelope{}, fmt.Errorf("validate change envelope: %w", err)
	}
	return envelope, nil
}

func SaveChangeEnvelope(path string, envelope ChangeEnvelope) error {
	envelope.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if envelope.CreatedAt == "" {
		envelope.CreatedAt = envelope.UpdatedAt
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("refuse to persist invalid change envelope: %w", err)
	}
	var (
		data []byte
		err  error
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		data, err = json.MarshalIndent(envelope, "", "  ")
		if err == nil {
			data = append(data, '\n')
		}
	default:
		data, err = yaml.Marshal(envelope)
	}
	if err != nil {
		return fmt.Errorf("encode change envelope: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write change envelope: %w", err)
	}
	return nil
}

func (e *ChangeEnvelope) AddOrReplaceProof(proof ProofRecord) {
	for idx := range e.Proofs {
		if e.Proofs[idx].Scenario == proof.Scenario &&
			e.Proofs[idx].CandidateRevision == proof.CandidateRevision {
			e.Proofs[idx] = proof
			return
		}
	}
	e.Proofs = append(e.Proofs, proof)
}

func (e *ChangeEnvelope) AddOrReplaceTest(test TestRecord) {
	for idx := range e.Tests {
		if e.Tests[idx].Name == test.Name &&
			e.Tests[idx].CandidateRevision == test.CandidateRevision {
			e.Tests[idx] = test
			return
		}
	}
	e.Tests = append(e.Tests, test)
}

// ReconcileProofStage keeps CANDIDATE/PROVEN aligned with current exact-revision
// evidence. A failed rerun can downgrade PROVEN back to CANDIDATE. ADMITTED and
// later stages are immutable history; changing their proof set requires a new
// candidate revision instead of rewriting accepted evidence.
func (e *ChangeEnvelope) ReconcileProofStage() bool {
	if e.Stage != StageCandidate && e.Stage != StageProven {
		return false
	}
	candidate := *e
	candidate.Stage = StageProven
	if err := candidate.Validate(); err != nil {
		e.Stage = StageCandidate
		return false
	}
	e.Stage = StageProven
	return true
}

// MarkProvenIfComplete is kept as the intent-revealing caller API.
func (e *ChangeEnvelope) MarkProvenIfComplete() bool {
	return e.ReconcileProofStage()
}
