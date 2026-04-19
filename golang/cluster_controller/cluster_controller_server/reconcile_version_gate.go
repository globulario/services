package main

import (
	"log"

	"github.com/globulario/services/golang/versionutil"
)

// minSafeReconcileVersion is the minimum controller version that is allowed
// to mutate desired state and run reconciliation. Controllers below this
// version are structurally unsafe — they may contain auto-import paths that
// create phantom desired entries from runtime observations.
//
// This was introduced after a state poisoning incident where old controllers
// (0.0.8 and below) auto-imported fallback 0.1.0 observations from nodes
// into desired state, creating 22 phantom services.
//
// Bump this version when future reconciliation invariants are added that
// older controllers would violate.
const minSafeReconcileVersion = "0.0.10"

// isReconcileSafe returns true if the given controller version is at or above
// the minimum safe reconcile version. Controllers below this version must not
// mutate desired state or run reconciliation.
func isReconcileSafe(version string) bool {
	if version == "" {
		// Empty version = injected via ldflags at build time; treat as current build = safe.
		return true
	}
	cmp, err := versionutil.Compare(version, minSafeReconcileVersion)
	if err != nil {
		// If version parsing fails, err on the side of caution.
		return false
	}
	return cmp >= 0
}

// reconcileVersionGate checks if this controller is at or above the minimum
// safe reconcile version. If not, it logs a loud warning and returns false.
// Callers should skip desired-state mutation and reconciliation when this
// returns false.
func reconcileVersionGate() bool {
	if isReconcileSafe(Version) {
		return true
	}
	log.Printf("CRITICAL: controller version %s is below minimum safe reconcile version %s — "+
		"desired-state mutation and reconciliation are DISABLED. "+
		"Deploy controller >= %s to restore reconciliation capability.",
		Version, minSafeReconcileVersion, minSafeReconcileVersion)
	return false
}
