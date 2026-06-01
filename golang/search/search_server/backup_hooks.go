package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/backup_hook"
	"github.com/globulario/services/golang/backup_hook/backup_hookpb"
)

// newBackupHookHandler creates a backup hook handler for the search service.
// PrepareBackup: close all Bleve indexes to flush writes and release lock files.
// FinalizeBackup: reopen all indexes so normal operation resumes.
func (srv *server) newBackupHookHandler() *backup_hook.HookHandler {
	h := backup_hook.NewHookHandler(
		srv.Name,
		true, // write-gate: block indexing RPCs during backup
		srv.flushForBackup,
		srv.resumeAfterBackup,
	)
	h.OnDatasets = srv.serviceDataForBackup
	return h
}

func (srv *server) flushForBackup(ctx context.Context, backupID string) (map[string]string, error) {
	details := make(map[string]string)

	if srv.search_engine == nil {
		details["engine_status"] = "not_initialized"
		return details, nil
	}

	// Save the list of open index paths before closing.
	srv.indexPathsBeforeBackup = srv.search_engine.GetIndexPaths()
	details["index_count"] = fmt.Sprintf("%d", len(srv.indexPathsBeforeBackup))

	// Close all indexes — flushes pending writes and releases lock files
	// so restic can snapshot a consistent on-disk state.
	if err := srv.search_engine.CloseAll(); err != nil {
		details["close_error"] = err.Error()
		return details, fmt.Errorf("close indexes for backup: %w", err)
	}

	details["engine_status"] = "closed_for_backup"
	return details, nil
}

func (srv *server) resumeAfterBackup(ctx context.Context, backupID string) error {
	if srv.search_engine == nil || len(srv.indexPathsBeforeBackup) == 0 {
		return nil
	}
	err := srv.search_engine.ReopenAll(srv.indexPathsBeforeBackup)
	srv.indexPathsBeforeBackup = nil
	return err
}

// serviceDataForBackup reports the search service's Bleve index paths as
// AUTHORITATIVE data so restic includes them in backup and restore.
func (srv *server) serviceDataForBackup(_ context.Context, _ string) ([]*backup_hookpb.ServiceDataEntry, error) {
	if srv.search_engine == nil {
		return nil, nil
	}

	// Use saved paths if indexes are currently closed for backup,
	// otherwise read live paths from the engine.
	paths := srv.indexPathsBeforeBackup
	if len(paths) == 0 {
		paths = srv.search_engine.GetIndexPaths()
	}

	var entries []*backup_hookpb.ServiceDataEntry

	for _, path := range paths {
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
			RebuildSupported: true, // can still be rebuilt from source if needed
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
