package main

import (
	"errors"
	"testing"

	"github.com/globulario/services/golang/subsystem"
)

func catalogFallbackState(t *testing.T) subsystem.SubsystemState {
	t.Helper()
	for _, e := range subsystem.SubsystemSnapshot() {
		if e.Name == catalogFallbackSubsystem {
			return e.State
		}
	}
	t.Fatalf("subsystem %q not registered", catalogFallbackSubsystem)
	return subsystem.SubsystemHealthy
}

// The whole point of this mechanism is that an activated fallback becomes
// visible in an authoritative surface. If the subsystem never leaves healthy,
// the code is decoration — the "declared but not wired" failure that bit this
// session twice (a CLI flag registered but never assigned, and a scanner pragma
// whose match window was one line too narrow). Both looked done and did nothing.
func TestCatalogFallback_DegradesAfterRepeatedFallbacks(t *testing.T) {
	noteCatalogPrimary() // reset state left by another test in this package

	if got := catalogFallbackState(t); got == subsystem.SubsystemDegraded {
		t.Fatalf("precondition: expected non-degraded, got %v", got)
	}

	// TickError degrades at three consecutive errors — a real condition, not a
	// single blip.
	for i := 0; i < 3; i++ {
		noteCatalogFallback("test.surface", errors.New("storage unreachable"))
	}

	if got := catalogFallbackState(t); got != subsystem.SubsystemDegraded {
		t.Fatalf("after 3 fallbacks: state = %v, want SubsystemDegraded — "+
			"an activated fallback that never surfaces is exactly the defect being fixed", got)
	}
}

// A latching degraded flag would be worse than silence: operators learn to
// ignore a signal that never clears, and then it is worthless when it matters.
func TestCatalogFallback_SelfClearsOnPrimarySuccess(t *testing.T) {
	for i := 0; i < 3; i++ {
		noteCatalogFallback("test.surface", errors.New("storage unreachable"))
	}
	if got := catalogFallbackState(t); got != subsystem.SubsystemDegraded {
		t.Fatalf("precondition: want degraded, got %v", got)
	}

	noteCatalogPrimary()

	if got := catalogFallbackState(t); got == subsystem.SubsystemDegraded {
		t.Error("still degraded after a primary-path success — the signal latches and becomes untrustworthy")
	}
}

// A single transient blip must NOT degrade, or the surface becomes noise and
// the finding stops meaning anything.
func TestCatalogFallback_SingleBlipDoesNotDegrade(t *testing.T) {
	noteCatalogPrimary()
	noteCatalogFallback("test.surface", errors.New("one transient failure"))

	if got := catalogFallbackState(t); got == subsystem.SubsystemDegraded {
		t.Error("a single fallback degraded the subsystem; the 3-strike threshold is what keeps this signal meaningful")
	}
}
