package boundarycheck

import (
	"context"
	"errors"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/release_boundary"
)

// installedByKind builds Fetchers from validEvidence but serves the installed
// package ONLY under the given kind; every other kind returns a NotFound-style
// error (exactly how node-agent's GetInstalledPackage replies for the wrong
// kind: "package <KIND>/<name> not found"). This pins the kind-aware lookup
// added to Collect — the SERVICE-only lookup falsely reported A2/A4
// INDETERMINATE for INFRASTRUCTURE subjects like xds.
func installedByKind(servedKind string) Fetchers {
	ev := validEvidence()
	f := fetchersFromEvidence(ev)
	f.Installed = func(_ context.Context, _ string, kind, name string) (*node_agentpb.InstalledPackage, error) {
		if kind == servedKind {
			return ev.Installed, nil
		}
		return nil, errors.New("rpc error: code = NotFound desc = package " + kind + "/" + name + " not found")
	}
	return f
}

// SERVICE packages must keep proving exactly as before — the common case must
// not regress when INFRASTRUCTURE support is added.
func TestCollect_ServicePackage_StillProves(t *testing.T) {
	report, ev := Run(context.Background(), installedByKind("SERVICE"), "globular/echo", "globule-ryzen", Options{})
	if got := ev.CollectionErrors["installed"]; got != "" {
		t.Fatalf("unexpected installed collection error for SERVICE: %q", got)
	}
	if report.Verdict != release_boundary.VerdictProven {
		t.Fatalf("verdict = %q, want PROVEN", report.Verdict)
	}
	if a := findAssertion(t, report, release_boundary.AssertionInstalledMatches); a.Verdict != release_boundary.VerdictProven {
		t.Errorf("A2 installed-matches = %q, want PROVEN", a.Verdict)
	}
}

// INFRASTRUCTURE subjects (xds) — installed-state keyed under INFRASTRUCTURE,
// not SERVICE — must now resolve A2 instead of falsely reporting INDETERMINATE.
func TestCollect_InfrastructurePackage_ResolvesInstalled(t *testing.T) {
	f := installedByKind("INFRASTRUCTURE")
	// Model xds: an INFRASTRUCTURE desired subject.
	f.Desired = func(context.Context) ([]*cluster_controllerpb.DesiredService, error) {
		return []*cluster_controllerpb.DesiredService{{
			ServiceId: "xds", Version: "1.2.234", Platform: "linux_amd64",
			BuildNumber: 1, BuildId: "build-B",
		}}, nil
	}
	report, ev := Run(context.Background(), f, "xds", "globule-ryzen", Options{Publisher: "core@globular.io"})

	if got := ev.CollectionErrors["installed"]; got != "" {
		t.Fatalf("installed collection error for INFRASTRUCTURE/xds: %q (the kind-aware lookup should have found it)", got)
	}
	if ev.Installed == nil {
		t.Fatal("ev.Installed is nil; INFRASTRUCTURE lookup did not resolve")
	}
	if a := findAssertion(t, report, release_boundary.AssertionInstalledMatches); a.Verdict != release_boundary.VerdictProven {
		t.Errorf("A2 installed-matches = %q, want PROVEN for INFRASTRUCTURE/xds", a.Verdict)
	}
}

// A package installed under NEITHER kind must still yield an honest
// INDETERMINATE with the real error recorded — never a fabricated green.
func TestCollect_MissingUnderAllKinds_HonestIndeterminate(t *testing.T) {
	f := fetchersFromEvidence(validEvidence())
	f.Installed = func(_ context.Context, _ string, kind, name string) (*node_agentpb.InstalledPackage, error) {
		return nil, errors.New("rpc error: code = NotFound desc = package " + kind + "/" + name + " not found")
	}
	report, ev := Run(context.Background(), f, "globular/echo", "globule-ryzen", Options{})

	if ev.Installed != nil {
		t.Fatal("ev.Installed should be nil when the package is installed under no kind")
	}
	got := ev.CollectionErrors["installed"]
	if got == "" || !contains(got, "not found") {
		t.Errorf("installed collection error = %q, want the real NotFound error recorded", got)
	}
	if a := findAssertion(t, report, release_boundary.AssertionInstalledMatches); a.Verdict != release_boundary.VerdictIndeterminate {
		t.Errorf("A2 installed-matches = %q, want INDETERMINATE (absence, not fabricated green)", a.Verdict)
	}
}
