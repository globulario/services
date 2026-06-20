package infra_truth

import (
	"os"
	"path/filepath"
	"strings"
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
	v := AttestScyllaConfig(joiningDesired(), validRendered(), nil)
	if len(v) != 0 {
		t.Fatalf("expected no violations, got %d: %+v", len(v), v)
	}
}

func TestAttestScyllaConfig_LocalhostListenAddress(t *testing.T) {
	r := validRendered()
	r.ListenAddress = "127.0.0.1"
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL scylla.loopback_forbidden, got %+v", v)
	}
}

func TestAttestScyllaConfig_LocalhostRPCAddress(t *testing.T) {
	r := validRendered()
	r.RPCAddress = "localhost"
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL scylla.loopback_forbidden for rpc_address, got %+v", v)
	}
}

func TestAttestScyllaConfig_BroadcastLoopback(t *testing.T) {
	r := validRendered()
	r.BroadcastAddress = "::1"
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL scylla.loopback_forbidden for broadcast_address, got %+v", v)
	}
}

func TestAttestScyllaConfig_ListenUnspecified(t *testing.T) {
	r := validRendered()
	r.ListenAddress = "0.0.0.0"
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if !containsViolation(v, "scylla.loopback_forbidden", SeverityCritical) {
		t.Fatalf("expected CRITICAL for unspecified listen_address, got %+v", v)
	}
}

func TestAttestScyllaConfig_SelfOnlySeed_JoiningNode(t *testing.T) {
	r := validRendered()
	r.Seeds = []string{"10.0.0.63"} // only self
	v := AttestScyllaConfig(joiningDesired(), r, nil)
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
	v := AttestScyllaConfig(d, r, nil)
	if containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("self-only seed must be allowed for first-node, got %+v", v)
	}
}

// TestAttestScyllaConfig_SelfOnlySeed_EstablishedMember_NoError covers the
// false-positive fix: a node whose membership-derived intent is "joining" and
// whose rendered seeds are self-only is NOT broken if runtime proves it is already
// a converged member of a multi-node ring (operation_mode NORMAL + a live non-self
// gossip peer). The "will bootstrap an isolated ring" prediction is counterfactual
// there, so it must NOT be an ERROR — it is downgraded to an INFO seed-hygiene
// note. This is the founding-node-flagged-as-joining case (INC: scylla.config_valid
// false positive on globule-ryzen after globule-nuc joined).
func TestAttestScyllaConfig_SelfOnlySeed_EstablishedMember_NoError(t *testing.T) {
	r := validRendered()
	r.Seeds = []string{"10.0.0.63"} // only self
	rt := &ScyllaRuntimeState{
		DaemonActive:  true,
		RESTAPIReady:  true,
		OperationMode: "NORMAL",
		ObservedPeers: []string{"10.0.0.63", "10.0.0.8"}, // self + a live peer
		GossipLive:    2,
	}
	v := AttestScyllaConfig(joiningDesired(), r, rt)
	if containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("self-only seed on an established NORMAL multi-node member must NOT be an ERROR, got %+v", v)
	}
	if !containsViolation(v, "scylla.config_valid", SeverityInfo) {
		t.Fatalf("expected an INFO seed-hygiene note for self-only seed on an established member, got %+v", v)
	}
}

// TestAttestScyllaConfig_SelfOnlySeed_ActuallyIsolated_StillError ensures the fix
// does NOT mask a genuinely isolated node: a joining node that came up NORMAL but
// observes only itself in live gossip really did form a one-node ring. That is the
// scylla.wrong_config_appears_healthy failure mode and must remain an ERROR.
func TestAttestScyllaConfig_SelfOnlySeed_ActuallyIsolated_StillError(t *testing.T) {
	r := validRendered()
	r.Seeds = []string{"10.0.0.63"} // only self
	rt := &ScyllaRuntimeState{
		DaemonActive:  true,
		RESTAPIReady:  true,
		OperationMode: "NORMAL",
		ObservedPeers: []string{"10.0.0.63"}, // only self is live → isolated
		GossipLive:    1,
	}
	v := AttestScyllaConfig(joiningDesired(), r, rt)
	if !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("self-only seed + self-only gossip is a genuinely isolated ring; expected ERROR, got %+v", v)
	}
}

