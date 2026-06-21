package main

// behavioral_migration.go — distributed ScyllaDB schema migration coordinator for
// the behavioral_memory keyspace.
//
// This mirrors migration_coordinator.go (the ai_memory coordinator) but uses a
// SEPARATE etcd lock/state key and its own schema version, so the two keyspaces
// migrate independently and this PR does not alter any ai_memory migration
// behavior. The duplication is deliberate: parameterizing the existing helpers
// would change ai_memory call sites, which PR-2 must not touch.
//
// etcd keys:
//   /globular/migrations/scylla/behavioral_memory/lock  — mutex
//   /globular/migrations/scylla/behavioral_memory/state — JSON migration record
//
// IMPORTANT: the mutex key and the state key must be SIBLINGS — the mutex key
// must never be a prefix of the state key. etcd's concurrency.Mutex treats every
// key under its prefix as a lock contender, so a non-lock key nested under the
// mutex prefix (the historical bug: mutex="…/behavioral_memory", state nested at
// "…/behavioral_memory/state") is mistaken for an un-releasable lock holder and
// every Mutex.Lock waits for it forever, timing out. Keep both as leaf siblings.
//
// Failure handling matches the ai_memory coordinator: etcd-unreachable falls
// back to uncoordinated (IF NOT EXISTS) DDL; all DDL is idempotent.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

// newSchemaSession opens a keyspace-less gocql session for running CREATE
// KEYSPACE / CREATE TABLE DDL, with the same consistency policy the ai_memory
// connection uses (Quorum for RF>=2, One otherwise).
func newSchemaSession(hosts []string, port, rf int) (*gocql.Session, error) {
	consistency := gocql.Quorum
	if rf < 2 {
		consistency = gocql.One
	}
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	return cluster.CreateSession()
}

const (
	// behavioralMigrationMutexKey MUST be a leaf sibling of behavioralMigrationStateKey,
	// never a prefix of it (see the package comment). The historical value
	// "/globular/migrations/scylla/behavioral_memory" was a prefix of the state
	// key, which poisoned every Mutex.Lock; the "/lock" suffix fixes that.
	behavioralMigrationMutexKey = "/globular/migrations/scylla/behavioral_memory/lock"
	behavioralMigrationStateKey = "/globular/migrations/scylla/behavioral_memory/state"

	// behavioralSchemaVersion is independent of the ai_memory schemaVersion. Bump
	// when behavioral_memory tables change. v1 (PR-2) ingestion tables; v2 (PR-3)
	// governance tables + contradictions_by_target; v3 (PR-4) runtime tables
	// (principles_by_condition, action_checks, outcomes, outcomes_by_theme);
	// v4 (PR-9) governed observation fields for signals/evidence; v5 (PR-10)
	// outcome-derived promotion-candidate review queue; v6 (PR-11)
	// AWG↔behavioral advisory reconciliation reports; v7 (PR-13) action_checks
	// governed column + governance_coverage counters; v8 backfills the PR-9
	// signals/evidence columns via ALTER (they shipped only in CREATE TABLE, so
	// pre-PR-9 tables never got them) — the bump forces re-application on
	// clusters already marked complete-but-missing-columns.
	behavioralSchemaVersion = 8
)

