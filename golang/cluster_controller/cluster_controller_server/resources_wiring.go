package main

import (
	"context"
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
	if n := srv.cleanupStaleDesiredKeys(context.Background()); n > 0 {
		log.Printf("resources: cleaned up %d stale desired-service key(s)", n)
	}
}
