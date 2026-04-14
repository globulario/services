package engine

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/workflow/compiler"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

func TestValidatePreflight_AllRegistered(t *testing.T) {
	router := NewRouter()
	router.Register("test-actor", "test.action_one", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})
	router.Register("test-actor", "test.action_two", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})

	cw := &compiler.CompiledWorkflow{
		Name: "test-workflow",
		Steps: map[string]*compiler.CompiledStep{
			"step1": {ID: "step1", Actor: "test-actor", Action: "test.action_one"},
			"step2": {ID: "step2", Actor: "test-actor", Action: "test.action_two"},
		},
	}

	if err := ValidatePreflight(cw, router); err != nil {
		t.Errorf("expected nil, got: %v", err)
	}
}

func TestValidatePreflight_MissingAction(t *testing.T) {
	router := NewRouter()
	router.Register("test-actor", "test.action_one", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})

	cw := &compiler.CompiledWorkflow{
		Name: "test-workflow",
		Steps: map[string]*compiler.CompiledStep{
			"step1": {ID: "step1", Actor: "test-actor", Action: "test.action_one"},
			"step2": {ID: "step2", Actor: "test-actor", Action: "test.missing_action"},
		},
	}

	err := ValidatePreflight(cw, router)
	if err == nil {
		t.Fatal("expected error for missing action, got nil")
	}
	pErr, ok := err.(*PreflightError)
	if !ok {
		t.Fatalf("expected *PreflightError, got %T", err)
	}
	if len(pErr.Missing) != 1 {
		t.Fatalf("expected 1 missing, got %d", len(pErr.Missing))
	}
	if pErr.Missing[0].Action != "test.missing_action" {
		t.Errorf("expected missing action=test.missing_action, got %s", pErr.Missing[0].Action)
	}
}

func TestValidatePreflight_FallbackAccepted(t *testing.T) {
	router := NewRouter()
	// Only a fallback for the actor — no specific action registered.
	// This is the remote dispatch pattern: the fallback forwards to the remote actor.
	router.RegisterFallback(v1alpha1.ActorType("remote-actor"), func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})

	cw := &compiler.CompiledWorkflow{
		Name: "test-workflow",
		Steps: map[string]*compiler.CompiledStep{
			"step1": {ID: "step1", Actor: "remote-actor", Action: "remote.any_action"},
		},
	}

	// Fallback means the actor can handle any action (validation deferred to runtime).
	if err := ValidatePreflight(cw, router); err != nil {
		t.Errorf("expected nil (fallback covers actor), got: %v", err)
	}
}

func TestValidatePreflight_HookMissing(t *testing.T) {
	router := NewRouter()
	router.Register("test-actor", "test.action", func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		return &ActionResult{OK: true}, nil
	})

	cw := &compiler.CompiledWorkflow{
		Name: "test-workflow",
		Steps: map[string]*compiler.CompiledStep{
			"step1": {ID: "step1", Actor: "test-actor", Action: "test.action"},
		},
		OnFailure: &compiler.CompiledHook{
			Actor:  "test-actor",
			Action: "test.missing_hook",
		},
	}

	err := ValidatePreflight(cw, router)
	if err == nil {
		t.Fatal("expected error for missing hook action")
	}
	pErr := err.(*PreflightError)
	if pErr.Missing[0].StepID != "onFailure" {
		t.Errorf("expected step=onFailure, got %s", pErr.Missing[0].StepID)
	}
}
