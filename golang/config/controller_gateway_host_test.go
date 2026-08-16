package config

import (
	"os"
	"path/filepath"
	"testing"
)

// node-agent writes its state to the canonical `node-agent/` directory and
// MigrateLegacyStatePathOnce deletes the legacy `nodeagent/` directory after
// relocating it. A reader that knows only one of the two paths is empty on
// half the cluster: the legacy path is gone on migrated nodes, and the
// canonical path does not exist yet on a node whose join script has written
// state but whose agent has not started.
//
// controllerGatewayHost feeds the fallback used when the local gateway is
// unreachable and when routing a CSR to the CA-holding node, so returning ""
// there fails silently in exactly the degraded case the fallback exists for.

func TestNodeAgentStatePathsPrefersCanonicalOverLegacy(t *testing.T) {
	if len(nodeAgentStatePaths) < 2 {
		t.Fatalf("expected both canonical and legacy state paths, got %v", nodeAgentStatePaths)
	}
	if got := filepath.Base(filepath.Dir(nodeAgentStatePaths[0])); got != "node-agent" {
		t.Errorf("first path should be the canonical node-agent/ dir, got %q", nodeAgentStatePaths[0])
	}
	if got := filepath.Base(filepath.Dir(nodeAgentStatePaths[1])); got != "nodeagent" {
		t.Errorf("second path should be the legacy nodeagent/ dir, got %q", nodeAgentStatePaths[1])
	}
}

// The resolution logic itself, exercised over a temp root so it does not depend
// on the host's /var/lib/globular.
func TestControllerEndpointResolvesFromEitherStatePath(t *testing.T) {
	const want = "10.0.0.63"
	body := `{"controller_endpoint":"` + want + `:12000"}`

	for _, tc := range []struct {
		name string
		dir  string
	}{
		{"canonical only (migrated node)", "node-agent"},
		{"legacy only (join script wrote it, agent not started)", "nodeagent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, tc.dir)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			got := readControllerHostFrom([]string{
				filepath.Join(root, "node-agent", "state.json"),
				filepath.Join(root, "nodeagent", "state.json"),
			})
			if got != want {
				t.Fatalf("controller host = %q, want %q — this path is the fallback used when "+
					"the local gateway is unreachable, so an empty result fails silently", got, want)
			}
		})
	}
}

func TestControllerEndpointEmptyWhenNoStateFileExists(t *testing.T) {
	root := t.TempDir()
	got := readControllerHostFrom([]string{filepath.Join(root, "node-agent", "state.json")})
	if got != "" {
		t.Fatalf("expected empty host with no state file, got %q", got)
	}
}
