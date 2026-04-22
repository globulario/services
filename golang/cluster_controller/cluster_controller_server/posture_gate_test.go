package main

// posture_gate_test.go: focused unit tests for Gate 1 posture enforcement.
//
// Tests are intentionally self-contained: they call the pure helper functions
// (mapWorkflowToClass, postureGateCheck) directly without needing a live server,
// etcd, or workflow client.  This keeps Gate 1 logic independently verifiable.

import (
	"strings"
	"testing"
)

// ── mapWorkflowToClass ────────────────────────────────────────────────────────

func TestMapWorkflowToClass_KnownWorkflows(t *testing.T) {
	cases := []struct {
		workflow string
		want     WorkloadClass
	}{
		// LIVENESS — always allowed
		{"cluster.invariant.enforcement", WorkClassLiveness},

		// TOPOLOGY — always allowed (node.join deferred at RECOVERY_ONLY in Gate 6)
		{"node.bootstrap", WorkClassTopology},
		{"node.join", WorkClassTopology},
		{"node.remove", WorkClassTopology},

		// REPAIR — always allowed
		{"node.recover.full_reseed", WorkClassRepairReseed},
		{"node.repair", WorkClassRepairTargeted},

		// CONVERGENCE — always allowed
		{"cluster.reconcile", WorkClassConvergence},

		// ROLLOUT — suppressed in RECOVERY_ONLY (Gate 1)
		{"release.apply.package", WorkClassRollout},
		{"release.remove.package", WorkClassRollout},

		// BACKGROUND — always allowed in Gate 1 (Gate 2 scope)
		{"repository.sync.upstream", WorkClassBackground},
	}

	for _, tc := range cases {
		t.Run(tc.workflow, func(t *testing.T) {
			got := mapWorkflowToClass(tc.workflow)
			if got != tc.want {
				t.Errorf("mapWorkflowToClass(%q) = %q, want %q", tc.workflow, got, tc.want)
			}
		})
	}
}

func TestMapWorkflowToClass_UnknownWorkflow_DefaultsToBackground(t *testing.T) {
	got := mapWorkflowToClass("some.future.workflow")
	if got != WorkClassBackground {
		t.Errorf("unknown workflow: got class %q, want %q", got, WorkClassBackground)
	}
}

// ── postureGateCheck ──────────────────────────────────────────────────────────

// ROLLOUT allowed in NORMAL — Gate 1 must not interfere with healthy operation.
func TestPostureGateCheck_Rollout_Normal_Allowed(t *testing.T) {
	err := postureGateCheck(PostureNormal, "release.apply.package")
	if err != nil {
		t.Errorf("ROLLOUT in NORMAL should be allowed, got error: %v", err)
	}
}

// ROLLOUT allowed in DEGRADED — Gate 1 only fires at RECOVERY_ONLY.
func TestPostureGateCheck_Rollout_Degraded_Allowed(t *testing.T) {
	err := postureGateCheck(PostureDegraded, "release.apply.package")
	if err != nil {
		t.Errorf("ROLLOUT in DEGRADED should be allowed (Gate 1 scope), got error: %v", err)
	}
}

// ROLLOUT suppressed in RECOVERY_ONLY — the core Gate 1 rule.
func TestPostureGateCheck_Rollout_RecoveryOnly_Suppressed(t *testing.T) {
	for _, wf := range []string{"release.apply.package", "release.remove.package"} {
		t.Run(wf, func(t *testing.T) {
			err := postureGateCheck(PostureRecoveryOnly, wf)
			if err == nil {
				t.Fatalf("ROLLOUT %q in RECOVERY_ONLY should be suppressed", wf)
			}
			// Error must contain the sentinel string so the release pipeline
			// keeps the release RESOLVED (retryable) rather than FAILED.
			if !strings.Contains(err.Error(), "posture gate") {
				t.Errorf("error must contain 'posture gate' for transient classification, got: %v", err)
			}
			if !strings.Contains(err.Error(), "RECOVERY_ONLY") {
				t.Errorf("error should name the posture state, got: %v", err)
			}
			if !strings.Contains(err.Error(), wf) {
				t.Errorf("error should name the suppressed workflow, got: %v", err)
			}
		})
	}
}

