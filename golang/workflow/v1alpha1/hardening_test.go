package v1alpha1

import (
	"testing"
)

const hardenedYAML = `
apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: test.hardened
spec:
  strategy:
    mode: single
  steps:
    - id: safe_step
      actor: node-agent
      action: node.verify_services_active
      execution:
        idempotency: safe_retry
        resume_policy: retry

    - id: install_step
      actor: node-agent
      action: node.install_package
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_pkg
        receipt_required: false
      verification:
        actor: node-agent
        action: node.verify_package_installed
        with:
          package_name: test-pkg
        success:
          expr: result.installed == true

    - id: dangerous_step
      actor: cluster-controller
      action: controller.dangerous_action
      execution:
        idempotency: manual_approval
        resume_policy: pause_for_approval
      compensation:
        enabled: true
        actor: cluster-controller
        action: controller.rollback_action
        with:
          reason: compensation
`

func TestHardeningFieldsParse(t *testing.T) {
	loader := NewLoader()
	def, err := loader.LoadBytes([]byte(hardenedYAML))
	if err != nil {
		t.Fatalf("parse hardened YAML: %v", err)
	}

	if len(def.Spec.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(def.Spec.Steps))
	}

	// Step 1: safe_retry
	s1 := def.Spec.Steps[0]
	if s1.Execution == nil {
		t.Fatal("step safe_step: execution should not be nil")
	}
	if s1.Execution.Idempotency != "safe_retry" {
		t.Errorf("idempotency = %q, want safe_retry", s1.Execution.Idempotency)
	}
	if s1.Execution.ResumePolicy != "retry" {
		t.Errorf("resume_policy = %q, want retry", s1.Execution.ResumePolicy)
	}
	if s1.Verification != nil {
		t.Error("safe_step should have no verification")
	}

	// Step 2: verify_then_continue with verification
	s2 := def.Spec.Steps[1]
	if s2.Execution == nil {
		t.Fatal("step install_step: execution should not be nil")
	}
	if s2.Execution.Idempotency != "verify_then_continue" {
		t.Errorf("idempotency = %q, want verify_then_continue", s2.Execution.Idempotency)
	}
	if s2.Execution.ReceiptKey != "install_pkg" {
		t.Errorf("receipt_key = %q, want install_pkg", s2.Execution.ReceiptKey)
	}
	if s2.Verification == nil {
		t.Fatal("step install_step: verification should not be nil")
	}
	if s2.Verification.Action != "node.verify_package_installed" {
		t.Errorf("verification.action = %q, want node.verify_package_installed", s2.Verification.Action)
	}
	if s2.Verification.Success.Expr != "result.installed == true" {
		t.Errorf("verification.success.expr = %q, want result.installed == true", s2.Verification.Success.Expr)
	}

	// Step 3: manual_approval with compensation
	s3 := def.Spec.Steps[2]
	if s3.Execution == nil {
		t.Fatal("step dangerous_step: execution should not be nil")
	}
	if s3.Execution.Idempotency != "manual_approval" {
		t.Errorf("idempotency = %q, want manual_approval", s3.Execution.Idempotency)
	}
	if s3.Compensation == nil {
		t.Fatal("step dangerous_step: compensation should not be nil")
	}
	if !s3.Compensation.Enabled {
		t.Error("compensation.enabled should be true")
	}
	if s3.Compensation.Action != "controller.rollback_action" {
		t.Errorf("compensation.action = %q, want controller.rollback_action", s3.Compensation.Action)
	}
}

func TestLegacyYAMLStillParsesWithoutHardeningFields(t *testing.T) {
	legacyYAML := `
apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: test.legacy
spec:
  strategy:
    mode: single
  steps:
    - id: plain_step
      actor: node-agent
      action: node.verify_services_active
`
	loader := NewLoader()
	def, err := loader.LoadBytes([]byte(legacyYAML))
	if err != nil {
		t.Fatalf("parse legacy YAML: %v", err)
	}

	s := def.Spec.Steps[0]
	if s.Execution != nil {
		t.Error("legacy step should have nil execution")
	}
	if s.Verification != nil {
		t.Error("legacy step should have nil verification")
	}
	if s.Compensation != nil {
		t.Error("legacy step should have nil compensation")
	}
}
