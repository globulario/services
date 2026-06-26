package main

import (
	"context"
	"fmt"
	"log"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// ensureServiceRelease creates or updates a ServiceRelease object for the given
// service so that the release reconciler can track per-service lifecycle phases.
// Idempotent: if a ServiceRelease already exists with the same version, build
// number, and publisher, it is left unchanged.
//
// publisherID overrides the artifact publisher used by the release resolver.
// Empty string means use the default official publisher (core@globular.io).
// Set to a non-official publisher (e.g. local@ryzen) when a local override is
// active — the resolver will look up the artifact under the correct identity lane.
// The ServiceRelease KEY always uses defaultPublisherID() so there is never more
// than one release record per service regardless of override state.
func (srv *server) ensureServiceRelease(ctx context.Context, serviceName, publisherID, version string, buildNumber int64) {
	if !srv.mustBeLeader() {
		return
	}
	if srv.resources == nil {
		return
	}
	canon := canonicalServiceName(serviceName)
	if canon == "" || version == "" {
		return
	}

	effectivePublisher := publisherID
	if effectivePublisher == "" {
		effectivePublisher = defaultPublisherID()
	}

	// Release key always uses the official publisher prefix so there is exactly
	// one ServiceRelease per service, regardless of override state.
	releaseName := defaultPublisherID() + "/" + canon

	// Check for existing release — skip if version+build+publisher match and not being removed.
	// If the release is in a removal state (Removing flag, REMOVING, or REMOVED phase),
	// recreate it so the install workflow can proceed.
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", releaseName)
	if err == nil && obj != nil {
		if existing, ok := obj.(*cluster_controllerpb.ServiceRelease); ok && existing.Spec != nil {
			needsRecreate := existing.Spec.Removing
			existingPhase := ""
			if existing.Status != nil {
				existingPhase = existing.Status.Phase
				needsRecreate = needsRecreate ||
					existingPhase == ReleasePhaseRemoving || existingPhase == ReleasePhaseRemoved
				// Only recreate FAILED/ROLLED_BACK releases if the desired version
				// actually changed. Otherwise, respect the 5-minute backoff in the
				// reconciler — the bridge must not reset FAILED releases, which
				// causes a tight FAILED→PENDING→FAILED loop.
				if (existingPhase == cluster_controllerpb.ReleasePhaseFailed ||
					existingPhase == cluster_controllerpb.ReleasePhaseRolledBack) &&
					existing.Spec.Version != version {
					needsRecreate = true
				}
			}
			existingPublisher := existing.Spec.PublisherID
			if existingPublisher == "" {
				existingPublisher = defaultPublisherID()
			}
			if !needsRecreate && existing.Spec.Version == version &&
				existing.Spec.BuildNumber == buildNumber &&
				existingPublisher == effectivePublisher {
				return // already up-to-date and in a healthy state
			}
			// If the release is FAILED/ROLLED_BACK but version+publisher haven't changed,
			// let the reconciler handle retry via backoff — don't recreate.
			if !needsRecreate && (existingPhase == cluster_controllerpb.ReleasePhaseFailed ||
				existingPhase == cluster_controllerpb.ReleasePhaseRolledBack) {
				return
			}
			log.Printf("ensureServiceRelease: %s: recreating (phase=%s removing=%v needsRecreate=%v publisher=%s→%s)",
				releaseName, existingPhase, existing.Spec.Removing, needsRecreate, existingPublisher, effectivePublisher)
		}
	} else {
		log.Printf("ensureServiceRelease: %s: no existing release, creating (version=%s publisher=%s)",
			releaseName, version, effectivePublisher)
	}

	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: releaseName},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: effectivePublisher,
			ServiceName: canon,
			Version:     version,
			BuildNumber: buildNumber,
			Platform:    "", // resolved per-node by the reconciler
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase: cluster_controllerpb.ReleasePhasePending,
		},
	}

	if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
		log.Printf("ensureServiceRelease: %s: apply failed: %v", releaseName, err)
	} else {
		log.Printf("ensureServiceRelease: %s: created with phase=PENDING publisher=%s", releaseName, effectivePublisher)
	}
}

// ensureServiceReleasesFromDesired scans all ServiceDesiredVersion objects and
// creates corresponding ServiceRelease objects for any that are missing.
// Safe to call periodically — only creates releases, does not clean up infra.
func (srv *server) ensureServiceReleasesFromDesired(ctx context.Context) {
	if !srv.mustBeLeader() {
		return
	}
	if srv.resources == nil {
		return
	}
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		log.Printf("ensureServiceReleasesFromDesired: list: %v", err)
		return
	}
	// Build a set of names managed by InfrastructureRelease so we don't
	// create duplicate ServiceRelease objects for infrastructure packages.
	infraManaged := make(map[string]bool)
	if infraItems, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		for _, obj := range infraItems {
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
				infraManaged[canonicalServiceName(rel.Spec.Component)] = true
			}
		}
	}

	created := 0
	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" || sdv.Spec.Version == "" {
			continue
		}
		// Skip infrastructure packages — they are managed by InfrastructureRelease,
		// not ServiceRelease. Creating a ServiceRelease for them causes resolution
		// failures (wrong artifact kind) and stale "Planned" entries in the UI.
		if infraManaged[canon] {
			continue
		}
		srv.ensureServiceRelease(ctx, canon, sdv.Spec.PublisherID, sdv.Spec.Version, sdv.Spec.BuildNumber)
		created++
	}
	if created > 0 {
		log.Printf("ensureServiceReleasesFromDesired: processed %d desired entries", created)
	}

	// Prune any legacy pre-guard cross-kind ServiceDesiredVersion pollution
	// (an infrastructure-owned name carrying a service-desired record). Running
	// it here gives both startup and periodic coverage, so a backup-restore that
	// reintroduces a stale cross-kind record is cleaned on the next pass.
	srv.cleanupLegacyCrossKindDesiredState(ctx)

	// Re-enqueue releases stuck in RESOLVED: no watch event fires when a
	// release's status doesn't change, so periodic re-reconcile is the only
	// retry path. APPLYING releases are owned by an executing workflow and
	// are driven by workflow callbacks (or the run reaper on crash).
	srv.retryStuckReleases(ctx)
}

