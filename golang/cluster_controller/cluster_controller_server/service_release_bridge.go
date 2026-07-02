package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

func targetNodeAssignments(targetNodeIDs []string) []*cluster_controllerpb.NodeAssignment {
	targetNodeIDs = normalizeTargetNodeIDs(targetNodeIDs)
	if len(targetNodeIDs) == 0 {
		return nil
	}
	out := make([]*cluster_controllerpb.NodeAssignment, 0, len(targetNodeIDs))
	for _, id := range targetNodeIDs {
		out = append(out, &cluster_controllerpb.NodeAssignment{NodeID: id})
	}
	return out
}

func releaseTargetNodeIDs(assignments []*cluster_controllerpb.NodeAssignment) []string {
	if len(assignments) == 0 {
		return nil
	}
	ids := make([]string, 0, len(assignments))
	for _, a := range assignments {
		if a == nil {
			continue
		}
		ids = append(ids, a.NodeID)
	}
	return normalizeTargetNodeIDs(ids)
}

func sameTargetNodeIDs(a, b []string) bool {
	a = normalizeTargetNodeIDs(a)
	b = normalizeTargetNodeIDs(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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
func (srv *server) ensureServiceRelease(ctx context.Context, serviceName, publisherID, version string, buildNumber int64, targetNodeIDs []string, allowDowngrade ...bool) {
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

	targetNodeIDs = normalizeTargetNodeIDs(targetNodeIDs)
	downgradeAllowed := len(allowDowngrade) > 0 && allowDowngrade[0]
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
				existing.Spec.AllowDowngrade == downgradeAllowed &&
				existingPublisher == effectivePublisher &&
				sameTargetNodeIDs(releaseTargetNodeIDs(existing.Spec.NodeAssignments), targetNodeIDs) {
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
			PublisherID:     effectivePublisher,
			ServiceName:     canon,
			Version:         version,
			BuildNumber:     buildNumber,
			AllowDowngrade:  downgradeAllowed,
			NodeAssignments: targetNodeAssignments(targetNodeIDs),
			Platform:        "", // resolved per-node by the reconciler
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
		// Skip non-service packages — INFRASTRUCTURE/COMMAND are not managed by
		// ServiceRelease. Creating a ServiceRelease for them causes resolution
		// failures (wrong artifact kind) and stale "Planned" entries in the UI.
		//
		// Kind comes from the component catalog — the SAME single oracle the write
		// guard (rejectCrossKindDesiredWrite) uses — NOT a second "does an
		// InfrastructureRelease object exist" proxy. The proxy diverged from the
		// write guard and went inert before/without an InfrastructureRelease (the
		// bootstrap/join/restore window), which is where the legacy cross-kind
		// xds ServiceRelease survived. The catalog is build-gated and always
		// present, so classification is deterministic and ordering-independent.
		if nameIsNonServiceCatalogKind(canon) {
			// Seam 2: skipping CREATE is not enough. A pre-existing cross-kind
			// ServiceRelease for this name (e.g. a legacy xds@1.2.235 left from
			// before the cross-kind write guard) persists and keeps driving
			// SERVICE-kind install dispatches from a stale pinned tarball.
			// Actively reconcile-delete it. pruneCrossKindServiceReleases (below)
			// is the periodic backstop for names that no longer have any
			// ServiceDesiredVersion at all (the xds incident — its SDV was already
			// cleaned, but the ServiceRelease faucet stayed open).
			srv.deleteCrossKindServiceRelease(ctx, canon)
			continue
		}
		srv.ensureServiceRelease(ctx, canon, sdv.Spec.PublisherID, sdv.Spec.Version, sdv.Spec.BuildNumber, sdv.Spec.TargetNodeIDs, sdv.Spec.AllowDowngrade)
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

// nameIsNonServiceCatalogKind reports whether the canonical name is a known
// INFRASTRUCTURE or COMMAND package per the component catalog — the single,
// build-gated, canonical kind authority. This is the SAME oracle the write guard
// (rejectCrossKindDesiredWrite) uses, so the cleanup classifies kind identically
// to the path that prevents the write. A name absent from the catalog is treated
// as a service (fail-open), so third-party services are never pruned.
//
// This replaces the prior ownership proxy ("does an InfrastructureRelease object
// exist for this name"). That proxy was a SECOND, divergent kind oracle: it went
// inert when no InfrastructureRelease existed yet (bootstrap / join / backup-
// restore ordering) — precisely the window in which the legacy cross-kind xds
// ServiceRelease survived every cleanup. The catalog is deterministic and
// ordering-independent: if it says xds is INFRASTRUCTURE, a SERVICE record for
// xds is invalid whether or not an InfrastructureRelease has been created yet
// (Prime Rule 4 — never duplicate package-kind classification).
func nameIsNonServiceCatalogKind(canon string) bool {
	comp := CatalogByName(canon)
	return comp != nil && (comp.Kind == KindInfrastructure || comp.Kind == KindCommand)
}

// pruneCrossKindServiceDesired removes ServiceDesiredVersion entries for a name
// the component catalog classifies as INFRASTRUCTURE or COMMAND — legacy
// PRE-GUARD cross-kind writes. Returns the canonical names removed. Store-pure so
// it is unit-testable with a MemStore.
//
// SAFETY (intent:delete_requires_explicit_intent_marker): the deletion criterion
// is explicit and narrow — a ServiceDesiredVersion is removed ONLY when the
// catalog says the name is non-service (a SERVICE record for it is definitionally
// invalid). Valid service-desired state — and any name not in the catalog
// (third-party) — is NEVER touched. It removes only the invalid desired record;
// it does NOT drive a removal workflow or uninstall (the InfrastructureRelease
// remains the sole authority and reconverges the package).
func pruneCrossKindServiceDesired(ctx context.Context, store resourcestore.Store) ([]string, error) {
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
		if canon == "" || !nameIsNonServiceCatalogKind(canon) {
			continue // service-kind (or third-party not in catalog) — never touch
		}
		if err := store.Delete(ctx, "ServiceDesiredVersion", canon); err != nil {
			log.Printf("cleanup-cross-kind: delete ServiceDesiredVersion %q: %v", canon, err)
			continue
		}
		removed = append(removed, canon)
	}
	return removed, nil
}

// pruneCrossKindServiceReleases removes ServiceRelease objects for a name the
// component catalog classifies as INFRASTRUCTURE or COMMAND — the cross-kind
// faucet that survives the ServiceDesiredVersion cleanup.
//
// pruneCrossKindServiceDesired closes the *desired* faucet, but
// ensureServiceReleasesFromDesired "only creates releases, does not clean up",
// so a legacy SERVICE-kind ServiceRelease for an infrastructure name (e.g.
// xds@1.2.235) keeps driving SERVICE-kind install dispatches from a stale pinned
// tarball long after its ServiceDesiredVersion is gone. Each dispatch reinstalls
// the wrong-kind/old artifact, and node-agent's disk-truth cleanup then keeps
// the (now disk-true) SERVICE record and discards the canonical INFRASTRUCTURE
// record — the recurring xds cache_digest_mismatch. Closing the desired faucet
// (PR #154) without closing this release faucet is "patching the drain while the
// pipe still pours".
//
// SAFETY mirrors pruneCrossKindServiceDesired (intent:delete_requires_explicit_intent_marker):
// a ServiceRelease is removed ONLY when the catalog says the name is non-service
// (a SERVICE record for it is definitionally invalid). Valid service releases —
// and any name not in the catalog (third-party) — are NEVER touched. It removes
// only the invalid release record; it does NOT drive an uninstall — the
// InfrastructureRelease remains the sole authority and reconverges the package.
// Store-pure so it is unit-testable with a MemStore.
func pruneCrossKindServiceReleases(ctx context.Context, store resourcestore.Store) ([]string, error) {
	relItems, _, err := store.List(ctx, "ServiceRelease", "")
	if err != nil {
		return nil, fmt.Errorf("list ServiceRelease: %w", err)
	}
	var removed []string
	for _, obj := range relItems {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok {
			continue
		}
		canon := ""
		if rel.Spec != nil {
			canon = canonicalServiceName(rel.Spec.ServiceName)
		}
		if canon == "" && rel.Meta != nil {
			canon = canonicalServiceName(serviceReleaseNameFromKey(rel.Meta.Name))
		}
		if canon == "" || !nameIsNonServiceCatalogKind(canon) {
			continue // service-kind (or third-party not in catalog) — never touch
		}
		// Delete by the actual stored key (Meta.Name, e.g. "core@globular.io/xds")
		// so a key-shape mismatch can never silently no-op the delete.
		key := canon
		if rel.Meta != nil && rel.Meta.Name != "" {
			key = rel.Meta.Name
		}
		if err := store.Delete(ctx, "ServiceRelease", key); err != nil {
			log.Printf("cleanup-cross-kind: delete ServiceRelease %q: %v", key, err)
			continue
		}
		removed = append(removed, canon)
	}
	return removed, nil
}

// serviceReleaseNameFromKey extracts the canonical service name from a
// ServiceRelease meta key of the form "<publisher>/<name>" (e.g.
// "core@globular.io/xds" -> "xds"). Falls back to the whole key when it carries
// no publisher prefix.
func serviceReleaseNameFromKey(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 && i+1 < len(key) {
		return key[i+1:]
	}
	return key
}

// deleteCrossKindServiceRelease removes the ServiceRelease for a single
// infrastructure-owned name (seam 2, create-path). ensureServiceReleasesFromDesired
// skips CREATING releases for infraManaged names, but a pre-existing one must be
// actively removed — otherwise it persists and drives SERVICE-kind installs.
func (srv *server) deleteCrossKindServiceRelease(ctx context.Context, canon string) {
	if canon == "" || srv.resources == nil {
		return
	}
	key := defaultPublisherID() + "/" + canon
	if obj, _, err := srv.resources.Get(ctx, "ServiceRelease", key); err != nil || obj == nil {
		return // nothing to remove
	}
	if err := srv.resources.Delete(ctx, "ServiceRelease", key); err != nil {
		log.Printf("cleanup-cross-kind: delete ServiceRelease %q (infra-owned, create-path): %v", key, err)
		return
	}
	log.Printf("cleanup-cross-kind: removed ServiceRelease %q (name is InfrastructureRelease-owned)", key)
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

	// Seam 1: also prune cross-kind ServiceRelease objects. Closing only the
	// ServiceDesiredVersion faucet (above) leaves a legacy ServiceRelease for an
	// infrastructure-owned name (e.g. xds@1.2.235) driving SERVICE-kind installs
	// from a stale pinned tarball — the recurring xds cache_digest_mismatch loop,
	// whose SDV was already gone. This is the release-side faucet.
	relRemoved, relErr := pruneCrossKindServiceReleases(ctx, srv.resources)
	if relErr != nil {
		log.Printf("cleanup-cross-kind: %v", relErr)
		return len(removed)
	}
	for _, name := range relRemoved {
		log.Printf("cleanup-cross-kind: removed legacy cross-kind ServiceRelease %q (owned by InfrastructureRelease)", name)
	}
	if len(relRemoved) > 0 {
		log.Printf("cleanup-cross-kind: removed %d legacy cross-kind ServiceRelease entry(ies)", len(relRemoved))
	}

	return len(removed) + len(relRemoved)
}
