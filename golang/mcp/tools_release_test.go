package main

import (
	"errors"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// validEvidence returns evidence that maps to a fully-PROVEN report, so each
// test can mutate one source and assert the resulting Inputs / verdict.
func validEvidence() *releaseBoundaryEvidence {
	return &releaseBoundaryEvidence{
		desired: &cluster_controllerpb.DesiredService{
			ServiceId:   "globular/echo",
			Version:     "1.0.0",
			Platform:    "linux_amd64",
			BuildNumber: 7,
			BuildId:     "build-B",
		},
		manifest: &repositorypb.ArtifactManifest{
			BuildId:            "build-B",
			PublishState:       repositorypb.PublishState_PUBLISHED,
			EntrypointChecksum: "ec-sha",
		},
		verify: &repositorypb.VerifyArtifactResponse{
			Status: repositorypb.ArtifactVerifyStatus_ARTIFACT_VERIFY_OK,
			Reason: "ok",
		},
		installed: &node_agentpb.InstalledPackage{
			BuildId: "build-B",
			Metadata: map[string]string{
				"entrypoint_checksum": "ec-sha",
				"installed_at":        "1000",
			},
		},
		runtime: &node_agentpb.ServiceRuntimeProof{
			ServiceName:        "echo",
			ServiceId:          "globular/echo",
			SystemdActiveState: "active",
			RunningPid:         42,
			RunningExeSha256:   "ec-sha",
			ProcessStartTime:   timestamppb.New(time.Unix(2000, 0)),
			InstalledPath:      "/usr/lib/globular/bin/echo",
		},
	}
}

func mapValid(mutate func(ev *releaseBoundaryEvidence)) release_boundary.Inputs {
	ev := validEvidence()
	if mutate != nil {
		mutate(ev)
	}
	return mapReleaseBoundaryInputs("globular/echo", "globule-ryzen", ev)
}

func TestMap_HappyPath_Proven(t *testing.T) {
	in := mapValid(nil)
	if got := release_boundary.Evaluate(in).Verdict; got != release_boundary.VerdictProven {
		t.Fatalf("verdict = %q, want PROVEN; inputs=%+v", got, in)
	}
}

// 1 + 4: installed_at parses into InstallCommittedUnix; proto InstalledUnix is
// ignored even when present and contradictory.
func TestMap_InstalledAt_FeedsInstallCommittedUnix_IgnoresProtoInstalledUnix(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		ev.installed.Metadata["installed_at"] = "1234"
		ev.installed.InstalledUnix = 99999 // must be ignored
	})
	if in.Installed.InstallCommittedUnix != 1234 {
		t.Errorf("InstallCommittedUnix = %d, want 1234 (from installed_at)", in.Installed.InstallCommittedUnix)
	}
}

// 2: absent installed_at leaves the timestamp at 0 → A4 INDETERMINATE.
func TestMap_InstalledAtAbsent_A4Indeterminate(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		delete(ev.installed.Metadata, "installed_at")
		ev.installed.InstalledUnix = 500 // proto field present but must NOT be used
	})
	if in.Installed.InstallCommittedUnix != 0 {
		t.Fatalf("InstallCommittedUnix = %d, want 0 (no fallback)", in.Installed.InstallCommittedUnix)
	}
	rep := release_boundary.Evaluate(in)
	if a := findAssertion(t, rep, release_boundary.AssertionRestartAfterInstall); a.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("A4 = %q, want INDETERMINATE", a.Verdict)
	}
}

// 3: malformed installed_at → 0 → A4 INDETERMINATE.
func TestMap_InstalledAtMalformed_A4Indeterminate(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		ev.installed.Metadata["installed_at"] = "not-a-number"
	})
	if in.Installed.InstallCommittedUnix != 0 {
		t.Fatalf("InstallCommittedUnix = %d, want 0 for malformed input", in.Installed.InstallCommittedUnix)
	}
}

