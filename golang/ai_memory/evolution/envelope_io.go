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
	// Write through a sibling temp file and rename, so a reader never observes a
	// half-written envelope and a crash mid-write cannot truncate the record.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".change-envelope-*")
	if err != nil {
		return fmt.Errorf("stage change envelope: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write change envelope: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("set change envelope mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("flush change envelope: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("commit change envelope: %w", err)
	}
	return nil
}

// EnvelopeIdentity is the part of a ChangeEnvelope an in-flight invocation
// assumed when it started. A mutation is only applied if the envelope on disk
// still describes that same thing.
type EnvelopeIdentity struct {
	ID                string
	CandidateRevision string
	PlanDigest        string
}

func (e ChangeEnvelope) Identity() EnvelopeIdentity {
	return EnvelopeIdentity{
		ID:                e.ID,
		CandidateRevision: e.CandidateRevision,
		PlanDigest:        e.PlanDigest,
	}
}

// MutateEnvelope is the single durable owner of ChangeEnvelope mutation.
//
// A scenario runs for a long time between reading the envelope and persisting a
// result, and invocations for the same change can overlap. Writing back the
// snapshot read before the run would silently undo whatever another invocation
// decided meanwhile — including a withdrawal, which would resurrect a proof that
// had been invalidated. So the long simulation happens outside any lock, and
// only the mutation is transactional: take the lock, reload what is actually on
// disk now, confirm it is still the same candidate under the same frozen plan,
// apply just this invocation's delta, reconcile, and commit atomically.
//
// Every path that changes an envelope goes through here — recording a proof and
// withdrawing one alike. Two owners would reintroduce exactly the race this
// exists to remove.
func MutateEnvelope(path string, identity EnvelopeIdentity, apply func(*ChangeEnvelope)) (bool, error) {
	var marked bool
	err := withEnvelopeLock(path, func() error {
		current, err := LoadChangeEnvelope(path)
		if err != nil {
			return err
		}
		if current.Identity() != identity {
			return fmt.Errorf(
				"change envelope moved under this invocation: expected %s@%s plan %s, found %s@%s plan %s; "+
					"a result cannot be applied to a different candidate",
				identity.ID, identity.CandidateRevision, identity.PlanDigest,
				current.ID, current.CandidateRevision, current.PlanDigest,
			)
		}
		if current.Stage != StageCandidate && current.Stage != StageProven {
			return fmt.Errorf(
				"change envelope is at stage %s; proof results only apply to CANDIDATE or PROVEN",
				current.Stage,
			)
		}
		apply(&current)
		marked = current.ReconcileProofStage()
		return SaveChangeEnvelope(path, current)
	})
	return marked, err
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
