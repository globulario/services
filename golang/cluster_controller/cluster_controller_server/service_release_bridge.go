package main

import (
	"context"
	"log"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// ensureServiceRelease creates or updates a ServiceRelease object for the given
// service so that the release reconciler can track per-service lifecycle phases.
// Idempotent: if a ServiceRelease already exists with the same version and
// build number, it is left unchanged.
func (srv *server) ensureServiceRelease(ctx context.Context, serviceName, version string, buildNumber int64) {
	if srv.resources == nil {
		return
	}
	canon := canonicalServiceName(serviceName)
	if canon == "" || version == "" {
		return
	}

	releaseName := defaultPublisherID() + "/" + canon

	// Check for existing release — skip if version+build match and not being removed.
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
					existingPhase == ReleasePhaseRemoving || existingPhase == ReleasePhaseRemoved ||
					existingPhase == cluster_controllerpb.ReleasePhaseFailed ||
					existingPhase == cluster_controllerpb.ReleasePhaseRolledBack
			}
			if !needsRecreate && existing.Spec.Version == version && existing.Spec.BuildNumber == buildNumber {
				return // already up-to-date and in a healthy state
			}
			log.Printf("ensureServiceRelease: %s: recreating (phase=%s removing=%v needsRecreate=%v)",
				releaseName, existingPhase, existing.Spec.Removing, needsRecreate)
		}
	} else {
		log.Printf("ensureServiceRelease: %s: no existing release, creating (version=%s)", releaseName, version)
	}

	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: releaseName},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID:  defaultPublisherID(),
			ServiceName:  canon,
			Version:      version,
			BuildNumber:  buildNumber,
			Platform:     "", // resolved per-node by the reconciler
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase: cluster_controllerpb.ReleasePhasePending,
		},
	}

	if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
		log.Printf("ensureServiceRelease: %s: apply failed: %v", releaseName, err)
	} else {
		log.Printf("ensureServiceRelease: %s: created with phase=PENDING", releaseName)
	}
}

// ensureServiceReleasesFromDesired scans all ServiceDesiredVersion objects and
// creates corresponding ServiceRelease objects for any that are missing.
// Safe to call periodically — only creates releases, does not clean up infra.
func (srv *server) ensureServiceReleasesFromDesired(ctx context.Context) {
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
		srv.ensureServiceRelease(ctx, canon, sdv.Spec.Version, sdv.Spec.BuildNumber)
		created++
	}
	if created > 0 {
		log.Printf("ensureServiceReleasesFromDesired: processed %d desired entries", created)
	}
}

// cleanupStaleInfraServiceReleases removes ServiceRelease and ServiceDesiredVersion
// objects for infrastructure packages that were incorrectly created by earlier
// versions of the bridge or auto-import. Called once at startup only.
func (srv *server) cleanupStaleInfraServiceReleases(ctx context.Context) {
	if srv.resources == nil {
		return
	}
	infraManaged := make(map[string]bool)
	if infraItems, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		for _, obj := range infraItems {
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
				infraManaged[canonicalServiceName(rel.Spec.Component)] = true
			}
		}
	}
	for canon := range infraManaged {
		relKey := defaultPublisherID() + "/" + canon
		if err := srv.resources.Delete(ctx, "ServiceRelease", relKey); err == nil {
			log.Printf("cleanupStaleInfraServiceReleases: removed stale ServiceRelease for infra package %s", relKey)
		}
		if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", canon); err == nil {
			log.Printf("cleanupStaleInfraServiceReleases: removed stale ServiceDesiredVersion for infra package %s", canon)
		}
	}
}
