package release_boundary

import (
	"strings"
	"testing"
)

// validInputs returns an Inputs whose evidence makes every assertion (A0..A4)
// pass, i.e. the overall verdict is PROVEN. Individual cases clone this and
// mutate exactly one truth source to exercise a single boundary link.
func validInputs() Inputs {
	return Inputs{
		ServiceName:    "repository",
		NodeName:       "globule-ryzen",
		DesiredBuildID: "build-B",
		Manifest: &ManifestEvidence{
			BuildID:            "build-B",
			PublishState:       publishStatePublished,
			EntrypointChecksum: "ec-sha",
			ProvenanceGitSHA:   "abc123",
		},
		Repository: &RepositoryEvidence{Present: true, Verified: true},
		Installed: &InstalledEvidence{
			BuildID:            "build-B",
			EntrypointChecksum: "ec-sha",
			InstalledUnix:      1000,
		},
		Runtime: &RuntimeEvidence{
			Running:          true,
			PID:              42,
			RunningExeSHA256: "ec-sha",
			ProcessStartUnix: 2000,
		},
	}
}

func assertionByID(t *testing.T, r Report, id AssertionID) AssertionReport {
	t.Helper()
	for _, a := range r.Assertions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("assertion %s not present in report", id)
	return AssertionReport{}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name string
		// mutate adjusts a valid baseline to exercise one link.
		mutate func(in *Inputs)
		// wantOverall is the expected aggregate verdict.
		wantOverall Verdict
		// focusID / focusVerdict assert the relevant assertion's verdict.
		focusID      AssertionID
		focusVerdict Verdict
		// reasonContains, when set, must appear in the focus assertion reason
		// (used to confirm the missing link is named).
		reasonContains string
	}{
		{
			name:         "1 all evidence valid -> PROVEN",
			mutate:       func(*Inputs) {},
			wantOverall:  VerdictProven,
			focusID:      AssertionRepositoryArtifactIntact,
			focusVerdict: VerdictProven,
		},
		{
			name:           "2 A0 repository evidence missing -> INDETERMINATE",
			mutate:         func(in *Inputs) { in.Repository = nil },
			wantOverall:    VerdictIndeterminate,
			focusID:        AssertionRepositoryArtifactIntact,
			focusVerdict:   VerdictIndeterminate,
			reasonContains: "repository",
		},
		{
			name:         "3 A0 repository verification failed -> FAILED",
			mutate:       func(in *Inputs) { in.Repository = &RepositoryEvidence{Present: true, Verified: false, Reason: "checksum mismatch"} },
			wantOverall:  VerdictFailed,
			focusID:      AssertionRepositoryArtifactIntact,
			focusVerdict: VerdictFailed,
		},
		{
			name:         "4 A1 desired build differs from manifest -> FAILED",
			mutate:       func(in *Inputs) { in.Manifest.BuildID = "build-OTHER" },
			wantOverall:  VerdictFailed,
			focusID:      AssertionDesiredPublished,
			focusVerdict: VerdictFailed,
		},
		{
			name:         "5 A1 manifest not PUBLISHED -> FAILED",
			mutate:       func(in *Inputs) { in.Manifest.PublishState = "STAGING" },
			wantOverall:  VerdictFailed,
			focusID:      AssertionDesiredPublished,
			focusVerdict: VerdictFailed,
		},
		{
			name:         "6 A2 installed build differs -> FAILED",
			mutate:       func(in *Inputs) { in.Installed.BuildID = "build-OTHER" },
			wantOverall:  VerdictFailed,
			focusID:      AssertionInstalledMatches,
			focusVerdict: VerdictFailed,
		},
		{
			name:         "7 A2 installed checksum differs -> FAILED",
			mutate:       func(in *Inputs) { in.Installed.EntrypointChecksum = "ec-DIFFERENT" },
			wantOverall:  VerdictFailed,
			focusID:      AssertionInstalledMatches,
			focusVerdict: VerdictFailed,
		},
		{
			name:           "8 A2 installed checksum missing -> INDETERMINATE",
			mutate:         func(in *Inputs) { in.Installed.EntrypointChecksum = "" },
			wantOverall:    VerdictIndeterminate,
			focusID:        AssertionInstalledMatches,
			focusVerdict:   VerdictIndeterminate,
			reasonContains: "checksum",
		},
		{
			name:           "9 A3 runtime proof missing -> INDETERMINATE",
			mutate:         func(in *Inputs) { in.Runtime = nil },
			wantOverall:    VerdictIndeterminate,
			focusID:        AssertionRuntimeMatches,
			focusVerdict:   VerdictIndeterminate,
			reasonContains: "runtime proof missing",
		},
		{
			name:         "10 A3 runtime checksum differs -> FAILED",
			mutate:       func(in *Inputs) { in.Runtime.RunningExeSHA256 = "ec-DIFFERENT" },
			wantOverall:  VerdictFailed,
			focusID:      AssertionRuntimeMatches,
			focusVerdict: VerdictFailed,
		},
		{
			name:         "11 A4 process started before install -> FAILED",
			mutate:       func(in *Inputs) { in.Runtime.ProcessStartUnix = 500 },
			wantOverall:  VerdictFailed,
			focusID:      AssertionRestartAfterInstall,
			focusVerdict: VerdictFailed,
		},
		{
			name:           "12 A4 process start equals install -> INDETERMINATE",
			mutate:         func(in *Inputs) { in.Runtime.ProcessStartUnix = in.Installed.InstalledUnix },
			wantOverall:    VerdictIndeterminate,
			focusID:        AssertionRestartAfterInstall,
			focusVerdict:   VerdictIndeterminate,
			reasonContains: "tie",
		},
		{
			name:         "13 wrapper package -> NOT_APPLICABLE",
			mutate:       func(in *Inputs) { in.PackageKind = "wrapper" },
			wantOverall:  VerdictNotApplicable,
			focusID:      AssertionRuntimeMatches,
			focusVerdict: VerdictNotApplicable,
		},
		{
			name: "15 missing lower-layer truth source -> INDETERMINATE names the link",
			mutate: func(in *Inputs) {
				in.Installed = nil
			},
			wantOverall:    VerdictIndeterminate,
			focusID:        AssertionInstalledMatches,
			focusVerdict:   VerdictIndeterminate,
			reasonContains: "installed-package evidence missing",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := validInputs()
			tc.mutate(&in)
			got := Evaluate(in)

			if got.Verdict != tc.wantOverall {
				t.Errorf("overall verdict = %q, want %q (assertions: %+v)", got.Verdict, tc.wantOverall, got.Assertions)
			}
			a := assertionByID(t, got, tc.focusID)
			if a.Verdict != tc.focusVerdict {
				t.Errorf("assertion %s verdict = %q, want %q (reason: %q)", tc.focusID, a.Verdict, tc.focusVerdict, a.Reason)
			}
			if tc.reasonContains != "" && !strings.Contains(a.Reason, tc.reasonContains) {
				t.Errorf("assertion %s reason = %q, want it to contain %q", tc.focusID, a.Reason, tc.reasonContains)
			}

			// Every report must always expose all five assertions (full
			// boundary state), except wrapper which still preserves the shape.
			if len(got.Assertions) != 5 {
				t.Errorf("report exposed %d assertions, want 5", len(got.Assertions))
			}
		})
	}
}