// 5: VerifyArtifact RPC error → A0 INDETERMINATE (not FAILED).
func TestMap_VerifyRPCError_A0Indeterminate(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		ev.verify = nil
		ev.verifyErr = errors.New("connection refused")
	})
	if in.Repository != nil {
		t.Fatalf("Repository evidence should be absent on RPC error, got %+v", in.Repository)
	}
	rep := release_boundary.Evaluate(in)
	if a := findAssertion(t, rep, release_boundary.AssertionRepositoryArtifactIntact); a.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("A0 = %q, want INDETERMINATE on RPC error", a.Verdict)
	}
}

// 6: explicit broken verification status → A0 FAILED.
func TestMap_VerifyBrokenStatus_A0Failed(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		ev.verify = &repositorypb.VerifyArtifactResponse{
			Status: repositorypb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_CHECKSUM_MISMATCH,
			Reason: "checksum mismatch",
		}
	})
	if in.Repository == nil || !in.Repository.Present || in.Repository.Verified {
		t.Fatalf("Repository should be present+unverified, got %+v", in.Repository)
	}
	rep := release_boundary.Evaluate(in)
	if a := findAssertion(t, rep, release_boundary.AssertionRepositoryArtifactIntact); a.Verdict != release_boundary.VerdictFailed {
		t.Errorf("A0 = %q, want FAILED on broken status", a.Verdict)
	}
}

// 7: publish state maps to A1 (explicit PUBLISHED string; non-published fails).
func TestMap_PublishState_DrivesA1(t *testing.T) {
	if in := mapValid(nil); in.Manifest.PublishState != "PUBLISHED" {
		t.Errorf("PublishState = %q, want PUBLISHED", in.Manifest.PublishState)
	}
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		ev.manifest.PublishState = repositorypb.PublishState_STAGING
	})
	if in.Manifest.PublishState == "PUBLISHED" {
		t.Fatalf("STAGING must not map to PUBLISHED")
	}
	rep := release_boundary.Evaluate(in)
	if a := findAssertion(t, rep, release_boundary.AssertionDesiredPublished); a.Verdict != release_boundary.VerdictFailed {
		t.Errorf("A1 = %q, want FAILED for non-published", a.Verdict)
	}
}

// 8: wrapper package (installed_path outside managed bin) → NOT_APPLICABLE.
func TestMap_WrapperPackage_NotApplicable(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) {
		ev.runtime.InstalledPath = "/usr/sbin/keepalived"
	})
	if !in.Unhashable {
		t.Fatalf("expected Unhashable=true for upstream installed_path")
	}
	if got := release_boundary.Evaluate(in).Verdict; got != release_boundary.VerdictNotApplicable {
		t.Errorf("verdict = %q, want NOT_APPLICABLE", got)
	}
}

// Desired service absent → DesiredBuildID empty (A1 INDETERMINATE upstream).
func TestMap_DesiredAbsent_NoBuildID(t *testing.T) {
	in := mapValid(func(ev *releaseBoundaryEvidence) { ev.desired = nil })
	if in.DesiredBuildID != "" {
		t.Errorf("DesiredBuildID = %q, want empty when desired absent", in.DesiredBuildID)
	}
}

func TestParseInstalledAtUnix(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want int64
	}{
		{map[string]string{"installed_at": "1000"}, 1000},
		{map[string]string{"installed_at": ""}, 0},
		{map[string]string{"installed_at": "  42 "}, 42},
		{map[string]string{"installed_at": "abc"}, 0},
		{map[string]string{"installed_at": "-5"}, 0},
		{map[string]string{}, 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := parseInstalledAtUnix(c.in); got != c.want {
			t.Errorf("parseInstalledAtUnix(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func findAssertion(t *testing.T, r release_boundary.Report, id release_boundary.AssertionID) release_boundary.AssertionReport {
	t.Helper()
	for _, a := range r.Assertions {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("assertion %s not found", id)
	return release_boundary.AssertionReport{}
}
