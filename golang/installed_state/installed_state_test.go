package installed_state

import (
	"testing"
)

func TestPackageKey(t *testing.T) {
	tests := []struct {
		nodeID, kind, name string
		want               string
	}{
		{"node-1", "SERVICE", "gateway", "/globular/nodes/node-1/packages/SERVICE/gateway"},
		{"node-2", "application", "admin", "/globular/nodes/node-2/packages/APPLICATION/admin"},
		{"node-1", "INFRASTRUCTURE", "etcd", "/globular/nodes/node-1/packages/INFRASTRUCTURE/etcd"},
	}
	for _, tt := range tests {
		got := packageKey(tt.nodeID, tt.kind, tt.name)
		if got != tt.want {
			t.Errorf("packageKey(%q, %q, %q) = %q, want %q", tt.nodeID, tt.kind, tt.name, got, tt.want)
		}
	}
}

func TestNodePackagesPrefix(t *testing.T) {
	got := nodePackagesPrefix("node-1")
	want := "/globular/nodes/node-1/packages/"
	if got != want {
		t.Errorf("nodePackagesPrefix(node-1) = %q, want %q", got, want)
	}
}

func TestNodeKindPrefix(t *testing.T) {
	got := nodeKindPrefix("node-1", "service")
	want := "/globular/nodes/node-1/packages/SERVICE/"
	if got != want {
		t.Errorf("nodeKindPrefix(node-1, service) = %q, want %q", got, want)
	}
}

func TestUnmarshalPackage(t *testing.T) {
	data := []byte(`{"nodeId":"n1","name":"gateway","version":"1.2.3","kind":"SERVICE","status":"installed"}`)
	pkg, err := unmarshalPackage(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.GetNodeId() != "n1" {
		t.Errorf("node_id = %q, want n1", pkg.GetNodeId())
	}
	if pkg.GetName() != "gateway" {
		t.Errorf("name = %q, want gateway", pkg.GetName())
	}
	if pkg.GetVersion() != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", pkg.GetVersion())
	}
	if pkg.GetKind() != "SERVICE" {
		t.Errorf("kind = %q, want SERVICE", pkg.GetKind())
	}
}

func TestUnmarshalPackage_Invalid(t *testing.T) {
	_, err := unmarshalPackage([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
