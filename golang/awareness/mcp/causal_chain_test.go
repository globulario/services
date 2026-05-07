package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// setupCausalServer creates a server with a docs dir containing causal_rules.yaml.
func setupCausalServer(t *testing.T) (*Server, string) {
	t.Helper()
	docsDir := t.TempDir()
	knowledgeDir := filepath.Join(docsDir, "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write the causal rules fixture.
	causalYAML := `rules:
  - id: etcd_disk_pressure_to_workflow_timeout
    root_signal: etcd_disk_pressure
    trigger_keywords:
      - NOSPACE
      - database space exceeded
      - etcd disk
    sequence:
      - event: etcd_nospace
        component: etcd
        keywords: [NOSPACE, database space]
      - event: leader_instability
        component: etcd/controller
        keywords: [lost leader, elected leader, leader changed]
      - event: controller_lease_churn
        component: controller
        keywords: [lease expired, lease lost, leadership]
      - event: workflow_dispatch_timeout
        component: workflow
        keywords: [dispatch timeout, context deadline exceeded, workflow]
    confidence: medium
    explanation_template: "etcd disk pressure likely destabilized control-plane operations, leading to workflow dispatch failures."
    recommended_fix_order:
      - clear etcd NOSPACE alarm (etcdctl alarm disarm)
      - verify etcd quorum and leader stability
      - verify controller leadership
      - re-run workflow dispatch verification

  - id: port_squatting_to_unknown_grpc_service
    root_signal: wrong_process_on_port
    trigger_keywords:
      - address already in use
      - port in use
    sequence:
      - event: port_in_use
        component: network
        keywords: [address already in use, bind]
      - event: unknown_grpc_service
        component: grpc
        keywords: [unknown service, unknown gRPC service, connection refused]
      - event: service_client_failure
        component: service
        keywords: [workflow, service client, unavailable]
    confidence: high
    explanation_template: "A wrong process is occupying the expected port, causing gRPC clients to receive 'unknown service' errors."
    recommended_fix_order:
      - identify listener process (ss -tlnp or lsof -i)
      - verify cgroup or systemd unit ownership
      - kill orphan if proven safe
      - restart expected service
      - verify gRPC service descriptor

  - id: minio_offline_disk_to_artifact_failure
    root_signal: minio_disk_failure
    trigger_keywords:
      - offline disk
      - healing
      - drive offline
    sequence:
      - event: minio_disk_offline
        component: minio
        keywords: [offline disk, drive offline, healing]
      - event: artifact_unavailable
        component: repository
        keywords: [artifact, download failed, not found, checksum]
      - event: service_install_blocked
        component: node_agent
        keywords: [install failed, artifact, workflow blocked]
    confidence: medium
    explanation_template: "MinIO disk failure is blocking artifact availability, causing service install workflows to fail."
    recommended_fix_order:
      - check MinIO disk health (mc admin info)
      - verify MinIO healing status

  - id: systemd_notify_failure_to_install_drift
    root_signal: systemd_notify_missing
    trigger_keywords:
      - activating
      - start operation timed out
    sequence:
      - event: service_stuck_activating
        component: systemd
        keywords: [activating, start operation timed out]
      - event: port_held_by_orphan
        component: network
        keywords: [address already in use, orphan]
      - event: waitactive_timeout
        component: node_agent
        keywords: [WaitActive, timeout, install FAILED]
    confidence: high
    explanation_template: "Service failed to send READY=1, causing systemd restarts and orphaned processes."
    recommended_fix_order:
      - verify service sends READY=1
      - kill orphaned processes
`
	rulesPath := filepath.Join(knowledgeDir, "causal_rules.yaml")
	if err := os.WriteFile(rulesPath, []byte(causalYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	s := NewWithGraph(Config{DocsDir: docsDir}, nil)
	t.Cleanup(func() { s.Close() })
	return s, docsDir
}

// TestCausalChain_Registered verifies the tool is available.
func TestCausalChain_Registered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	if !s.HasTool("awareness.causal_chain") {
		t.Error("awareness.causal_chain must be registered")
	}
}

// TestCausalChain_EtcdNspaceToWorkflowTimeout verifies etcd disk pressure chain matched.
func TestCausalChain_EtcdNspaceToWorkflowTimeout(t *testing.T) {
	s, _ := setupCausalServer(t)

	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"events": []interface{}{
			"etcd: NOSPACE alarm is activated — database space exceeded",
			"leader changed: globule-ryzen is now leader",
			"workflow context deadline exceeded while dispatching",
		},
	})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	found := false
	for _, c := range chains {
		if c.RootSignal == "etcd_disk_pressure" {
			found = true
			if c.MatchedSteps < 2 {
				t.Errorf("expected at least 2 matched steps, got %d", c.MatchedSteps)
			}
			if c.Explanation == "" {
				t.Error("expected non-empty explanation")
			}
			if len(c.RecommendedFixOrder) == 0 {
				t.Error("expected non-empty recommended_fix_order")
			}
		}
	}
	if !found {
		t.Errorf("expected etcd_disk_pressure chain, got: %+v", chains)
	}
}

