package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestHoldsActive_RejectsUnitThatDiesDuringSettle pins the exact distinction the
// repair path could not previously make.
//
// Observed 2026-08-30 (authority suite, node-5): etcd reported active at
// 10:52:20, the repair logged success at 10:52:20, and etcd was dead at
// 10:52:25. supervisor.WaitActive returns on the FIRST active reading, so it
// called that a repair. A unit that becomes active and then exits during the
// settle window must be reported as a failure instead.
func TestHoldsActive_RejectsUnitThatDiesDuringSettle(t *testing.T) {
	calls := 0
	isActive := func(context.Context, string) (bool, error) {
		calls++
		return calls < 2, nil // active once, then gone — the etcd shape
	}

	err := holdsActive(context.Background(), "globular-etcd.service", 3*time.Second, isActive)
	if err == nil {
		t.Fatal("a unit that went inactive during the settle window must not be reported as repaired")
	}
	if !strings.Contains(err.Error(), "start is not proof of repair") {
		t.Errorf("the error should say why the claim was refused, got: %v", err)
	}
}

// TestHoldsActive_AcceptsUnitThatStaysActive is the negative control. Without
// it, "always return an error" would satisfy the test above — the check has to
// still accept a genuine repair.
func TestHoldsActive_AcceptsUnitThatStaysActive(t *testing.T) {
	isActive := func(context.Context, string) (bool, error) { return true, nil }
	if err := holdsActive(context.Background(), "globular-etcd.service", 2*time.Second, isActive); err != nil {
		t.Fatalf("a unit that stayed active must pass, got: %v", err)
	}
}

// TestHoldsActive_PropagatesProbeError — an unreadable unit state is UNKNOWN,
// not healthy. It must surface rather than be read as "still active".
func TestHoldsActive_PropagatesProbeError(t *testing.T) {
	want := errors.New("systemctl unavailable")
	isActive := func(context.Context, string) (bool, error) { return false, want }
	if err := holdsActive(context.Background(), "u.service", 2*time.Second, isActive); !errors.Is(err, want) {
		t.Fatalf("probe error must propagate, got: %v", err)
	}
}

// TestRepairSettleWindow_CoversObservedExit guards the constant: the window has
// to outlast the failure it was introduced to catch, or the check is decorative.
func TestRepairSettleWindow_CoversObservedExit(t *testing.T) {
	const observedEtcdExit = 5 * time.Second
	if repairSettleWindow <= observedEtcdExit {
		t.Fatalf("repairSettleWindow %s must exceed the observed %s etcd exit it exists to catch",
			repairSettleWindow, observedEtcdExit)
	}
}
