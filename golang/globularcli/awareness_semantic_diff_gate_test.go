package main

// awareness_semantic_diff_gate_test.go — acceptance tests for the P1-6
// authority gate logic. The gate returns a non-empty failure reason
// when an authority-crossing diff has a trust verdict below 'trusted',
// and "" in every other case.

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/assurance"
	"github.com/globulario/services/golang/awareness/semanticdiff"
)

// reportWithGate builds a SemanticDiffReport with the gate-relevant
// fields populated. The helper keeps each test compact while still
// exercising the exact field shape the CLI reads.
func reportWithGate(requiresReview bool, fromLayer, toLayer string, verdict assurance.TrustVerdict) *semanticdiff.SemanticDiffReport {
	r := &semanticdiff.SemanticDiffReport{
		AuthorityChange: &semanticdiff.AuthorityChange{
			RequiresReview: requiresReview,
			FromLayer:      fromLayer,
			ToLayer:        toLayer,
		},
	}
	if verdict != "" {
		r.Trust = &assurance.TrustEnvelope{Verdict: verdict}
	}
	return r
}

// TestAuthorityGate_NilReportClears pins the no-data path: a nil report
// must not trip the gate (callers may invoke with no inputs).
func TestAuthorityGate_NilReportClears(t *testing.T) {
	if reason := authorityGateFailure(nil); reason != "" {
		t.Errorf("expected empty reason for nil report; got %q", reason)
	}
}

// TestAuthorityGate_NoAuthorityChangeClears pins that a diff WITHOUT
// any layer crossing always clears the gate, regardless of trust verdict.
func TestAuthorityGate_NoAuthorityChangeClears(t *testing.T) {
	r := &semanticdiff.SemanticDiffReport{
		Trust: &assurance.TrustEnvelope{Verdict: assurance.TrustUnknown},
	}
	if reason := authorityGateFailure(r); reason != "" {
		t.Errorf("no-authority-change diff must clear the gate; got %q", reason)
	}
}

// TestAuthorityGate_RequiresReviewFalseClears pins that an authority
// crossing WITHOUT RequiresReview=true clears the gate (the semantic
// diff already decided the crossing is allowed).
func TestAuthorityGate_RequiresReviewFalseClears(t *testing.T) {
	r := reportWithGate(false, "Repository", "Desired", assurance.TrustStale)
	if reason := authorityGateFailure(r); reason != "" {
		t.Errorf("RequiresReview=false must clear the gate; got %q", reason)
	}
}

// TestAuthorityGate_TrustedVerdictClears pins the happy path: a
// review-required authority crossing with verdict=trusted is allowed
// to merge — awareness has strong evidence the crossing is safe.
func TestAuthorityGate_TrustedVerdictClears(t *testing.T) {
	r := reportWithGate(true, "Desired", "Installed", assurance.TrustTrusted)
	if reason := authorityGateFailure(r); reason != "" {
		t.Errorf("trusted verdict must clear the gate; got %q", reason)
	}
}

// TestAuthorityGate_NonTrustedVerdictsAllTrip pins the load-bearing
// rule: every trust verdict BELOW 'trusted' must trip the gate when
// an authority crossing requires review. This is the rule that
// prevents Repository→Desired moves from landing without strong
// coverage.
func TestAuthorityGate_NonTrustedVerdictsAllTrip(t *testing.T) {
	cases := []assurance.TrustVerdict{
		assurance.TrustUsable,
		assurance.TrustLimited,
		assurance.TrustStale,
		assurance.TrustUnknown,
		assurance.TrustUnsafe,
	}
	for _, v := range cases {
		t.Run(string(v), func(t *testing.T) {
			r := reportWithGate(true, "Repository", "Desired", v)
			reason := authorityGateFailure(r)
			if reason == "" {
				t.Errorf("verdict=%q must trip the gate", v)
			}
			// The reason MUST name both layers and the verdict so the
			// agent reading the CI log sees why.
			for _, want := range []string{"Repository", "Desired", string(v)} {
				if !strings.Contains(reason, want) {
					t.Errorf("reason missing %q: %s", want, reason)
				}
			}
		})
	}
}

// TestAuthorityGate_EmptyTrustVerdictTrips pins the failure-safe
// default: a report with AuthorityChange but no Trust field at all
// (e.g. the assurance layer didn't run) must trip the gate. Absence
// is never trusted.
func TestAuthorityGate_EmptyTrustVerdictTrips(t *testing.T) {
	r := reportWithGate(true, "Installed", "Runtime", "")
	if reason := authorityGateFailure(r); reason == "" {
		t.Error("empty trust verdict must trip the gate (absence is never trusted)")
	}
}
