package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestInfraTools_GatedUnderDoctor(t *testing.T) {
	names := []string{"infra_probe_component", "infra_probe_all", "infra_explain_stall", "infra_diff"}

	// Doctor enabled → tools present.
	on := newServer(defaultConfig())
	registerAllTools(on)
	for _, n := range names {
		if _, ok := on.tools[n]; !ok {
			t.Errorf("%s should be registered when Doctor group enabled", n)
		}
	}

	// All groups disabled → tools absent.
	cfg := defaultConfig()
	cfg.ToolGroups = ToolGroupConfig{}
	off := newServer(cfg)
	registerAllTools(off)
	for _, n := range names {
		if _, ok := off.tools[n]; ok {
			t.Errorf("%s must not be registered when Doctor group disabled", n)
		}
	}
}

func stalledLoopbackProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:    "scylladb",
		Installed:    true,
		DaemonActive: true,
		Healthy:      false,
		ConfigValid:  false,
		Rendered: map[string]string{
			"present": "true", "cluster_name": "Globular", "listen_address": "127.0.0.1", "seeds": "127.0.0.1",
		},
		Runtime: map[string]string{"daemon_active": "true", "rest_api_ready": "true", "cql_ready": "false"},
		Desired: map[string]string{"cluster_name": "Globular", "expected_listen": "10.0.0.63", "expected_seeds": "10.0.0.8,10.0.0.20"},
		Violations: []*cluster_controllerpb.InfraViolation{
			{Id: "scylla.loopback_forbidden", Severity: "CRITICAL", Message: "listen_address is a loopback address (127.0.0.1)", Evidence: "listen_address=127.0.0.1", Remediation: "Fix the ScyllaDB renderer/desired state, not the file."},
		},
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:          cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			StateLabel:     "stalled",
			BlockingReason: "daemon is active but listen_address is a loopback address (127.0.0.1)",
		},
	}
}

func TestInfraExplainStall_ConfigViolation(t *testing.T) {
	exp := explainInfraStall("globule-ryzen", stalledLoopbackProbe())

	if exp["stalled"] != true {
		t.Errorf("expected stalled=true, got %v", exp["stalled"])
	}
	if exp["actual_state"] != "stalled" {
		t.Errorf("actual_state=%v", exp["actual_state"])
	}
	if exp["expected_state"] != "member_ready" {
		t.Errorf("expected_state=%v", exp["expected_state"])
	}
	// REST was ready, so the furthest successful stage is local_api_ready.
	if exp["last_successful_stage"] != "local_api_ready" {
		t.Errorf("last_successful_stage=%v", exp["last_successful_stage"])
	}
	blocking, _ := exp["blocking_violations"].([]map[string]interface{})
	if len(blocking) == 0 {
		t.Fatal("expected blocking_violations to be populated")
	}
	if exp["recommended_repair_target"] == "" {
		t.Error("recommended_repair_target must point at the owner")
	}
	if _, ok := exp["safe_next_commands"].([]string); !ok {
		t.Error("safe_next_commands must be present")
	}
}

func TestInfraDiff_DesiredVsRendered(t *testing.T) {
	p := stalledLoopbackProbe()
	p.Rendered["cluster_name"] = "WrongName" // disagree with desired
	d := infraDiff(p)
	mismatches, _ := d["mismatches"].([]map[string]string)
	found := false
	for _, m := range mismatches {
		if m["field"] == "cluster_name" && m["desired"] == "Globular" && m["rendered"] == "WrongName" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cluster_name mismatch, got %+v", mismatches)
	}
}

func TestInfraDiff_ListenAddressMismatch(t *testing.T) {
	// desired listen 10.0.0.63, rendered 127.0.0.1 → mismatch on listen_address.
	d := infraDiff(stalledLoopbackProbe())
	mismatches, _ := d["mismatches"].([]map[string]string)
	found := false
	for _, m := range mismatches {
		if m["field"] == "listen_address" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected listen_address mismatch, got %+v", mismatches)
	}
}