// Case 14 — multiple simultaneous failures: overall is FAILED and every
// assertion report is preserved so the operator sees the full boundary state.
func TestEvaluate_MultipleFailuresPreserveAllReports(t *testing.T) {
	in := validInputs()
	in.Repository = &RepositoryEvidence{Present: true, Verified: false, Reason: "missing blob"} // A0 FAILED
	in.Installed.EntrypointChecksum = "ec-DIFFERENT"                                            // A2 FAILED
	in.Runtime.RunningExeSHA256 = "ec-DIFFERENT"                                                // A3 FAILED

	got := Evaluate(in)

	if got.Verdict != VerdictFailed {
		t.Fatalf("overall verdict = %q, want FAILED", got.Verdict)
	}
	if len(got.Assertions) != 5 {
		t.Fatalf("expected 5 assertion reports, got %d", len(got.Assertions))
	}
	// A0, A2, A3 must each be FAILED; A1, A4 still evaluated (not short-circuited).
	for _, id := range []AssertionID{AssertionRepositoryArtifactIntact, AssertionInstalledMatches, AssertionRuntimeMatches} {
		if a := assertionByID(t, got, id); a.Verdict != VerdictFailed {
			t.Errorf("assertion %s = %q, want FAILED", id, a.Verdict)
		}
	}
	if a := assertionByID(t, got, AssertionDesiredPublished); a.Verdict != VerdictProven {
		t.Errorf("A1 = %q, want PROVEN (must still be evaluated despite earlier failures)", a.Verdict)
	}
}

// Wrapper packages must never be FAILED merely for being unhashable, across
// every wrapper signal the package recognizes — even when other evidence is
// absent (which would otherwise be INDETERMINATE).
func TestEvaluate_WrapperNeverFailed(t *testing.T) {
	signals := []func(*Inputs){
		func(in *Inputs) { in.PackageKind = "wrapper" },
		func(in *Inputs) { in.PackageKind = "bin/noop" },
		func(in *Inputs) { in.Unhashable = true },
	}
	for _, sig := range signals {
		in := Inputs{ServiceName: "keepalived", NodeName: "globule-nuc"} // no evidence at all
		sig(&in)
		got := Evaluate(in)
		if got.Verdict != VerdictNotApplicable {
			t.Errorf("wrapper verdict = %q, want NOT_APPLICABLE", got.Verdict)
		}
		for _, a := range got.Assertions {
			if a.Verdict == VerdictFailed {
				t.Errorf("wrapper assertion %s = FAILED, must never fail for unhashable package", a.ID)
			}
		}
	}
}
