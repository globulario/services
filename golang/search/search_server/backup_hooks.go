package main

import (
	"context"

	"github.com/globulario/services/golang/backup_hook"
)

// newBackupHookHandler creates a backup hook handler for the search service.
// Search holds Bleve indices in memory (map[string]bleve.Index).
// PrepareBackup: no explicit flush needed (Bleve persists on each Index() call),
// but we verify the engine is healthy.
// FinalizeBackup: no-op.
func (srv *server) newBackupHookHandler() *backup_hook.HookHandler {
	return backup_hook.NewHookHandler(
		srv.Name,
		false, // no write-gate
		srv.flushForBackup,
		nil,
	)
}

func (srv *server) flushForBackup(ctx context.Context, backupID string) (map[string]string, error) {
	details := make(map[string]string)

	if srv.search_engine != nil {
		details["engine_version"] = srv.search_engine.GetVersion()
		details["engine_status"] = "healthy"
	} else {
		details["engine_status"] = "not_initialized"
	}

	return details, nil
}
