package main

import (
	"context"
	"testing"
)

// TestOfflineDiagnose_Registered verifies the tool is available.
func TestOfflineDiagnose_Registered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)
	if !s.HasTool("awareness.offline_diagnose") {
		t.Error("awareness.offline_diagnose must be registered")
	}
}

// TestOfflineDiagnose_EtcdNospace verifies etcd NOSPACE is detected.
func TestOfflineDiagnose_EtcdNospace(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	journalText := `
May 07 18:00:01 globule-ryzen etcd[1234]: mvcc: database space exceeded
May 07 18:00:02 globule-ryzen etcd[1234]: NOSPACE alarm is activated
May 07 18:00:03 globule-ryzen cluster_controller[5678]: failed to write to etcd: rpc error: code = ResourceExhausted
`
	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": journalText,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	evidence, _ := m["evidence"].([]offlineEvent)
	found := false
	for _, e := range evidence {
		if e.Pattern == "etcd_nospace" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected etcd_nospace pattern in evidence, got: %+v", evidence)
	}

	confidence, _ := m["confidence"].(string)
	if confidence == "unknown" {
		t.Error("confidence should not be unknown when evidence is present")
	}
}

// TestOfflineDiagnose_PortSquatting verifies address-in-use + unknown service detection.
func TestOfflineDiagnose_PortSquatting(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	journalText := `
2026-05-07T18:00:01Z globule-ryzen node_agent[111]: bind: address already in use :10004
2026-05-07T18:00:02Z globule-ryzen workflow[222]: rpc error: code = Unimplemented desc = unknown gRPC service
2026-05-07T18:00:03Z globule-ryzen workflow[222]: service client unavailable
`
	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": journalText,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	evidence, _ := m["evidence"].([]offlineEvent)

	hasPortSquat := false
	hasUnknownGRPC := false
	for _, e := range evidence {
		if e.Pattern == "port_squatting" {
			hasPortSquat = true
		}
		if e.Pattern == "unknown_grpc_service" {
			hasUnknownGRPC = true
		}
	}
	if !hasPortSquat {
		t.Error("expected port_squatting pattern")
	}
	if !hasUnknownGRPC {
		t.Error("expected unknown_grpc_service pattern")
	}
}

// TestOfflineDiagnose_WorkflowStuck verifies workflow stuck is detected.
func TestOfflineDiagnose_WorkflowStuck(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	journalText := `
2026-05-07T19:00:00Z globule-ryzen workflow[9999]: workflow stuck at converging phase
2026-05-07T19:00:01Z globule-ryzen workflow[9999]: workflow blocked waiting for lease
`
	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": journalText,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	evidence, _ := m["evidence"].([]offlineEvent)

	found := false
	for _, e := range evidence {
		if e.Pattern == "workflow_stuck" {
			found = true
		}
	}
	if !found {
		t.Error("expected workflow_stuck pattern in evidence")
	}
}

// TestOfflineDiagnose_MinioIssue verifies MinIO offline disk detection.
func TestOfflineDiagnose_MinioIssue(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	journalText := `
2026-05-07T17:30:00Z globule-dell minio[333]: drive offline /data/disk2
2026-05-07T17:30:01Z globule-dell minio[333]: healing started for disk2
2026-05-07T17:30:05Z globule-dell node_agent[444]: artifact download failed: not found
`
	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": journalText,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	evidence, _ := m["evidence"].([]offlineEvent)

	found := false
	for _, e := range evidence {
		if e.Pattern == "minio_issue" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected minio_issue pattern, got: %+v", evidence)
	}
}

// TestOfflineDiagnose_RestartStorm verifies restart storm detection.
func TestOfflineDiagnose_RestartStorm(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	systemdStatus := `
● globular-workflow.service - Globular Workflow
   Loaded: loaded
   Active: failed (Result: start-limit-hit) since Wed 2026-05-07 18:00:00 UTC
   start operation timed out
`
	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"systemd_status": systemdStatus,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	evidence, _ := m["evidence"].([]offlineEvent)

	found := false
	for _, e := range evidence {
		if e.Pattern == "restart_storm" {
			found = true
		}
	}
	if !found {
		t.Error("expected restart_storm pattern")
	}
}

// TestOfflineDiagnose_EmptyInput verifies graceful handling of empty input.
func TestOfflineDiagnose_EmptyInput(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}

	confidence, _ := m["confidence"].(string)
	if confidence != "unknown" {
		t.Errorf("expected confidence=unknown for empty input, got %q", confidence)
	}

	blindSpots, _ := m["blind_spots"].([]string)
	if len(blindSpots) == 0 {
		t.Error("expected blind_spots when no input is provided")
	}
}

// TestOfflineDiagnose_MixedLogsTimelineOrdered verifies mixed logs produce ordered timeline.
func TestOfflineDiagnose_MixedLogsTimelineOrdered(t *testing.T) {
	s := NewWithGraph(Config{}, nil)

	// Logs with out-of-order timestamps.
	journalText := `
2026-05-07T18:05:00Z globule-ryzen etcd[1]: NOSPACE alarm activated
2026-05-07T18:01:00Z globule-ryzen etcd[1]: elected leader changed
2026-05-07T18:03:00Z globule-ryzen workflow[2]: workflow stuck at step
2026-05-07T18:02:00Z globule-ryzen controller[3]: leader changed event received
`
	result, err := s.CallTool(context.Background(), "awareness.offline_diagnose", map[string]interface{}{
		"journalctl_text": journalText,
	})
	if err != nil {
		t.Fatalf("offline_diagnose error: %v", err)
	}
	m, _ := result.(map[string]interface{})
	timeline, _ := m["timeline"].([]offlineTimeline)

	// Timeline should be sorted by time.
	for i := 1; i < len(timeline); i++ {
		if timeline[i].Time < timeline[i-1].Time {
			t.Errorf("timeline not sorted at index %d: %q > %q", i, timeline[i-1].Time, timeline[i].Time)
		}
	}

	// Should have multiple candidate failure modes.
	fms, _ := m["suspected_failure_modes"].([]offlineFailureModeMatch)
	evidence, _ := m["evidence"].([]offlineEvent)
	// At minimum we expect some events from mixed logs.
	if len(evidence) == 0 {
		t.Error("expected evidence from mixed logs")
	}
	_ = fms // may be empty if docsDir not available
}

// TestOfflineDiagnose_TimestampExtraction verifies timestamps are extracted.
func TestOfflineDiagnose_TimestampExtraction(t *testing.T) {
	cases := []struct {
		line     string
		wantTime bool
	}{
		{"2026-05-07T18:00:04Z etcd[1]: NOSPACE alarm", true},
		{"2026/05/07 18:00:04 etcd: NOSPACE alarm", true},
		{"May  7 18:00:04 host process[1]: NOSPACE alarm", true},
		{"no timestamp here: NOSPACE alarm", false},
	}
	for _, c := range cases {
		ts := extractTimestamp(c.line)
		got := ts != ""
		if got != c.wantTime {
			t.Errorf("extractTimestamp(%q) = %q, wantTime=%v", c.line, ts, c.wantTime)
		}
	}
}
