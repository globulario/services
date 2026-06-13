package main

import (
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
