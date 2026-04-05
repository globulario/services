package projections

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gocql/gocql"
)

// NodeIdentity is the projection's internal row shape — exactly what the
// scylla tables hold. The RPC response message (NodeIdentityProjection in
// cluster_controller.proto) is assembled from this plus the source tag.
//
// Per projection-clauses.md this MUST NOT carry services, packages, metrics,
// logs, health, or heartbeat age. If you're tempted to add a field here,
// start a new projection instead.
type NodeIdentity struct {
	NodeID     string
	Hostname   string
	IPs        []string
	MACs       []string
	Labels     []string
	ObservedAt int64 // unix seconds
}

// NodeIdentityProjector writes the node_identity projection to ScyllaDB.
// It has no knowledge of etcd or cluster-controller internals — callers
// hand it fully-formed NodeIdentity rows derived from the source of truth.
type NodeIdentityProjector struct {
	session *gocql.Session
	logger  *slog.Logger

	// ensureOnce runs the CREATE TABLE migrations on first use.
	ensureOnce sync.Once
	ensureErr  error
}

// NewNodeIdentityProjector takes an already-opened gocql session pointed at
// the projections keyspace. The caller owns session lifecycle.
func NewNodeIdentityProjector(session *gocql.Session, logger *slog.Logger) *NodeIdentityProjector {
	if logger == nil {
		logger = slog.Default()
	}
	return &NodeIdentityProjector{session: session, logger: logger}
}

// EnsureSchema runs idempotent CREATE TABLE statements. Safe to call
// repeatedly; migrations are applied once per projector instance.
func (p *NodeIdentityProjector) EnsureSchema() error {
	p.ensureOnce.Do(func() {
		for _, stmt := range nodeIdentityTables() {
			if err := p.session.Query(stmt).Exec(); err != nil {
				p.ensureErr = fmt.Errorf("node_identity schema: %w", err)
				return
			}
		}
	})
	return p.ensureErr
}

// Upsert writes the identity to the main table and all three reverse-lookup
// tables in a LOGGED batch, giving atomic-ish semantics across the denormalized
// set. Failures are logged and returned; callers typically swallow the error
// and continue (projectors are best-effort — see Clause 3 fallback guarantee).
func (p *NodeIdentityProjector) Upsert(ctx context.Context, id NodeIdentity) error {
	if id.NodeID == "" {
		return fmt.Errorf("node_identity: empty node_id")
	}
	if err := p.EnsureSchema(); err != nil {
		return err
	}

	batch := p.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)

	batch.Query(
		`INSERT INTO globular_projections.node_identity
            (node_id, hostname, macs, ips, labels, observed_at)
            VALUES (?, ?, ?, ?, ?, ?)`,
		id.NodeID, id.Hostname, id.MACs, id.IPs, id.Labels, id.ObservedAt,
	)
	if id.Hostname != "" {
		batch.Query(
			`INSERT INTO globular_projections.node_identity_by_hostname
                (hostname, node_id, observed_at) VALUES (?, ?, ?)`,
			id.Hostname, id.NodeID, id.ObservedAt,
		)
	}
	for _, mac := range id.MACs {
		if mac == "" {
			continue
		}
		batch.Query(
			`INSERT INTO globular_projections.node_identity_by_mac
                (mac, node_id, observed_at) VALUES (?, ?, ?)`,
			mac, id.NodeID, id.ObservedAt,
		)
	}
	for _, ip := range id.IPs {
		if ip == "" {
			continue
		}
		batch.Query(
			`INSERT INTO globular_projections.node_identity_by_ip
                (ip, node_id, observed_at) VALUES (?, ?, ?)`,
			ip, id.NodeID, id.ObservedAt,
		)
	}

	if err := p.session.ExecuteBatch(batch); err != nil {
		return fmt.Errorf("node_identity upsert %s: %w", id.NodeID, err)
	}
	return nil
}

// GetByNodeID reads the projection by primary key. Returns (nil, nil) on
// cache miss — the caller falls back to the source of truth per Clause 3.
func (p *NodeIdentityProjector) GetByNodeID(ctx context.Context, nodeID string) (*NodeIdentity, error) {
	var id NodeIdentity
	err := p.session.Query(
		`SELECT node_id, hostname, macs, ips, labels, observed_at
            FROM globular_projections.node_identity WHERE node_id = ?`,
		nodeID,
	).WithContext(ctx).Scan(&id.NodeID, &id.Hostname, &id.MACs, &id.IPs, &id.Labels, &id.ObservedAt)
	if err == gocql.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("node_identity get by id: %w", err)
	}
	return &id, nil
}

