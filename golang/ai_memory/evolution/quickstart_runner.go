package evolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type QuickstartProofArtifact struct {
	Scenario       string        `json:"scenario"`
	Suite          string        `json:"suite"`
	SourceRevision string        `json:"source_revision"`
	Status         string        `json:"status"`
	Change         ChangeBinding `json:"change"`
	Execution      struct {
		Result        string `json:"result"`
		ProofEligible bool   `json:"proof_eligible"`
	} `json:"execution"`
}

type QuickstartRunOptions struct {
	QuickstartDir string
	Scenario      string
	EnvelopePath  string
	KeepArtifacts bool
	Verbose       bool
}

type QuickstartRunResult struct {
	ExitCode     int
	ProofPath    string
	LearningPath string
	Proof        ProofRecord
	MarkedProven bool
}

// RunQuickstartScenario executes one proof-boundary scenario against an exact
// candidate revision and frozen proof plan, then persists the resulting proof
// into its ChangeEnvelope. Required local/static tests must already be green.
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

	testBin := filepath.Join(opts.QuickstartDir, "tests", "harness", "bin", "globular-test")
	if _, err := os.Stat(testBin); err != nil {
		return QuickstartRunResult{}, fmt.Errorf("quickstart proof runner unavailable: %w", err)
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
	)

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return QuickstartRunResult{}, fmt.Errorf("run quickstart scenario: %w", runErr)
		}
	}

	latest := filepath.Join(opts.QuickstartDir, "tests", "reports", "latest")
	concreteRunDir, err := filepath.EvalSymlinks(latest)
	if err != nil {
		return QuickstartRunResult{ExitCode: exitCode}, fmt.Errorf(
			"resolve quickstart concrete report run from %q: %w",
			latest,
			err,
		)
	}
	if !filepath.IsAbs(concreteRunDir) {
		concreteRunDir, err = filepath.Abs(concreteRunDir)
		if err != nil {
			return QuickstartRunResult{ExitCode: exitCode}, fmt.Errorf(
				"resolve quickstart report run path: %w",
				err,
			)
		}
	}
	proofPath := filepath.Join(concreteRunDir, "scenario-proof.json")
	learningPath := filepath.Join(concreteRunDir, "learning.json")
	artifact, err := loadQuickstartProof(proofPath)
	if err != nil {
		return QuickstartRunResult{
			ExitCode:     exitCode,
			ProofPath:    proofPath,
			LearningPath: learningPath,
		}, err
	}
	if artifact.Change.ID != envelope.ID {
		return QuickstartRunResult{}, fmt.Errorf(
			"proof change id %q does not match envelope %q",
			artifact.Change.ID,
			envelope.ID,
		)
	}
	if artifact.Change.CandidateRepository != envelope.CandidateRepository ||
		artifact.Change.CandidateRevision != envelope.CandidateRevision {
		return QuickstartRunResult{}, fmt.Errorf(
			"proof candidate %s@%s does not match envelope %s@%s",
			artifact.Change.CandidateRepository,
			artifact.Change.CandidateRevision,
			envelope.CandidateRepository,
			envelope.CandidateRevision,
		)
	}
	if artifact.Change.PlanDigest != envelope.PlanDigest {
		return QuickstartRunResult{}, fmt.Errorf(
			"proof plan digest %q does not match envelope %q",
			artifact.Change.PlanDigest,
			envelope.PlanDigest,
		)
	}
	if artifact.Change.SimulationRevision != artifact.SourceRevision {
		return QuickstartRunResult{}, fmt.Errorf(
			"proof simulation revision %q does not match proof source revision %q",
			artifact.Change.SimulationRevision,
			artifact.SourceRevision,
		)
	}

	proof := ProofRecord{
		Scenario:            artifact.Scenario,
		Repository:          "globulario/globular-quickstart",
		SimulationRevision:  artifact.SourceRevision,
		CandidateRepository: artifact.Change.CandidateRepository,
		CandidateRevision:   artifact.Change.CandidateRevision,
		Result:              artifact.Execution.Result,
		ProofEligible:       artifact.Execution.ProofEligible && artifact.Status == "SUPPORTED",
		ProofRef:            proofPath,
		EvidenceRef:         filepath.Join(concreteRunDir, "evidence.json"),
	}
	envelope.AddOrReplaceProof(proof)
	marked := envelope.ReconcileProofStage()
	if err := SaveChangeEnvelope(opts.EnvelopePath, envelope); err != nil {
		return QuickstartRunResult{}, err
	}

	return QuickstartRunResult{
		ExitCode:     exitCode,
		ProofPath:    proofPath,
		LearningPath: learningPath,
		Proof:        proof,
		MarkedProven: marked,
	}, nil
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
	return proof, nil
}
