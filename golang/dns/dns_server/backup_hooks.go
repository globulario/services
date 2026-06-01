// @awareness namespace=globular.platform
// @awareness component=platform_dns.backup_hooks
// @awareness file_role=dns_zone_state_durable_backup_via_badger_syncwrites
// @awareness implements=globular.platform:intent.dns.zone_list_never_regresses_to_empty
// @awareness risk=medium
package main

import (
	"context"

	"github.com/globulario/services/golang/backup_hook"
)

// newBackupHookHandler creates a backup hook handler for the DNS service.
// DNS stores records in Badger KV (syncWrites=true, so data is durable on write).
// PrepareBackup: verify store health.
// FinalizeBackup: no-op.
func (s *server) newBackupHookHandler() *backup_hook.HookHandler {
	return backup_hook.NewHookHandler(
		s.Name,
		false, // no write-gate
		s.flushForBackup,
		nil,
	)
}

func (s *server) flushForBackup(ctx context.Context, backupID string) (map[string]string, error) {
	details := make(map[string]string)

	if s.store != nil && s.connection_is_open {
		details["store_status"] = "healthy"
	} else {
		details["store_status"] = "not_connected"
	}

	return details, nil
}
