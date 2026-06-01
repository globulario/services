// @awareness namespace=globular.platform
// @awareness component=platform_controller.reconciler
// @awareness file_role=reconcile_projection_initialization
// @awareness implements=globular.platform:intent.desired_state.is_authority
// @awareness risk=high
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

var projectionLaneTimeout = 45 * time.Second
var projectionLaneInitialDelay = 30 * time.Second

type nodeIdentityReconciler interface {
	Reconcile(ctx context.Context, snapshot []projections.NodeIdentity) error
}

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

	// Start the reconciler lane. Projection rebuilds are best-effort and must
	// never block critical reconcile lanes; enforce timeout + lane isolation.
	go srv.startProjectionReconcileLane(ctx, proj, 5*time.Minute)

	log.Printf("projections: node_identity projector online")
	return func() { session.Close() }, nil
}

func (srv *server) startProjectionReconcileLane(ctx context.Context, proj nodeIdentityReconciler, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	// Initial reconcile after warmup.
	timer := time.NewTimer(projectionLaneInitialDelay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !srv.isLeader() {
				timer.Reset(interval)
				continue
			}
			if !srv.projectionReconcileRunning.CompareAndSwap(false, true) {
				srv.projectionReconcilePending.Store(true)
				reconcilePreviousRunActiveTotal.WithLabelValues("projections").Inc()
				srv.publishReconcileLaneStatus(ctx, "projections", reconcileLaneStatus{
					Phase:            "BLOCKED",
					Running:          true,
					PreviousRunAlive: true,
					LastError:        "previous run still active",
				})
				log.Printf("projections: previous reconcile run still active")
				timer.Reset(interval)
				continue
			}

			func() {
				defer srv.projectionReconcileRunning.Store(false)
				for {
					srv.projectionReconcilePending.Store(false)
					start := time.Now()
					reconcileLaneRunning.WithLabelValues("projections").Set(1)
					rctx, cancel := context.WithTimeout(ctx, projectionLaneTimeout)
					err := proj.Reconcile(rctx, srv.snapshotNodeIdentities())
					cancel()
					reconcileLaneRunning.WithLabelValues("projections").Set(0)
					reconcileLaneDurationSeconds.WithLabelValues("projections").Observe(time.Since(start).Seconds())

					if err != nil {
						log.Printf("projections: node_identity reconcile failed: %v", err)
						if rctx.Err() == context.DeadlineExceeded {
							reconcileLaneTimeoutsTotal.WithLabelValues("projections").Inc()
							reconcileBlockedPhase.WithLabelValues("projections").Set(1)
							srv.publishReconcileLaneStatus(ctx, "projections", reconcileLaneStatus{
								Phase:     "TIMEOUT",
								Running:   false,
								LastError: err.Error(),
							})
						} else {
							srv.publishReconcileLaneStatus(ctx, "projections", reconcileLaneStatus{
								Phase:     "DEGRADED",
								Running:   false,
								LastError: err.Error(),
							})
						}
					} else {
						reconcileBlockedPhase.WithLabelValues("projections").Set(0)
						srv.publishReconcileLaneStatus(ctx, "projections", reconcileLaneStatus{
							Phase:   "OK",
							Running: false,
						})
					}
					if ctx.Err() != nil {
						return
					}
					if !srv.projectionReconcilePending.CompareAndSwap(true, false) {
						return
					}
					log.Printf("projections: running coalesced follow-up reconcile")
				}
			}()
			timer.Reset(interval)
		}
	}
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
// The entire initialization is bounded by a 15-second timeout to prevent
// blocking the controller startup path.
func openProjectionsSession(ctx context.Context) (*gocql.Session, error) {
	initCtx, initCancel := context.WithTimeout(ctx, 15*time.Second)
	defer initCancel()

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
		if initCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("scylla bootstrap connect timed out after 15s")
		}
		return nil, fmt.Errorf("scylla bootstrap connect: %w", err)
	}
	ctx = initCtx
	rf := projectionReplicationFactor(len(hosts))
	createKs := fmt.Sprintf(
		"CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}",
		projections.Keyspace, rf,
	)
	if err := bootstrap.Query(createKs).WithContext(ctx).Exec(); err != nil {
		bootstrap.Close()
		return nil, fmt.Errorf("create keyspace: %w", err)
	}
	// Enforce RF on existing keyspace (CREATE IF NOT EXISTS is a no-op on existing keyspaces).
	// This ensures a pre-existing RF=1 keyspace is promoted at startup without waiting for the
	// 45s schema guard tick.
	if rf > 1 {
		alterKs := fmt.Sprintf(
			"ALTER KEYSPACE %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}",
			projections.Keyspace, rf,
		)
		if aerr := bootstrap.Query(alterKs).WithContext(ctx).Consistency(gocql.One).Exec(); aerr != nil {
			log.Printf("projections: ALTER KEYSPACE %s RF=%d failed (best-effort): %v", projections.Keyspace, rf, aerr)
		} else {
			log.Printf("projections: ALTER KEYSPACE %s RF=%d applied at startup", projections.Keyspace, rf)
		}
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
