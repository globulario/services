package main

import (
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/clustercontroller/resourcestore"
)

func TestInitResourceStoreMem(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.initResourceStore(nil)
	if srv.resources == nil {
		t.Fatalf("resources not initialized")
	}
	if resourcestore.IsEtcdStore(srv.resources) {
		t.Fatalf("expected mem store, got etcd")
	}
}

func TestInitResourceStoreEtcd(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	dummy := &clientv3.Client{}
	srv.initResourceStore(dummy)
	if !resourcestore.IsEtcdStore(srv.resources) {
		t.Fatalf("expected etcd store")
	}
	if srv.etcdClient != dummy {
		t.Fatalf("etcd client not stored on server")
	}
}
