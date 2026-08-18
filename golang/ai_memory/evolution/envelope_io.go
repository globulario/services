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
		if e.Proofs[idx].Scenario == proof.Scenario && e.Proofs[idx].CandidateRevision == proof.CandidateRevision {
			e.Proofs[idx] = proof
			return
		}
	}
	e.Proofs = append(e.Proofs, proof)
}

// MarkProvenIfComplete advances only the local proof stage. It does not perform
// Sensei admission, release, production verification, or behavioral promotion.
func (e *ChangeEnvelope) MarkProvenIfComplete() bool {
	if e.Stage != StageCandidate && e.Stage != StageProven {
		return false
	}
	candidate := *e
	candidate.Stage = StageProven
	if err := candidate.Validate(); err != nil {
		return false
	}
	e.Stage = StageProven
	return true
}
