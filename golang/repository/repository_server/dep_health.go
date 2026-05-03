package main

// dep_health.go — dependency health watchdog for the repository service.
//
// The repository is the heart of the cluster. If its distributed metadata store
// (ScyllaDB) is down, the service MUST mark itself as broken — not silently degrade.
//
// MinIO is now a best-effort mirror only. If MinIO is unreachable, the service
// continues operating from the local POSIX store and logs a warning. MinIO
// unavailability does NOT cause NOT_SERVING.
//
// The watchdog runs every 15 seconds. It pings both ScyllaDB and the MinIO mirror.
//
// ScyllaDB (required):
//   - Service transitions to NOT_SERVING when ScyllaDB is unreachable.
//   - New RPCs receive codes.Unavailable with a clear dependency-failure message.
//   - Subsystem registry reflects the failure (visible to cluster_doctor).
//   - Recovery is automatic.
//
// MinIO mirror (optional / informational):
//   - Unavailability is logged as a warning.
//   - Does NOT affect service health or RPC gating.
//   - mirrorOK tracks mirror reachability for observability.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/storage_backend"
	"github.com/globulario/services/golang/subsystem"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	healthCheckInterval = 15 * time.Second
	healthCheckTimeout  = 5 * time.Second
)

// depHealthWatchdog monitors the repository's distributed dependencies.
type depHealthWatchdog struct {
	storage       storage_backend.Storage
	scylla        *scyllaStore
	healthy       *atomic.Bool // true = ScyllaDB is OK (service can serve)
	mirrorOK      *atomic.Bool // true = MinIO mirror is reachable (informational only)
	minioSub      *subsystem.SubsystemHandle
	scyllaSub     *subsystem.SubsystemHandle
	logger        *slog.Logger
	onScyllaReady func(*scyllaStore) // called when late-connect succeeds
}

// newDepHealthWatchdog creates a watchdog that monitors MinIO mirror and ScyllaDB.
// storage is the mirror storage (used only for mirror ping); it may be nil.
func newDepHealthWatchdog(storage storage_backend.Storage, scylla *scyllaStore, logger *slog.Logger) *depHealthWatchdog {
	healthy := &atomic.Bool{}
	healthy.Store(true) // optimistic at start — first check runs immediately

	mirrorOK := &atomic.Bool{}
	mirrorOK.Store(true) // optimistic — first check runs immediately

	return &depHealthWatchdog{
		storage:   storage,
		scylla:    scylla,
		healthy:   healthy,
		mirrorOK:  mirrorOK,
		minioSub:  subsystem.RegisterSubsystem("dep:minio", healthCheckInterval),
		scyllaSub: subsystem.RegisterSubsystem("dep:scylladb", healthCheckInterval),
		logger:    logger,
	}
}

// Start launches the background health check loop. Blocks until ctx is cancelled.
func (w *depHealthWatchdog) Start(ctx context.Context) {
	// Run first check immediately.
	w.check(ctx)

	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

// check pings ScyllaDB (required) and the MinIO mirror (informational).
func (w *depHealthWatchdog) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	minioOK := w.pingMinio(checkCtx)
	scyllaOK := w.pingScylla()

	// MinIO mirror is informational — doesn't affect service health.
	w.mirrorOK.Store(minioOK)

	wasHealthy := w.healthy.Load()
	// Service is healthy as long as ScyllaDB is OK.
	// Local storage is always available — no need to gate on it.
	nowHealthy := scyllaOK
	w.healthy.Store(nowHealthy)

	// Log transitions.
	if wasHealthy && !nowHealthy {
		w.logger.Error("repository metadata (ScyllaDB) UNHEALTHY — service is NOT_SERVING",
			"scylladb", scyllaOK)
	} else if !wasHealthy && nowHealthy {
		w.logger.Info("repository metadata recovered — service is SERVING")
	}
	if !minioOK {
		w.logger.Warn("MinIO mirror unavailable — serving from local POSIX store")
	}
}

// pingMinio checks mirror (MinIO) reachability — informational only.
func (w *depHealthWatchdog) pingMinio(ctx context.Context) bool {
	if w.storage == nil {
		// No mirror configured — that's fine.
		w.minioSub.Tick()
		return true
	}
	// For a ResilientStorage, ping only the mirror component.
	var pingTarget storage_backend.Storage
	if rs, ok := w.storage.(*storage_backend.ResilientStorage); ok {
		// Use PingMirror to avoid pinging the local (always-OK) store.
		if err := rs.PingMirror(ctx); err != nil {
			w.minioSub.TickError(err)
			w.logger.Warn("minio mirror health check failed", "err", err)
			return false
		}
		w.minioSub.Tick()
		return true
	}
	// Plain storage (non-resilient) — ping directly.
	pingTarget = w.storage
	if err := pingTarget.Ping(ctx); err != nil {
		w.minioSub.TickError(err)
		w.logger.Warn("minio health check failed", "err", err)
		return false
	}
	w.minioSub.Tick()
	return true
}

// pingScylla checks ScyllaDB reachability.
func (w *depHealthWatchdog) pingScylla() bool {
	if w.scylla == nil {
		// ScyllaDB was unreachable at startup — attempt late connection.
		// This self-heals after power outages where ScyllaDB starts after
		// the repository service.
		scylla, err := connectScylla()
		if err != nil {
			w.scyllaSub.TickError(fmt.Errorf("scylladb not connected"))
			w.logger.Warn("scylladb late connect failed", "err", err)
			return false
		}
		w.scylla = scylla
		if w.onScyllaReady != nil {
			w.onScyllaReady(scylla)
		}
		w.logger.Info("scylladb late connect succeeded — service recovering")
	}
	if err := w.scylla.Ping(); err != nil {
		w.scyllaSub.TickError(err)
		w.logger.Warn("scylladb health check failed", "err", err)

		// Attempt reconnect on failure.
		if reconnErr := w.scylla.Reconnect(); reconnErr != nil {
			w.logger.Error("scylladb reconnect failed", "err", reconnErr)
		} else {
			w.logger.Info("scylladb reconnected after health check failure")
			w.scyllaSub.Tick()
			return true
		}
		return false
	}
	w.scyllaSub.Tick()
	return true
}

// IsHealthy returns true if the metadata store (ScyllaDB) is reachable.
func (w *depHealthWatchdog) IsHealthy() bool {
	return w.healthy.Load()
}

// IsMirrorHealthy returns true if the optional MinIO mirror is reachable.
// Mirror availability does not affect service health.
func (w *depHealthWatchdog) IsMirrorHealthy() bool {
	return w.mirrorOK.Load()
}

// RequireHealthy returns a gRPC UNAVAILABLE error if ScyllaDB is down.
// MinIO mirror unavailability does NOT trigger this error.
func (w *depHealthWatchdog) RequireHealthy() error {
	if w.healthy.Load() {
		return nil
	}

	// Build a specific error message.
	var problems []string
	snap := subsystem.SubsystemSnapshot()
	for _, s := range snap {
		if s.Name == "dep:scylladb" && s.State >= subsystem.SubsystemDegraded {
			problems = append(problems, fmt.Sprintf("scylladb: %s (%s)", s.State, s.LastError))
		}
	}

	msg := "repository service unavailable: metadata store (ScyllaDB) is down"
	if len(problems) > 0 {
		msg += " — " + strings.Join(problems, "; ")
	}
	return status.Error(codes.Unavailable, msg)
}
