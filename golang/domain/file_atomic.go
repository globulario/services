package domain

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to a file atomically by writing to a temp file
// first, then renaming it to the target path. This prevents partial writes
// from being observed by readers (e.g., certificate watchers, Envoy SDS).
//
// The temp file is created in the same directory as the target to ensure the
// rename operation is atomic (same filesystem). If the write fails, the temp
// file is removed.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	// Create temp file in same directory (ensures atomic rename on same filesystem)
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	tempPath := tempFile.Name()

	// Ensure temp file is removed on failure
	defer func() {
		if tempFile != nil {
			tempFile.Close()
			os.Remove(tempPath)
		}
	}()

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Close temp file before rename
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions before rename
	if err := os.Chmod(tempPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename (on same filesystem)
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Success - prevent cleanup
	tempFile = nil
	return nil
}
