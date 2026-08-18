package evolution

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ProofInvocation is the identity of exactly one scenario execution. The runner
// mints it, hands it to the harness, and refuses any artifact that does not
// carry it back. It is what makes "this proof came from this run" checkable
// rather than assumed.
type ProofInvocation struct {
	ID     string `json:"id"`
	RunDir string `json:"run_dir,omitempty"`
}

type QuickstartProofArtifact struct {
	Scenario       string          `json:"scenario"`
	Suite          string          `json:"suite"`
	SourceRevision string          `json:"source_revision"`
	Status         string          `json:"status"`
	Change         ChangeBinding   `json:"change"`
	Invocation     ProofInvocation `json:"invocation"`
	Execution      struct {
		Result        string `json:"result"`
		ProofEligible bool   `json:"proof_eligible"`
	} `json:"execution"`
}

type QuickstartRunOptions struct {
	QuickstartDir string
	Scenario      string
	// ScenarioName is the required-scenario name this run answers for. It is
	// what a stale proof would otherwise be credited against, so the runner
	// needs it to invalidate that claim when this invocation proves nothing.
	ScenarioName  string
	EnvelopePath  string
	KeepArtifacts bool
	Verbose       bool
}

type QuickstartRunResult struct {
	ExitCode     int
	InvocationID string
	ProofPath    string
	LearningPath string
	Proof        ProofRecord
	MarkedProven bool
}

