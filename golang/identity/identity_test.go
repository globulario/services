package identity_test

import (
	"testing"

	"github.com/globulario/services/golang/identity"
)

func TestNormalizeServiceKey_NodeAgent(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"node-agent", "node-agent"},
		{"node_agent.NodeAgentService", "node-agent"},
		{"globular-node-agent.service", "node-agent"},
		{"node_agent_server", "node-agent"},
		{"nodeagent", "node-agent"},
		{"node_agent", "node-agent"},
		{"NODEAGENT.NodeAgentService", "node-agent"},
	}
	for _, tc := range cases {
		got, ok := identity.NormalizeServiceKey(tc.input)
		if !ok {
			t.Errorf("NormalizeServiceKey(%q): got ok=false, want ok=true", tc.input)
		}
		if got != tc.want {
			t.Errorf("NormalizeServiceKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeServiceKey_ClusterController(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"cluster-controller", "cluster-controller"},
		{"cluster_controller.ClusterControllerService", "cluster-controller"},
		{"globular-cluster-controller.service", "cluster-controller"},
		{"cluster_controller_server", "cluster-controller"},
		{"clustercontroller", "cluster-controller"},
		{"cluster_controller", "cluster-controller"},
	}
	for _, tc := range cases {
		got, ok := identity.NormalizeServiceKey(tc.input)
		if !ok {
			t.Errorf("NormalizeServiceKey(%q): got ok=false, want ok=true", tc.input)
		}
		if got != tc.want {
			t.Errorf("NormalizeServiceKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeServiceKey_KnownServices(t *testing.T) {
	cases := []struct{ input, want string }{
		{"dns.DnsService", "dns"},
		{"globular-dns.service", "dns"},
		{"file.FileService", "file"},
		{"globular-file.service", "file"},
		{"event.EventService", "event"},
		{"rbac.RbacService", "rbac"},
		{"resource.ResourceService", "resource"},
		{"repository.PackageRepository", "repository"},
		{"media.MediaService", "media"},
		{"minio", "minio"},
		{"globular-minio.service", "minio"},
		{"etcd", "etcd"},
		{"globular-etcd.service", "etcd"},
		{"envoy", "envoy"},
		{"envoy.service", "envoy"},
		{"globular-gateway", "gateway"},
		{"globular-gateway.service", "gateway"},
		{"globular-xds", "xds"},
		{"globular-xds.service", "xds"},
	}
	for _, tc := range cases {
		got, ok := identity.NormalizeServiceKey(tc.input)
		if !ok {
			t.Errorf("NormalizeServiceKey(%q): got ok=false, want ok=true", tc.input)
		}
		if got != tc.want {
			t.Errorf("NormalizeServiceKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIdentityByKey(t *testing.T) {
	id, ok := identity.IdentityByKey("node-agent")
	if !ok {
		t.Fatal("IdentityByKey(node-agent): not found")
	}
	if id.UnitName != "globular-node-agent.service" {
		t.Errorf("UnitName = %q, want globular-node-agent.service", id.UnitName)
	}
	if id.Binary != "node_agent_server" {
		t.Errorf("Binary = %q, want node_agent_server", id.Binary)
	}
}

func TestUnitForService(t *testing.T) {
	cases := []struct{ input, want string }{
		{"node-agent", "globular-node-agent.service"},
		{"node_agent.NodeAgentService", "globular-node-agent.service"},
		{"globular-node-agent.service", "globular-node-agent.service"},
		{"cluster-controller", "globular-cluster-controller.service"},
		{"cluster_controller.ClusterControllerService", "globular-cluster-controller.service"},
		{"envoy", "globular-envoy.service"},
		{"file.FileService", "globular-file.service"},
	}
	for _, tc := range cases {
		got := identity.UnitForService(tc.input)
		if got != tc.want {
			t.Errorf("UnitForService(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMustIdentityByKey_Unknown(t *testing.T) {
	id := identity.MustIdentityByKey("my-custom-service")
	if id.UnitName != "globular-my-custom-service.service" {
		t.Errorf("unknown service UnitName = %q, want globular-my-custom-service.service", id.UnitName)
	}
}
