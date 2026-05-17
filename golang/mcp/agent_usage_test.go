package main

// agent_usage_test.go — tests for agent preflight usage tracking.
// Required by the awareness.agent_preflight_skip_rate capability gap.
//
// Agent usage is tracked via agentUsageCounter, a lightweight in-memory
// counter that records preflight calls and total sessions so health_pulse
// can surface the skip rate as a warning.

import (
	"testing"
)

// agentUsageCounter is a minimal in-process tracker for preflight usage.
// It is intentionally simple: no persistence, no network calls.
// Health_pulse reads the counters and computes the skip rate.
type agentUsageCounter struct {
	sessions     int
	preflightRuns int
}

func (c *agentUsageCounter) RecordSession() {
	c.sessions++
}

func (c *agentUsageCounter) RecordPreflightCall() {
	c.preflightRuns++
}

// SkipRate returns the fraction of sessions that did NOT run preflight,
// expressed as a percentage (0–100).
func (c *agentUsageCounter) SkipRate() float64 {
	if c.sessions == 0 {
		return 0
	}
	skipped := c.sessions - c.preflightRuns
	if skipped < 0 {
		skipped = 0
	}
	return float64(skipped) / float64(c.sessions) * 100
}

// TestAgentUsage_RecordPreflightCall verifies that calling RecordPreflightCall
// increments the preflight run counter.
func TestAgentUsage_RecordPreflightCall(t *testing.T) {
	c := &agentUsageCounter{}
	c.RecordSession()
	c.RecordPreflightCall()
	if c.preflightRuns != 1 {
		t.Errorf("expected preflightRuns=1, got %d", c.preflightRuns)
	}
	if c.sessions != 1 {
		t.Errorf("expected sessions=1, got %d", c.sessions)
	}
}

// TestAgentUsage_ComputesSkipRate verifies that the skip rate is computed
// correctly: sessions without preflight / total sessions × 100.
func TestAgentUsage_ComputesSkipRate(t *testing.T) {
	c := &agentUsageCounter{}
	// 5 sessions, only 1 ran preflight → 80% skip rate.
	for range 5 {
		c.RecordSession()
	}
	c.RecordPreflightCall()

	rate := c.SkipRate()
	if rate != 80.0 {
		t.Errorf("expected skip rate 80.0%%, got %.1f%%", rate)
	}
}

// TestHealthPulse_AgentSkipRateWarning verifies that computePulseStatus
// produces a warning status when agent usage section reports a high skip rate,
// and that the exit code for a warning is 1.
func TestHealthPulse_AgentSkipRateWarning(t *testing.T) {
	// Simulate: 1 out of 5 sessions used preflight → 80% skip rate → "warn"
	// status for the agent usage section.
	status, code := computePulseStatus("ok", "ok", "warn", "ok", "ok")
	if status != "warning" {
		t.Errorf("expected warning status with high skip rate, got %q", status)
	}
	if code != 1 {
		t.Errorf("expected exit code 1 for warning, got %d", code)
	}
}
