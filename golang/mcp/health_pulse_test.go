package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestHealthPulse_Healthy verifies that a fully configured server with no
// stale proposals and no missing coverage produces a healthy status and exit code 0.
func TestHealthPulse_Healthy(t *testing.T) {
	status, code := computePulseStatus("ok", "ok", "ok", "ok", "ok")
	if status != "healthy" {
		t.Errorf("expected healthy, got %q", status)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
}

// TestHealthPulse_StaleProposalWarning verifies that a stale proposal in the
// queue produces a warning status, an alert, and exit code 1.
func TestHealthPulse_StaleProposalWarning(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := `proposal:
  id: stale-001
  status: DRAFT
  created_at: "2000-01-01T00:00:00Z"
`
	if err := os.WriteFile(filepath.Join(proposalsDir, "stale-001.yaml"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	section, alerts := buildQueueSection(docsDir, 24.0)
	if section.Status != "warn" {
		t.Errorf("expected warn, got %q", section.Status)
	}
	if len(alerts) == 0 {
		t.Error("expected at least one alert for stale proposal")
	}
	if alerts[0].Severity != "warning" {
		t.Errorf("expected warning severity, got %q", alerts[0].Severity)
	}
}

// TestHealthPulse_RuntimeNoopWarning verifies that an empty repoRoot (no
// runtime_sources.yaml) produces a noop warning.
func TestHealthPulse_RuntimeNoopWarning(t *testing.T) {
	section, alerts := buildRuntimeSection(t.TempDir())
	if section.Status != "warn" {
		t.Errorf("expected warn for noop config, got %q", section.Status)
	}
	if section.RuntimeAwarenessStatus != "noop" {
		t.Errorf("expected noop, got %q", section.RuntimeAwarenessStatus)
	}
	found := false
	for _, a := range alerts {
		if a.ID == "runtime.noop" {
			found = true
		}
	}
	if !found {
		t.Error("expected runtime.noop alert")
	}
}

// TestHealthPulse_GraphStaleCritical verifies that computePulseStatus returns
// critical and exit code 2 when the graph section is critical.
func TestHealthPulse_GraphStaleCritical(t *testing.T) {
	status, code := computePulseStatus("ok", "ok", "ok", "critical", "ok")
	if status != "critical" {
		t.Errorf("expected critical, got %q", status)
	}
	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
}

// TestHealthPulse_ExitCodes verifies the full exit code mapping:
// healthy=0, warning=1, critical=2.
func TestHealthPulse_ExitCodes(t *testing.T) {
	cases := []struct {
		statuses []string
		wantCode int
	}{
		{[]string{"ok", "ok", "ok"}, 0},
		{[]string{"ok", "warn", "ok"}, 1},
		{[]string{"ok", "critical", "ok"}, 2},
		{[]string{"critical", "warn", "ok"}, 2},
	}
	for _, c := range cases {
		_, code := computePulseStatus(c.statuses...)
		if code != c.wantCode {
			t.Errorf("statuses=%v: expected code %d, got %d", c.statuses, c.wantCode, code)
		}
	}
}

// TestHealthPulse_IncludesProposalQueueHealth verifies that the proposal queue
// section is populated in health_pulse output.
func TestHealthPulse_IncludesProposalQueueHealth(t *testing.T) {
	docsDir := t.TempDir()
	proposalsDir := filepath.Join(docsDir, "proposals")
	if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A healthy queue (no proposals).
	section, alerts := buildQueueSection(docsDir, 24.0)
	if section.Status == "" {
		t.Error("proposal queue section status must not be empty")
	}
	// No stale proposals → no alerts.
	for _, a := range alerts {
		if a.ID == "proposal_queue.stale" {
			t.Errorf("unexpected stale proposal alert on empty queue")
		}
	}
}
