package serviceports

import (
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

// TestExecutableForServiceResolvesManifestEntrypoint verifies that a service
// whose binary does NOT follow the {name}_server convention resolves to its
// real binary via the manifest entrypoint sidecar — not the awareness_graph_server
// guess. Regression for the node-agent reinstall storm where describe/port-config
// looked at /usr/lib/globular/bin/awareness_graph_server (which does not exist)
// instead of bin/awareness-graph.
func TestExecutableForServiceResolvesManifestEntrypoint(t *testing.T) {
	versionutil.SetBaseDir(t.TempDir())
	t.Cleanup(func() { versionutil.SetBaseDir("/var/lib/globular/services") })

	if err := versionutil.WriteEntrypoint("awareness-graph", "bin/awareness-graph"); err != nil {
		t.Fatalf("WriteEntrypoint: %v", err)
	}

	if got := executableForService("awareness-graph"); got != "awareness-graph" {
		t.Fatalf("executableForService(awareness-graph) = %q, want awareness-graph (from manifest entrypoint, not the _server guess)", got)
	}
	// The globular-<name>.service form must resolve identically.
	if got := executableForService("globular-awareness-graph.service"); got != "awareness-graph" {
		t.Fatalf("executableForService(globular-awareness-graph.service) = %q, want awareness-graph", got)
	}
}

// TestExecutableForServiceFallsBackToServerConvention verifies that a service
// with no identity-registry entry and no entrypoint sidecar still resolves via
// the legacy {name}_server convention (pre-Project-T installs whose sidecar was
// never written).
func TestExecutableForServiceFallsBackToServerConvention(t *testing.T) {
	versionutil.SetBaseDir(t.TempDir()) // empty dir: no sidecar
	t.Cleanup(func() { versionutil.SetBaseDir("/var/lib/globular/services") })

	if got := executableForService("made-up-svc"); got != "made_up_svc_server" {
		t.Fatalf("executableForService(made-up-svc) = %q, want made_up_svc_server (fallback)", got)
	}
}
