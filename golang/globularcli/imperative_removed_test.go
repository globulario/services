package main

import (
	"strings"
	"testing"
)

func TestServicesApply_AlwaysReturnsError(t *testing.T) {
	err := runServicesApply(servicesApplyCmd, nil)
	if err == nil {
		t.Fatal("expected error from services apply")
	}
	msg := err.Error()
	if !strings.Contains(msg, "imperative install has been removed") {
		t.Errorf("expected removal message, got: %s", msg)
	}
}

func TestServicesApply_ContainsMigrationInstructions(t *testing.T) {
	err := runServicesApply(servicesApplyCmd, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()

	requiredInstructions := []string{
		"desired set",
		"apply-desired",
		"seed",
		"repair",
	}
	for _, instr := range requiredInstructions {
		if !strings.Contains(msg, instr) {
			t.Errorf("expected migration instruction %q in error message", instr)
		}
	}
}

func TestServicesApply_DangerousImperativeFlagIgnored(t *testing.T) {
	// Even with --dangerous-imperative set, apply should still return error.
	oldVal := svcDangerousImperative
	svcDangerousImperative = true
	defer func() { svcDangerousImperative = oldVal }()

	err := runServicesApply(servicesApplyCmd, nil)
	if err == nil {
		t.Fatal("expected error even with --dangerous-imperative")
	}
}
