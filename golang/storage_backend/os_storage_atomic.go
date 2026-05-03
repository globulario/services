package storage_backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// WriteFileAtomic streams r into a temporary file in the same directory as
// key, verifies the sha256 and size, then renames the temp file into place.
//
// Rules:
//   - The write is atomic from the OS perspective (rename is atomic on Linux).
//   - If expectedSha256 is non-empty, the actual digest must match.
//   - If expectedSize > 0, the actual byte count must match.
//   - The temp file is removed on any error, including digest/size mismatch.
//
// Returns the actual sha256 digest ("sha256:<hex>") and any error.
func (s *OSStorage) WriteFileAtomic(
	ctx context.Context,
	key string,
	r io.Reader,
	expectedSha256 string,
	expectedSize int64,
) (actualSha256 string, err error) {
	abs := s.resolve(key)
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Write to temp file in the same directory so rename is same-filesystem.
	tmp := abs + ".tmp." + uuid.New().String()
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		// Remove temp on any failure path.
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()

	h := sha256.New()
	limited := io.LimitReader(r, maxStreamBytes+1)
	n, copyErr := io.Copy(io.MultiWriter(f, h), limited)
	if syncErr := f.Sync(); syncErr != nil && copyErr == nil {
		copyErr = syncErr
	}
	_ = f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("write temp file: %w", copyErr)
	}
	if n > maxStreamBytes {
		return "", fmt.Errorf("artifact exceeds %d bytes limit", maxStreamBytes)
	}

	actual := "sha256:" + hex.EncodeToString(h.Sum(nil))

	if expectedSha256 != "" && actual != expectedSha256 {
		return "", fmt.Errorf("sha256 mismatch: expected %s got %s", expectedSha256, actual)
	}
	if expectedSize > 0 && n != expectedSize {
		return "", fmt.Errorf("size mismatch: expected %d bytes got %d", expectedSize, n)
	}

	if err := os.Rename(tmp, abs); err != nil {
		return "", fmt.Errorf("atomic rename to %s: %w", abs, err)
	}
	return actual, nil
}

// maxStreamBytes caps a single artifact stream at 500 MiB.
const maxStreamBytes int64 = 500 << 20
