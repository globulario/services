package main

// migration_coordinator.go — distributed ScyllaDB schema migration coordinator.
//
// Problem: When ai_memory starts on multiple nodes simultaneously, each node
// runs CREATE KEYSPACE / CREATE TABLE independently. While IF NOT EXISTS makes
// individual DDL statements idempotent, concurrent schema operations can race
// inside ScyllaDB's schema agreement protocol and produce partial/corrupted state.
//
// Solution: An etcd-backed distributed mutex ensures only one node runs schema
// migration at a time. The result is recorded in etcd so subsequent nodes skip
// the migration entirely instead of racing.
//
// etcd keys:
//   /globular/migrations/scylla/ai_memory       — mutex (concurrency.NewMutex)
//   /globular/migrations/scylla/ai_memory/state — JSON { version, status, node_id, timestamp }
//
// Status values: "complete" | "failed"
//
// Failure handling:
//   - If etcd is unreachable, we fall back to uncoordinated schema init.
//     CREATE IF NOT EXISTS is safe in isolation; the risk is concurrent execution.
//   - If the holding node crashes mid-migration, the lease-backed session TTL
//     releases the lock after migrationLockTTL seconds. The next node to acquire
//     the lock will find state absent or "failed" and re-run migrations.
//   - Migrations must therefore be idempotent. All DDL uses IF NOT EXISTS.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
)

const (
	// migrationMutexKey is the etcd prefix used by concurrency.NewMutex.
	// The mutex appends a per-session suffix under this prefix.
	migrationMutexKey = "/globular/migrations/scylla/ai_memory"

	// migrationStateKey stores the migration completion record.
	migrationStateKey = "/globular/migrations/scylla/ai_memory/state"

	// schemaVersion is a monotonic counter. Bump this whenever the schema
	// changes (new table, new index, ALTER TABLE). The coordinator skips
	// migration only when etcd shows version >= schemaVersion.
	schemaVersion = 1

	// migrationLockTTL is the etcd session TTL in seconds.
	// If the holder crashes, the lock releases after this duration.
	migrationLockTTL = 60

	// migrationTimeout caps how long a node will wait to acquire the lock
	// before failing startup. 3 minutes is enough for a slow first migration.
	migrationTimeout = 3 * time.Minute
)

// migrationRecord is the value written to migrationStateKey.
type migrationRecord struct {
	Version   int    `json:"version"`
	Status    string `json:"status"` // "complete" | "failed"
	NodeID    string `json:"node_id"`
	Timestamp string `json:"timestamp"`
}

// runSchemaWithCoordination runs the ScyllaDB schema migration under an
// etcd-backed distributed mutex. Only one node applies migrations at a time;
// all others wait for the lock, confirm state==complete, and return immediately.
//
// Falls back to uncoordinated execution if etcd is unreachable, which is safe
// for the current schema (all DDL uses IF NOT EXISTS).
func (srv *server) runSchemaWithCoordination(ctx context.Context) error {
	cli, err := config.NewEtcdClient()
	if err != nil {
		// etcd unreachable — fall back. Uncoordinated IF NOT EXISTS DDL is
		// safe when only one node is starting; risky on simultaneous startup.
		logger.Warn("migration coordinator: etcd unavailable, running uncoordinated schema init",
			"err", err)
		return srv.applySchema(ctx)
	}
	defer cli.Close()

	// Fast path: if a previous run already completed for this schema version,
	// skip the mutex overhead entirely.
	if done, err := isMigrationComplete(ctx, cli); err == nil && done {
		logger.Debug("schema migration: already complete (fast path skip)",
			"schema_version", schemaVersion)
		return nil
	}

	// Slow path: acquire the distributed lock and re-check under it.
	sess, err := concurrency.NewSession(cli, concurrency.WithTTL(migrationLockTTL))
	if err != nil {
		logger.Warn("migration coordinator: etcd session failed, running uncoordinated",
			"err", err)
		return srv.applySchema(ctx)
	}
	defer sess.Close()

	mu := concurrency.NewMutex(sess, migrationMutexKey)

	lockCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()

	logger.Info("schema migration: waiting for lock", "key", migrationMutexKey, "node_id", srv.Id)
	if err := mu.Lock(lockCtx); err != nil {
		return fmt.Errorf("schema migration: acquire lock: %w", err)
	}
	defer func() {
		if uerr := mu.Unlock(context.Background()); uerr != nil {
			logger.Warn("schema migration: unlock failed (lock will expire via TTL)", "err", uerr)
		}
	}()

	logger.Info("schema migration: lock acquired", "node_id", srv.Id)

	// Re-check under the lock — another node may have finished just before us.
	if done, err := isMigrationComplete(ctx, cli); err == nil && done {
		logger.Debug("schema migration: already complete (post-lock check)")
		return nil
	}

	// Apply schema DDL.
	logger.Info("schema migration: applying DDL", "schema_version", schemaVersion, "node_id", srv.Id)
	if err := srv.applySchema(ctx); err != nil {
		// Record failure so operators can inspect etcd and no node silently skips.
		if werr := writeMigrationRecord(ctx, cli, "failed", srv.Id); werr != nil {
			logger.Warn("schema migration: failed to record failure state in etcd", "err", werr)
		}
		return fmt.Errorf("schema migration: DDL failed: %w", err)
	}

	// Record success. Non-fatal if the write fails — the next node will simply
	// re-acquire the lock, run the idempotent DDL again, and succeed.
	if err := writeMigrationRecord(ctx, cli, "complete", srv.Id); err != nil {
		logger.Warn("schema migration: failed to record completion state in etcd (non-fatal)",
			"err", err)
	}

	logger.Info("schema migration: complete", "schema_version", schemaVersion, "node_id", srv.Id)
	return nil
}

// isMigrationComplete checks etcd for a successful migration record at or
// above the current schemaVersion.
func isMigrationComplete(ctx context.Context, cli *clientv3.Client) (bool, error) {
	resp, err := cli.Get(ctx, migrationStateKey)
	if err != nil {
		return false, fmt.Errorf("migration state check: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return false, nil
	}
	var rec migrationRecord
	if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err != nil {
		return false, fmt.Errorf("migration state parse: %w", err)
	}
	return rec.Status == "complete" && rec.Version >= schemaVersion, nil
}

// writeMigrationRecord persists a migration record in etcd.
func writeMigrationRecord(ctx context.Context, cli *clientv3.Client, status, nodeID string) error {
	rec := migrationRecord{
		Version:   schemaVersion,
		Status:    status,
		NodeID:    nodeID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal migration record: %w", err)
	}
	if _, err := cli.Put(ctx, migrationStateKey, string(data)); err != nil {
		return fmt.Errorf("put migration record: %w", err)
	}
	return nil
}