// retryStuckReleases finds ServiceRelease and InfrastructureRelease objects
// stuck in RESOLVED and re-enqueues them through the work queue so the
// workflow path picks them up again. Unlike the previous implementation, this
// does NOT call reconcileRelease directly — doing so bypassed the work queue's
// dedup and rate limiting, amplifying the reconcile storm.
//
// InfrastructureRelease coverage is required because their "retry" patch was a
// no-op (missing case in applyPatchToInfraStatus) which stopped the
// watch-driven retry loop — making this periodic safety net the only
// path back when the watcher loop stalls.
func (srv *server) retryStuckReleases(ctx context.Context) {
	if srv.resources == nil || srv.releaseEnqueue == nil {
		return
	}
	releases, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		return
	}
	for _, obj := range releases {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok || rel.Status == nil || rel.Meta == nil {
			continue
		}
		if rel.Status.Phase == cluster_controllerpb.ReleasePhaseResolved {
			srv.releaseEnqueue(rel.Meta.Name)
		}
	}

	if srv.infraReleaseEnqueue == nil {
		return
	}
	infraReleases, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		return
	}
	for _, obj := range infraReleases {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel.Status == nil || rel.Meta == nil {
			continue
		}
		if rel.Status.Phase == cluster_controllerpb.ReleasePhaseResolved {
			srv.infraReleaseEnqueue(rel.Meta.Name)
		}
	}
}

// pruneCrossKindServiceDesired removes ServiceDesiredVersion entries whose name
// is owned by an InfrastructureRelease — legacy PRE-GUARD cross-kind writes.
// Returns the canonical names removed. Store-pure so it is unit-testable with a
// MemStore.
//
// SAFETY (intent:delete_requires_explicit_intent_marker): the deletion criterion
// is explicit and narrow — a ServiceDesiredVersion is removed ONLY when its
// canonical name matches a component that currently has an InfrastructureRelease
// (the authoritative owner). Valid service-desired state is NEVER touched. It
// removes only the invalid desired record; it does NOT drive a removal workflow
// or uninstall (the InfrastructureRelease stays the sole authority). If no
// InfrastructureRelease exists at all it deletes nothing (fail-safe: ownership
// cannot be classified). This replaces the old global cleanupStaleInfraServiceReleases,
// which was disabled for deleting desired state joining nodes still needed.
func pruneCrossKindServiceDesired(ctx context.Context, store resourcestore.Store) ([]string, error) {
	infraItems, _, err := store.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		return nil, fmt.Errorf("list InfrastructureRelease: %w", err)
	}
	infraOwned := make(map[string]bool)
	for _, obj := range infraItems {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel.Spec == nil {
			continue
		}
		name := canonicalServiceName(rel.Spec.Component)
		if name == "" && rel.Meta != nil {
			name = canonicalServiceName(infraReleaseNameFromKey(rel.Meta.Name))
		}
		if name != "" {
			infraOwned[name] = true
		}
	}
	if len(infraOwned) == 0 {
		return nil, nil // cannot classify ownership — delete nothing (fail-safe)
	}

	sdvItems, _, err := store.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return nil, fmt.Errorf("list ServiceDesiredVersion: %w", err)
	}
	var removed []string
	for _, obj := range sdvItems {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" && sdv.Meta != nil {
			canon = canonicalServiceName(sdv.Meta.Name)
		}
		if canon == "" || !infraOwned[canon] {
			continue // valid service-desired state — never touch
		}
		if err := store.Delete(ctx, "ServiceDesiredVersion", canon); err != nil {
			log.Printf("cleanup-cross-kind: delete ServiceDesiredVersion %q: %v", canon, err)
			continue
		}
		removed = append(removed, canon)
	}
	return removed, nil
}

// cleanupLegacyCrossKindDesiredState removes legacy pre-guard cross-kind
// ServiceDesiredVersion entries for infrastructure-owned packages (e.g. a stale
// xds@1.2.235 service-desired record left over from before the cross-kind guard
// existed). Such records are invalid authority: infrastructure is not a service,
// so a ServiceDesiredVersion for it poisons the node-agent's desired-version
// drift check (I2) and re-stages stale tarballs. The cross-kind guard
// (rejectCrossKindDesiredWrite) prevents CREATING new ones; this is the audited
// CLEANUP path for pre-guard pollution. Runs wherever ensureServiceReleasesFromDesired
// runs (startup + periodic), so a backup-restore that reintroduces a stale
// cross-kind record is cleaned on the next pass.
func (srv *server) cleanupLegacyCrossKindDesiredState(ctx context.Context) int {
	if !srv.mustBeLeader() || srv.resources == nil {
		return 0
	}
	removed, err := pruneCrossKindServiceDesired(ctx, srv.resources)
	if err != nil {
		log.Printf("cleanup-cross-kind: %v", err)
		return 0
	}
	for _, name := range removed {
		log.Printf("cleanup-cross-kind: removed legacy cross-kind ServiceDesiredVersion %q (owned by InfrastructureRelease)", name)
	}
	if len(removed) > 0 {
		log.Printf("cleanup-cross-kind: removed %d legacy cross-kind ServiceDesiredVersion entry(ies)", len(removed))
	}
	return len(removed)
}