// TestAttestScyllaConfig_SelfOnlySeed_PreBootstrap_StillError ensures a config
// that would isolate a fresh node is caught BEFORE the daemon proves otherwise:
// with no runtime proof (nil) or a not-yet-NORMAL node, the rule stays an ERROR
// (infra.config_must_be_attested_before_start).
func TestAttestScyllaConfig_SelfOnlySeed_PreBootstrap_StillError(t *testing.T) {
	r := validRendered()
	r.Seeds = []string{"10.0.0.63"} // only self

	// nil runtime: no proof yet.
	if v := AttestScyllaConfig(joiningDesired(), r, nil); !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("self-only seed with no runtime proof must be ERROR, got %+v", v)
	}

	// daemon still bootstrapping (JOINING) is not proof of membership.
	rt := &ScyllaRuntimeState{DaemonActive: true, RESTAPIReady: true, OperationMode: "JOINING", ObservedPeers: []string{"10.0.0.63", "10.0.0.8"}}
	if v := AttestScyllaConfig(joiningDesired(), r, rt); !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("self-only seed while not yet NORMAL must be ERROR, got %+v", v)
	}
}

func TestAttestScyllaConfig_EmptyClusterName(t *testing.T) {
	r := validRendered()
	r.ClusterName = ""
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("expected ERROR scylla.config_valid for empty cluster_name, got %+v", v)
	}
}

func TestAttestScyllaConfig_ClusterNameMismatch(t *testing.T) {
	r := validRendered()
	r.ClusterName = "WrongName"
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if !containsViolation(v, "scylla.config_valid", SeverityError) {
		t.Fatalf("expected ERROR scylla.config_valid for cluster_name mismatch, got %+v", v)
	}
}

func TestAttestScyllaConfig_NotPresentNoViolations(t *testing.T) {
	v := AttestScyllaConfig(joiningDesired(), &ScyllaRenderedConfig{Path: ScyllaConfigPath, Present: false}, nil)
	if len(v) != 0 {
		t.Fatalf("absent config must yield no config violations, got %+v", v)
	}
}

// TestAttestScyllaConfig_RemediationTargetsOwnerNotManualEdit directly covers
// infra.config_file_is_artifact_not_authority: the rendered config is an
// artifact, so every attestation violation must point repair at the OWNER that
// generated it (renderer + desired state) and must NOT present a manual edit of
// scylla.yaml as the fix — a render would overwrite it.
func TestAttestScyllaConfig_RemediationTargetsOwnerNotManualEdit(t *testing.T) {
	r := validRendered()
	r.ListenAddress = "127.0.0.1" // force a violation to inspect its remediation
	v := AttestScyllaConfig(joiningDesired(), r, nil)
	if len(v) == 0 {
		t.Fatal("expected at least one violation to inspect remediation")
	}
	for _, viol := range v {
		rem := viol.GetRemediation()
		if strings.TrimSpace(rem) == "" {
			t.Errorf("violation %q has empty remediation; config-is-artifact requires owner-targeted repair", viol.GetId())
			continue
		}
		// Owner-targeted: names the renderer/desired-state owner.
		if !strings.Contains(rem, "renderer") {
			t.Errorf("violation %q remediation must target the config owner (renderer/desired state); got: %q", viol.GetId(), rem)
		}
		// Must forbid a manual file edit as the permanent fix.
		if !strings.Contains(rem, "Do NOT hand-edit") {
			t.Errorf("violation %q remediation must forbid manual scylla.yaml edits; got: %q", viol.GetId(), rem)
		}
	}
}
