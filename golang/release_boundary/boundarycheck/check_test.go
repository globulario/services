package boundarycheck

import (
	"context"
	"errors"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// validEvidence maps to a fully-PROVEN report; cases mutate one source.
func validEvidence() *Evidence {
	return &Evidence{
		Desired: &cluster_controllerpb.DesiredService{
			ServiceId: "globular/echo", Version: "1.0.0", Platform: "linux_amd64",
			BuildNumber: 7, BuildId: "build-B",
		},
		Manifest: &repositorypb.ArtifactManifest{
			BuildId: "build-B", PublishState: repositorypb.PublishState_PUBLISHED,
			EntrypointChecksum: "ec-sha",
		},
		Verify: &repositorypb.VerifyArtifactResponse{
			Status: repositorypb.ArtifactVerifyStatus_ARTIFACT_VERIFY_OK, Reason: "ok",
		},
		Installed: &node_agentpb.InstalledPackage{
			BuildId:  "build-B",
			Metadata: map[string]string{"entrypoint_checksum": "ec-sha", "installed_at": "1000"},
		},
		Runtime: &node_agentpb.ServiceRuntimeProof{
			ServiceName: "echo", ServiceId: "globular/echo", SystemdActiveState: "active",
			RunningPid: 42, RunningExeSha256: "ec-sha",
			ProcessStartTime: timestamppb.New(time.Unix(2000, 0)),
			InstalledPath:    "/usr/lib/globular/bin/echo",
		},
	}
}

func mapValid(mutate func(ev *Evidence)) release_boundary.Inputs {
	ev := validEvidence()
	if mutate != nil {
		mutate(ev)
	}
	return MapInputs("globular/echo", "globule-ryzen", ev)
}

func TestMapInputs_HappyPath_Proven(t *testing.T) {
	if got := release_boundary.Evaluate(mapValid(nil)).Verdict; got != release_boundary.VerdictProven {
		t.Fatalf("verdict = %q, want PROVEN", got)
	}
}

// installed_at feeds InstallCommittedUnix; proto InstalledUnix is ignored.
func TestMapInputs_InstalledAt_FeedsInstallCommitted_IgnoresProtoInstalledUnix(t *testing.T) {
	in := mapValid(func(ev *Evidence) {
		ev.Installed.Metadata["installed_at"] = "1234"
		ev.Installed.InstalledUnix = 99999 // must be ignored
	})
	if in.Installed.InstallCommittedUnix != 1234 {
		t.Errorf("InstallCommittedUnix = %d, want 1234", in.Installed.InstallCommittedUnix)
	}
}

func TestMapInputs_InstalledAtAbsent_A4Indeterminate(t *testing.T) {
	in := mapValid(func(ev *Evidence) {
		delete(ev.Installed.Metadata, "installed_at")
		ev.Installed.InstalledUnix = 500 // present but must NOT be used
	})
	if in.Installed.InstallCommittedUnix != 0 {
		t.Fatalf("InstallCommittedUnix = %d, want 0 (no fallback)", in.Installed.InstallCommittedUnix)
	}
	if a := findAssertion(t, release_boundary.Evaluate(in), release_boundary.AssertionRestartAfterInstall); a.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("A4 = %q, want INDETERMINATE", a.Verdict)
	}
}

func TestMapInputs_InstalledAtMalformed_Zero(t *testing.T) {
	in := mapValid(func(ev *Evidence) { ev.Installed.Metadata["installed_at"] = "not-a-number" })
	if in.Installed.InstallCommittedUnix != 0 {
		t.Fatalf("InstallCommittedUnix = %d, want 0 for malformed", in.Installed.InstallCommittedUnix)
	}
}

func TestMapInputs_VerifyRPCError_A0Indeterminate(t *testing.T) {
	in := mapValid(func(ev *Evidence) { ev.Verify = nil; ev.VerifyErr = errors.New("connection refused") })
	if in.Repository != nil {
		t.Fatalf("Repository should be absent on RPC error, got %+v", in.Repository)
	}
	if a := findAssertion(t, release_boundary.Evaluate(in), release_boundary.AssertionRepositoryArtifactIntact); a.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("A0 = %q, want INDETERMINATE", a.Verdict)
	}
}

func TestMapInputs_VerifyBrokenStatus_A0Failed(t *testing.T) {
	in := mapValid(func(ev *Evidence) {
		ev.Verify = &repositorypb.VerifyArtifactResponse{
			Status: repositorypb.ArtifactVerifyStatus_ARTIFACT_VERIFY_BROKEN_CHECKSUM_MISMATCH, Reason: "mismatch",
		}
	})
	if a := findAssertion(t, release_boundary.Evaluate(in), release_boundary.AssertionRepositoryArtifactIntact); a.Verdict != release_boundary.VerdictFailed {
		t.Errorf("A0 = %q, want FAILED", a.Verdict)
	}
}

