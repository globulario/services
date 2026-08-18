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
	data, err := encodeChangeEnvelope(path, envelope)
	if err != nil {
		return err
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

// CreateChangeEnvelope publishes a new envelope and fails if one already exists.
//
// An envelope is the durable record of one candidate's proof, admission, and
// release history, so creating over an existing one erases that history in place
// rather than superseding it. Checking for the file and then writing it is not
// enough: two concurrent creations both observe the absence and the second
// silently replaces the first. Exclusive create makes the check and the publish
// one operation, so exactly one caller can win.
func CreateChangeEnvelope(path string, envelope ChangeEnvelope) error {
	envelope.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if envelope.CreatedAt == "" {
		envelope.CreatedAt = envelope.UpdatedAt
	}
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("refuse to persist invalid change envelope: %w", err)
	}
	data, err := encodeChangeEnvelope(path, envelope)
	if err != nil {
		return err
	}
	// Writing straight to the published path makes a partial file visible at the
	// name readers use, and a crash leaves that corrupt file behind where every
	// retry is then refused as already-existing. Stage the complete file
	// privately, then install it with a hard link: link is atomic and fails if
	// the destination exists, so the no-overwrite guarantee survives without
	// publishing anything half-written.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".change-envelope-new-*")
	if err != nil {
		return fmt.Errorf("stage change envelope: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write change envelope: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set change envelope mode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("flush change envelope: %w", err)
	}
	if err := os.Link(tmpName, path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf(
				"%s already exists; a new change needs its own envelope, and an existing one is superseded by a new candidate rather than overwritten",
				path,
			)
		}
		return fmt.Errorf("publish change envelope: %w", err)
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

// mutateEnvelopeLocked is the transactional core every durable envelope change
// goes through. It is a compare-and-swap on the pre-state: the caller says what
// it believed the envelope was when it started, and the change is applied only
// if that is still what is on disk.
//
// A long operation runs outside this — a simulation, a test, an operator
// deciding. Writing back the snapshot read before it would silently undo
// whatever else happened meanwhile, including a withdrawal, which would
// resurrect an invalidated proof.
func mutateEnvelopeLocked(path string, expected EnvelopeIdentity, apply func(*ChangeEnvelope) error) error {
	return withEnvelopeLock(path, func() error {
		current, err := LoadChangeEnvelope(path)
		if err != nil {
			return err
		}
		if current.Identity() != expected {
			return fmt.Errorf(
				"change envelope moved under this operation: expected %s@%s plan %s, found %s@%s plan %s; "+
					"reload before applying a change",
				expected.ID, expected.CandidateRevision, expected.PlanDigest,
				current.ID, current.CandidateRevision, current.PlanDigest,
			)
		}
		if err := apply(&current); err != nil {
			return err
		}
		return SaveChangeEnvelope(path, current)
	})
}

// MutateEnvelope applies one proof result. It is the single durable owner of
// proof state: recording a proof and withdrawing one both come through here,
// because two owners would reintroduce the race this exists to remove.
//
// Artifact verification belongs to this transition rather than to whichever
// wrapper is driving. ReconcileProofStage checks stored references and digests,
// which is portable and stays that way so an envelope can be reviewed away from
// its artifacts. This only ever runs where the evidence lives, so a digest that
// can no longer be re-derived demotes the claim before it is committed.
func MutateEnvelope(path string, identity EnvelopeIdentity, apply func(*ChangeEnvelope)) (bool, error) {
	var marked bool
	err := mutateEnvelopeLocked(path, identity, func(current *ChangeEnvelope) error {
		if current.Stage != StageCandidate && current.Stage != StageProven {
			return fmt.Errorf(
				"change envelope is at stage %s; proof results only apply to CANDIDATE or PROVEN",
				current.Stage,
			)
		}
		apply(current)
		marked = current.ReconcileProofStage()
		if marked {
			if err := current.VerifyEvidenceArtifacts(); err != nil {
				current.Stage = StageCandidate
				marked = false
			}
		}
		return nil
	})
	return marked, err
}

// RebindCandidate moves an envelope to a new candidate revision through the same
// transaction. Rebinding deliberately changes the identity, so it cannot use
// MutateEnvelope — but it is still a durable read-modify-write and must not be
// the one path that writes a stale snapshot outside the lock.
func RebindCandidate(path string, expected EnvelopeIdentity, repository, revision string) (ChangeEnvelope, error) {
	var out ChangeEnvelope
	err := mutateEnvelopeLocked(path, expected, func(current *ChangeEnvelope) error {
		if err := current.BindCandidate(repository, revision); err != nil {
			return err
		}
		out = *current
		return nil
	})
	return out, err
}

func encodeChangeEnvelope(path string, envelope ChangeEnvelope) ([]byte, error) {
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
		return nil, fmt.Errorf("encode change envelope: %w", err)
	}
	return data, nil
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
