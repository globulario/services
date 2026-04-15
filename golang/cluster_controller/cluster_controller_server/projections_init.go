package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/projections"
	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
)

// initProjections brings up the read-only projections backed by ScyllaDB.
// Best-effort: if ScyllaDB is unreachable or schema init fails, the server
// continues without projections and callers fall back to direct state scans
// (Clause 3). The returned close function MUST be called on server shutdown.
func (srv *server) initProjections(ctx context.Context) (close func(), err error) {
	session, err := openProjectionsSession(ctx)
	if err != nil {
		log.Printf("projections: ScyllaDB unavailable (%v) — running without projections; resolvers fall back to in-memory state", err)
		return func() {}, nil
	}

	logger := slog.Default().With("component", "projections")
	proj := projections.NewNodeIdentityProjector(session, logger)
	if err := proj.EnsureSchema(); err != nil {
		log.Printf("projections: schema init failed (%v) — running without projections", err)
		session.Close()
		return func() {}, nil
	}
	srv.nodeIdentityProj = proj

	// Start the reconciler. It re-derives the entire projection from
	// srv.state every 5 minutes, catching missed synchronous writes.
	go proj.StartReconcileLoop(ctx, 5*time.Minute, srv.snapshotNodeIdentities)

	log.Printf("projections: node_identity projector online")
	return func() { session.Close() }, nil
}

// snapshotNodeIdentities returns the current source-of-truth view of all
// nodes as NodeIdentity rows. Used by the reconciler to rebuild the scylla
// view from scratch.
func (srv *server) snapshotNodeIdentities() []projections.NodeIdentity {
	srv.lock("projection-snapshot")
	defer srv.unlock()
	if srv.state == nil {
		return nil
	}
	out := make([]projections.NodeIdentity, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		if node == nil || node.NodeID == "" {
			continue
		}
		out = append(out, *nodeToIdentity(node))
	}
	return out
}

// openProjectionsSession connects to ScyllaDB, creates the projections
// keyspace if it doesn't exist, and returns a session bound to that keyspace.
func openProjectionsSession(ctx context.Context) (*gocql.Session, error) {
	hosts, err := config.GetScyllaHosts()
	if err != nil || len(hosts) == 0 {
		return nil, fmt.Errorf("cannot resolve ScyllaDB hosts from etcd: %v", err)
	}

	// Adapt consistency to the number of ScyllaDB nodes.
	// With a single node, QUORUM is impossible (requires 2 of 3).
	consistency := gocql.Quorum
	if len(hosts) < 2 {
		consistency = gocql.One
	}

	// First session: no keyspace, used only to create the keyspace.
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = 9042
	cluster.Consistency = consistency
	cluster.Timeout = 5 * time.Second
	cluster.ConnectTimeout = 5 * time.Second

	bootstrap, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("scylla bootstrap connect: %w", err)
	}
	rf := projectionReplicationFactor(len(hosts))
	createKs := fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}",
		projections.Keyspace, rf,
	)
	if err := bootstrap.Query(createKs).WithContext(ctx).Exec(); err != nil {
		bootstrap.Close()
		return nil, fmt.Errorf("create keyspace: %w", err)
	}
	bootstrap.Close()

	// Real session: scoped to the keyspace.
	cluster.Keyspace = projections.Keyspace
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("scylla session: %w", err)
	}
	return session, nil
}

// projectionReplicationFactor picks a sane RF from cluster size. Single-node
// dev clusters use RF=1; multi-node uses 3 (or hostCount if smaller).
func projectionReplicationFactor(hostCount int) int {
	switch {
	case hostCount <= 1:
		return 1
	case hostCount < 3:
		return hostCount
	default:
		return 3
	}
}
