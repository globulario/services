package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestMarkSchemaGuardStatus_OwnerGuarded proves the RT-3 funnel migration of the
// per-keyspace schema-guard status write: markSchemaGuardStatus now routes through
// the governed critical-write seam, so a writer that does not own
// /globular/scylla/schema_guard/ is rejected before any etcd I/O. The owner guard
// runs inside PutRuntimeWithClass, so a non-owner identity makes the write return
// an error without reaching etcd.
func TestMarkSchemaGuardStatus_OwnerGuarded(t *testing.T) {
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.etcdClient = &clientv3.Client{} // non-nil → pass the early-bootstrap gate

	t.Cleanup(func() { config.SetLocalWriterIdentity("") })

	// node-agent is not an authorized writer of the controller-owned schema_guard
	// prefix → the status write must be rejected by the owner guard.
	config.SetLocalWriterIdentity("node-agent")
	if err := srv.markSchemaGuardStatus(context.Background(), "system_auth", schemaGuardStatus{}); err == nil {
		t.Fatal("expected node-agent write to /globular/scylla/schema_guard/ to be rejected by the owner guard")
	}
}