// stalledEtcdLoopbackProbe is an etcd member with a loopback peer URL — the
// etcd analog of the scylla loopback stall. Its runtime/rendered/desired keys
// follow the etcd adapter's vocabulary.
func stalledEtcdLoopbackProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:    "etcd",
		Installed:    true,
		DaemonActive: true,
		Healthy:      false,
		ConfigValid:  false,
		Rendered: map[string]string{
			"present": "true", "name": "globule-ryzen",
			"initial_cluster_token": "wrong-token",
			"advertise_client_urls": "https://127.0.0.1:2379",
			"listen_peer_urls":      "https://127.0.0.1:2380",
		},
		Runtime: map[string]string{"daemon_active": "true", "local_reachable": "false"},
		Desired: map[string]string{"cluster_name": "test-cluster-etcd-cluster", "expected_listen": "10.0.0.63"},
		Violations: []*cluster_controllerpb.InfraViolation{
			{Id: "etcd.loopback_forbidden", Severity: "CRITICAL", Message: "listen-peer-urls advertises a loopback address (127.0.0.1)", Evidence: "listen-peer-urls=https://127.0.0.1:2380", Remediation: "Fix the etcd renderer, not the file."},
		},
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:          cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			StateLabel:     "stalled",
			BlockingReason: "daemon is active but listen-peer-urls advertises a loopback address (127.0.0.1)",
		},
	}
}

func TestInfraLogUnit_PerComponent(t *testing.T) {
	if u := infraLogUnit("etcd"); u != "globular-etcd" {
		t.Errorf("etcd log unit=%q, want globular-etcd", u)
	}
	if u := infraLogUnit("scylladb"); u != "scylla-server" {
		t.Errorf("scylladb log unit=%q, want scylla-server", u)
	}
}

func TestInfraDiff_EtcdTokenAndListenMismatch(t *testing.T) {
	d := infraDiff(stalledEtcdLoopbackProbe())
	mismatches, _ := d["mismatches"].([]map[string]string)
	var token, listen bool
	for _, m := range mismatches {
		if m["field"] == "cluster_token" && m["desired"] == "test-cluster-etcd-cluster" && m["rendered"] == "wrong-token" {
			token = true
		}
		if m["field"] == "listen_address" && m["desired"] == "10.0.0.63" && m["rendered"] == "127.0.0.1" {
			listen = true
		}
	}
	if !token || !listen {
		t.Fatalf("expected etcd token + listen_address mismatches, got %+v", mismatches)
	}
}

func TestInfraExplainStall_EtcdUsesEtcdLogUnit(t *testing.T) {
	exp := explainInfraStall("globule-ryzen", stalledEtcdLoopbackProbe())
	cmds, _ := exp["safe_next_commands"].([]string)
	found := false
	for _, c := range cmds {
		if strings.Contains(c, "globular-etcd") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an etcd log command in safe_next_commands, got %+v", cmds)
	}
}

// stalledMinioSplitBrainProbe is a MinIO member rendered standalone while the
// desired topology is distributed — the format.json blast-radius split-brain.
func stalledMinioSplitBrainProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:    "minio",
		Installed:    true,
		DaemonActive: true,
		Healthy:      false,
		ConfigValid:  false,
		Rendered: map[string]string{
			"present": "true", "mode": "standalone", "volume_count": "1",
		},
		Runtime: map[string]string{"daemon_active": "true", "live": "true", "write_quorum": "true"},
		Desired: map[string]string{"mode": "distributed", "expected_volume_count": "3"},
		Violations: []*cluster_controllerpb.InfraViolation{
			{Id: "minio.topology_matches_desired", Severity: "CRITICAL", Message: "desired distributed but rendered standalone — split-brain", Evidence: "desired_mode=distributed rendered_mode=standalone", Remediation: "Fix the MinIO renderer, not the file."},
		},
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:          cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			StateLabel:     "stalled",
			BlockingReason: "daemon is active but this node would form an isolated single-node store (split-brain)",
		},
	}
}