func TestMapInputs_PublishState_DrivesA1(t *testing.T) {
	if mapValid(nil).Manifest.PublishState != "PUBLISHED" {
		t.Error("PublishState should map to PUBLISHED")
	}
	in := mapValid(func(ev *Evidence) { ev.Manifest.PublishState = repositorypb.PublishState_STAGING })
	if a := findAssertion(t, release_boundary.Evaluate(in), release_boundary.AssertionDesiredPublished); a.Verdict != release_boundary.VerdictFailed {
		t.Errorf("A1 = %q, want FAILED for non-published", a.Verdict)
	}
}

func TestMapInputs_WrapperPackage_NotApplicable(t *testing.T) {
	in := mapValid(func(ev *Evidence) { ev.Runtime.InstalledPath = "/usr/sbin/keepalived" })
	if !in.Unhashable {
		t.Fatal("expected Unhashable=true for upstream installed_path")
	}
	if got := release_boundary.Evaluate(in).Verdict; got != release_boundary.VerdictNotApplicable {
		t.Errorf("verdict = %q, want NOT_APPLICABLE", got)
	}
}

// Regression for the live INC: the manifest stores the entrypoint checksum
// with a "sha256:" prefix while the node-agent records bare lowercase hex. The
// bytes are identical, so A2/A3 must PROVE — a prefix difference is not drift.
func TestMapInputs_DigestPrefixNormalized_A2A3Proven(t *testing.T) {
	const bare = "b4429a27015b20bf573bc8bb8f13f850a68a71af92df77ed725084bf46509937"
	ev := validEvidence()
	ev.Manifest.EntrypointChecksum = "sha256:" + bare // prefixed (repository form)
	ev.Installed.Metadata["entrypoint_checksum"] = bare
	ev.Runtime.RunningExeSha256 = bare

	rep := release_boundary.Evaluate(MapInputs("globular/echo", "globule-ryzen", ev))
	if a := findAssertion(t, rep, release_boundary.AssertionInstalledMatches); a.Verdict != release_boundary.VerdictProven {
		t.Errorf("A2 = %q, want PROVEN (prefix-only difference is not drift): %s", a.Verdict, a.Reason)
	}
	if a := findAssertion(t, rep, release_boundary.AssertionRuntimeMatches); a.Verdict != release_boundary.VerdictProven {
		t.Errorf("A3 = %q, want PROVEN: %s", a.Verdict, a.Reason)
	}
}

