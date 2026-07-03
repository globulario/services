package config

// Tier 2B ratchet: control-plane authority RPCs must resolve to DIRECT admitted
// node/control endpoints, never through the managed data-plane mesh (Envoy :443).
//
// Governing contract: meta.control_plane_must_not_depend_on_managed_data_plane_mesh.
// The control plane manages the mesh; if control-plane authority reads/callbacks
// route THROUGH the mesh, a mesh blackout (day-0/day-1, or an envoy adopt/restart)
// severs the very plane that would repair it — a dependency cycle. Envoy is the
// data plane; xDS is its authority (intent:infrastructure.envoy.service_mesh_data_plane_with_xds_authority).
//
// These tests are SOURCE-SCAN ratchets: they fail the build if a future edit
// reintroduces mesh resolution (config.ResolveServiceAddr / ResolveServiceAddrs /
// GetMeshAddress) on a control-plane authority path, or a "direct failed -> try
// :443/gateway" fallback that reintroduces the cycle through the back door.
//
// Runtime behavior can't be unit-tested here without a live cluster + etcd, so the
// ratchet guards the SOURCE: control-plane authority files must use the direct
// resolvers (ResolveControllerDirectAddr / ResolveServiceDirectAddr[s]).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// meshResolverTokens are the mesh-routing resolvers. On a control-plane authority
// path they rewrite host:port -> host:443 (Envoy), which is exactly the dependency
// we forbid. Data-plane transport and explicit mesh-health probes may still use them.
var meshResolverTokens = []string{
	"ResolveServiceAddr(",
	"ResolveServiceAddrs(",
	"GetMeshAddress(",
}

// controlPlaneAuthorityFiles are files whose gRPC address resolution serves the
// CONTROL PLANE (heartbeat, convergence callbacks, doctor authority clients,
// desired-state / catalog authority reads). None of them may resolve via the mesh.
// Paths are relative to the golang/ root (the parent of this config package dir).
var controlPlaneAuthorityFiles = []string{
	// node-agent heartbeat + control channels
	"node_agent/node_agent_server/heartbeat.go",
	"node_agent/node_agent_server/server.go",
	"node_agent/node_agent_server/main.go",
	"node_agent/node_agent_server/event_publisher.go",
	"node_agent/node_agent_server/event_handler.go",
	"node_agent/node_agent_server/installer_api.go",
	// doctor authority clients
	"cluster_doctor/cluster_doctor_server/server.go",
	"cluster_doctor/cluster_doctor_server/main.go",
	// controller convergence callbacks + Layer-1 catalog/desired authority reads
	"cluster_controller/cluster_controller_server/server.go",
	"cluster_controller/cluster_controller_server/desired_state_handlers.go",
	"cluster_controller/cluster_controller_server/component_catalog.go",
}

// dataPlaneMeshAllowlist are files that LEGITIMATELY use a mesh/gateway resolver
// because they operate on the DATA plane, not control-plane authority:
//
//   - service_discovery.go: discoverGatewayAddr() resolves the gateway/mesh EDGE
//     itself — finding the routing edge is data-plane discovery by definition.
//   - internal/actions/artifact.go, verify_integrity.go: node-agent EXECUTOR
//     byte-fetch of an already-decided artifact (bulk transport, load-balanced),
//     with an explicit discoverRepositoryViaGateway() data-plane fallback.
//   - collector/gateway_backend_divergence.go: an explicit mesh-health probe that
//     compares gateway backends — it MUST talk to the mesh to measure it.
//
// This allowlist is documentation-as-test: it names every sanctioned exception so
// a new mesh use can't hide as "probably fine".
var dataPlaneMeshAllowlist = map[string]string{
	"node_agent/node_agent_server/service_discovery.go":                       "resolves the gateway/mesh edge itself (data-plane discovery)",
	"node_agent/node_agent_server/internal/actions/artifact.go":               "executor byte-fetch of a decided artifact (data-plane transport)",
	"node_agent/node_agent_server/internal/actions/verify_integrity.go":       "executor byte-fetch of a decided artifact (data-plane transport)",
	"cluster_doctor/cluster_doctor_server/collector/gateway_backend_divergence.go": "explicit mesh-health probe (must measure the mesh)",
}

// golangRoot returns the golang/ directory (parent of this config package dir).
func golangRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd) // .../golang/config -> .../golang
}

func readSource(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// nonCommentMeshHits returns the mesh-resolver tokens found on non-comment,
// non-string-doc lines of src. Comments (// ...) are skipped so a doc-reference
// like "// via config.ResolveServiceAddr(...)" doesn't trip the ratchet.
func nonCommentMeshHits(src string) []string {
	var hits []string
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
			continue
		}
		// Strip any trailing line comment before scanning.
		if i := strings.Index(line, "//"); i >= 0 {
			line = line[:i]
		}
		for _, tok := range meshResolverTokens {
			if strings.Contains(line, tok) {
				hits = append(hits, strings.TrimSpace(line))
			}
		}
	}
	return hits
}

