// migration.go — background migration of local user files to MinIO.
package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const minioMigratedMarker = ".minio-migrated"

// migrateLocalUsersToMinio walks the local users/ directory and uploads
// files to MinIO storage. It is idempotent: files that already exist in
// MinIO are skipped, and a marker file prevents re-running.
func (srv *server) migrateLocalUsersToMinio(ctx context.Context) {
	if !srv.minioEnabled() {
		return
	}

	localUsersDir := filepath.Join(srv.Root, "users")
	if _, err := os.Stat(localUsersDir); os.IsNotExist(err) {
		return // no local user files to migrate
	}

	markerPath := filepath.Join(localUsersDir, minioMigratedMarker)
	if _, err := os.Stat(markerPath); err == nil {
		slog.Debug("minio migration already completed", "marker", markerPath)
		return // already migrated
	}

	slog.Info("starting local→MinIO user file migration", "source", localUsersDir)

	var migrated, skipped, errors int
	err := filepath.WalkDir(localUsersDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == minioMigratedMarker {
			return nil
		}

		// Compute the virtual path: /users/...
		relPath, err := filepath.Rel(srv.Root, path)
		if err != nil {
			return nil
		}
		virtualPath := "/" + filepath.ToSlash(relPath)

		// Skip if already in MinIO
		if srv.storage.Exists(ctx, virtualPath) {
			skipped++
			return nil
		}

		// Read local file
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("migration: read failed", "path", path, "err", err)
			errors++
			return nil
		}

		// Ensure parent dir exists in MinIO
		parentDir := "/" + filepath.ToSlash(strings.TrimPrefix(filepath.Dir(relPath), "/"))
		if mkErr := srv.storage.MkdirAll(ctx, parentDir, 0o755); mkErr != nil {
			slog.Warn("migration: mkdir failed", "path", parentDir, "err", mkErr)
			errors++
			return nil
		}

		// Write to MinIO
		if wErr := srv.storage.WriteFile(ctx, virtualPath, data, 0o644); wErr != nil {
			slog.Warn("migration: write failed", "path", virtualPath, "err", wErr)
			errors++
			return nil
		}

		migrated++
		if migrated%100 == 0 {
			slog.Info("migration progress", "migrated", migrated, "skipped", skipped, "errors", errors)
		}
		return nil
	})

	if err != nil {
		slog.Error("migration walk failed", "err", err)
		return
	}

	slog.Info("local→MinIO migration complete", "migrated", migrated, "skipped", skipped, "errors", errors)

	// Write marker to prevent re-running
	if err := os.WriteFile(markerPath, []byte("migrated\n"), 0o644); err != nil {
		slog.Warn("migration: failed to write marker", "path", markerPath, "err", err)
	}
}