func TestInfraDiff_MinioModeAndVolumeMismatch(t *testing.T) {
	d := infraDiff(stalledMinioSplitBrainProbe())
	mismatches, _ := d["mismatches"].([]map[string]string)
	var mode, vols bool
	for _, m := range mismatches {
		if m["field"] == "mode" && m["desired"] == "distributed" && m["rendered"] == "standalone" {
			mode = true
		}
		if m["field"] == "volume_count" && m["desired"] == "3" && m["rendered"] == "1" {
			vols = true
		}
	}
	if !mode || !vols {
		t.Fatalf("expected minio mode + volume_count mismatches, got %+v", mismatches)
	}
}

func TestInfraExplainStall_MinioUsesMinioLogUnit(t *testing.T) {
	exp := explainInfraStall("globule-ryzen", stalledMinioSplitBrainProbe())
	cmds, _ := exp["safe_next_commands"].([]string)
	found := false
	for _, c := range cmds {
		if strings.Contains(c, "globular-minio") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a minio log command in safe_next_commands, got %+v", cmds)
	}
}

// wedgedEnvoyProbe is an Envoy data plane in the LDS wedge — CDS applied but LDS
// never attempted, so port 443 never binds.
func wedgedEnvoyProbe() *cluster_controllerpb.InfraProbeResult {
	return &cluster_controllerpb.InfraProbeResult{
		Component:    "envoy",
		Installed:    true,
		DaemonActive: true,
		Healthy:      false,
		ConfigValid:  true,
		Rendered: map[string]string{
			"present": "true", "node_id": "globule-ryzen", "ads_cluster": "xds_cluster", "lds_config": "true",
		},
		Runtime: map[string]string{"daemon_active": "true", "admin_reachable": "true", "cds_update_success": "4", "lds_update_attempt": "0", "active_listeners": "0"},
		Desired: map[string]string{"node_id": "globule-ryzen", "cluster_name": "xds_cluster"},
		Lifecycle: &cluster_controllerpb.InfraLifecycleObservation{
			State:          cluster_controllerpb.InfraLifecycleState_INFRA_STALLED,
			StateLabel:     "stalled",
			BlockingReason: "Envoy mesh WEDGED — CDS applied 4 update(s) but LDS update_attempt is 0",
		},
	}
}

func TestInfraExplainStall_EnvoyWedgeUsesEnvoyLogUnit(t *testing.T) {
	exp := explainInfraStall("globule-ryzen", wedgedEnvoyProbe())
	if exp["stalled"] != true {
		t.Errorf("expected stalled=true, got %v", exp["stalled"])
	}
	cmds, _ := exp["safe_next_commands"].([]string)
	found := false
	for _, c := range cmds {
		if strings.Contains(c, "globular-envoy") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an envoy log command in safe_next_commands, got %+v", cmds)
	}
}

func TestInfraDiff_EnvoyNodeIDMismatch(t *testing.T) {
	p := wedgedEnvoyProbe()
	p.Rendered["node_id"] = "wrong-node"
	d := infraDiff(p)
	mismatches, _ := d["mismatches"].([]map[string]string)
	found := false
	for _, m := range mismatches {
		if m["field"] == "node_id" && m["desired"] == "globule-ryzen" && m["rendered"] == "wrong-node" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected envoy node_id mismatch, got %+v", mismatches)
	}
}

func TestInfraProbeToMap_Projection(t *testing.T) {
	m := infraProbeToMap(stalledLoopbackProbe())
	if m["component"] != "scylladb" {
		t.Errorf("component=%v", m["component"])
	}
	if m["config_valid"] != false {
		t.Errorf("config_valid=%v", m["config_valid"])
	}
	lc, ok := m["lifecycle"].(map[string]interface{})
	if !ok || lc["state"] != "stalled" {
		t.Errorf("lifecycle projection wrong: %v", m["lifecycle"])
	}
}
