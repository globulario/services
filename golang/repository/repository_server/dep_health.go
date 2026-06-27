package main

// dep_health.go — capability-aware dependency health watchdog for the repository service.
//
// The repository's blob authority is the local POSIX CAS, full stop. Packages
// never live in MinIO — not even as a mirror tier (operator decision
// 2026-06-12). The only distributed dependency that gates capabilities is
// ScyllaDB (the package index). The repository therefore operates at two
// capability tiers:
//
//   FULL       — ScyllaDB healthy
//   READ_ONLY  — ScyllaDB down (writes/queries blocked; verified local POSIX
//                reads still answer — the local CAS is the installability authority)
//
// Capability constants (used with RequireCapability):
//   CapRepoWrite  — any operation that writes Scylla state; blocked when Scylla is down
//   CapRepoQuery  — Scylla-indexed reads (listing, search, resolution); blocked when Scylla is down
//   CapRepoRead   — verified local POSIX reads (download, verify, explain); never blocked
//
// The gRPC health protocol (SERVING / NOT_SERVING) is independent of operational mode.
// A service may be SERVING with mode READ_ONLY. Operational mode is exposed via
// GetRepositoryStatus, not via the gRPC health endpoint.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/operational"
	"github.com/globulario/services/golang/subsystem"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Capability constants ───────────────────────────────────────────────────────

// Repository capability names — used with server.requireCapability().
const (
	// CapRepoWrite gates any operation that writes Scylla publish state
	// (upload, publish, import, delete, sync, gc, state transitions, receipts,
	// signing writes, upstream writes). Blocked when ScyllaDB is unavailable.
	CapRepoWrite = "repository.write"

	// CapRepoQuery gates Scylla-indexed discovery RPCs (ListArtifacts,
	// SearchArtifacts, GetArtifactVersions, ResolveArtifact, ListInstalledRevisions,
	// ListTrustedPublishers, ListArtifactSignatures, VerifyArtifactSignature,
	// ListConfigReceipts, ListUpstreams, ResolveByEntrypointChecksum).
	// Blocked when ScyllaDB is unavailable. Phase 2 will add local-receipt fallbacks.
	CapRepoQuery = "repository.query"

	// CapRepoRead gates verified local POSIX reads (DownloadArtifact, VerifyArtifact,
	// ExplainArtifact, RepairArtifact, ListRepositoryFindings). These operations work
	// directly from the local POSIX CAS and are never blocked by ScyllaDB state.
	// RequireCapability(CapRepoRead) always returns nil.
	CapRepoRead = "repository.read"
)

const (
	healthCheckInterval = 15 * time.Second
	healthCheckTimeout  = 5 * time.Second
)

// depHealthWatchdog monitors the repository's distributed dependencies.
type depHealthWatchdog struct {
	scylla        *scyllaStore
	healthy       *atomic.Bool // true = ScyllaDB is OK (service can serve writes/queries)
	initialized   *atomic.Bool // true after first check() completes; false during startup
	scyllaSub     *subsystem.SubsystemHandle
	logger        *slog.Logger
	onScyllaReady func(*scyllaStore) // called when late-connect succeeds
}

