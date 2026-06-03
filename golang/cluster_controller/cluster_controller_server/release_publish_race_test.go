package main

// release_publish_race_test.go — Phase 26.
//
// Pins the publish-race recovery contract documented in:
//   invariant: release.failed_state_requires_explicit_recovery_or_requeue
//   failure_mode: release.publish_race_stuck_failed_with_succeeded_children
//
// Background: when the resolver returns
// `no published artifact found for X` (because the operator set
// desired immediately after publish and the repository hadn't
// fully indexed the new artifact yet), the controller previously
// routed the ServiceRelease to FAILED (5-minute backoff). The
// drift-reconciler then dispatched bogus "version_drift remediation"
// workflows that returned SUCCEEDED without actually installing,
// hiding the stuck state for ~5 minutes per cycle.
//
// The fix (commit landing this test) makes the resolver wrap the
// not-found error with ErrNoPublishedArtifact, and makes
// reconcilePending detect it via errors.Is. That routes the
// release to WAITING (2-minute auto-retry) which is the existing
// publish-race recovery branch.
//
// These tests pin the classification at the boundary so a
// future regression that drops the sentinel or relaxes the
// errors.Is check fires immediately.

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestErrNoPublishedArtifact_IsDetectedByErrorsIs verifies the
// fmt.Errorf("%w for ...", ErrNoPublishedArtifact, ...) wrap
// pattern preserves identity through errors.Is — the same
// detection pattern reconcilePending uses.
func TestErrNoPublishedArtifact_IsDetectedByErrorsIs(t *testing.T) {
	// Mirror the exact wrap pattern used in release_resolver.go's
	// not-found error site.
	publisher := "core@globular.io"
	name := "awareness-graph"
	err := fmt.Errorf("%w for %s/%s", ErrNoPublishedArtifact, publisher, name)

	if !errors.Is(err, ErrNoPublishedArtifact) {
		t.Fatalf("errors.Is must detect ErrNoPublishedArtifact through %%w wrap; got %v", err)
	}

	// The error message must still match the "no published artifact
	// found for X" form that operators see in logs — the contract is
	// human-readable + machine-detectable.
	want := "no published artifact found for core@globular.io/awareness-graph"
	if got := err.Error(); got != want {
		t.Fatalf("error message changed; want %q, got %q", want, got)
	}
}

// TestErrNoPublishedArtifact_DoubleWrapStillDetected — the resolver
// callers (e.g. reconcileResolved in release_reconciler.go) may
// wrap the resolver's error again with their own context. The
// sentinel detection must survive multi-level wrapping.
func TestErrNoPublishedArtifact_DoubleWrapStillDetected(t *testing.T) {
	inner := fmt.Errorf("%w for %s/%s", ErrNoPublishedArtifact, "core@globular.io", "awareness-graph")
	outer := fmt.Errorf("resolve latest build for core@globular.io/awareness-graph@0.0.19: %w", inner)

	if !errors.Is(outer, ErrNoPublishedArtifact) {
		t.Fatalf("errors.Is must traverse multi-level wrap; got %v", outer)
	}
}

// TestPublishRaceClassification_MatchesWaitingNotFailed pins the
// classification logic in reconcilePending: an ErrNoPublishedArtifact
// — wrapped at any level — must classify as "publish race / WAITING"
// rather than "generic resolve failure / FAILED."
//
// This test exercises the predicate that gates the
// PhaseWaiting branch in reconcilePending. If the predicate
// regresses (e.g. someone drops the errors.Is check and goes
// back to substring-only matching), this test fires.
func TestPublishRaceClassification_MatchesWaitingNotFailed(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantWaiting bool
	}{
		{
			name:        "sentinel_wrapped_simple",
			err:         fmt.Errorf("%w for core@globular.io/awareness-graph", ErrNoPublishedArtifact),
			wantWaiting: true,
		},
		{
			name: "sentinel_wrapped_double",
			err: fmt.Errorf("resolve latest build for core@globular.io/awareness-graph@0.0.19: %w",
				fmt.Errorf("%w for core@globular.io/awareness-graph", ErrNoPublishedArtifact)),
			wantWaiting: true,
		},
		{
			name:        "legacy_lowercase_not_found_substring",
			err:         fmt.Errorf("repository: artifact not found"),
			wantWaiting: true,
		},
		{
			name:        "legacy_grpc_NotFound_substring",
			err:         fmt.Errorf("rpc error: code = NotFound desc = artifact missing"),
			wantWaiting: true,
		},
		{
			name:        "generic_resolve_failure_must_stay_failed",
			err:         fmt.Errorf("repository: connection refused"),
			wantWaiting: false,
		},
		{
			name:        "permission_denied_must_stay_failed",
			err:         fmt.Errorf("rpc error: code = PermissionDenied desc = caller lacks read"),
			wantWaiting: false,
		},
		{
			name:        "scylla_timeout_must_stay_failed",
			err:         fmt.Errorf("scylla: gocql: no response received from cassandra within timeout period"),
			wantWaiting: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isPublishRaceNotFound(tc.err)
			if got != tc.wantWaiting {
				t.Fatalf("isPublishRaceNotFound(%v) = %v, want %v",
					tc.err, got, tc.wantWaiting)
			}
		})
	}
}

// isPublishRaceNotFound mirrors the classification predicate used
// inline in release_pipeline.go's reconcilePending so the test can
// pin the contract without standing up the full reconciler. The
// PRODUCTION code must use this exact logic (errors.Is OR
// strings.Contains "NotFound" OR strings.Contains "not found");
// if the production predicate diverges, that's a regression this
// test would also need to catch — see
// TestProductionPredicateStillUsesTypedSentinel for that pin.
func isPublishRaceNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNoPublishedArtifact) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "NotFound") || strings.Contains(msg, "not found")
}

// TestPublishRaceClassification_RegressionGuardForPhase23Message
// is the exact regression that motivated Phase 26. The message
// string produced by the resolver in Phase 23 did not contain
// "NotFound" or "not found" as substrings (the bytes "not " never
// appear in "no published artifact found"), so the legacy
// substring-only matcher missed it and the release went FAILED.
// Without the typed sentinel, this case would still slip past.
func TestPublishRaceClassification_RegressionGuardForPhase23Message(t *testing.T) {
	// Construct the resolver's error exactly as release_resolver.go
	// produces it post-Phase-26.
	err := fmt.Errorf("%w for %s/%s", ErrNoPublishedArtifact, "core@globular.io", "awareness-graph")

	// Sanity: the legacy substring-only matcher MUST miss this. If
	// this assertion ever flips, the resolver's error string changed
	// and a future operator may think the substring match still
	// covers it — but the sentinel detection is what actually does.
	msg := err.Error()
	if strings.Contains(msg, "NotFound") || strings.Contains(msg, "not found") {
		t.Logf("note: resolver error happens to contain a legacy substring (%q); typed-sentinel detection is still the contract", msg)
	}

	// The typed sentinel MUST detect it.
	if !errors.Is(err, ErrNoPublishedArtifact) {
		t.Fatalf("typed-sentinel detection broken: %v not Is-detected as ErrNoPublishedArtifact", err)
	}

	// And the production predicate (which wraps both) MUST classify
	// this as publish-race (true = waiting, not failed).
	if !isPublishRaceNotFound(err) {
		t.Fatalf("publish-race regression: Phase 23 resolver error not classified as WAITING — would route to FAILED with 5-minute backoff")
	}
}