// RunQuickstartScenario executes one proof-boundary scenario against an exact
// candidate revision and frozen proof plan, then persists the resulting proof
// into its ChangeEnvelope. Required local/static tests must already be green.
//
// The proof it consumes is the one this invocation produced, in a directory this
// invocation created, stamped with an identity this invocation minted. It never
// reads the shared `tests/reports/latest` pointer: that pointer names whichever
// run rotated it last, so a failing rerun that exits before rotating it, or a
// concurrent run for the same change, could otherwise hand back an earlier PASS.
func RunQuickstartScenario(ctx context.Context, opts QuickstartRunOptions) (QuickstartRunResult, error) {
	if strings.TrimSpace(opts.QuickstartDir) == "" ||
		strings.TrimSpace(opts.Scenario) == "" ||
		strings.TrimSpace(opts.EnvelopePath) == "" {
		return QuickstartRunResult{}, fmt.Errorf("quickstart-dir, scenario, and envelope path are required")
	}
	envelope, err := LoadChangeEnvelope(opts.EnvelopePath)
	if err != nil {
		return QuickstartRunResult{}, err
	}
	if envelope.Stage != StageCandidate && envelope.Stage != StageProven {
		return QuickstartRunResult{}, fmt.Errorf(
			"quickstart proof requires envelope stage CANDIDATE or PROVEN, got %s",
			envelope.Stage,
		)
	}
	if envelope.CandidateRepository == "" || envelope.CandidateRevision == "" || envelope.PlanDigest == "" {
		return QuickstartRunResult{}, fmt.Errorf("candidate repository/revision/plan digest are required before simulation")
	}
	if err := envelope.ValidateRequiredTestClosure(); err != nil {
		return QuickstartRunResult{}, fmt.Errorf("local proof gate: %w", err)
	}

	scenarioName := strings.TrimSpace(opts.ScenarioName)
	if scenarioName == "" {
		scenarioName = strings.TrimSuffix(filepath.Base(opts.Scenario), filepath.Ext(opts.Scenario))
	}

	testBin := filepath.Join(opts.QuickstartDir, "tests", "harness", "bin", "globular-test")
	if _, err := os.Stat(testBin); err != nil {
		return QuickstartRunResult{}, fmt.Errorf("quickstart proof runner unavailable: %w", err)
	}

	invocationID, err := newInvocationID()
	if err != nil {
		return QuickstartRunResult{}, err
	}
	runDir, err := createInvocationRunDir(opts.QuickstartDir, invocationID)
	if err != nil {
		return QuickstartRunResult{}, err
	}

	// From here on the invocation owns a proof slot. Any path that fails to fill
	// it with a valid current proof must drop whatever older claim stood for this
	// scenario, or a failed rerun would leave the durable envelope at PROVEN.
	fail := func(runResult QuickstartRunResult, cause error) (QuickstartRunResult, error) {
		if invalidateErr := invalidateScenarioProof(opts.EnvelopePath, scenarioName); invalidateErr != nil {
			return runResult, fmt.Errorf("%w (and could not withdraw the prior proof claim: %v)", cause, invalidateErr)
		}
		return runResult, cause
	}

	args := []string{"scenario", opts.Scenario}
	if opts.KeepArtifacts {
		args = append(args, "--keep-artifacts")
	}
	if opts.Verbose {
		args = append(args, "--verbose")
	}
	cmd := exec.CommandContext(ctx, testBin, args...)
	cmd.Dir = opts.QuickstartDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(
		os.Environ(),
		"GLOBULAR_CHANGE_ID="+envelope.ID,
		"GLOBULAR_CHANGE_ENVELOPE_REF="+opts.EnvelopePath,
		"GLOBULAR_CANDIDATE_REPOSITORY="+envelope.CandidateRepository,
		"GLOBULAR_CANDIDATE_REVISION="+envelope.CandidateRevision,
		"GLOBULAR_CHANGE_PLAN_DIGEST="+envelope.PlanDigest,
		"GLOBULAR_REQUIRE_CHANGE_BINDING=1",
		"GLOBULAR_PROOF_RUN_DIR="+runDir,
		"GLOBULAR_PROOF_INVOCATION_ID="+invocationID,
	)

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return fail(
				QuickstartRunResult{InvocationID: invocationID},
				fmt.Errorf("run quickstart scenario: %w", runErr),
			)
		}
	}

	proofPath := filepath.Join(runDir, "scenario-proof.json")
	learningPath := filepath.Join(runDir, "learning.json")
	partial := QuickstartRunResult{
		ExitCode:     exitCode,
		InvocationID: invocationID,
		ProofPath:    proofPath,
		LearningPath: learningPath,
	}

	artifact, err := loadQuickstartProof(proofPath)
	if err != nil {
		return fail(partial, err)
	}
	if artifact.Invocation.ID != invocationID {
		return fail(partial, fmt.Errorf(
			"proof invocation %q does not belong to this run %q; refusing a proof this invocation did not produce",
			artifact.Invocation.ID,
			invocationID,
		))
	}
	if artifact.Scenario != scenarioName {
		// A stamped, otherwise valid PASS that answers a different obligation
		// must not be recorded: it would land under the other scenario's name
		// and leave this scenario's earlier PASS standing, so closure would stay
		// green on evidence this invocation never produced.
		return fail(partial, fmt.Errorf(
			"proof answers scenario %q, not the requested %q; a proof for another obligation cannot satisfy this one",
			artifact.Scenario,
			scenarioName,
		))
	}
	if artifact.Change.ID != envelope.ID {
		return fail(partial, fmt.Errorf(
			"proof change id %q does not match envelope %q",
			artifact.Change.ID,
			envelope.ID,
		))
	}
	if artifact.Change.CandidateRepository != envelope.CandidateRepository ||
		artifact.Change.CandidateRevision != envelope.CandidateRevision {
		return fail(partial, fmt.Errorf(
			"proof candidate %s@%s does not match envelope %s@%s",
			artifact.Change.CandidateRepository,
			artifact.Change.CandidateRevision,
			envelope.CandidateRepository,
			envelope.CandidateRevision,
		))
	}
	if artifact.Change.PlanDigest != envelope.PlanDigest {
		return fail(partial, fmt.Errorf(
			"proof plan digest %q does not match envelope %q",
			artifact.Change.PlanDigest,
			envelope.PlanDigest,
		))
	}
	if artifact.Change.SimulationRevision != artifact.SourceRevision {
		return fail(partial, fmt.Errorf(
			"proof simulation revision %q does not match proof source revision %q",
			artifact.Change.SimulationRevision,
			artifact.SourceRevision,
		))
	}

	proof := ProofRecord{
		Scenario:            artifact.Scenario,
		Repository:          "globulario/globular-quickstart",
		SimulationRevision:  artifact.SourceRevision,
		CandidateRepository: artifact.Change.CandidateRepository,
		CandidateRevision:   artifact.Change.CandidateRevision,
		PlanDigest:          artifact.Change.PlanDigest,
		InvocationID:        invocationID,
		Result:              artifact.Execution.Result,
		ProofEligible:       artifact.Execution.ProofEligible && artifact.Status == "SUPPORTED",
		ProofRef:            proofPath,
	}
	evidencePath := filepath.Join(runDir, "evidence.json")
	if _, statErr := os.Stat(evidencePath); statErr == nil {
		proof.EvidenceRef = evidencePath
	}
	// The digest is taken over exactly the artifacts the record names, in the
	// order VerifyEvidenceArtifacts recomputes them.
	digest, err := DigestFiles(proof.evidenceArtifacts()...)
	if err != nil {
		return fail(partial, fmt.Errorf("digest scenario proof artifacts: %w", err))
	}
	proof.Digest = digest

	envelope.AddOrReplaceProof(proof)
	marked := envelope.ReconcileProofStage()
	if err := SaveChangeEnvelope(opts.EnvelopePath, envelope); err != nil {
		return partial, err
	}

	return QuickstartRunResult{
		ExitCode:     exitCode,
		InvocationID: invocationID,
		ProofPath:    proofPath,
		LearningPath: learningPath,
		Proof:        proof,
		MarkedProven: marked,
	}, nil
}

