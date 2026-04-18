package main

// dep_health.go — dependency health watchdog for the repository service.
//
// The repository is the heart of the cluster. If its distributed dependencies
// (MinIO for artifact storage, ScyllaDB for manifest metadata) are down, the
// service MUST mark itself as broken — not silently degrade.
//
// The watchdog runs every 15 seconds. It pings both MinIO and ScyllaDB. If
// either is unreachable, the service transitions to NOT_SERVING:
//   - New RPCs receive codes.Unavailable with a clear dependency-failure message
//   - Subsystem registry reflects the failure (visible to cluster_doctor)
//   - Logs record every transition
//
// Recovery is automatic: once both dependencies respond to pings, the service
// transitions back to SERVING.

import (
	"context"
	"fmt"
	"log/slog"
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
	healthy       *atomic.Bool
	minioSub      *subsystem.SubsystemHandle
	scyllaSub     *subsystem.SubsystemHandle
	logger        *slog.Logger
	onScyllaReady func(*scyllaStore) // called when late-connect succeeds
}

// newDepHealthWatchdog creates a watchdog that monitors MinIO and ScyllaDB.
func newDepHealthWatchdog(storage storage_backend.Storage, scylla *scyllaStore, logger *slog.Logger) *depHealthWatchdog {
	healthy := &atomic.Bool{}
	healthy.Store(true) // optimistic at start — first check runs immediately

	return &depHealthWatchdog{
		storage:   storage,
		scylla:    scylla,
		healthy:   healthy,
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

// check pings both dependencies and updates health state.
func (w *depHealthWatchdog) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	minioOK := w.pingMinio(checkCtx)
	scyllaOK := w.pingScylla()

	wasHealthy := w.healthy.Load()
	nowHealthy := minioOK && scyllaOK
	w.healthy.Store(nowHealthy)

	// Log transitions.
	if wasHealthy && !nowHealthy {
		w.logger.Error("repository dependencies UNHEALTHY — service is NOT_SERVING",
			"minio", minioOK, "scylladb", scyllaOK)
	} else if !wasHealthy && nowHealthy {
		w.logger.Info("repository dependencies recovered — service is SERVING")
	}
}

// pingMinio checks MinIO storage reachability.
func (w *depHealthWatchdog) pingMinio(ctx context.Context) bool {
	if w.storage == nil {
		w.minioSub.TickError(fmt.Errorf("storage backend not initialized"))
		return false
	}
	if err := w.storage.Ping(ctx); err != nil {
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

// IsHealthy returns true if all distributed dependencies are reachable.
func (w *depHealthWatchdog) IsHealthy() bool {
	return w.healthy.Load()
}

// RequireHealthy returns a gRPC UNAVAILABLE error if dependencies are down.
// Call this at the top of RPC handlers that require distributed storage.
func (w *depHealthWatchdog) RequireHealthy() error {
	if w.healthy.Load() {
		return nil
	}

	// Build a specific error message.
	var problems []string
	snap := subsystem.SubsystemSnapshot()
	for _, s := range snap {
		if s.Name == "dep:minio" && s.State >= subsystem.SubsystemDegraded {
			problems = append(problems, fmt.Sprintf("minio: %s (%s)", s.State, s.LastError))
		}
		if s.Name == "dep:scylladb" && s.State >= subsystem.SubsystemDegraded {
			problems = append(problems, fmt.Sprintf("scylladb: %s (%s)", s.State, s.LastError))
		}
	}

	msg := "repository service unavailable: distributed dependencies are down"
	if len(problems) > 0 {
		msg += " — "
		for i, p := range problems {
			if i > 0 {
				msg += "; "
			}
			msg += p
		}
	}
	return status.Error(codes.Unavailable, msg)
}