// newDepHealthWatchdog creates a watchdog that monitors ScyllaDB (the package
// index). The repository has no MinIO dependency — the local POSIX CAS is the
// sole blob authority.
func newDepHealthWatchdog(scylla *scyllaStore, logger *slog.Logger) *depHealthWatchdog {
	healthy := &atomic.Bool{}
	healthy.Store(scylla != nil) // false if Scylla was not reachable at startup

	return &depHealthWatchdog{
		scylla:      scylla,
		healthy:     healthy,
		initialized: &atomic.Bool{}, // false until first check() completes
		scyllaSub:   subsystem.RegisterSubsystem("dep:scylladb", healthCheckInterval),
		logger:      logger,
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

// check pings ScyllaDB — the only dependency that gates repository capabilities.
func (w *depHealthWatchdog) check(ctx context.Context) {
	_, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	scyllaOK := w.pingScylla()

	wasHealthy := w.healthy.Load()
	// Service is healthy as long as ScyllaDB is OK.
	// Local POSIX storage is always available — no need to gate on it.
	nowHealthy := scyllaOK
	w.healthy.Store(nowHealthy)

	// Log transitions.
	if wasHealthy && !nowHealthy {
		w.logger.Error("repository metadata (ScyllaDB) UNHEALTHY — service is NOT_SERVING",
			"scylladb", scyllaOK)
	} else if !wasHealthy && nowHealthy {
		w.logger.Info("repository metadata recovered — service is SERVING")
	}

	w.initialized.Store(true)
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

// RequireHealthy returns a gRPC UNAVAILABLE error if ScyllaDB is down.
// Deprecated: prefer RequireCapability(cap) for fine-grained capability gating.
func (w *depHealthWatchdog) RequireHealthy() error {
	return w.RequireCapability(CapRepoWrite)
}

// RequireCapability returns a gRPC error if the named capability is unavailable.
//
//   - CapRepoWrite: blocked when ScyllaDB is down (cannot write publish state)
//   - CapRepoQuery: blocked when ScyllaDB is down (cannot query indexed metadata)
//   - CapRepoRead:  never blocked (local POSIX CAS is always the installability authority)
//
// The gRPC health SERVING state is not changed by this method. A service can be
// SERVING (grpc) with mode READ_ONLY (operational).
func (w *depHealthWatchdog) RequireCapability(cap string) error {
	switch cap {
	case CapRepoRead:
		// Local POSIX reads are never blocked — localStorage is always available.
		return nil

	default: // CapRepoWrite, CapRepoQuery, and any unknown cap require ScyllaDB.
		if w.healthy.Load() {
			return nil
		}
		var problems []string
		snap := subsystem.SubsystemSnapshot()
		for _, s := range snap {
			if s.Name == "dep:scylladb" && s.State >= subsystem.SubsystemDegraded {
				problems = append(problems, fmt.Sprintf("scylladb: %s (%s)", s.State, s.LastError))
			}
		}
		msg := fmt.Sprintf("%s blocked: ScyllaDB unavailable (service mode: %s)", cap, string(w.serviceMode()))
		if len(problems) > 0 {
			msg += " — " + strings.Join(problems, "; ")
		}
		return status.Error(codes.Unavailable, msg)
	}
}

// ServiceMode returns the composite operating mode and a human-readable reason.
func (w *depHealthWatchdog) ServiceMode() (operational.ServiceHealthMode, string) {
	return w.serviceMode(), w.serviceModeReason()
}

func (w *depHealthWatchdog) serviceMode() operational.ServiceHealthMode {
	if w.initialized != nil && !w.initialized.Load() {
		return operational.ModeDegraded
	}
	if w.healthy.Load() {
		return operational.ModeFull
	}
	return operational.ModeReadOnly
}

func (w *depHealthWatchdog) serviceModeReason() string {
	if w.initialized != nil && !w.initialized.Load() {
		return "dependency_health_initializing: first check not yet complete; dependency state unproven"
	}
	if !w.healthy.Load() {
		return "scylla_unavailable: write/query capabilities blocked; verified local reads available"
	}
	return ""
}

// OperationalStatus builds the full live operational status for the service.
// It reads only atomic booleans — safe to call from any goroutine, no Scylla I/O.
func (w *depHealthWatchdog) OperationalStatus() *operational.ServiceOperationalStatus {
	scyllaOK := w.healthy.Load()
	mode, reason := w.ServiceMode()
	now := time.Now()

	scyllaDep := operational.DependencyHealth{
		Name:                "scylladb",
		Kind:                operational.DepIndex,
		Status:              operational.DepHealthy,
		AffectsCapabilities: []string{CapRepoWrite, CapRepoQuery},
	}
	if !scyllaOK {
		scyllaDep.Status = operational.DepUnavailable
		scyllaDep.Reason = "metadata store unreachable; write and query capabilities blocked"
	}

	caps := []operational.CapabilityHealth{
		capHealth(CapRepoWrite, scyllaOK, mode),
		capHealth(CapRepoQuery, scyllaOK, mode),
		capHealth(CapRepoRead, true, mode), // local reads never blocked
	}

	return &operational.ServiceOperationalStatus{
		Service:        "repository.PackageRepository",
		Mode:           mode,
		Reason:         reason,
		Dependencies:   []operational.DependencyHealth{scyllaDep},
		Capabilities:   caps,
		ObservedAt:     now,
		ObservedAtUnix: now.Unix(),
	}
}

// capHealth builds a CapabilityHealth entry from an availability boolean.
func capHealth(name string, available bool, mode operational.ServiceHealthMode) operational.CapabilityHealth {
	if available {
		return operational.CapabilityHealth{
			Name:   name,
			Status: operational.CapAvailable,
			Mode:   mode,
		}
	}
	return operational.CapabilityHealth{
		Name:   name,
		Status: operational.CapBlocked,
		Mode:   mode,
		Reason: "required dependency unavailable",
	}
}
