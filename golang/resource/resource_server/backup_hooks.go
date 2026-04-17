package main

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/backup_hook"
)

// newBackupHookHandler creates a backup hook handler for the resource service.
// Resource is stateful: it writes to Scylla/SQL.
// PrepareBackup: verify store health so backup captures consistent DB state.
// FinalizeBackup: no-op (no background workers to resume).
func (s *server) newBackupHookHandler() *backup_hook.HookHandler {
	return backup_hook.NewHookHandler(
		s.Name,
		false, // write-gate disabled by default
		s.flushForBackup,
		nil, // no resume needed
	)
}

func (s *server) flushForBackup(ctx context.Context, backupID string) (map[string]string, error) {
	details := make(map[string]string)

	// Ensure the persistence store connection is healthy before snapshot
	s.storeMu.Lock()
	store := s.store
	s.storeMu.Unlock()

	if store != nil {
		if err := store.Ping(ctx, "local_resource"); err != nil {
			return details, fmt.Errorf("persistence store health check failed: %w", err)
		}
		details["store_status"] = "healthy"
	} else {
		details["store_status"] = "not_initialized"
	}

	return details, nil
}
