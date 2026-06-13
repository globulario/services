package infra_truth

import (
	"os"
	"path/filepath"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// joiningDesired returns a desired state for a node joining an existing cluster.
func joiningDesired() *InfraDesiredState {
	return &InfraDesiredState{
		Component:               ComponentScylla,
		NodeID:                  "globule-ryzen",
		ClusterID:               "test-cluster",
		Source:                  SourceComputedFromMembership,
		ExpectedListenAddresses: []string{"10.0.0.63"},
		ExpectedPeers:           []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		ExpectedSeeds:           []string{"10.0.0.8", "10.0.0.20"},
		ExpectedClusterName:     "Globular",
		BootstrapIntent:         BootstrapJoining,
	}
}

// validRendered is a correctly rendered config for a joining node.
func validRendered() *ScyllaRenderedConfig {
	return &ScyllaRenderedConfig{
		Path:                ScyllaConfigPath,
		Present:             true,
		ClusterName:         "Globular",
		ListenAddress:       "10.0.0.63",
		RPCAddress:          "10.0.0.63",
		BroadcastAddress:    "10.0.0.63",
		BroadcastRPCAddress: "10.0.0.63",
		APIAddress:          "127.0.0.1",
		Seeds:               []string{"10.0.0.8", "10.0.0.20"},
	}
}

func containsViolation(vs []*cluster_controllerpb.InfraViolation, id, severity string) bool {
	for _, v := range vs {
		if v.GetId() == id && v.GetSeverity() == severity {
			return true
		}
	}
	return false
}

func TestParseScyllaYAML_GeneratedConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scylla.yaml")
	content := `cluster_name: 'Globular'
listen_address: 10.0.0.63
rpc_address: 10.0.0.63
broadcast_address: 10.0.0.63
broadcast_rpc_address: 10.0.0.63
api_address: 127.0.0.1
seed_provider:
    - class_name: org.apache.cassandra.locator.SimpleSeedProvider
      parameters:
          - seeds: "10.0.0.8,10.0.0.20"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseScyllaYAML(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.Present {
		t.Fatal("expected Present=true")
	}
	if cfg.ClusterName != "Globular" {
		t.Errorf("cluster_name=%q", cfg.ClusterName)
	}
	if cfg.ListenAddress != "10.0.0.63" {
		t.Errorf("listen_address=%q", cfg.ListenAddress)
	}
	if cfg.APIAddress != "127.0.0.1" {
		t.Errorf("api_address=%q", cfg.APIAddress)
	}
	if len(cfg.Seeds) != 2 || cfg.Seeds[0] != "10.0.0.8" || cfg.Seeds[1] != "10.0.0.20" {
		t.Errorf("seeds=%v", cfg.Seeds)
	}
}

func TestParseScyllaYAML_Missing(t *testing.T) {
	cfg, err := parseScyllaYAML(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if cfg.Present {
		t.Fatal("expected Present=false for missing file")
	}
}

func TestAttestScyllaConfig_ValidConfig(t *testing.T) {
	v := AttestScyllaConfig(joiningDesired(), validRendered())
	if len(v) != 0 {
		t.Fatalf("expected no violations, got %d: %+v", len(v), v)
	}
}

func TestAttestScyllaConfig_LocalhostListenAddress(t *testing.T) {
	r := validRendered()
	r.ListenAddress = "127.0.0.1"
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL scylla.loopback_forbidden, got %+v", v)
	}
}

func TestAttestScyllaConfig_LocalhostRPCAddress(t *testing.T) {
	r := validRendered()
	r.RPCAddress = "localhost"
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL scylla.loopback_forbidden for rpc_address, got %+v", v)
	}
}

func TestAttestScyllaConfig_BroadcastLoopback(t *testing.T) {
	r := validRendered()
	r.BroadcastAddress = "::1"
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL scylla.loopback_forbidden for broadcast_address, got %+v", v)
	}
}

func TestAttestScyllaConfig_ListenUnspecified(t *testing.T) {
	r := validRendered()
	r.ListenAddress = "0.0.0.0"
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL for unspecified listen_address, got %+v", v)
	}
}

func TestAttestScyllaConfig_SelfOnlySeed_JoiningNode(t *testing.T) {
	r := validRendered()
	r.Seeds = []string{"10.0.0.63"} // only self
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("expected ERROR scylla.config_valid for self-only seed on joining node, got %+v", v)
	}
}

func TestAttestScyllaConfig_SelfOnlySeed_FirstNodeAllowed(t *testing.T) {
	d := joiningDesired()
	d.BootstrapIntent = BootstrapFirstNode
	d.ExpectedSeeds = []string{"10.0.0.63"}
	r := validRendered()
	r.Seeds = []string{"10.0.0.63"} // self-only is fine for first node
	v := AttestScyllaConfig(d, r)
	if containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("self-only seed must be allowed for first-node, got %+v", v)
	}
}

func TestAttestScyllaConfig_EmptyClusterName(t *testing.T) {
	r := validRendered()
	r.ClusterName = ""
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("expected ERROR scylla.config_valid for empty cluster_name, got %+v", v)
	}
}

func TestAttestScyllaConfig_ClusterNameMismatch(t *testing.T) {
	r := validRendered()
	r.ClusterName = "WrongName"
	v := AttestScyllaConfig(joiningDesired(), r)
	if !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("expected ERROR scylla.config_valid for cluster_name mismatch, got %+v", v)
	}
}

func TestAttestScyllaConfig_NotPresentNoViolations(t *testing.T) {
	v := AttestScyllaConfig(joiningDesired(), &ScyllaRenderedConfig{Path: ScyllaConfigPath, Present: false})
	if len(v) != 0 {
		t.Fatalf("absent config must yield no config violations, got %+v", v)
	}
}
