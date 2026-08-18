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
		if scenarioResult.ExitCode != 0 || !scenarioResult.Proof.ProofEligible || scenarioResult.Proof.Result != "PASS" {
			return withProofStatus(opts.EnvelopePath, result), fmt.Errorf(
				"required scenario %q did not produce eligible PASS proof",
				requirement.Name,
			)
		}
	}

	result = withProofStatus(opts.EnvelopePath, result)
	if !result.Status.ProofComplete || result.Status.Stage != StageProven {
		return result, fmt.Errorf("proof plan completed without reaching PROVEN")
	}
	return result, nil
}

func withProofStatus(path string, result ProofPlanResult) ProofPlanResult {
	if envelope, err := LoadChangeEnvelope(path); err == nil {
		result.Status = envelope.ProofStatus()
	}
	return result
}
