package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// F6 ordering law:
//
//	persist_run → dispatch_step → restart_unit → persist_receipt → persist_terminal
//
// The counterfeit StartChild branch inverted it entirely: it called
// restartInfraUnit (the mutation) FIRST, then minted a run ID from
// time.Now().UnixNano(), then stored a synthetic status in a process-local map.
// No durable run ever existed, so "persist_run" never happened at all — the
// mutation preceded the identity, not merely the persistence.
//
// This is a source-level ratchet rather than an engine harness because the
// defect is structural: the branch bypasses the workflow service entirely, so
// there is no seam to instrument until it is routed through
// executeWorkflowCentralized.

func reconcileActionsSource(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("reconcile_actions.go")
	if err != nil {
		t.Fatalf("read reconcile_actions.go: %v", err)
	}
	// Strip comments so prose describing the defect cannot satisfy or trip it.
	var out strings.Builder
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return out.String()
}

// No execution identity may be derived from the wall clock.
func TestInfraRestart_NoWallClockChildIdentity(t *testing.T) {
	src := reconcileActionsSource(t)
	if regexp.MustCompile(`restart-%s-%s-%d`).MatchString(src) ||
		strings.Contains(src, `"restart-"`) {
		t.Error("child run identity must not be a formatted restart-* string")
	}
	if strings.Contains(src, "time.Now().UnixNano()") {
		t.Error("execution identity must not derive from the wall clock; use a stable " +
			"correlation identity from parent run + step + node + component")
	}
}

// The controller must not perform the restart mutation inline.
func TestInfraRestart_ControllerDoesNotMutateInline(t *testing.T) {
	src := reconcileActionsSource(t)
	if strings.Contains(src, "srv.restartInfraUnit(") {
		t.Error("the controller must not invoke restartInfraUnit inline; the restart must " +
			"run as a governed node-agent step inside a durable workflow run")
	}
}

// Every child result must be backed by a real Workflow Service run.
func TestInfraRestart_ChildResultBackedByDurableRun(t *testing.T) {
	src := reconcileActionsSource(t)
	if !strings.Contains(src, "RunRestartInfraUnitWorkflow(") {
		t.Error("node.restart_infra_unit must dispatch through RunRestartInfraUnitWorkflow " +
			"so childResults is keyed by a real, queryable Workflow Service RunId")
	}
}

// The declarative definition must exist — the workflow name must resolve.
func TestInfraRestart_WorkflowDefinitionExists(t *testing.T) {
	p := "../../workflow/definitions/node.restart_infra_unit.yaml"
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("node.restart_infra_unit has no declarative definition at %s: %v", p, err)
	}
	s := string(b)
	for _, needle := range []string{
		"name: node.restart_infra_unit",
		"node.restart_package_service",
		"node.verify_package_runtime",
		"receipt_key: restart_infra_unit",
	} {
		if !strings.Contains(s, needle) {
			t.Errorf("workflow definition missing %q", needle)
		}
	}
}