// Non-ROLLOUT classes must pass through at RECOVERY_ONLY — repair, liveness,
// and convergence must never be blocked by the Gate 1 rule.
func TestPostureGateCheck_NonRollout_RecoveryOnly_Allowed(t *testing.T) {
	allowed := []string{
		"cluster.invariant.enforcement", // LIVENESS
		"cluster.reconcile",             // CONVERGENCE
		"node.recover.full_reseed",      // REPAIR_RESEED
		"node.repair",                   // REPAIR_TARGETED
		"node.bootstrap",                // TOPOLOGY
		"repository.sync.upstream",      // BACKGROUND (Gate 2 scope)
		"some.unknown.workflow",         // defaults to BACKGROUND
	}
	for _, wf := range allowed {
		t.Run(wf, func(t *testing.T) {
			err := postureGateCheck(PostureRecoveryOnly, wf)
			if err != nil {
				t.Errorf("non-ROLLOUT %q must pass through RECOVERY_ONLY, got: %v", wf, err)
			}
		})
	}
}

// ── transient error classification ───────────────────────────────────────────

// Suppressed rollout must be retryable — the error must classify as transient.
// This is the critical safety property: if the posture gate fires incorrectly
// (false positive), the release stays RESOLVED and retries on the next cycle
// rather than permanently transitioning to FAILED.
func TestPostureGateCheck_SuppressedError_IsTransient(t *testing.T) {
	err := postureGateCheck(PostureRecoveryOnly, "release.apply.package")
	if err == nil {
		t.Fatal("expected suppression error")
	}
	// Mirror the production check from release_pipeline.go.
	if !strings.Contains(err.Error(), "posture gate") {
		t.Errorf("suppression error must contain 'posture gate' for transient classification: %v", err)
	}
}

// ── false-positive safety ─────────────────────────────────────────────────────

// A false-positive posture (RECOVERY_ONLY when cluster is actually healthy)
// must not corrupt release state. The release stays RESOLVED; no permanent
// transition occurs. Verify by calling postureGateCheck and confirming:
//   - error is returned (dispatch blocked)
//   - error is transient (release stays RESOLVED for retry)
//   - calling it again with NORMAL posture returns nil (gate clears)
func TestPostureGateCheck_FalsePositive_StateIsPreserved(t *testing.T) {
	// Simulate false positive: posture incorrectly RECOVERY_ONLY.
	err := postureGateCheck(PostureRecoveryOnly, "release.apply.package")
	if err == nil {
		t.Fatal("expected suppression during false-positive posture")
	}
	if !strings.Contains(err.Error(), "posture gate") {
		t.Errorf("must be transient: %v", err)
	}

	// Posture clears (loop recomputes). Same release retries with NORMAL posture.
	err = postureGateCheck(PostureNormal, "release.apply.package")
	if err != nil {
		t.Errorf("gate must clear when posture returns to NORMAL: %v", err)
	}
}

// ── server posture atomic ──────────────────────────────────────────────────────

// Confirm the server's posture atomic is readable by the gate check pattern
// used in executeWorkflowCentralized. This ensures the integration plumbing
// (atomic.Load → ClusterPosture cast → postureGateCheck) works correctly.
func TestServerPosture_AtomicReadPattern(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", nil, nil)

	// Default: posture starts at PostureNormal (zero value).
	got := ClusterPosture(srv.posture.Load())
	if got != PostureNormal {
		t.Errorf("default posture: got %v, want %v", got, PostureNormal)
	}
	if err := postureGateCheck(got, "release.apply.package"); err != nil {
		t.Errorf("ROLLOUT must be allowed at default NORMAL posture: %v", err)
	}

	// Set RECOVERY_ONLY via the atomic — mirrors what the posture loop does.
	srv.posture.Store(int32(PostureRecoveryOnly))
	got = ClusterPosture(srv.posture.Load())
	if got != PostureRecoveryOnly {
		t.Errorf("posture after Store: got %v, want %v", got, PostureRecoveryOnly)
	}
	if err := postureGateCheck(got, "release.apply.package"); err == nil {
		t.Error("ROLLOUT must be suppressed at RECOVERY_ONLY")
	}

	// Non-ROLLOUT work must still pass through.
	if err := postureGateCheck(got, "cluster.reconcile"); err != nil {
		t.Errorf("CONVERGENCE must pass through RECOVERY_ONLY: %v", err)
	}

	// Restore NORMAL — gate clears.
	srv.posture.Store(int32(PostureNormal))
	got = ClusterPosture(srv.posture.Load())
	if err := postureGateCheck(got, "release.apply.package"); err != nil {
		t.Errorf("ROLLOUT must be allowed after posture restores to NORMAL: %v", err)
	}
}
