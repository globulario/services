package main

import (
	"context"

	"github.com/globulario/services/golang/backup_hook"
)

// newBackupHookHandler creates a backup hook handler for the catalog service.
// Catalog delegates all storage to the Persistence service (no local state).
// PrepareBackup: verify persistence connectivity.
// FinalizeBackup: no-op.
func (srv *server) newBackupHookHandler() *backup_hook.HookHandler {
	return backup_hook.NewHookHandler(
		srv.Name,
		false, // no write-gate needed — no local state
		srv.flushForBackup,
		nil,
	)
}

func (srv *server) flushForBackup(ctx context.Context, backupID string) (map[string]string, error) {
	details := make(map[string]string)

	// Catalog has no local cache or queue.
	// Best we can do: verify persistence client is reachable.
	if srv.persistenceClient != nil {
		details["persistence_status"] = "connected"
	} else {
		details["persistence_status"] = "not_initialized"
	}

	return details, nil
}