// TestCausalChain_PortSquatting verifies port squatting chain is matched.
func TestCausalChain_PortSquatting(t *testing.T) {
	s, _ := setupCausalServer(t)

	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"offline_evidence": `
bind: address already in use :10004
rpc error: code = Unimplemented desc = unknown gRPC service
workflow service client unavailable
`,
	})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	found := false
	for _, c := range chains {
		if c.RootSignal == "wrong_process_on_port" {
			found = true
			if c.Confidence != "high" {
				t.Errorf("expected high confidence for port squatting chain, got %q", c.Confidence)
			}
		}
	}
	if !found {
		t.Errorf("expected wrong_process_on_port chain, got: %+v", chains)
	}
}

// TestCausalChain_MinioChain verifies MinIO offline disk chain is matched.
func TestCausalChain_MinioChain(t *testing.T) {
	s, _ := setupCausalServer(t)

	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"events": []interface{}{
			"minio: drive offline /data/disk2",
			"node_agent: artifact download failed: not found",
			"node_agent: install failed for workflow:1.0.90",
		},
	})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	found := false
	for _, c := range chains {
		if c.RootSignal == "minio_disk_failure" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected minio_disk_failure chain, got: %+v", chains)
	}
}

// TestCausalChain_UnrelatedSymptoms verifies unrelated symptoms produce no chains.
func TestCausalChain_UnrelatedSymptoms(t *testing.T) {
	s, _ := setupCausalServer(t)

	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"events": []interface{}{
			"TLS certificate expired: x509 validation failed",
			"certificate mismatch on globular.io",
		},
	})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	if len(chains) > 0 {
		// Verify none of the chains have high step coverage — TLS doesn't map to any rule.
		for _, c := range chains {
			coverage := float64(c.MatchedSteps) / float64(c.TotalSteps)
			if coverage >= 0.5 {
				t.Errorf("unexpected chain matched for TLS symptoms: %+v", c)
			}
		}
	}

	// blind_spots should mention no match if chains are empty.
	if len(chains) == 0 {
		blindSpots, _ := m["blind_spots"].([]string)
		if len(blindSpots) == 0 {
			t.Error("expected blind_spots when no chains matched")
		}
	}
}

// TestCausalChain_EmptyEvents verifies empty input returns no chains with unknown confidence.
func TestCausalChain_EmptyEvents(t *testing.T) {
	s, _ := setupCausalServer(t)

	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	if len(chains) != 0 {
		t.Errorf("expected no chains for empty input, got: %+v", chains)
	}

	confidence, _ := m["confidence"].(string)
	if confidence != "unknown" {
		t.Errorf("expected confidence=unknown for empty input, got %q", confidence)
	}
}

// TestCausalChain_PartialMatchBelowThreshold verifies a partial match (< 50%) is not returned.
func TestCausalChain_PartialMatchBelowThreshold(t *testing.T) {
	s, _ := setupCausalServer(t)

	// etcd_disk_pressure_to_workflow_timeout has 4 steps.
	// We provide only 1 matching event (25% < 50% threshold).
	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"events": []interface{}{
			// Only matches etcd_nospace step, not the others.
			"etcd: NOSPACE alarm detected",
		},
	})
	if err != nil {
		t.Fatalf("causal_chain error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	for _, c := range chains {
		if c.RootSignal == "etcd_disk_pressure" {
			// The 4-step rule with 1/4 = 25% < 50% must not appear.
			t.Errorf("etcd_disk_pressure chain should not appear with only 1/4 steps matched: %+v", c)
		}
	}
}

// TestCausalChain_NoCausalRulesFile verifies graceful degradation when rules file is missing.
func TestCausalChain_NoCausalRulesFile(t *testing.T) {
	docsDir := t.TempDir()
	s := NewWithGraph(Config{DocsDir: docsDir}, nil)
	defer s.Close()

	result, err := s.CallTool(context.Background(), "awareness.causal_chain", map[string]interface{}{
		"events": []interface{}{"NOSPACE alarm"},
	})
	if err != nil {
		t.Fatalf("causal_chain should not error, got: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	chains, _ := m["chains"].([]causalChainResult)
	if len(chains) != 0 {
		t.Errorf("expected no chains when rules file missing, got: %+v", chains)
	}
	blindSpots, _ := m["blind_spots"].([]string)
	if len(blindSpots) == 0 {
		t.Error("expected blind_spots when rules file is missing")
	}
}
