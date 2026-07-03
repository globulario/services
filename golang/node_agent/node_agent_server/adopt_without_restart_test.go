package main

import (
	"context"
	"errors"
	"testing"
)

// TestRuntimeAlreadyMatchesExpected is the regression guard for
// meta.repository.adopt_correct_runtime_before_disrupting_it. The critical
// safety property: a STALE running binary (live /proc/exe hash != expected)
// MUST NOT be adopted — it must fall through to a restart. Adoption is allowed
// ONLY when the live process is provably running the expected artifact.
func TestRuntimeAlreadyMatchesExpected(t *testing.T) {
	const expected = "abc123def456"
	ctx := context.Background()

	deps := func(pid int, exePath, liveHash string, exeErr, hashErr error) adoptProofDeps {
		return adoptProofDeps{
			FindRunningPID: func(context.Context, string) int { return pid },
			ReadProcExe:    func(int) (string, error) { return exePath, exeErr },
			HashFile:       func(string) (string, error) { return liveHash, hashErr },
		}
	}

	cases := []struct {
		name       string
		expectedSA string
		deps       adoptProofDeps
		wantAdopt  bool
	}{
		{
			name:       "live process runs expected binary — adopt (no restart)",
			expectedSA: "sha256:" + expected,
			deps:       deps(4242, "/usr/lib/globular/bin/envoy", "sha256:"+expected, nil, nil),
			wantAdopt:  true,
		},
		{
			name:       "STALE running binary (hash mismatch) — MUST restart",
			expectedSA: "sha256:" + expected,
			deps:       deps(4242, "/usr/lib/globular/bin/envoy", "sha256:staleoldhash999", nil, nil),
			wantAdopt:  false,
		},
		{
			name:       "service not running (pid 0) — MUST start",
			expectedSA: "sha256:" + expected,
			deps:       deps(0, "", "", nil, nil),
			wantAdopt:  false,
		},
		{
			name:       "no expected_sha256 (unproven identity) — MUST restart",
			expectedSA: "",
			deps:       deps(4242, "/usr/lib/globular/bin/envoy", "sha256:"+expected, nil, nil),
			wantAdopt:  false,
		},
		{
			name:       "cannot read /proc/exe — MUST restart",
			expectedSA: "sha256:" + expected,
			deps:       deps(4242, "", "", errors.New("readlink: permission denied"), nil),
			wantAdopt:  false,
		},
		{
			name:       "cannot hash live binary — MUST restart",
			expectedSA: "sha256:" + expected,
			deps:       deps(4242, "/usr/lib/globular/bin/envoy", "", nil, errors.New("open: no such file")),
			wantAdopt:  false,
		},
		{
			name:       "match ignoring sha256: prefix and case",
			expectedSA: expected, // no prefix
			deps:       deps(4242, "/usr/lib/globular/bin/envoy", "SHA256:"+expected, nil, nil),
			wantAdopt:  true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runtimeAlreadyMatchesExpected(ctx, "envoy", tc.expectedSA, tc.deps)
			if got != tc.wantAdopt {
				t.Fatalf("runtimeAlreadyMatchesExpected = %v, want %v", got, tc.wantAdopt)
			}
		})
	}
}