// TestControlPlaneAuthorityFilesUseDirectAddressing is the core ratchet: no
// control-plane authority file may resolve a gRPC address through the mesh.
func TestControlPlaneAuthorityFilesUseDirectAddressing(t *testing.T) {
	root := golangRoot(t)
	for _, rel := range controlPlaneAuthorityFiles {
		if _, allowlisted := dataPlaneMeshAllowlist[rel]; allowlisted {
			t.Fatalf("%s is both a control-plane authority file and data-plane allowlisted — resolve the contradiction", rel)
		}
		src := readSource(t, root, rel)
		if hits := nonCommentMeshHits(src); len(hits) > 0 {
			t.Errorf("%s resolves a control-plane RPC through the mesh (forbidden by "+
				"meta.control_plane_must_not_depend_on_managed_data_plane_mesh). Use "+
				"ResolveControllerDirectAddr / ResolveServiceDirectAddr[s] instead.\n  offending lines:\n    %s",
				rel, strings.Join(hits, "\n    "))
		}
	}
}

// TestNodeAgentHeartbeatUsesDirectControlAddress — the heartbeat is the node's
// primary control-plane channel; it must reach the controller directly.
func TestNodeAgentHeartbeatUsesDirectControlAddress(t *testing.T) {
	src := readSource(t, golangRoot(t), "node_agent/node_agent_server/heartbeat.go")
	if !strings.Contains(src, "ResolveControllerDirectAddr(") {
		t.Error("heartbeat.go must resolve the controller via ResolveControllerDirectAddr() (direct, not mesh)")
	}
	if hits := nonCommentMeshHits(src); len(hits) > 0 {
		t.Errorf("heartbeat.go must not use a mesh resolver:\n    %s", strings.Join(hits, "\n    "))
	}
}

// TestDoctorAuthorityClientsRejectMeshResolvedAddress — the doctor's authority
// clients (controller, workflow, repository, ai_memory, event) must be direct.
func TestDoctorAuthorityClientsRejectMeshResolvedAddress(t *testing.T) {
	root := golangRoot(t)
	for _, rel := range []string{
		"cluster_doctor/cluster_doctor_server/server.go",
		"cluster_doctor/cluster_doctor_server/main.go",
	} {
		src := readSource(t, root, rel)
		if hits := nonCommentMeshHits(src); len(hits) > 0 {
			t.Errorf("%s (doctor authority client) must not resolve via the mesh:\n    %s",
				rel, strings.Join(hits, "\n    "))
		}
	}
}

// TestControlPlaneClientDoesNotFallbackToGateway443 — a "direct failed -> try
// :443" fallback reintroduces the mesh dependency through the back door. Guard
// the file we deliberately stripped that fallback from (installer_api.go) so it
// can't creep back.
func TestControlPlaneClientDoesNotFallbackToGateway443(t *testing.T) {
	src := readSource(t, golangRoot(t), "node_agent/node_agent_server/installer_api.go")
	for _, line := range strings.Split(src, "\n") {
		code := line
		if i := strings.Index(code, "//"); i >= 0 {
			code = code[:i]
		}
		if strings.Contains(code, `":443"`) {
			t.Errorf("installer_api.go reintroduced a :443 mesh fallback (forbidden):\n    %s", strings.TrimSpace(line))
		}
	}
}

// TestMeshHealthProbeIsExplicitlyMarkedDataPlaneProbe — the sanctioned mesh uses
// live only in the data-plane allowlist, and the doctor's GetMeshAddress use is
// confined to the explicit gateway-backend-divergence probe (which must measure
// the mesh). If a mesh resolver appears in a doctor file outside that probe, fail.
func TestMeshHealthProbeIsExplicitlyMarkedDataPlaneProbe(t *testing.T) {
	root := golangRoot(t)
	// The divergence collector is the one doctor file allowed to touch the mesh.
	probe := "cluster_doctor/cluster_doctor_server/collector/gateway_backend_divergence.go"
	if _, ok := dataPlaneMeshAllowlist[probe]; !ok {
		t.Fatalf("the mesh-health probe %s must be in the data-plane allowlist", probe)
	}
	src := readSource(t, root, probe)
	if !strings.Contains(src, "GetMeshAddress(") {
		t.Errorf("%s is allowlisted as the mesh probe but no longer resolves the mesh — "+
			"remove it from the allowlist or restore the probe", probe)
	}
}
