package main

import "testing"

func TestDetectNativeDependencyBlock(t *testing.T) {
	reason := "step apply_per_node: item 1 failed: NATIVE_LIBRARY_DEPENDENCY_MISSING: sql requires native libraries not installed on this node: [libodbc.so.2]"
	block, ok := classifyDeterministicBlock(reason)
	if !ok {
		t.Fatal("expected native dependency block to be detected")
	}
	if block.MissingLibrary != "libodbc.so.2" {
		t.Fatalf("missing library = %q, want libodbc.so.2", block.MissingLibrary)
	}
	if block.Provider != "debian:unixodbc" {
		t.Fatalf("provider = %q, want debian:unixodbc", block.Provider)
	}
	if block.ManualAction != "sudo apt-get install -y unixodbc" {
		t.Fatalf("manual action = %q, want apt install command", block.ManualAction)
	}
	if block.BlockedReason != blockedReasonNativeDependencyMissing {
		t.Fatalf("blocked reason = %q, want %q", block.BlockedReason, blockedReasonNativeDependencyMissing)
	}
}

func TestDetectNativeDependencyBlock_NoMatch(t *testing.T) {
	if _, ok := classifyDeterministicBlock("connection refused"); ok {
		t.Fatal("unexpected block detection for non-dependency error")
	}
}

func TestClassifyDeterministicBlock_ManualApproval(t *testing.T) {
	block, ok := classifyDeterministicBlock("verification inconclusive: step requires manual approval to resume")
	if !ok {
		t.Fatal("expected manual approval to classify as deterministic block")
	}
	if block.BlockedReason != blockedReasonOperatorApproval {
		t.Fatalf("blocked reason = %q, want %q", block.BlockedReason, blockedReasonOperatorApproval)
	}
}

func TestClassifyDeterministicBlock_MissingSecret(t *testing.T) {
	block, ok := classifyDeterministicBlock("deploy failed: missing secret db_password")
	if !ok {
		t.Fatal("expected missing secret to classify as deterministic block")
	}
	if block.BlockedReason != blockedReasonMissingPrerequisite {
		t.Fatalf("blocked reason = %q, want %q", block.BlockedReason, blockedReasonMissingPrerequisite)
	}
}

func TestIsDeterministicBlockedReason(t *testing.T) {
	if !isDeterministicBlockedReason(blockedReasonNativeDependencyMissing) {
		t.Fatal("expected blocked reason to be recognized")
	}
	if !isDeterministicBlockedReason(blockedReasonMissingPrerequisite) {
		t.Fatal("expected missing-prerequisite blocked reason to be recognized")
	}
	if isDeterministicBlockedReason("workflow_unavailable") {
		t.Fatal("unexpected true for transient blocked reason")
	}
}

func TestUnblockSignalForDeterministicBlock(t *testing.T) {
	if signal, ok := unblockSignalForDeterministicBlock(
		blockedReasonNativeDependencyMissing,
		map[string]string{annotationUnblockResume: "true"},
		nil,
		false,
	); !ok || signal != "operator_resume" {
		t.Fatalf("operator resume signal not detected: ok=%v signal=%q", ok, signal)
	}

	if signal, ok := unblockSignalForDeterministicBlock(
		blockedReasonNativeDependencyMissing,
		map[string]string{annotationUnblockDependencyPresent: "present"},
		nil,
		false,
	); !ok || signal != "dependency_present" {
		t.Fatalf("dependency-present signal not detected: ok=%v signal=%q", ok, signal)
	}

	if signal, ok := unblockSignalForDeterministicBlock(
		blockedReasonNativeDependencyMissing,
		nil,
		map[string]string{"native_dependency_policy": "auto_install"},
		false,
	); !ok || signal != "policy_changed_auto_install" {
		t.Fatalf("policy-change signal not detected: ok=%v signal=%q", ok, signal)
	}

	if signal, ok := unblockSignalForDeterministicBlock(
		blockedReasonMissingPrerequisite,
		nil,
		nil,
		true,
	); !ok || signal != "generation_changed" {
		t.Fatalf("generation signal not detected: ok=%v signal=%q", ok, signal)
	}
}
