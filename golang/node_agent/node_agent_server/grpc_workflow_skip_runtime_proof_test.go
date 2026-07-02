package main

// grpc_workflow_skip_runtime_proof_test.go — Phase 27.
//
// Pins the runtime-proof refresh contract documented in:
//   invariant: node_agent.install_skip_must_refresh_runtime_proof
//   failure_mode: node_agent.install_skip_without_service_config_update
//
// Background: when canSkipInstallPackage returns installSkipAllowed
// (on-disk binary matches desired version AND systemd unit is active),
// the caller previously returned RunWorkflowResponse SUCCEEDED without
// verifying that the running PID's binary actually matches expected.
// If the binary on disk was swapped out-of-band (e.g. `globular pkg
// build` to /usr/lib/globular/bin) while the unit kept running the
// old binary, the skip path would silently report convergence while
// the service was still old AND its /globular/services/<id>/config
// etcd record (only refreshed on service self-registration at
// startup) was stale.
//
// The fix wires verifyRunningBinaryMatchesExpected into the
// installSkipAllowed branch. These tests pin the helper's
// classification directly (no systemd, no /proc) so a regression
// that drops the runtime-proof check fires immediately.

import (
	"context"
	"testing"
)

// withRuntimeBinariesFunc temporarily swaps the runtimeBinariesFunc seam
// for the duration of a test, restoring it on cleanup.
func withRuntimeBinariesFunc(t *testing.T, fn runtimeBinaryProvider) {
	t.Helper()
	prev := runtimeBinariesFunc
	runtimeBinariesFunc = fn
	t.Cleanup(func() { runtimeBinariesFunc = prev })
}

func withSystemdRuntimeBinaryFunc(t *testing.T, fn systemdRuntimeBinaryProvider) {
	t.Helper()
	prev := systemdRuntimeBinaryFunc
	systemdRuntimeBinaryFunc = fn
	t.Cleanup(func() { systemdRuntimeBinaryFunc = prev })
}

func TestVerifyRunningBinary_NoExpectedSha256_MatchesByDefault(t *testing.T) {
	// When the dispatch carries no expected_sha256, there's no manifest
	// opinion to enforce. The on-disk + active-unit proof in
	// canSkipInstallPackage is the strongest evidence; runtime-proof
	// returns matches (no opinion).
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		t.Fatalf("runtimeBinariesFunc must not be called when expectedSha256 is empty")
		return nil
	})
	verdict, reason := verifyRunningBinaryMatchesExpected("awareness-graph", "")
	if verdict != runtimeProofMatches {
		t.Fatalf("verdict: want runtimeProofMatches, got %v (%s)", verdict, reason)
	}
}

func TestVerifyRunningBinary_RunningMatchesExpected_ReturnsMatches(t *testing.T) {
	// Happy path: a running PID exists whose checksum matches expected.
	const expected = "abc123def456"
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{
			"awareness-graph": {
				ServiceName: "awareness-graph",
				BinaryPath:  "/usr/lib/globular/bin/awareness-graph",
				Checksum:    expected,
				PID:         1234,
			},
		}
	})
	verdict, reason := verifyRunningBinaryMatchesExpected("awareness-graph", expected)
	if verdict != runtimeProofMatches {
		t.Fatalf("verdict: want runtimeProofMatches, got %v (%s)", verdict, reason)
	}
}

func TestVerifyRunningBinary_NormalizesHashes(t *testing.T) {
	// "sha256:" prefix + uppercase + whitespace must all normalize to
	// the same canonical form (mirrors normalizedHash semantics).
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{
			"awareness-graph": {
				ServiceName: "awareness-graph",
				Checksum:    "ABC123DEF456",
				PID:         1234,
			},
		}
	})
	verdict, reason := verifyRunningBinaryMatchesExpected("awareness-graph", "sha256:abc123def456")
	if verdict != runtimeProofMatches {
		t.Fatalf("normalization failure: want runtimeProofMatches, got %v (%s)", verdict, reason)
	}
}

func TestVerifyRunningBinary_RunningDiffersFromExpected_ReturnsStale(t *testing.T) {
	// The Phase 23 scenario: binary on disk is new, unit is active, but
	// the running PID is still the OLD binary. canSkipInstallPackage
	// returned installSkipAllowed; verifyRunningBinaryMatchesExpected
	// MUST return runtimeProofStale so the caller restarts the unit.
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{
			"awareness-graph": {
				ServiceName: "awareness-graph",
				Checksum:    "OLDOLDOLDOLD",
				PID:         1234,
			},
		}
	})
	verdict, reason := verifyRunningBinaryMatchesExpected("awareness-graph", "NEWNEWNEWNEW")
	if verdict != runtimeProofStale {
		t.Fatalf("verdict: want runtimeProofStale (Phase 23 regression), got %v (%s)", verdict, reason)
	}
	// The reason string MUST include both checksums so operators
	// (and downstream log analysis) can see what diverged.
	if !containsSubstring(reason, "oldoldoldold") || !containsSubstring(reason, "newnewnewnew") {
		t.Errorf("reason must surface both checksums for diagnostics; got %q", reason)
	}
}

