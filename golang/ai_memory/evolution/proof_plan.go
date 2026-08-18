package evolution

import (
	"context"
	"fmt"
	"path/filepath"
)

type ProofPlanOptions struct {
	EnvelopePath  string
	WorkspaceDir  string
	QuickstartDir string
	KeepArtifacts bool
	Verbose       bool
}

type ProofPlanResult struct {
	Tests     []TestRunResult       `json:"tests"`
	Scenarios []QuickstartRunResult `json:"scenarios"`
	Status    ProofStatus           `json:"status"`
}

// RunProofPlan executes the complete pre-admission proof plan declared by one
// ChangeEnvelope. It reruns required local tests first and stops immediately on
// a failure. Only after local closure does it run required quickstart scenarios,
// also stopping on the first failed scenario. It may reach PROVEN, never ADMITTED.
func RunProofPlan(ctx context.Context, opts ProofPlanOptions) (ProofPlanResult, error) {
	envelope, err := LoadChangeEnvelope(opts.EnvelopePath)
	if err != nil {
		return ProofPlanResult{}, err
	}
	if envelope.Stage != StageCandidate && envelope.Stage != StageProven {
		return ProofPlanResult{}, fmt.Errorf(
			"proof plan requires envelope stage CANDIDATE or PROVEN, got %s",
			envelope.Stage,
		)
	}

	result := ProofPlanResult{}
	for _, requirement := range envelope.RequiredTests {
		if !requirement.Required {
			continue
		}
		testResult, runErr := RunDeclaredTest(ctx, TestRunOptions{
			EnvelopePath: opts.EnvelopePath,
			WorkspaceDir: opts.WorkspaceDir,
			TestName:     requirement.Name,
		})
		result.Tests = append(result.Tests, testResult)
		if runErr != nil {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required test %q could not run: %w",
				requirement.Name,
				runErr,
			)
		}
		if testResult.ExitCode != 0 {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required test %q failed with exit code %d",
				requirement.Name,
				testResult.ExitCode,
			)
		}
	}

	// Reload because trusted test runs persist their evidence into the envelope.
	envelope, err = LoadChangeEnvelope(opts.EnvelopePath)
	if err != nil {
		return result, err
	}
	if err := envelope.ValidateRequiredTestClosure(); err != nil {
		return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
			"local proof closure incomplete: %w",
			err,
		)
	}

	for _, requirement := range envelope.RequiredScenarios {
		if !requirement.Required {
			continue
		}
		if requirement.Repository != "" && requirement.Repository != "globulario/globular-quickstart" {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"scenario %q uses unsupported simulation repository %q",
				requirement.Name,
				requirement.Repository,
			)
		}
		if requirement.Path == "" {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required scenario %q has no path",
				requirement.Name,
			)
		}
		scenarioPath := requirement.Path
		if !filepath.IsAbs(scenarioPath) {
			scenarioPath = filepath.Join(opts.QuickstartDir, scenarioPath)
		}
		scenarioResult, runErr := RunQuickstartScenario(ctx, QuickstartRunOptions{
			QuickstartDir: opts.QuickstartDir,
			Scenario:      scenarioPath,
			ScenarioName:  requirement.Name,
			EnvelopePath:  opts.EnvelopePath,
			KeepArtifacts: opts.KeepArtifacts,
			Verbose:       opts.Verbose,
		})
		result.Scenarios = append(result.Scenarios, scenarioResult)
		if runErr != nil {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required scenario %q could not complete proof: %w",
				requirement.Name,
				runErr,
			)
		}
		if scenarioResult.ExitCode != 0 {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required scenario %q exited %d",
				requirement.Name, scenarioResult.ExitCode,
			)
		}
		// Ask the same question certification asks, not the record's own account
		// of itself. A scenario run from a checkout that contradicts the frozen
		// plan reports an eligible PASS and is correctly refused certification —
		// but reading only its self-report would let orchestration carry on
		// launching later, potentially destructive lab scenarios and discover the
		// contradiction at final closure.
		if err := scenarioResult.CertifiesRequirement(requirement.Name); err != nil {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required scenario %q did not produce certifying proof: %w",
				requirement.Name, err,
			)
		}
	}

	result = withProofStatus(opts.EnvelopePath, result)
	if !result.Status.ProofComplete || result.Status.Stage != StageProven {
		return result, fmt.Errorf("proof plan completed without reaching PROVEN")
	}

	// Last gate before PROVEN stands. Everything above checks evidence identity,
	// which is portable metadata; here the artifacts themselves are still local,
	// so the recorded digests can be re-derived from them. An artifact that was
	// deleted or edited after it was recorded is not proof, and PROVEN must not
	// survive it.
	proven, err := LoadChangeEnvelope(opts.EnvelopePath)
	if err != nil {
		return result, err
	}
	if verifyErr := proven.VerifyEvidenceArtifacts(); verifyErr != nil {
		if err := withdrawUnreproducibleProof(opts.EnvelopePath, proven.Identity()); err != nil {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"%w (and could not withdraw the PROVEN claim: %v)", verifyErr, err,
			)
		}
		return withProofStatus(opts.EnvelopePath, result), fmt.Errorf("proof evidence is not reproducible: %w", verifyErr)
	}
	return result, nil
}

// withdrawUnreproducibleProof demotes through the one durable owner. Reading,
// verifying, and saving outside the lock would make this a second writer: a test
// or scenario recording or withdrawing proof between the load and the save would
// be overwritten by this stale snapshot, and a later mutation could reconcile the
// resurrected set straight back to PROVEN.
func withdrawUnreproducibleProof(path string, identity EnvelopeIdentity) error {
	_, err := MutateEnvelope(path, identity, func(e *ChangeEnvelope) {})
	return err
}

func withProofStatus(path string, result ProofPlanResult) ProofPlanResult {
	if envelope, err := LoadChangeEnvelope(path); err == nil {
		result.Status = envelope.ProofStatus()
	}
	return result
}
