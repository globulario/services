package evolution

import (
	"context"
	"path/filepath"
	"testing"
)

func TestProofPlanCanReachProvenWithDeclaredLocalTestsOnly(t *testing.T) {
	workspace, revision := initTestRepo(t)
	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-plan", ChangeFeature, "feature", revision, RiskMedium)
	e.RequiredTests = []TestRequirement{{
		Name:       "unit",
		Repository: "globulario/services",
		Command:    []string{"sh", "-c", "echo proof-plan"},
		Required:   true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	result, err := RunProofPlan(context.Background(), ProofPlanOptions{
		EnvelopePath: envelopePath,
		WorkspaceDir: workspace,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Tests) != 1 || len(result.Scenarios) != 0 {
		t.Fatalf("unexpected proof result: %+v", result)
	}
	if !result.Status.ProofComplete || result.Status.Stage != StageProven {
		t.Fatalf("proof plan did not reach PROVEN: %+v", result.Status)
	}
}

func TestProofPlanStopsBeforeSimulationOnLocalTestFailure(t *testing.T) {
	workspace, revision := initTestRepo(t)
	envelopePath := filepath.Join(t.TempDir(), "change.yaml")
	e := NewChangeEnvelope("chg-plan-fail", ChangeFeature, "feature", revision, RiskHigh)
	e.RequiredTests = []TestRequirement{{
		Name:       "must-pass",
		Repository: "globulario/services",
		Command:    []string{"sh", "-c", "exit 7"},
		Required:   true,
	}}
	e.RequiredScenarios = []ScenarioRequirement{{
		Name:       "must-not-run",
		Repository: "globulario/globular-quickstart",
		Path:       "tests/scenarios/resilience/must-not-run.yaml",
		Required:   true,
	}}
	if err := e.BindCandidate("globulario/services", revision); err != nil {
		t.Fatal(err)
	}
	if err := SaveChangeEnvelope(envelopePath, e); err != nil {
		t.Fatal(err)
	}
	result, err := RunProofPlan(context.Background(), ProofPlanOptions{
		EnvelopePath:  envelopePath,
		WorkspaceDir:  workspace,
		QuickstartDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected local proof failure")
	}
	if len(result.Tests) != 1 || len(result.Scenarios) != 0 {
		t.Fatalf("simulation ran after local test failure: %+v", result)
	}
	if result.Status.Stage != StageCandidate {
		t.Fatalf("failed proof plan must remain CANDIDATE: %+v", result.Status)
	}
}