// runBehavioralSchemaWithCoordination applies the behavioral_memory schema under
// an etcd-backed distributed mutex (separate from the ai_memory lock). Falls back
// to uncoordinated DDL if etcd is unreachable.
func (srv *server) runBehavioralSchemaWithCoordination(ctx context.Context) error {
	cli, etcdErr := config.NewEtcdClient()
	if etcdErr != nil {
		logger.Error("behavioral migration: etcd unavailable, running UNCOORDINATED schema init — concurrent DDL may race",
			"etcd_err", etcdErr)
		if schemaErr := srv.applyBehavioralSchema(ctx); schemaErr != nil {
			return fmt.Errorf("uncoordinated behavioral schema init (etcd unavailable: %v): %w", etcdErr, schemaErr)
		}
		return nil
	}
	defer cli.Close()

	if done, err := isBehavioralMigrationComplete(ctx, cli); err == nil && done {
		logger.Debug("behavioral schema migration: already complete (fast path skip)",
			"schema_version", behavioralSchemaVersion)
		return nil
	}

	sess, sessErr := concurrency.NewSession(cli, concurrency.WithTTL(migrationLockTTL))
	if sessErr != nil {
		logger.Error("behavioral migration: etcd session failed, running UNCOORDINATED schema init — concurrent DDL may race",
			"etcd_session_err", sessErr)
		if schemaErr := srv.applyBehavioralSchema(ctx); schemaErr != nil {
			return fmt.Errorf("uncoordinated behavioral schema init (etcd session failed: %v): %w", sessErr, schemaErr)
		}
		return nil
	}
	defer sess.Close()

	mu := concurrency.NewMutex(sess, behavioralMigrationMutexKey)

	lockCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()

	logger.Info("behavioral schema migration: waiting for lock", "key", behavioralMigrationMutexKey, "node_id", srv.Id)
	if err := mu.Lock(lockCtx); err != nil {
		return fmt.Errorf("behavioral schema migration: acquire lock: %w", err)
	}
	defer func() {
		if uerr := mu.Unlock(context.Background()); uerr != nil {
			logger.Warn("behavioral schema migration: unlock failed (lock will expire via TTL)", "err", uerr)
		}
	}()

	if done, err := isBehavioralMigrationComplete(ctx, cli); err == nil && done {
		logger.Debug("behavioral schema migration: already complete (post-lock check)")
		return nil
	}

	logger.Info("behavioral schema migration: applying DDL", "schema_version", behavioralSchemaVersion, "node_id", srv.Id)
	if err := srv.applyBehavioralSchema(ctx); err != nil {
		if werr := writeBehavioralMigrationRecord(ctx, cli, "failed", srv.Id); werr != nil {
			logger.Warn("behavioral schema migration: failed to record failure state in etcd", "err", werr)
		}
		return fmt.Errorf("behavioral schema migration: DDL failed: %w", err)
	}

	if err := writeBehavioralMigrationRecord(ctx, cli, "complete", srv.Id); err != nil {
		logger.Warn("behavioral schema migration: failed to record completion state in etcd (non-fatal)", "err", err)
	}

	logger.Info("behavioral schema migration: complete", "schema_version", behavioralSchemaVersion, "node_id", srv.Id)
	return nil
}

// applyBehavioralSchema runs the behavioral_memory keyspace + table DDL. Connects
// without a keyspace to run CREATE KEYSPACE, then creates the PR-2 tables. All
// statements are idempotent (IF NOT EXISTS).
func (srv *server) applyBehavioralSchema(_ context.Context) error {
	hosts := srv.ScyllaHosts
	port := srv.ScyllaPort
	if port == 0 {
		port = 9042
	}
	rf := len(hosts)
	if rf > 3 {
		rf = 3
	}

	session, err := newSchemaSession(hosts, port, rf)
	if err != nil {
		return fmt.Errorf("behavioral scylla connect (schema): %w", err)
	}
	defer session.Close()

	if err := session.Query(createBehavioralKeyspaceCQL(rf)).Exec(); err != nil {
		return fmt.Errorf("create behavioral keyspace: %w", err)
	}
	for _, stmt := range behavioralSchemaStatements {
		if err := session.Query(stmt).Exec(); err != nil {
			// Additive ALTER TABLE ADD is idempotent but ScyllaDB has no
			// ADD IF NOT EXISTS: tolerate the "column already exists" response so a
			// re-run (or a fresh install whose CREATE already added the column)
			// does not block the migration. Every other DDL is CREATE ... IF NOT
			// EXISTS and must still succeed.
			if isAlterStatement(stmt) && isExistingColumnError(err) {
				continue
			}
			return fmt.Errorf("behavioral schema DDL: %w", err)
		}
	}
	return nil
}

// isAlterStatement reports whether a DDL statement is an ALTER TABLE.
func isAlterStatement(stmt string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(stmt)), "ALTER ")
}

// isExistingColumnError reports whether err is ScyllaDB/Cassandra's response to an
// ALTER TABLE ADD for a column that already exists — the idempotent case.
func isExistingColumnError(err error) bool {
	if err == nil {
		return false
	}
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "conflicts with an existing column") ||
		strings.Contains(m, "already exists") ||
		strings.Contains(m, "duplicate column")
}

func isBehavioralMigrationComplete(ctx context.Context, cli *clientv3.Client) (bool, error) {
	resp, err := cli.Get(ctx, behavioralMigrationStateKey)
	if err != nil {
		return false, fmt.Errorf("behavioral migration state check: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return false, nil
	}
	var rec migrationRecord
	if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err != nil {
		return false, fmt.Errorf("behavioral migration state parse: %w", err)
	}
	return rec.Status == "complete" && rec.Version >= behavioralSchemaVersion, nil
}

func writeBehavioralMigrationRecord(ctx context.Context, cli *clientv3.Client, status, nodeID string) error {
	rec := migrationRecord{
		Version:   behavioralSchemaVersion,
		Status:    status,
		NodeID:    nodeID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal behavioral migration record: %w", err)
	}
	if _, err := cli.Put(ctx, behavioralMigrationStateKey, string(data)); err != nil {
		return fmt.Errorf("put behavioral migration record: %w", err)
	}
	return nil
}
