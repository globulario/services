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
	ExitCode int
	// ChangeID is carried so a caller ingesting the learning artifact can bind
	// it to this exact proof occurrence without reopening the envelope.
	ChangeID     string
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

	identity := envelope.Identity()

	// From here on the invocation owns a proof slot. Any path that fails to fill
	// it with a valid current proof must drop whatever older claim stood for this
	// scenario, or a failed rerun would leave the durable envelope at PROVEN.
	fail := func(runResult QuickstartRunResult, cause error) (QuickstartRunResult, error) {
		if invalidateErr := invalidateScenarioProof(opts.EnvelopePath, identity, scenarioName); invalidateErr != nil {
			return runResult, fmt.Errorf("%w (and could not withdraw the prior proof claim: %v)", cause, invalidateErr)
		}
		return runResult, cause
	}

	// The simulator is the other half of the experiment, so it needs the same
	// exactness as the candidate. A quickstart checkout can carry tracked,
	// staged, or untracked edits while still reporting a clean HEAD, which would
	// let modified simulator code earn proof attributed to the committed
	// revision. Executing from a throwaway detached worktree makes the simulator
	// contents exactly that revision by construction:
	//
	//     exact candidate revision × exact simulator revision = proof occurrence
	simulationRevision, err := resolveHeadRevision(ctx, opts.QuickstartDir)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}
	simTree, releaseSimTree, err := materializeCandidateTree(ctx, opts.QuickstartDir, simulationRevision)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, fmt.Errorf("materialize simulator revision: %w", err))
	}
	defer releaseSimTree()

	testBin := filepath.Join(simTree, "tests", "harness", "bin", "globular-test")
	if _, err := os.Stat(testBin); err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, fmt.Errorf("quickstart proof runner unavailable: %w", err))
	}
	scenarioPath, err := rebaseScenarioPath(opts.QuickstartDir, simTree, opts.Scenario)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}

	invocationID, err := newInvocationID()
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}
	runDir, err := createInvocationRunDir(opts.EnvelopePath, envelope.ID, invocationID)
	if err != nil {
		return fail(QuickstartRunResult{ChangeID: envelope.ID}, err)
	}

	args := []string{"scenario", scenarioPath}
	if opts.KeepArtifacts {
		args = append(args, "--keep-artifacts")
	}
	if opts.Verbose {
		args = append(args, "--verbose")
	}
	cmd := exec.CommandContext(ctx, testBin, args...)
	cmd.Dir = simTree
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
				QuickstartRunResult{ChangeID: envelope.ID, InvocationID: invocationID},
				fmt.Errorf("run quickstart scenario: %w", runErr),
			)
		}
	}

	proofPath := filepath.Join(runDir, "scenario-proof.json")
	learningPath := filepath.Join(runDir, "learning.json")
	partial := QuickstartRunResult{
		ExitCode:     exitCode,
		ChangeID:     envelope.ID,
		InvocationID: invocationID,
		ProofPath:    proofPath,
		LearningPath: learningPath,
	}

	// A run that did not succeed cannot certify, whatever it wrote. A harness can
	// emit a valid PASS artifact and then exit nonzero — teardown or artifact
	// cleanup failing after the scenario itself passed — and recording that would
	// leave the durable envelope PROVEN off a run the operator was told failed.
	if exitCode != 0 {
		return fail(partial, fmt.Errorf(
			"quickstart scenario exited %d; a run that did not succeed cannot certify a candidate",
			exitCode,
		))
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
	if artifact.SourceRevision != simulationRevision {
		return fail(partial, fmt.Errorf(
			"proof simulation revision %q is not the simulator revision this run executed %q",
			artifact.SourceRevision,
			simulationRevision,
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

	marked, err := MutateEnvelope(opts.EnvelopePath, identity, func(e *ChangeEnvelope) {
		e.AddOrReplaceProof(proof)
	})
	if err != nil {
		return partial, err
	}

	return QuickstartRunResult{
		ExitCode:     exitCode,
		ChangeID:     envelope.ID,
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

// createInvocationRunDir creates the directory this invocation owns.
//
// It sits beside the envelope's other evidence, deliberately outside both the
// candidate and the simulator checkouts. The simulator now executes from a
// throwaway worktree that is discarded afterwards, so anything written inside it
// would not survive to be verified; and the real quickstart checkout is not a
// scratch space for proof output either.
//
// It must not already exist: an existing directory could hold a sibling run's
// artifacts, and consuming those is the ambiguity this mechanism removes.
func createInvocationRunDir(envelopePath, changeID, invocationID string) (string, error) {
	base := filepath.Join(
		filepath.Dir(envelopePath),
		".evolution-evidence",
		safeEvidenceName(changeID),
		"scenarios",
	)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("create proof evidence dir: %w", err)
	}
	runDir := filepath.Join(base, safeEvidenceName(invocationID))
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
func invalidateScenarioProof(envelopePath string, identity EnvelopeIdentity, scenario string) error {
	_, err := MutateEnvelope(envelopePath, identity, func(e *ChangeEnvelope) {
		kept := e.Proofs[:0]
		for _, proof := range e.Proofs {
			if proof.Scenario == scenario && proof.CandidateRevision == e.CandidateRevision {
				continue
			}
			kept = append(kept, proof)
		}
		e.Proofs = kept
	})
	return err
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

func resolveHeadRevision(ctx context.Context, dir string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("resolve simulator revision in %q: %w", dir, err)
	}
	revision := strings.TrimSpace(string(out))
	if revision == "" {
		return "", fmt.Errorf("simulator checkout %q has no resolvable revision", dir)
	}
	return revision, nil
}

// rebaseScenarioPath maps a scenario named against the operator's quickstart
// checkout onto the detached worktree that is actually executing. Running the
// committed harness against an uncommitted scenario file would reintroduce the
// contamination the worktree exists to prevent.
func rebaseScenarioPath(quickstartDir, simTree, scenario string) (string, error) {
	root, err := filepath.Abs(quickstartDir)
	if err != nil {
		return "", fmt.Errorf("resolve quickstart dir: %w", err)
	}
	target := scenario
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", fmt.Errorf("resolve scenario path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("scenario %q is outside the quickstart checkout", scenario)
	}
	rebased := filepath.Join(simTree, rel)
	if _, err := os.Stat(rebased); err != nil {
		return "", fmt.Errorf(
			"scenario %q is not present at simulator revision: %w; commit it before proving against it",
			rel, err,
		)
	}
	return rebased, nil
}
