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
			BuildID:              "build-B",
			EntrypointChecksum:   "ec-sha",
			InstallCommittedUnix: 1000,
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
			name: "3 A0 repository verification failed -> FAILED",
			mutate: func(in *Inputs) {
				in.Repository = &RepositoryEvidence{Present: true, Verified: false, Reason: "checksum mismatch"}
			},
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
			// THE REGRESSION TEST for the 2026-08-14 false positive.
			//
			// This case previously asserted FAILED, on the theory that a process
			// older than the install must be stale. It is the exact shape the
			// node-agent produces on a CORRECT install: the service is started
			// and the receipt is committed afterwards, so process_start lands
			// before install_commit. `event` on globule-nuc and globule-lenovo
			// reported A4 FAILED this way while A0-A3 were PROVEN against a
			// byte-identical checksum.
			//
			// Identity settles it. The install is an os.Rename over a temp file,
			// so a genuinely pre-install process would still hold the superseded
			// inode and could not hash to the installed checksum. It does, so it
			// restarted.
			name:           "11 A4 process start predates install-commit but identity matches -> PROVEN",
			mutate:         func(in *Inputs) { in.Runtime.ProcessStartUnix = 500 },
			wantOverall:    VerdictProven,
			focusID:        AssertionRestartAfterInstall,
			focusVerdict:   VerdictProven,
			reasonContains: "superseded inode",
		},
		{
			// A4 must still be able to say NO, otherwise the fix has merely
			// replaced a false positive with a check that cannot fail. A process
			// executing bytes other than the installed artifact is the real
			// stale-process condition, and it is detected without any clock.
			name:           "11b A4 running executable is not the installed artifact -> FAILED",
			mutate:         func(in *Inputs) { in.Runtime.RunningExeSHA256 = "ec-OLD" },
			wantOverall:    VerdictFailed,
			focusID:        AssertionRestartAfterInstall,
			focusVerdict:   VerdictFailed,
			reasonContains: "not the installed artifact",
		},
		{
			// A4 IS NOT A DUPLICATE OF A3.
			//
			// Here the node installed something other than the published
			// artifact and is faithfully running exactly what it installed. A3
			// (runtime vs PUBLISHED) fails, because the node is not running the
			// release. A4 (runtime vs INSTALLED) passes, because the process did
			// restart onto the bytes this install placed.
			//
			// Two different defects — "wrong artifact installed" and "right
			// artifact installed, process never restarted" — need two different
			// assertions, and this case is what keeps them separable.
			name: "11c A4 runtime matches installed but not published -> A4 PROVEN, A3 FAILED",
			mutate: func(in *Inputs) {
				in.Installed.EntrypointChecksum = "ec-OLD"
				in.Runtime.RunningExeSHA256 = "ec-OLD"
			},
			wantOverall:  VerdictFailed,
			focusID:      AssertionRestartAfterInstall,
			focusVerdict: VerdictProven,
		},
		{
			// Same-second used to need a special case, because a tie under the
			// old ordering rule was unresolvable and produced a permanent
			// UNKNOWN. Identity has no tie.
			name:         "12 A4 process start equals install-commit -> PROVEN",
			mutate:       func(in *Inputs) { in.Runtime.ProcessStartUnix = in.Installed.InstallCommittedUnix },
			wantOverall:  VerdictProven,
			focusID:      AssertionRestartAfterInstall,
			focusVerdict: VerdictProven,
		},
		{
			// Timestamps are corroboration, not the verdict: losing them entirely
			// must not change the answer, because they were never carrying it.
			name:         "16 A4 install-commit timestamp absent -> still PROVEN by identity",
			mutate:       func(in *Inputs) { in.Installed.InstallCommittedUnix = 0 },
			wantOverall:  VerdictProven,
			focusID:      AssertionRestartAfterInstall,
			focusVerdict: VerdictProven,
		},
		{
			// The one case that legitimately cannot be answered. It is
			// INDETERMINATE and never FAILED: with no identity evidence there is
			// nothing to convict on, and timestamp ordering is not evidence for
			// this installer. A0-A3 still carry the boundary verdict.
			name:           "16b A4 runtime checksum unavailable -> INDETERMINATE, not FAILED",
			mutate:         func(in *Inputs) { in.Runtime.RunningExeSHA256 = "" },
			wantOverall:    VerdictIndeterminate,
			focusID:        AssertionRestartAfterInstall,
			focusVerdict:   VerdictIndeterminate,
			reasonContains: "wall-clock ordering is not evidence",
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

// TestA4_LiveFalsePositive_2026_08_14 replays the evidence that exposed the
// defect, copied verbatim from `release_verify_boundary(service_id="event",
// node_id="681710ee-6966-5df3-b155-3cef8b4e1a96")` on the production cluster.
//
// The old predicate returned FAILED "process started before the artifact was
// installed (stale process)" on a one-second gap, while A0-A3 were all PROVEN
// against the same checksum on all three layers — repository, installed, and
// running. Nothing was stale. The install committed its receipt one second
// after starting the process, which is what a correct install does.
//
// The literal numbers are kept rather than reduced to a synthetic 500/1000
// pair, so that a future change to the ordering rule has to confront the real
// margin the installer actually produces.
func TestA4_LiveFalsePositive_2026_08_14(t *testing.T) {
	const ec = "98fa59cf37d82a6345a447b76c32db1fece697f9f2d680ec2de2b90c7462f89b"
	in := Inputs{
		ServiceName:    "event",
		NodeName:       "globule-nuc",
		DesiredBuildID: "019ffcdd-d51e-7649-953b-07b0faa811aa",
		Manifest: &ManifestEvidence{
			BuildID:            "019ffcdd-d51e-7649-953b-07b0faa811aa",
			PublishState:       publishStatePublished,
			EntrypointChecksum: ec,
		},
		Repository: &RepositoryEvidence{Present: true, Verified: true},
		Installed: &InstalledEvidence{
			BuildID:              "019ffcdd-d51e-7649-953b-07b0faa811aa",
			EntrypointChecksum:   ec,
			InstallCommittedUnix: 1786753640,
		},
		Runtime: &RuntimeEvidence{
			Running:          true,
			RunningExeSHA256: ec,
			ProcessStartUnix: 1786753639, // one second BEFORE install-commit
		},
	}

	rep := Evaluate(in)
	a4 := assertionByID(t, rep, AssertionRestartAfterInstall)
	if a4.Verdict != VerdictProven {
		t.Errorf("A4 = %q (%s), want PROVEN — this is the live false positive", a4.Verdict, a4.Reason)
	}
	if rep.Verdict != VerdictProven {
		t.Errorf("overall = %q, want PROVEN — every layer carries the same checksum", rep.Verdict)
	}
	// The identity evidence must be visible in the report. A verdict whose
	// grounds are not printed cannot be audited by the operator who receives it.
	for _, k := range []string{"running_exe_sha256", "installed_entrypoint_checksum"} {
		if a4.Evidence[k] == "" {
			t.Errorf("A4 evidence missing %q; the report must show what the verdict rests on", k)
		}
	}
}
