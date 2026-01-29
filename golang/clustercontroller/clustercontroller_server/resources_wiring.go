package main

import (
	"log"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/clustercontroller/resourcestore"
)

// initResourceStore selects the appropriate backend and assigns it to the server.
func (srv *server) initResourceStore(etcd *clientv3.Client) {
	srv.etcdClient = etcd
	if etcd != nil {
		srv.resources = resourcestore.NewEtcdStore(etcd)
		log.Printf("resources: using etcd store")
		return
	}
	srv.resources = resourcestore.NewMemStore()
	log.Printf("resources: using mem store")
}
