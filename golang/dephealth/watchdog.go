// Package dephealth provides a reusable dependency health watchdog for Globular services.
//
// In a distributed mesh, every service depends on shared infrastructure (ScyllaDB, MinIO).
// When those dependencies are down, the service MUST mark itself as broken — not silently
// degrade. This package provides a standard watchdog that:
//
//   - Pings each dependency every 15 seconds
//   - Registers subsystems in the global subsystem registry (visible to cluster_doctor)
//   - Gates RPCs with codes.Unavailable when any dependency is down
//   - Auto-recovers when dependencies come back
//
// Usage:
//
//	w := dephealth.NewWatchdog(logger,
//	    dephealth.Dep("scylladb", func(ctx context.Context) error { return session.Query("SELECT now() FROM system.local").Exec() }),
//	    dephealth.Dep("minio", func(ctx context.Context) error { return storage.Ping(ctx) }),
//	)
//	go w.Start(ctx)
//	// In RPC handlers:
//	if err := w.RequireHealthy(); err != nil { return nil, err }
package dephealth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/subsystem"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	checkInterval = 15 * time.Second
	checkTimeout  = 5 * time.Second
)

// Dependency represents a named infrastructure dependency with a ping function.
type Dependency struct {
	Name string
	Ping func(ctx context.Context) error
}

// Dep is a convenience constructor for Dependency.
func Dep(name string, ping func(ctx context.Context) error) Dependency {
	return Dependency{Name: name, Ping: ping}
}

// Watchdog monitors distributed dependencies and gates RPCs when they're down.
type Watchdog struct {
	deps    []Dependency
	subs    []*subsystem.SubsystemHandle
	healthy atomic.Bool
	logger  *slog.Logger
}

// NewWatchdog creates a watchdog that monitors the given dependencies.
func NewWatchdog(logger *slog.Logger, deps ...Dependency) *Watchdog {
	w := &Watchdog{
		deps:   deps,
		subs:   make([]*subsystem.SubsystemHandle, len(deps)),
		logger: logger,
	}
	w.healthy.Store(true) // optimistic at start
	for i, d := range deps {
		w.subs[i] = subsystem.RegisterSubsystem("dep:"+d.Name, checkInterval)
	}
	return w
}

// Start launches the health check loop. Blocks until ctx is cancelled.
func (w *Watchdog) Start(ctx context.Context) {
	// First check runs immediately.
	w.check(ctx)

	ticker := time.NewTicker(checkInterval)
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

func (w *Watchdog) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	allOK := true
	for i, d := range w.deps {
		if err := d.Ping(checkCtx); err != nil {
			w.subs[i].TickError(err)
			w.logger.Warn("dependency health check failed", "dep", d.Name, "err", err)
			allOK = false
		} else {
			w.subs[i].Tick()
		}
	}

	wasHealthy := w.healthy.Load()
	w.healthy.Store(allOK)

	if wasHealthy && !allOK {
		w.logger.Error("dependencies UNHEALTHY — service is NOT_SERVING")
	} else if !wasHealthy && allOK {
		w.logger.Info("dependencies recovered — service is SERVING")
	}
}

// IsHealthy returns true if all dependencies are reachable.
func (w *Watchdog) IsHealthy() bool {
	return w.healthy.Load()
}

// RequireHealthy returns a gRPC UNAVAILABLE error if any dependency is down.
// Call at the top of RPC handlers.
func (w *Watchdog) RequireHealthy() error {
	if w == nil || w.healthy.Load() {
		return nil
	}

	var problems []string
	snap := subsystem.SubsystemSnapshot()
	for _, s := range snap {
		if !strings.HasPrefix(s.Name, "dep:") {
			continue
		}
		if s.State >= subsystem.SubsystemDegraded {
			problems = append(problems, fmt.Sprintf("%s: %s (%s)",
				strings.TrimPrefix(s.Name, "dep:"), s.State, s.LastError))
		}
	}

	msg := "service unavailable: distributed dependencies are down"
	if len(problems) > 0 {
		msg += " — " + strings.Join(problems, "; ")
	}
	return status.Error(codes.Unavailable, msg)
}
