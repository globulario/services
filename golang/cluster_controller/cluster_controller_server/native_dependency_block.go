package main

import "strings"

const (
	blockedReasonNativeDependencyMissing = "blocked_native_dependency_missing"
	blockedReasonMissingPrerequisite     = "blocked_missing_prerequisite"
	blockedReasonPolicyBlocked           = "blocked_policy"
	blockedReasonOperatorApproval        = "blocked_operator_approval"

	annotationUnblockResume            = "globular.io/reconcile-resume"
	annotationUnblockDependencyPresent = "globular.io/dependency-present"
)

var nativeDependencyProviders = map[string]string{
	"libodbc.so.2": "debian:unixodbc",
}

type deterministicBlock struct {
	BlockedReason   string
	FailureClass    string
	ReasonCode      string
	UnblockSignals  []string
	MissingLibrary string
	Provider       string
	ManualAction   string
}

func classifyDeterministicBlock(reason string) (deterministicBlock, bool) {
	lower := strings.ToLower(reason)
	if strings.Contains(reason, "NATIVE_LIBRARY_DEPENDENCY_MISSING") {
		block := deterministicBlock{
			BlockedReason:  blockedReasonNativeDependencyMissing,
			FailureClass:   "OPERATOR_ACTION_REQUIRED",
			ReasonCode:     "NATIVE_DEPENDENCY_MISSING",
			UnblockSignals: []string{"native_dependency_present", "operator_resume", "policy_changed", "generation_changed"},
		}
		for lib, provider := range nativeDependencyProviders {
			if strings.Contains(reason, lib) {
				block.MissingLibrary = lib
				block.Provider = provider
				if provider == "debian:unixodbc" {
					block.ManualAction = "sudo apt-get install -y unixodbc"
				}
				break
			}
		}
		return block, true
	}
	switch {
	case strings.Contains(lower, "manual approval required"),
		strings.Contains(lower, "requires manual approval"),
		strings.Contains(lower, "requires operator approval"):
		return deterministicBlock{
			BlockedReason:  blockedReasonOperatorApproval,
			FailureClass:   "OPERATOR_ACTION_REQUIRED",
			ReasonCode:     "MANUAL_APPROVAL_REQUIRED",
			UnblockSignals: []string{"operator_resume", "generation_changed"},
		}, true
	case strings.Contains(lower, "missing secret"),
		strings.Contains(lower, "secret not found"),
		strings.Contains(lower, "permission denied"),
		strings.Contains(lower, "unsupported platform"),
		strings.Contains(lower, "checksum mismatch"),
		strings.Contains(lower, "invalid certificate"),
		strings.Contains(lower, "invalid config"),
		strings.Contains(lower, "invalid manifest"),
		strings.Contains(lower, "disk full"),
		strings.Contains(lower, "rf policy violation"):
		return deterministicBlock{
			BlockedReason:  blockedReasonMissingPrerequisite,
			FailureClass:   "DETERMINISTIC_BLOCKED",
			ReasonCode:     "MISSING_PREREQUISITE",
			UnblockSignals: []string{"operator_resume", "state_changed", "generation_changed", "policy_changed"},
		}, true
	}
	return deterministicBlock{}, false
}

func isDeterministicBlockedReason(statusBlockedReason string) bool {
	switch statusBlockedReason {
	case blockedReasonNativeDependencyMissing, blockedReasonMissingPrerequisite, blockedReasonPolicyBlocked, blockedReasonOperatorApproval:
		return true
	default:
		return false
	}
}

func hasTruthyAnnotation(annotations map[string]string, key string) bool {
	if annotations == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(annotations[key]))
	return v == "1" || v == "true" || v == "yes" || v == "resume" || v == "present"
}

func unblockSignalForDeterministicBlock(blockedReason string, annotations map[string]string, config map[string]string, generationAdvanced bool) (string, bool) {
	if generationAdvanced {
		return "generation_changed", true
	}
	if hasTruthyAnnotation(annotations, annotationUnblockResume) {
		return "operator_resume", true
	}
	if hasTruthyAnnotation(annotations, annotationUnblockDependencyPresent) {
		return "dependency_present", true
	}
	if blockedReason == blockedReasonNativeDependencyMissing {
		policy := strings.ToLower(strings.TrimSpace(config["native_dependency_policy"]))
		if policy == "auto_install" {
			return "policy_changed_auto_install", true
		}
	}
	return "", false
}