func TestNormalizeDigest(t *testing.T) {
	cases := map[string]string{
		"sha256:ABCDEF": "abcdef",
		"  ABCDEF  ":    "abcdef",
		"abcdef":        "abcdef",
		"":              "",
	}
	for in, want := range cases {
		if got := normalizeDigest(in); got != want {
			t.Errorf("normalizeDigest(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseInstalledAtUnix(t *testing.T) {
	cases := []struct {
		in   map[string]string
		want int64
	}{
		{map[string]string{"installed_at": "1000"}, 1000},
		{map[string]string{"installed_at": "  42 "}, 42},
		{map[string]string{"installed_at": ""}, 0},
		{map[string]string{"installed_at": "abc"}, 0},
		{map[string]string{"installed_at": "-5"}, 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := parseInstalledAtUnix(c.in); got != c.want {
			t.Errorf("parseInstalledAtUnix(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// ── Collect orchestration ──────────────────────────────────────────────────

// fetchersFromEvidence builds Fetchers that serve the given evidence, so Collect
// orchestration can be exercised without any gRPC.
func fetchersFromEvidence(ev *Evidence) Fetchers {
	return Fetchers{
		Desired: func(context.Context) ([]*cluster_controllerpb.DesiredService, error) {
			if ev.Desired == nil {
				return nil, nil
			}
			return []*cluster_controllerpb.DesiredService{ev.Desired}, nil
		},
		Resolve: func(context.Context, *repositorypb.ResolveArtifactRequest) (*repositorypb.ArtifactManifest, error) {
			return ev.Manifest, nil
		},
		Verify: func(context.Context, *repositorypb.ArtifactRef, string) (*repositorypb.VerifyArtifactResponse, error) {
			return ev.Verify, ev.VerifyErr
		},
		Installed: func(context.Context, string, string, string) (*node_agentpb.InstalledPackage, error) {
			return ev.Installed, nil
		},
		Runtime: func(context.Context, string, string) ([]*node_agentpb.ServiceRuntimeProof, error) {
			if ev.Runtime == nil {
				return nil, nil
			}
			return []*node_agentpb.ServiceRuntimeProof{ev.Runtime}, nil
		},
	}
}

func TestCollect_HappyPath_Proven(t *testing.T) {
	report, ev := Run(context.Background(), fetchersFromEvidence(validEvidence()), "globular/echo", "globule-ryzen", Options{})
	if report.Verdict != release_boundary.VerdictProven {
		t.Fatalf("verdict = %q, want PROVEN", report.Verdict)
	}
	if len(ev.CollectionErrors) != 0 {
		t.Errorf("unexpected collection errors: %v", ev.CollectionErrors)
	}
}

// A real RPC error is recorded (not absorbed) and degrades the verdict.
func TestCollect_RuntimeRPCError_RecordedAndIndeterminate(t *testing.T) {
	f := fetchersFromEvidence(validEvidence())
	f.Runtime = func(context.Context, string, string) ([]*node_agentpb.ServiceRuntimeProof, error) {
		return nil, errors.New("node agent unreachable")
	}
	report, ev := Run(context.Background(), f, "globular/echo", "globule-ryzen", Options{})
	if report.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("verdict = %q, want INDETERMINATE", report.Verdict)
	}
	if got := ev.CollectionErrors["runtime"]; got == "" || !contains(got, "node agent unreachable") {
		t.Errorf("runtime collection error = %q, want it to name the real error", got)
	}
}

// Missing desired service short-circuits repository calls and names the link.
func TestCollect_DesiredNotFound_NamesLink(t *testing.T) {
	f := fetchersFromEvidence(validEvidence())
	f.Desired = func(context.Context) ([]*cluster_controllerpb.DesiredService, error) {
		return []*cluster_controllerpb.DesiredService{{ServiceId: "globular/other"}}, nil
	}
	_, ev := Run(context.Background(), f, "globular/echo", "globule-ryzen", Options{})
	if got := ev.CollectionErrors["desired"]; !contains(got, "not found") {
		t.Errorf("desired error = %q, want 'not found'", got)
	}
	if ev.Manifest != nil || ev.Verify != nil {
		t.Error("manifest/verify must be skipped when desired service is absent")
	}
}

// PR-17 core: a BARE service_id (no publisher prefix) with a pinned build_id
// resolves to a publisher-qualified ref read from the resolved manifest — no
// publisher guessing — and proves.
func TestCollect_BareServiceID_ResolvesByBuildID(t *testing.T) {
	ev := validEvidence()
	ev.Desired.ServiceId = "echo" // bare, no publisher
	ev.Manifest.Ref = &repositorypb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		Kind: repositorypb.ArtifactKind_SERVICE,
	}
	report, e := Run(context.Background(), fetchersFromEvidence(ev), "echo", "globule-ryzen", Options{})
	if report.Verdict != release_boundary.VerdictProven {
		t.Fatalf("verdict = %q, want PROVEN (bare id resolved via build_id); errs=%v", report.Verdict, e.CollectionErrors)
	}
}

// No build_id pin and no publisher → INDETERMINATE with a named collection
// error; never a guess, never a storage scan.
func TestCollect_NoBuildIDNoPublisher_Indeterminate(t *testing.T) {
	ev := validEvidence()
	ev.Desired.ServiceId = "echo"
	ev.Desired.BuildId = ""
	report, e := Run(context.Background(), fetchersFromEvidence(ev), "echo", "globule-ryzen", Options{})
	if report.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("verdict = %q, want INDETERMINATE", report.Verdict)
	}
	if got := e.CollectionErrors["manifest"]; !contains(got, "cannot resolve artifact") {
		t.Errorf("manifest error = %q, want 'cannot resolve artifact'", got)
	}
}

// Explicit --publisher override resolves a legacy record lacking a build_id pin.
func TestCollect_PublisherOverride_Resolves(t *testing.T) {
	ev := validEvidence()
	ev.Desired.ServiceId = "echo"
	ev.Desired.BuildId = ""
	ev.Manifest.Ref = &repositorypb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		Kind: repositorypb.ArtifactKind_SERVICE,
	}
	_, e := Run(context.Background(), fetchersFromEvidence(ev), "echo", "globule-ryzen", Options{Publisher: "core@globular.io"})
	if got := e.CollectionErrors["manifest"]; got != "" {
		t.Errorf("unexpected manifest error with publisher override: %q", got)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
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
