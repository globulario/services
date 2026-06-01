package main

import (
	"log"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// initResourceStore selects the appropriate backend and assigns it to the server.
func (srv *server) initResourceStore(etcd *clientv3.Client) {
	srv.etcdClient = etcd
	if etcd != nil {
		srv.resources = resourcestore.NewEtcdStore(etcd)
		log.Printf("resources: using etcd store")
	} else {
		srv.resources = resourcestore.NewMemStore()
		log.Printf("resources: using mem store")
	}

	// One-time migration: clean up stale desired-service keys that don't match
	// their canonical form (domain-prefixed, underscore variants, etc.).
	migCtx, migCancel := withBounded(boundedLong)
	if n := srv.cleanupStaleDesiredKeys(migCtx); n > 0 {
		log.Printf("resources: cleaned up %d stale desired-service key(s)", n)
	}
	migCancel()

	// Backward compat: create ServiceRelease objects for any existing
	// ServiceDesiredVersion entries that predate the bridge.
	relCtx, relCancel := withBounded(boundedLong)
	srv.ensureServiceReleasesFromDesired(relCtx)
	relCancel()

	// One-time cleanup: remove stale ServiceRelease/ServiceDesiredVersion
	// objects for infrastructure packages (managed by InfrastructureRelease).
	infraCtx, infraCancel := withBounded(boundedLong)
	srv.cleanupStaleInfraServiceReleases(infraCtx)
	infraCancel()
}
