package verifier

// gap_classifier_test.go — unit tests for ClassifyGap.
//
// All tests are pure (no I/O). They pin the priority ordering documented in
// ClassifyGap so regressions in the classification logic surface immediately.

import (
	"testing"
	"time"
)

// TestClassifyGap_PendingSweep_RecentSweep: the verifier swept within the
// cadence window and there is no verdict yet. Expected: pending_sweep.
func TestClassifyGap_PendingSweep_RecentSweep(t *testing.T) {
	got := ClassifyGap(
		0,              // verdictAge — no verdict
		30*time.Second, // sweepAge — recent, within SweepCadence
		false,          // hasVerdict
		"",             // verdictProofStatus
		false,          // sweepRequested
	)
	if got.Class != GapClassPendingSweep {
		t.Errorf("expected pending_sweep, got %q; details: %s", got.Class, got.Details)
	}
	if got.SweepAgeSeconds != (30 * time.Second).Seconds() {
		t.Errorf("SweepAgeSeconds: got %.0f, want %.0f", got.SweepAgeSeconds, (30 * time.Second).Seconds())
	}
}

// TestClassifyGap_MissingVerdict_SweepAgeUnknown: no verdict has been written
// and the sweep age is unknown (zero). Expected: missing_verdict.
func TestClassifyGap_MissingVerdict_SweepAgeUnknown(t *testing.T) {
	got := ClassifyGap(
		0,    // verdictAge
		0,    // sweepAge — unknown
		false, // hasVerdict
		"",   // verdictProofStatus
		false, // sweepRequested
	)
	if got.Class != GapClassMissingVerdict {
		t.Errorf("expected missing_verdict, got %q; details: %s", got.Class, got.Details)
	}
}

// TestClassifyGap_StaleVerdict: a verdict exists with proof=unknown and is
// older than SweepCadence. Expected: stale_verdict.
func TestClassifyGap_StaleVerdict(t *testing.T) {
	got := ClassifyGap(
		2*SweepCadence, // verdictAge — older than cadence
		45*time.Second, // sweepAge — within cadence (so not verifier_stuck)
		true,           // hasVerdict
		ProofUnknown,   // verdictProofStatus
		false,          // sweepRequested
	)
	if got.Class != GapClassStaleVerdict {
		t.Errorf("expected stale_verdict, got %q; details: %s", got.Class, got.Details)
	}
	if got.AgeSeconds != (2 * SweepCadence).Seconds() {
		t.Errorf("AgeSeconds: got %.0f, want %.0f", got.AgeSeconds, (2 * SweepCadence).Seconds())
	}
}

// TestClassifyGap_VerifierStuck: sweepAge > 3×SweepCadence. Expected: verifier_stuck.
func TestClassifyGap_VerifierStuck(t *testing.T) {
	got := ClassifyGap(
		5*time.Minute,    // verdictAge (irrelevant — verifier_stuck takes priority)
		4*SweepCadence+1, // sweepAge — well past 3× cadence
		true,             // hasVerdict
		ProofUnknown,     // verdictProofStatus
		false,            // sweepRequested
	)
	if got.Class != GapClassVerifierStuck {
		t.Errorf("expected verifier_stuck, got %q; details: %s", got.Class, got.Details)
	}
}

// TestClassifyGap_ChecksumMismatch: verdictProofStatus==ProofMismatch must
// always classify as checksum_mismatch regardless of other fields. This class
// must NOT be auto-cleared.
func TestClassifyGap_ChecksumMismatch(t *testing.T) {
	got := ClassifyGap(
		10*time.Second, // verdictAge
		30*time.Second, // sweepAge (within cadence — would be pending_sweep otherwise)
		true,           // hasVerdict
		ProofMismatch,  // verdictProofStatus — highest priority
		false,          // sweepRequested
	)
	if got.Class != GapClassChecksumMismatch {
		t.Errorf("expected checksum_mismatch, got %q; details: %s", got.Class, got.Details)
	}
	// Sanity: SweepRequested is preserved.
	if got.SweepRequested {
		t.Error("expected SweepRequested=false")
	}
}

// TestClassifyGap_PendingSweep_VerdictPresent: verdict exists with
// proof=inventory_claim but is younger than SweepCadence. Expected: pending_sweep.
func TestClassifyGap_PendingSweep_VerdictPresent(t *testing.T) {
	got := ClassifyGap(
		30*time.Second,      // verdictAge — fresh, < SweepCadence
		45*time.Second,      // sweepAge
		true,                // hasVerdict
		ProofInventoryClaim, // verdictProofStatus
		false,               // sweepRequested
	)
	if got.Class != GapClassPendingSweep {
		t.Errorf("expected pending_sweep, got %q; details: %s", got.Class, got.Details)
	}
}

// TestClassifyGap_SweepRequested: SweepRequested=true must be preserved in
// the output regardless of classification class.
func TestClassifyGap_SweepRequested(t *testing.T) {
	got := ClassifyGap(
		0,    // verdictAge
		0,    // sweepAge — unknown → missing_verdict
		false, // hasVerdict
		"",   // verdictProofStatus
		true, // sweepRequested — must propagate
	)
	if !got.SweepRequested {
		t.Error("expected SweepRequested=true in classification output")
	}
	// Class is still missing_verdict (sweep age unknown, no verdict).
	if got.Class != GapClassMissingVerdict {
		t.Errorf("expected missing_verdict, got %q", got.Class)
	}
}