func TestVerifyRunningBinary_NoRunningPID_ReturnsNoRunningPID(t *testing.T) {
	// If no globular-bin process is found for the service name (e.g.
	// mid-restart window, crashed service), the runtime proof is
	// missing. Skip is NOT safe — caller must not claim SUCCEEDED.
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{
			// other services running, but not awareness-graph.
			"workflow": {ServiceName: "workflow", Checksum: "xxx", PID: 5678},
		}
	})
	withSystemdRuntimeBinaryFunc(t, func(context.Context, string) (RunningBinary, bool, string) {
		return RunningBinary{}, false, "not active"
	})
	verdict, _ := verifyRunningBinaryMatchesExpected("awareness-graph", "NEWNEWNEWNEW")
	if verdict != runtimeProofNoRunningPID {
		t.Fatalf("verdict: want runtimeProofNoRunningPID, got %v", verdict)
	}
}

func TestVerifyRunningBinary_ProcessScanMiss_SystemdMainPIDMatches(t *testing.T) {
	const expected = "abc123def456"
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{}
	})
	withSystemdRuntimeBinaryFunc(t, func(_ context.Context, pkgName string) (RunningBinary, bool, string) {
		return RunningBinary{
			ServiceName: pkgName,
			BinaryPath:  "/usr/local/bin/minio",
			Checksum:    expected,
			PID:         2222,
		}, true, ""
	})
	verdict, reason := verifyRunningBinaryMatchesExpected("minio", expected)
	if verdict != runtimeProofMatches {
		t.Fatalf("systemd MainPID fallback should prove non-globular-bin runtime: got %v (%s)", verdict, reason)
	}
}

func TestVerifyRunningBinary_ProcessScanMiss_SystemdMainPIDStale(t *testing.T) {
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{}
	})
	withSystemdRuntimeBinaryFunc(t, func(_ context.Context, pkgName string) (RunningBinary, bool, string) {
		return RunningBinary{
			ServiceName: pkgName,
			BinaryPath:  "/usr/local/bin/minio",
			Checksum:    "oldhash",
			PID:         2222,
		}, true, ""
	})
	verdict, reason := verifyRunningBinaryMatchesExpected("minio", "newhash")
	if verdict != runtimeProofStale {
		t.Fatalf("systemd MainPID fallback must still detect stale runtime: got %v (%s)", verdict, reason)
	}
}

func TestVerifyRunningBinary_RunningPIDWithoutChecksum_ReturnsNoRunningPID(t *testing.T) {
	// Process found but checksum couldn't be read (e.g. /proc/<pid>/exe
	// readlink succeeded but file no longer exists for sha256). Treat
	// as no-runtime-proof — refuse to skip on weak evidence.
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{
			"awareness-graph": {
				ServiceName: "awareness-graph",
				Checksum:    "", // hash read failed
				PID:         1234,
			},
		}
	})
	verdict, _ := verifyRunningBinaryMatchesExpected("awareness-graph", "NEWNEWNEWNEW")
	if verdict != runtimeProofNoRunningPID {
		t.Fatalf("verdict: want runtimeProofNoRunningPID for empty-checksum case, got %v", verdict)
	}
}

func TestVerifyRunningBinary_Idempotent_RepeatedCallsSameVerdict(t *testing.T) {
	// Calling the helper twice in a row with the same input must
	// produce the same verdict. The helper itself has no state
	// (runtimeBinariesFunc is a pure reader); this test pins that
	// property as a contract.
	withRuntimeBinariesFunc(t, func() map[string]RunningBinary {
		return map[string]RunningBinary{
			"awareness-graph": {ServiceName: "awareness-graph", Checksum: "matchhash", PID: 1234},
		}
	})
	v1, _ := verifyRunningBinaryMatchesExpected("awareness-graph", "matchhash")
	v2, _ := verifyRunningBinaryMatchesExpected("awareness-graph", "matchhash")
	if v1 != v2 {
		t.Fatalf("verifyRunningBinaryMatchesExpected not idempotent: first=%v second=%v", v1, v2)
	}
	if v1 != runtimeProofMatches {
		t.Fatalf("expected runtimeProofMatches on both calls, got %v", v1)
	}
}

// containsSubstring is a tiny helper because we want substring presence checks
// without pulling in strings.Contains and having a one-line dependency.
func containsSubstring(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