func newInvocationID() (string, error) {
	var entropy [8]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("generate proof invocation id: %w", err)
	}
	return fmt.Sprintf(
		"inv-%s-%s",
		time.Now().UTC().Format("20060102T150405Z"),
		hex.EncodeToString(entropy[:]),
	), nil
}

// createInvocationRunDir creates the directory this invocation owns. It must not
// already exist: an existing directory could hold a sibling run's artifacts, and
// consuming those is the ambiguity this whole mechanism removes.
func createInvocationRunDir(quickstartDir, invocationID string) (string, error) {
	reports := filepath.Join(quickstartDir, "tests", "reports")
	if err := os.MkdirAll(reports, 0o755); err != nil {
		return "", fmt.Errorf("create quickstart reports dir: %w", err)
	}
	runDir := filepath.Join(reports, invocationID)
	if err := os.Mkdir(runDir, 0o755); err != nil {
		return "", fmt.Errorf("create proof invocation run dir: %w", err)
	}
	abs, err := filepath.Abs(runDir)
	if err != nil {
		return "", fmt.Errorf("resolve proof invocation run dir: %w", err)
	}
	return abs, nil
}

// invalidateScenarioProof withdraws any standing proof claim for one required
// scenario at the current candidate revision and reconciles the stage. It only
// ever removes a claim; it never asserts a result this invocation cannot show.
func invalidateScenarioProof(envelopePath, scenario string) error {
	envelope, err := LoadChangeEnvelope(envelopePath)
	if err != nil {
		return err
	}
	if envelope.Stage != StageCandidate && envelope.Stage != StageProven {
		return nil
	}
	kept := envelope.Proofs[:0]
	removed := false
	for _, proof := range envelope.Proofs {
		if proof.Scenario == scenario && proof.CandidateRevision == envelope.CandidateRevision {
			removed = true
			continue
		}
		kept = append(kept, proof)
	}
	if !removed && envelope.Stage == StageCandidate {
		return nil
	}
	envelope.Proofs = kept
	envelope.ReconcileProofStage()
	return SaveChangeEnvelope(envelopePath, envelope)
}

func loadQuickstartProof(path string) (QuickstartProofArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return QuickstartProofArtifact{}, fmt.Errorf("read quickstart proof: %w", err)
	}
	var proof QuickstartProofArtifact
	if err := json.Unmarshal(data, &proof); err != nil {
		return QuickstartProofArtifact{}, fmt.Errorf("decode quickstart proof: %w", err)
	}
	if proof.Scenario == "" || proof.SourceRevision == "" || proof.Execution.Result == "" {
		return QuickstartProofArtifact{}, fmt.Errorf(
			"quickstart proof missing scenario/source_revision/execution.result",
		)
	}
	if strings.TrimSpace(proof.Invocation.ID) == "" {
		return QuickstartProofArtifact{}, fmt.Errorf(
			"quickstart proof carries no invocation identity; the harness must stamp the invocation that produced it",
		)
	}
	return proof, nil
}
