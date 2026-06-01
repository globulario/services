package main

// dep_health.go — capability-aware dependency health watchdog for the repository service.
//
// The repository operates at three capability tiers depending on which dependencies
// are available:
//
//   FULL        — ScyllaDB healthy, MinIO healthy
//   DEGRADED    — ScyllaDB healthy, MinIO down (mirror skipped; all core capabilities work)
//   READ_ONLY   — ScyllaDB down, MinIO healthy  (writes blocked; local reads work)
//   LOCAL_ONLY  — ScyllaDB down, MinIO down     (writes blocked; only local POSIX CAS)
//
// Capability constants (used with RequireCapability):
//   CapRepoWrite  — any operation that writes Scylla state; blocked when Scylla is down
//   CapRepoQuery  — Scylla-indexed reads (listing, search, resolution); blocked when Scylla is down
//   CapRepoRead   — verified local POSIX reads (download, verify, explain); never blocked
//   CapRepoMirror — MinIO mirror writes; blocked when mirror is down
//
// The gRPC health protocol (SERVING / NOT_SERVING) is independent of operational mode.
// A service may be SERVING with mode DEGRADED or READ_ONLY. Operational mode is
// exposed via GetRepositoryStatus, not via the gRPC health endpoint.

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/operational"
	"github.com/globulario/services/golang/storage_backend"
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

	// CapRepoMirror gates MinIO mirror write operations. Blocked when the mirror
	// is unavailable. Mirror unavailability never blocks CapRepoWrite, CapRepoQuery,
	// or CapRepoRead — local POSIX CAS is the installability authority.
	CapRepoMirror = "repository.mirror"
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
	initialized   *atomic.Bool // true after first check() completes; false during startup
	minioSub      *subsystem.SubsystemHandle
	scyllaSub     *subsystem.SubsystemHandle
	logger        *slog.Logger
	onScyllaReady func(*scyllaStore) // called when late-connect succeeds
}

