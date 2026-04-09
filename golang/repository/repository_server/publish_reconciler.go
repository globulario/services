package main

// publish_reconciler.go — Background goroutine that retries stuck VERIFIED artifacts.
//
// If completePublish fails during UploadArtifact, the artifact stays in VERIFIED
// state. This reconciler periodically scans for such artifacts and retries
// promotion, bounded by a per-artifact retry limit.

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

const (
	publishReconcileInterval = 60 * time.Second
	publishRetryThreshold    = 30 * time.Second // artifact must be older than this to retry
	publishMaxRetries        = 3
)

// publishReconciler retries promotion for artifacts stuck in VERIFIED state.
type publishReconciler struct {
	srv      *server
	interval time.Duration

	mu      sync.Mutex
	retries map[string]int // artifact key → retry count
}

func newPublishReconciler(srv *server) *publishReconciler {
	return &publishReconciler{
		srv:      srv,
		interval: publishReconcileInterval,
		retries:  make(map[string]int),
	}
}

// Start launches the reconciler as a background goroutine.
func (pr *publishReconciler) Start(ctx context.Context) {
	slog.Info("publish-reconciler: started", "interval", pr.interval)
	go func() {
		ticker := time.NewTicker(pr.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pr.reconcileOnce(ctx)
			}
		}
	}()
}

func (pr *publishReconciler) reconcileOnce(ctx context.Context) {
	entries, err := pr.srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		slog.Debug("publish-reconciler: ReadDir failed", "err", err)
		return // directory may not exist yet
	}

	now := time.Now()
	verified := 0

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")

		_, state, manifest, err := pr.srv.readManifestAndStateByKey(ctx, key)
		if err != nil {
			continue
		}
		if state != repopb.PublishState_VERIFIED {
			continue
		}
		verified++

		// Only retry artifacts that have been stuck for longer than the threshold.
		// Use ModifiedUnix as a proxy for when the artifact was last written.
		modifiedAt := time.Unix(manifest.GetModifiedUnix(), 0)
		if modifiedAt.IsZero() || now.Sub(modifiedAt) < publishRetryThreshold {
			continue
		}

		// Check retry limit.
		pr.mu.Lock()
		count := pr.retries[key]
		if count >= publishMaxRetries {
			pr.mu.Unlock()
			continue
		}
		pr.retries[key] = count + 1
		pr.mu.Unlock()

		ref := manifest.GetRef()
		slog.Info("publish-reconciler: retrying stuck VERIFIED artifact",
			"key", key,
			"publisher", ref.GetPublisherId(),
			"name", ref.GetName(),
			"version", ref.GetVersion(),
			"attempt", count+1,
		)

		if err := pr.srv.completePublish(ctx, manifest, key, nil); err != nil {
			slog.Error("publish-reconciler: retry failed",
				"key", key,
				"publisher", ref.GetPublisherId(),
				"name", ref.GetName(),
				"version", ref.GetVersion(),
				"attempt", count+1,
				"err", err,
			)
		} else {
			slog.Info("publish-reconciler: artifact promoted to PUBLISHED",
				"key", key,
				"name", ref.GetName(),
				"version", ref.GetVersion(),
			)
			// Clear retry count on success.
			pr.mu.Lock()
			delete(pr.retries, key)
			pr.mu.Unlock()
		}
	}

	if verified > 0 {
		slog.Info("publish-reconciler: scan complete", "manifests", len(entries), "verified", verified)
	}
}
