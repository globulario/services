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
			beingRemoved := existing.Spec.Removing
			if existing.Status != nil {
				phase := existing.Status.Phase
				beingRemoved = beingRemoved || phase == ReleasePhaseRemoving || phase == ReleasePhaseRemoved
			}
			if !beingRemoved && existing.Spec.Version == version && existing.Spec.BuildNumber == buildNumber {
				return // already up-to-date and not being removed
			}
		}
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
		log.Printf("ensureServiceRelease: %s: %v", releaseName, err)
	}
}

// ensureServiceReleasesFromDesired scans all ServiceDesiredVersion objects and
// creates corresponding ServiceRelease objects for any that are missing.
// Called at startup for backward compatibility with desired-state entries
// created before the bridge was added.
func (srv *server) ensureServiceReleasesFromDesired(ctx context.Context) {
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
		srv.ensureServiceRelease(ctx, canon, sdv.Spec.Version, sdv.Spec.BuildNumber)
		created++
	}
	if created > 0 {
		log.Printf("ensureServiceReleasesFromDesired: processed %d desired entries", created)
	}
}
