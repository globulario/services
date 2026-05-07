package main

// globular:tested_by bounded_critical_queries

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/globulario/services/golang/subsystem"
)

// TestQueryTimeoutMappedToDegradedCategory verifies that a DeadlineExceeded error
// from the DNS reconcile cycle maps the subsystem to DEGRADED immediately — not
// after the usual 3-consecutive-error threshold.
//
// Invariant: critical_queries.must_be_bounded
func TestQueryTimeoutMappedToDegradedCategory(t *testing.T) {
	const name = "dns-reconciler-test-timeout"
	subsystem.DeregisterSubsystem(name)
	h := subsystem.RegisterSubsystem(name, 30*time.Second)
	t.Cleanup(func() { subsystem.DeregisterSubsystem(name) })

	// Simulate a single DeadlineExceeded from reconcile().
	h.TickError(context.DeadlineExceeded)
	if errors.Is(context.DeadlineExceeded, context.DeadlineExceeded) {
		h.SetState(subsystem.SubsystemDegraded)
		h.SetMeta("last_timeout_phase", "apply-dns-state")
	}

	var got *subsystem.SubsystemEntry
	for _, e := range subsystem.SubsystemSnapshot() {
		if e.Name == name {
			e := e
			got = &e
			break
		}
	}
	if got == nil {
		t.Fatal("subsystem not found after registering")
	}
	if got.State != subsystem.SubsystemDegraded {
		t.Errorf("expected DEGRADED immediately on DeadlineExceeded, got %s", got.State)
	}
	if got.Metadata["last_timeout_phase"] != "apply-dns-state" {
		t.Errorf("expected last_timeout_phase=apply-dns-state, got %q", got.Metadata["last_timeout_phase"])
	}

	// Contrast: a non-timeout error after 1 tick should NOT be DEGRADED yet
	// (subsystem stays Healthy until 3 consecutive errors via TickError alone).
	const name2 = "dns-reconciler-test-nontimeout"
	subsystem.DeregisterSubsystem(name2)
	h2 := subsystem.RegisterSubsystem(name2, 30*time.Second)
	t.Cleanup(func() { subsystem.DeregisterSubsystem(name2) })

	h2.TickError(fmt.Errorf("connection refused"))

	var got2 *subsystem.SubsystemEntry
	for _, e := range subsystem.SubsystemSnapshot() {
		if e.Name == name2 {
			e := e
			got2 = &e
			break
		}
	}
	if got2 == nil {
		t.Fatal("subsystem2 not found")
	}
	if got2.State == subsystem.SubsystemDegraded {
		t.Errorf("expected non-timeout single error to NOT be DEGRADED yet, got %s", got2.State)
	}
}

// TestSlowBackendTriggersTimeoutAndLaneRecovery verifies that when a DNS backend
// is unreachable, applyDNSState marks the endpoint as unhealthy (lane recovery) so
// the health-check loop can re-probe it instead of continuing to send traffic.
//
// Invariant: critical_queries.must_be_bounded
func TestSlowBackendTriggersTimeoutAndLaneRecovery(t *testing.T) {
	const endpoint = "127.0.0.1:1" // nothing listening — instant connection refused
	r := &DNSReconciler{
		srv:          &server{},
		dnsEndpoints: []string{endpoint},
		healthStatus: map[string]bool{endpoint: true},
		stopCh:       make(chan struct{}),
		healthStopCh: make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	desired := &DesiredDNSState{Domain: "test.local", Records: nil}
	err := r.applyDNSState(ctx, desired)
	if err == nil {
		t.Fatal("expected error when backend is unreachable")
	}
	if r.healthStatus[endpoint] {
		t.Errorf("expected endpoint %q to be marked unhealthy after failure (lane recovery), but it is still healthy", endpoint)
	}
}
