package main

// catalog_fallback.go — making an activated read fallback observable.
//
// Enforces invariant fallback.must_emit_degraded_finding: any fallback from a
// primary dependency to a local / cache / read-only path must emit a degraded
// finding. Addresses failure mode service.silent_fallback_active — "a service is
// serving requests from a substitute backend because its primary dependency
// failed", without saying so.
//
// Several query RPCs fall back to a legacy directory scan (or to an empty
// result) when their primary source errors. Before 2026-08-14 that fallback was
// invisible: ListArtifacts logged at Debug, GetArtifactVersions and
// listTrustedPublishers logged nothing at all. A caller could not distinguish
// "there are no artifacts" from "I could not read the artifacts" — the
// authority-uncertainty collapse that uncertainty-scan hunts.
//
// Note the severe case was already handled: requireCapability(CapRepoQuery)
// fails closed when ScyllaDB is down. What was missing is observability of the
// residual case — primary healthy, secondary read failed — which is precisely
// the case the invariant is about.
//
// # Why a subsystem tick and not a new RepositoryFinding kind
//
// The repository must not compute its own cross-layer health verdict; see
// forbidden_fix bypass_doctor_for_cross_layer_health_finding. The service
// reports its own subsystem state, cluster-doctor observes that state and
// produces the finding. Minting a REPO_FIND_* kind here would move a health
// verdict into the wrong actor and duplicate the doctor's authority.
//
// # Why the state self-clears
//
// TickError degrades after three consecutive fallbacks — a real condition —
// rather than on a single blip, and a later primary-path success calls Tick,
// resetting the counter. A latching degraded flag would be worse than silence:
// it would make the signal untrustworthy and train operators to ignore it,
// which is how a monitoring surface dies.

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/globulario/services/golang/subsystem"
)

const catalogFallbackSubsystem = "repo:catalog-fallback"

var (
	catalogFallbackOnce   sync.Once
	catalogFallbackHandle *subsystem.SubsystemHandle
)

func catalogFallback() *subsystem.SubsystemHandle {
	catalogFallbackOnce.Do(func() {
		// register is idempotent; the interval is advisory for staleness only.
		catalogFallbackHandle = subsystem.RegisterSubsystem(catalogFallbackSubsystem, 5*time.Minute)
	})
	return catalogFallbackHandle
}

// noteCatalogFallback records that a read path could not use its primary source
// and served a degraded answer instead.
//
// surface names the RPC or reader, so an operator learns WHICH answer was
// degraded rather than merely that something was. The result returned to the
// caller is deliberately unchanged: this makes the fallback observable, it does
// not change the read semantics.
func noteCatalogFallback(surface string, err error) {
	h := catalogFallback()
	h.SetMeta("last_surface", surface)
	h.TickError(fmt.Errorf("%s served a fallback result: %w", surface, err))
	slog.Warn("catalog fallback active — result may be incomplete",
		"surface", surface,
		"err", err,
		"invariant", "fallback.must_emit_degraded_finding")
}

// noteCatalogPrimary records that a read path served from its primary source.
// This is what makes the degraded state self-clearing; without it the subsystem
// would latch on the first transient failure and never recover.
func noteCatalogPrimary() {
	catalogFallback().Tick()
}