// newDepHealthWatchdog creates a watchdog that monitors MinIO mirror and ScyllaDB.
// storage is the mirror storage (used only for mirror ping); it may be nil.
func newDepHealthWatchdog(storage storage_backend.Storage, scylla *scyllaStore, logger *slog.Logger) *depHealthWatchdog {
	healthy := &atomic.Bool{}
	healthy.Store(scylla != nil) // false if Scylla was not reachable at startup

	mirrorOK := &atomic.Bool{}
	mirrorOK.Store(true) // optimistic — first check runs immediately

	return &depHealthWatchdog{
		storage:     storage,
		scylla:      scylla,
		healthy:     healthy,
		mirrorOK:    mirrorOK,
		initialized: &atomic.Bool{}, // false until first check() completes
		minioSub:    subsystem.RegisterSubsystem("dep:minio", healthCheckInterval),
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

	w.initialized.Store(true)
}

// pingMinio checks mirror (MinIO) reachability — informational only.
// On each successful Ping it also runs a canary PUT/GET/DELETE to verify
// the mirror can actually serve writes. A canary failure disables the mirror
// until the next successful canary, but never blocks repository RPCs.
func (w *depHealthWatchdog) pingMinio(ctx context.Context) bool {
	if w.storage == nil {
		w.minioSub.Tick()
		return true
	}
	// For a ResilientStorage, ping only the mirror component.
	if rs, ok := w.storage.(*storage_backend.ResilientStorage); ok {
		if err := rs.PingMirror(ctx); err != nil {
			w.minioSub.TickError(err)
			w.logger.Warn("minio mirror health check failed", "err", err)
			return false
		}
		// Mirror is reachable — run canary write/read/delete.
		if err := w.runMirrorCanary(ctx, rs); err != nil {
			w.minioSub.TickError(err)
			w.logger.Warn("minio mirror canary failed — mirror disabled until next canary", "err", err)
			return false
		}
		w.minioSub.Tick()
		return true
	}
	// Plain storage (non-resilient) — ping directly.
	if err := w.storage.Ping(ctx); err != nil {
		w.minioSub.TickError(err)
		w.logger.Warn("minio health check failed", "err", err)
		return false
	}
	w.minioSub.Tick()
	return true
}

// runMirrorCanary performs a sentinel PUT/GET/DELETE against the mirror to
// verify it can complete write operations. Uses a unique key to avoid
// conflicts between concurrent repository instances.
func (w *depHealthWatchdog) runMirrorCanary(ctx context.Context, rs *storage_backend.ResilientStorage) error {
	const canaryPath = ".globular/repo-canary"
	want := []byte("globular-repository-canary-ok")

	if err := rs.WriteMirrorFile(ctx, canaryPath, want, 0o644); err != nil {
		return fmt.Errorf("canary PUT: %w", err)
	}
	got, err := rs.ReadMirrorFile(ctx, canaryPath)
	if err != nil {
		return fmt.Errorf("canary GET: %w", err)
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("canary GET: content mismatch (got %d bytes, want %d)", len(got), len(want))
	}
	if err := rs.RemoveMirrorFile(ctx, canaryPath); err != nil {
		// Non-fatal: stale canary keys are harmless.
		w.logger.Warn("minio canary DELETE failed (non-fatal)", "err", err)
	}
	return nil
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
// Deprecated: prefer RequireCapability(cap) for fine-grained capability gating.
// MinIO mirror unavailability does NOT trigger this error.
func (w *depHealthWatchdog) RequireHealthy() error {
	return w.RequireCapability(CapRepoWrite)
}

// RequireCapability returns a gRPC error if the named capability is unavailable.
//
//   - CapRepoWrite: blocked when ScyllaDB is down (cannot write publish state)
//   - CapRepoQuery: blocked when ScyllaDB is down (cannot query indexed metadata)
//   - CapRepoRead:  never blocked (local POSIX CAS is always the installability authority)
//   - CapRepoMirror: blocked when MinIO mirror is down
//
// The gRPC health SERVING state is not changed by this method. A service can be
// SERVING (grpc) with mode DEGRADED or READ_ONLY (operational).
func (w *depHealthWatchdog) RequireCapability(cap string) error {
	switch cap {
	case CapRepoRead:
		// Local POSIX reads are never blocked — localStorage is always available.
		return nil

	case CapRepoMirror:
		if !w.mirrorOK.Load() {
			return status.Errorf(codes.Unavailable,
				"%s blocked: MinIO mirror is unavailable — mirror writes require a reachable mirror",
				cap)
		}
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
	scyllaOK := w.healthy.Load()
	mirrorOK := w.mirrorOK.Load()
	switch {
	case !scyllaOK && !mirrorOK:
		return operational.ModeLocalOnly
	case !scyllaOK:
		return operational.ModeReadOnly
	case !mirrorOK:
		return operational.ModeDegraded
	default:
		return operational.ModeFull
	}
}

func (w *depHealthWatchdog) serviceModeReason() string {
	if w.initialized != nil && !w.initialized.Load() {
		return "dependency_health_initializing: first check not yet complete; dependency state unproven"
	}
	scyllaOK := w.healthy.Load()
	mirrorOK := w.mirrorOK.Load()
	switch {
	case !scyllaOK && !mirrorOK:
		return "scylla_unavailable minio_unavailable: writes blocked; serving only verified local POSIX CAS"
	case !scyllaOK:
		return "scylla_unavailable: write/query capabilities blocked; verified local reads available"
	case !mirrorOK:
		return "minio_mirror_unavailable: mirror writes skipped; local POSIX CAS is authoritative"
	default:
		return ""
	}
}

// OperationalStatus builds the full live operational status for the service.
// It reads only atomic booleans — safe to call from any goroutine, no Scylla I/O.
func (w *depHealthWatchdog) OperationalStatus() *operational.ServiceOperationalStatus {
	scyllaOK := w.healthy.Load()
	mirrorOK := w.mirrorOK.Load()
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

	minioDep := operational.DependencyHealth{
		Name:                "minio_mirror",
		Kind:                operational.DepMirror,
		Status:              operational.DepHealthy,
		AffectsCapabilities: []string{CapRepoMirror},
	}
	if !mirrorOK {
		minioDep.Status = operational.DepUnavailable
		minioDep.Reason = "mirror unreachable; mirror writes skipped; local POSIX CAS remains authoritative"
	}

	caps := []operational.CapabilityHealth{
		capHealth(CapRepoWrite, scyllaOK, mode),
		capHealth(CapRepoQuery, scyllaOK, mode),
		capHealth(CapRepoRead, true, mode),   // local reads never blocked
		capHealth(CapRepoMirror, mirrorOK, mode),
	}

	return &operational.ServiceOperationalStatus{
		Service:        "repository.PackageRepository",
		Mode:           mode,
		Reason:         reason,
		Dependencies:   []operational.DependencyHealth{scyllaDep, minioDep},
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
