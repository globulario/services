// @awareness namespace=globular.platform
// @awareness component=platform_repository
// @awareness file_role=publish_pipeline_state_reconciler
// @awareness implements=globular.platform:intent.repository.lifecycle_state_machine
// @awareness risk=high
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
	now := time.Now()
	verified := 0

	// Scylla-first: use ledger rows to find stuck VERIFIED artifacts.
	// Falls back to MinIO directory scan only when Scylla is nil.
	type candidate struct {
		key      string
		manifest *repopb.ArtifactManifest
	}
	var candidates []candidate

	if pr.srv.scylla != nil {
		rows, err := pr.srv.scylla.ListManifests(ctx)
		if err != nil {
			slog.Debug("publish-reconciler: Scylla list failed", "err", err)
			return
		}
		for _, row := range rows {
			if row.PublishState != repopb.PublishState_VERIFIED.String() {
				continue
			}
			m, _, parseErr := manifestFromRow(row)
			if parseErr != nil {
				continue
			}
			candidates = append(candidates, candidate{key: row.ArtifactKey, manifest: m})
		}
	} else {
		entries, err := pr.srv.Storage().ReadDir(ctx, artifactsDir)
		if err != nil {
			slog.Debug("publish-reconciler: ReadDir failed", "err", err)
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".manifest.json") {
				continue
			}
			key := strings.TrimSuffix(name, ".manifest.json")
			_, state, manifest, err := pr.srv.readManifestAndStateByKey(ctx, key)
			if err != nil || state != repopb.PublishState_VERIFIED {
				continue
			}
			candidates = append(candidates, candidate{key: key, manifest: manifest})
		}
	}

	for _, c := range candidates {
		verified++
		manifest := c.manifest

		// Only retry artifacts that have been stuck for longer than the threshold.
		modifiedAt := time.Unix(manifest.GetModifiedUnix(), 0)
		if modifiedAt.IsZero() || now.Sub(modifiedAt) < publishRetryThreshold {
			continue
		}

		// Check retry limit.
		pr.mu.Lock()
		count := pr.retries[c.key]
		if count >= publishMaxRetries {
			pr.mu.Unlock()
			continue
		}
		pr.retries[c.key] = count + 1
		pr.mu.Unlock()

		ref := manifest.GetRef()
		slog.Info("publish-reconciler: retrying stuck VERIFIED artifact",
			"key", c.key,
			"publisher", ref.GetPublisherId(),
			"name", ref.GetName(),
			"version", ref.GetVersion(),
			"attempt", count+1,
		)

		if err := pr.srv.completePublish(ctx, manifest, c.key, nil); err != nil {
			slog.Error("publish-reconciler: retry failed",
				"key", c.key,
				"publisher", ref.GetPublisherId(),
				"name", ref.GetName(),
				"version", ref.GetVersion(),
				"attempt", count+1,
				"err", err,
			)
		} else {
			slog.Info("publish-reconciler: artifact promoted to PUBLISHED",
				"key", c.key,
				"name", ref.GetName(),
				"version", ref.GetVersion(),
			)
			pr.mu.Lock()
			delete(pr.retries, c.key)
			pr.mu.Unlock()
		}
	}

	if verified > 0 {
		slog.Info("publish-reconciler: scan complete", "verified", verified)
	}
}