// GetNodeIDByHostname / ByMAC / ByIP do single-partition reads on the
// denormalized reverse-lookup tables. Each returns "" on miss so callers
// can chain into the source-of-truth fallback cleanly.
func (p *NodeIdentityProjector) GetNodeIDByHostname(ctx context.Context, hostname string) (string, error) {
	return p.reverseLookup(ctx, "node_identity_by_hostname", "hostname", hostname)
}

func (p *NodeIdentityProjector) GetNodeIDByMAC(ctx context.Context, mac string) (string, error) {
	return p.reverseLookup(ctx, "node_identity_by_mac", "mac", mac)
}

func (p *NodeIdentityProjector) GetNodeIDByIP(ctx context.Context, ip string) (string, error) {
	return p.reverseLookup(ctx, "node_identity_by_ip", "ip", ip)
}

func (p *NodeIdentityProjector) reverseLookup(ctx context.Context, table, key, val string) (string, error) {
	var nodeID string
	q := fmt.Sprintf(
		"SELECT node_id FROM globular_projections.%s WHERE %s = ?",
		table, key,
	)
	err := p.session.Query(q, val).WithContext(ctx).Scan(&nodeID)
	if err == gocql.ErrNotFound {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("%s lookup: %w", table, err)
	}
	return nodeID, nil
}

// Reconcile reconstructs the projection from the supplied source-of-truth
// snapshot. It upserts every row present in `snapshot` and deletes scylla
// rows whose node_id is no longer in the snapshot.
//
// Callers pass the current cluster-controller state — reconcile treats it as
// truth. Run periodically (default 5 min) to catch missed synchronous
// projector writes and correct split-brain between etcd and scylla.
func (p *NodeIdentityProjector) Reconcile(ctx context.Context, snapshot []NodeIdentity) error {
	if err := p.EnsureSchema(); err != nil {
		return err
	}
	// Upsert everything in the snapshot.
	wantByID := make(map[string]struct{}, len(snapshot))
	for _, id := range snapshot {
		wantByID[id.NodeID] = struct{}{}
		if err := p.Upsert(ctx, id); err != nil {
			p.logger.Warn("reconcile: upsert failed", "node_id", id.NodeID, "err", err)
		}
	}
	// Delete stale node_id rows. Reverse tables are rebuilt on next upsert
	// cycle — they're self-healing because we blindly INSERT every time.
	iter := p.session.Query(
		`SELECT node_id FROM globular_projections.node_identity`,
	).WithContext(ctx).Iter()
	var existing string
	var stale []string
	for iter.Scan(&existing) {
		if _, ok := wantByID[existing]; !ok {
			stale = append(stale, existing)
		}
	}
	if err := iter.Close(); err != nil {
		return fmt.Errorf("reconcile scan: %w", err)
	}
	for _, id := range stale {
		if err := p.DeleteByNodeID(ctx, id); err != nil {
			p.logger.Warn("reconcile: delete stale failed", "node_id", id, "err", err)
		}
	}
	p.logger.Debug("node_identity reconciled", "upserted", len(snapshot), "deleted_stale", len(stale))
	return nil
}

// DeleteByNodeID removes a node from the main table and from every reverse
// lookup that points at it. Called by the reconciler for nodes that have
// been removed from the cluster.
func (p *NodeIdentityProjector) DeleteByNodeID(ctx context.Context, nodeID string) error {
	// Fetch the identity first so we know which reverse entries to clean.
	id, err := p.GetByNodeID(ctx, nodeID)
	if err != nil {
		return err
	}
	if id == nil {
		return nil
	}
	batch := p.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	batch.Query(`DELETE FROM globular_projections.node_identity WHERE node_id = ?`, nodeID)
	if id.Hostname != "" {
		batch.Query(`DELETE FROM globular_projections.node_identity_by_hostname WHERE hostname = ?`, id.Hostname)
	}
	for _, mac := range id.MACs {
		if mac != "" {
			batch.Query(`DELETE FROM globular_projections.node_identity_by_mac WHERE mac = ?`, mac)
		}
	}
	for _, ip := range id.IPs {
		if ip != "" {
			batch.Query(`DELETE FROM globular_projections.node_identity_by_ip WHERE ip = ?`, ip)
		}
	}
	return p.session.ExecuteBatch(batch)
}

// StartReconcileLoop runs Reconcile on a fixed interval. Supply a snapshot
// callback that returns the current source-of-truth view. Returns once ctx
// is cancelled. Intended to run in its own goroutine.
func (p *NodeIdentityProjector) StartReconcileLoop(ctx context.Context, interval time.Duration, snapshot func() []NodeIdentity) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	// Initial reconcile after 30s — let the controller warm up.
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := p.Reconcile(ctx, snapshot()); err != nil {
				p.logger.Warn("node_identity reconcile failed", "err", err)
			}
			timer.Reset(interval)
		}
	}
}
