package infra_truth

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func envoyDesired() *InfraDesiredState {
	d, err := BuildEnvoyDesiredState(EnvoyDesiredInputs{
		NodeID: "globule-ryzen", ClusterID: "test-cluster", LocalIP: "10.0.0.63",
	})
	if err != nil {
		panic(err)
	}
	return d
}

// validEnvoyRendered is a correctly rendered bootstrap (ads+cds+lds, xds_cluster
// defined, admin set).
func validEnvoyRendered() *EnvoyRenderedConfig {
	return &EnvoyRenderedConfig{
		Path:               EnvoyBootstrapPath,
		Present:            true,
		NodeID:             "globule-ryzen",
		NodeCluster:        "globular",
		AdminAddress:       "127.0.0.1",
		AdminPort:          9901,
		HasADSConfig:       true,
		ADSClusterName:     "xds_cluster",
		HasCDSConfig:       true,
		HasLDSConfig:       true,
		StaticClusterNames: []string{"xds_cluster"},
	}
}

// ── parser ───────────────────────────────────────────────────────────────────

func TestParseEnvoyBootstrap_Generated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "envoy-bootstrap.json")
	content := `{
  "node": {"cluster": "globular", "id": "globule-ryzen"},
  "dynamic_resources": {
    "ads_config": {"api_type": "GRPC", "transport_api_version": "V3",
      "grpc_services": [{"envoy_grpc": {"cluster_name": "xds_cluster"}}]},
    "cds_config": {"resource_api_version": "V3", "ads": {}},
    "lds_config": {"resource_api_version": "V3", "ads": {}}
  },
  "static_resources": {"clusters": [{"name": "xds_cluster"}]},
  "admin": {"address": {"socket_address": {"address": "127.0.0.1", "port_value": 9901}}}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseEnvoyBootstrap(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.Present || cfg.NodeID != "globule-ryzen" {
		t.Fatalf("present=%t node_id=%q", cfg.Present, cfg.NodeID)
	}
	if !cfg.HasADSConfig || !cfg.HasCDSConfig || !cfg.HasLDSConfig {
		t.Errorf("ads=%t cds=%t lds=%t", cfg.HasADSConfig, cfg.HasCDSConfig, cfg.HasLDSConfig)
	}
	if cfg.ADSClusterName != "xds_cluster" || !cfg.hasStaticCluster("xds_cluster") {
		t.Errorf("ads_cluster=%q static=%v", cfg.ADSClusterName, cfg.StaticClusterNames)
	}
	if cfg.adminBaseURL() != "http://127.0.0.1:9901" {
		t.Errorf("admin base=%q", cfg.adminBaseURL())
	}
}

func TestParseEnvoyBootstrap_Missing(t *testing.T) {
	cfg, err := parseEnvoyBootstrap(filepath.Join(t.TempDir(), "no-such.json"))
	if err != nil {
		t.Fatalf("missing file should not error, got %v", err)
	}
	if cfg.Present {
		t.Fatal("expected Present=false for missing file")
	}
}

func TestEnvoyAdminBaseURL_Default(t *testing.T) {
	c := &EnvoyRenderedConfig{}
	if c.adminBaseURL() != "http://127.0.0.1:9901" {
		t.Errorf("default admin base=%q", c.adminBaseURL())
	}
}

// ── attestation ──────────────────────────────────────────────────────────────

func TestAttestEnvoyConfig_Valid(t *testing.T) {
	v := AttestEnvoyConfig(envoyDesired(), validEnvoyRendered())
	if len(v) != 0 {
		t.Fatalf("expected no violations, got %d: %+v", len(v), v)
	}
}

func TestAttestEnvoyConfig_MissingLDS_CriticalStaticWedge(t *testing.T) {
	r := validEnvoyRendered()
	r.HasLDSConfig = false
	v := AttestEnvoyConfig(envoyDesired(), r)
	if !containsViolation(v, "envoy.config_valid", SeverityCritical) {
		t.Fatalf("expected CRITICAL envoy.config_valid for missing lds_config, got %+v", v)
	}
}

func TestAttestEnvoyConfig_MissingADS_Critical(t *testing.T) {
	r := validEnvoyRendered()
	r.HasADSConfig = false
	v := AttestEnvoyConfig(envoyDesired(), r)
	if !containsViolation(v, "envoy.config_valid", SeverityCritical) {
		t.Fatalf("expected CRITICAL envoy.config_valid for missing ads_config, got %+v", v)
	}
}

func TestAttestEnvoyConfig_ADSClusterUndefined_Critical(t *testing.T) {
	r := validEnvoyRendered()
	r.StaticClusterNames = []string{"some_other_cluster"} // xds_cluster missing
	v := AttestEnvoyConfig(envoyDesired(), r)
	if !containsViolation(v, "envoy.config_valid", SeverityCritical) {
		t.Fatalf("expected CRITICAL for ADS cluster undefined in static_resources, got %+v", v)
	}
}

func TestAttestEnvoyConfig_MissingCDS_Error(t *testing.T) {
	r := validEnvoyRendered()
	r.HasCDSConfig = false
	v := AttestEnvoyConfig(envoyDesired(), r)
	if !containsViolation(v, "envoy.config_valid", SeverityError) {
		t.Fatalf("expected ERROR envoy.config_valid for missing cds_config, got %+v", v)
	}
}

func TestAttestEnvoyConfig_EmptyNodeID_Error(t *testing.T) {
	r := validEnvoyRendered()
	r.NodeID = ""
	v := AttestEnvoyConfig(envoyDesired(), r)
	if !containsViolation(v, "envoy.config_valid", SeverityError) {
		t.Fatalf("expected ERROR for empty node id, got %+v", v)
	}
}

func TestAttestEnvoyConfig_NotPresentNoViolations(t *testing.T) {
	v := AttestEnvoyConfig(envoyDesired(), &EnvoyRenderedConfig{Path: EnvoyBootstrapPath, Present: false})
	if len(v) != 0 {
		t.Fatalf("absent bootstrap must yield no config violations, got %+v", v)
	}
}

// ── lifecycle ────────────────────────────────────────────────────────────────

func TestDeriveEnvoyLifecycle_States(t *testing.T) {
	rendered := validEnvoyRendered()
	critical := []*cluster_controllerpb.InfraViolation{newViolation("envoy.config_valid", SeverityCritical, "m", "e", "r")}

	serving := func() *EnvoyRuntimeState {
		return &EnvoyRuntimeState{DaemonActive: true, AdminReachable: true, Ready: true, ServerState: "LIVE",
			CDSUpdateSuccess: 4, LDSUpdateAttempt: 4, LDSUpdateSuccess: 4, ActiveClusters: 3, ActiveListeners: 2}
	}

	cases := []struct {
		name      string
		installed bool
		rendered  *EnvoyRenderedConfig
		runtime   *EnvoyRuntimeState
		viol      []*cluster_controllerpb.InfraViolation
		want      cluster_controllerpb.InfraLifecycleState
	}{
		{"not installed", false, nil, nil, nil, cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT},
		{"no bootstrap", true, &EnvoyRenderedConfig{Present: false}, nil, nil, cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED},
		{"critical config stalls", true, rendered, serving(), critical, cluster_controllerpb.InfraLifecycleState_INFRA_STALLED},
		{"daemon down", true, rendered, &EnvoyRuntimeState{DaemonActive: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_CONFIG_ATTESTED},
		{"admin unreachable", true, rendered, &EnvoyRuntimeState{DaemonActive: true, AdminReachable: false}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING},
		{"LDS wedge stalls", true, rendered, &EnvoyRuntimeState{DaemonActive: true, AdminReachable: true, CDSUpdateSuccess: 4, LDSUpdateAttempt: 0}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_STALLED},
		{"warming no cds", true, rendered, &EnvoyRuntimeState{DaemonActive: true, AdminReachable: true, CDSUpdateSuccess: 0, LDSUpdateAttempt: 0}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_LOCAL_API_READY},
		{"lds rejected degraded", true, rendered, &EnvoyRuntimeState{DaemonActive: true, AdminReachable: true, CDSUpdateSuccess: 4, LDSUpdateAttempt: 4, LDSUpdateRejected: 1, ActiveListeners: 1, Ready: true}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"no active listeners degraded", true, rendered, &EnvoyRuntimeState{DaemonActive: true, AdminReachable: true, CDSUpdateSuccess: 4, LDSUpdateAttempt: 4, ActiveListeners: 0}, nil, cluster_controllerpb.InfraLifecycleState_INFRA_DEGRADED},
		{"member ready", true, rendered, serving(), nil, cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obs := deriveEnvoyLifecycle(c.installed, c.rendered, c.runtime, c.viol, 0)
			if obs.GetState() != c.want {
				t.Fatalf("state=%s want=%s (blocking=%q)", obs.GetStateLabel(), lifecycleLabel(c.want), obs.GetBlockingReason())
			}
		})
	}
}

// ── ProbeStructured ──────────────────────────────────────────────────────────

func writeValidEnvoyBootstrap(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "envoy-bootstrap.json")
	content := `{
  "node": {"cluster": "globular", "id": "globule-ryzen"},
  "dynamic_resources": {
    "ads_config": {"api_type": "GRPC", "grpc_services": [{"envoy_grpc": {"cluster_name": "xds_cluster"}}]},
    "cds_config": {"ads": {}},
    "lds_config": {"ads": {}}
  },
  "static_resources": {"clusters": [{"name": "xds_cluster"}]},
  "admin": {"address": {"socket_address": {"address": "127.0.0.1", "port_value": 9901}}}
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEnvoyProbeStructured_Serving(t *testing.T) {
	path := writeValidEnvoyBootstrap(t)
	var observedBase string
	p := &EnvoyProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		Observe: func(_ context.Context, base string) *EnvoyRuntimeState {
			observedBase = base
			return &EnvoyRuntimeState{AdminReachable: true, Ready: true, ServerState: "LIVE",
				CDSUpdateSuccess: 4, LDSUpdateAttempt: 4, LDSUpdateSuccess: 4, ActiveClusters: 3, ActiveListeners: 2}
		},
	}
	res := p.ProbeStructured(context.Background(), envoyDesired(), nil)
	if !res.GetInstalled() || !res.GetConfigValid() || !res.GetHealthy() {
		t.Fatalf("expected installed+valid+healthy, got installed=%t valid=%t healthy=%t violations=%+v",
			res.GetInstalled(), res.GetConfigValid(), res.GetHealthy(), res.GetViolations())
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY {
		t.Fatalf("lifecycle=%s blocking=%q", res.GetLifecycle().GetStateLabel(), res.GetLifecycle().GetBlockingReason())
	}
	if observedBase != "http://127.0.0.1:9901" {
		t.Errorf("observer dialed %q, expected the loopback admin URL", observedBase)
	}
}

func TestEnvoyProbeStructured_LDSWedgeStalled(t *testing.T) {
	path := writeValidEnvoyBootstrap(t)
	p := &EnvoyProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
		Observe: func(_ context.Context, _ string) *EnvoyRuntimeState {
			return &EnvoyRuntimeState{AdminReachable: true, CDSUpdateSuccess: 5, LDSUpdateAttempt: 0}
		},
	}
	res := p.ProbeStructured(context.Background(), envoyDesired(), nil)
	if res.GetHealthy() {
		t.Fatal("a wedged Envoy must not be reported healthy")
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_STALLED {
		t.Fatalf("expected STALLED for the LDS wedge, got %s", res.GetLifecycle().GetStateLabel())
	}
}

func TestEnvoyProbeStructured_ObserverMissingIsExplicit(t *testing.T) {
	path := writeValidEnvoyBootstrap(t)
	p := &EnvoyProber{
		ConfigPath:      path,
		DetectInstalled: func(context.Context) bool { return true },
		UnitActive:      func(context.Context) bool { return true },
		NowUnix:         func() int64 { return 0 },
	}
	res := p.ProbeStructured(context.Background(), envoyDesired(), nil)
	if res.GetHealthy() {
		t.Fatal("a data plane with no runtime observation must not be reported healthy")
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_DAEMON_STARTING {
		t.Fatalf("expected DAEMON_STARTING when runtime unobserved, got %s", res.GetLifecycle().GetStateLabel())
	}
	if len(res.GetErrors()) == 0 {
		t.Error("expected an explicit error documenting the missing observer")
	}
}
