package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/backup_hook"
	"github.com/globulario/services/golang/backup_hook/backup_hookpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
)

// newBackupHookHandler creates a backup hook handler for the title service.
// PrepareBackup: close all Bleve indexes to flush writes and release lock files.
// FinalizeBackup: reopen all indexes so normal operation resumes.
func (srv *server) newBackupHookHandler() *backup_hook.HookHandler {
	h := backup_hook.NewHookHandler(
		srv.Name,
		true, // write-gate: block title RPCs during backup
		srv.flushForBackup,
		srv.resumeAfterBackup,
	)
	h.OnDatasets = srv.serviceDataForBackup
	return h
}

func (srv *server) flushForBackup(ctx context.Context, backupID string) (map[string]string, error) {
	details := make(map[string]string)

	// Close all Bleve indexes — flushes pending writes and releases lock files
	// so restic can snapshot a consistent on-disk state.
	srv.indexPathsBeforeBackup = make([]string, 0, len(srv.indexs))
	for path, idx := range srv.indexs {
		if idx == nil {
			continue
		}
		srv.indexPathsBeforeBackup = append(srv.indexPathsBeforeBackup, path)
		if err := idx.Close(); err != nil {
			slog.Warn("backup hook: close index failed", "index", path, "error", err)
			details["index_"+filepath.Base(path)] = fmt.Sprintf("close_error: %v", err)
		} else {
			details["index_"+filepath.Base(path)] = "closed"
		}
	}
	// Clear the map so no stale handles remain.
	srv.indexs = make(map[string]bleve.Index)
	details["indices_closed"] = fmt.Sprintf("%d", len(srv.indexPathsBeforeBackup))

	// Check association stores are healthy.
	storeCount := 0
	if srv.associations != nil {
		srv.associations.Range(func(key any, val any) bool {
			name, _ := key.(string)
			store, ok := val.(storage_store.Store)
			if !ok || store == nil {
				return true
			}
			storeCount++
			details["store_"+name] = "ok"
			return true
		})
	}
	details["stores_checked"] = fmt.Sprintf("%d", storeCount)

	return details, nil
}

func (srv *server) resumeAfterBackup(ctx context.Context, backupID string) error {
	// Reopen all Bleve indexes that were closed during PrepareBackup.
	for _, path := range srv.indexPathsBeforeBackup {
		if _, err := srv.getIndex(path); err != nil {
			slog.Error("backup hook: reopen index failed", "path", path, "error", err)
		} else {
			slog.Info("backup hook: index reopened", "path", path)
		}
	}
	srv.indexPathsBeforeBackup = nil
	return nil
}

// serviceDataForBackup reports the title service's local data paths.
func (srv *server) serviceDataForBackup(_ context.Context, _ string) ([]*backup_hookpb.ServiceDataEntry, error) {
	dataDir := filepath.Clean(config.GetDataDir())
	var entries []*backup_hookpb.ServiceDataEntry

	// 1. Bleve search indexes — AUTHORITATIVE (restored by default for fast recovery).
	// Use saved paths if indexes are currently closed for backup.
	indexPaths := srv.indexPathsBeforeBackup
	if len(indexPaths) == 0 {
		indexPaths = make([]string, 0, len(srv.indexs))
		for path := range srv.indexs {
			indexPaths = append(indexPaths, path)
		}
	}
	for _, path := range indexPaths {
		entries = append(entries, &backup_hookpb.ServiceDataEntry{
			ServiceName:      srv.Name,
			DatasetName:      "bleve_index_" + filepath.Base(path),
			Path:             path,
			DataClass:        "AUTHORITATIVE",
			Description:      fmt.Sprintf("Bleve search index: %s", filepath.Base(path)),
			BackupByDefault:  true,
			RestoreByDefault: true,
			PathExists:       dirExists(path),
			SizeBytes:        dirSize(path),
			RebuildSupported: true, // can still be rebuilt from ScyllaDB if needed
			Scope:            "node",
		})
	}

	// 2. Association stores — REBUILDABLE file-title linkage data.
	if srv.associations != nil {
		storeCount := 0
		srv.associations.Range(func(key, val any) bool {
			_, _ = key.(string)
			store, ok := val.(storage_store.Store)
			if !ok || store == nil {
				return true
			}
			storeCount++
			return true
		})
		if storeCount > 0 {
			entries = append(entries, &backup_hookpb.ServiceDataEntry{
				ServiceName:      srv.Name,
				DatasetName:      "file_associations",
				Path:             dataDir,
				DataClass:        "AUTHORITATIVE",
				Description:      fmt.Sprintf("%d file-title association store(s) (%s)", storeCount, srv.CacheType),
				BackupByDefault:  true,
				RestoreByDefault: true, // no automated rebuild exists; associations are authoritative
				PathExists:       true,
				SizeBytes:        0,
				RebuildSupported: false,
				Scope:            "node",
			})
		}
	}

	// 3. Title metadata directory — AUTHORITATIVE per-index enrichment data.
	metaDir := filepath.Join(dataDir, "title_metadata")
	if dirExists(metaDir) {
		entries = append(entries, &backup_hookpb.ServiceDataEntry{
			ServiceName:      srv.Name,
			DatasetName:      "title_metadata",
			Path:             metaDir,
			DataClass:        "AUTHORITATIVE",
			Description:      "title metadata cache (per-index enrichment data)",
			BackupByDefault:  true,
			RestoreByDefault: true,
			PathExists:       true,
			SizeBytes:        dirSize(metaDir),
			RebuildSupported: false,
			Scope:            "node",
		})
	}

	// 4. Watching store — AUTHORITATIVE user watch history.
	watchDir := filepath.Join(dataDir, "watching")
	if dirExists(watchDir) {
		entries = append(entries, &backup_hookpb.ServiceDataEntry{
			ServiceName:      srv.Name,
			DatasetName:      "watching",
			Path:             watchDir,
			DataClass:        "AUTHORITATIVE",
			Description:      "user watch history store",
			BackupByDefault:  true,
			RestoreByDefault: true,
			PathExists:       true,
			SizeBytes:        dirSize(watchDir),
			RebuildSupported: false,
			Scope:            "node",
		})
	}

	return entries, nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func dirSize(path string) uint64 {
	var total uint64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += uint64(info.Size())
		return nil
	})
	return total
}
