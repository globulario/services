package storage_backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	"github.com/google/uuid"
)

// atomicTempInfix marks a temp file created by an atomic write. The full suffix
// is atomicTempInfix followed by a UUIDv4. WriteFileAtomic and AtomicWriteFile
// both use it; the orphan-temp sweeper matches this exact shape so it can reap
// crash-orphaned temps without ever touching a committed file.
const atomicTempInfix = ".tmp."

var atomicTempRe = regexp.MustCompile(`\.tmp\.[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsAtomicTempName reports whether name is a temp file produced by an atomic
// write (WriteFileAtomic / AtomicWriteFile): "<target>.tmp.<uuidv4>". The pattern
// is defined next to the writer so the sweeper and the writer can never drift.
func IsAtomicTempName(name string) bool { return atomicTempRe.MatchString(name) }

// atomicTempPath returns a unique same-directory temp path for abs so the final
// rename stays on one filesystem (atomic on Linux).
func atomicTempPath(abs string) string { return abs + atomicTempInfix + uuid.New().String() }

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
	tmp := atomicTempPath(abs)
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

// AtomicWriteFile writes data to key atomically: it streams into a same-directory
// temp file, fsyncs, then renames it into place. Unlike WriteFileAtomic it does
// not verify a caller-supplied digest — it is for content whose bytes are already
// in hand (e.g. manifest sidecars) where the requirement is crash-safety: a reader
// sees either the prior committed file or the new one, never a partially-written
// commit. On any failure the temp file is removed and the existing file is left
// untouched.
func (s *OSStorage) AtomicWriteFile(_ context.Context, key string, data []byte, perm fs.FileMode) (err error) {
	abs := s.resolve(key)
	dir := filepath.Dir(abs)
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return fmt.Errorf("mkdir %s: %w", dir, mkErr)
	}

	tmp := atomicTempPath(abs)
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm) // #nosec G304
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer func() {
		// Remove the temp on any failure path; on success it has been renamed away.
		if err != nil {
			_ = os.Remove(tmp)
		}
	}()

	if _, werr := f.Write(data); werr != nil {
		_ = f.Close()
		err = fmt.Errorf("write temp file: %w", werr)
		return err
	}
	if serr := f.Sync(); serr != nil {
		_ = f.Close()
		err = fmt.Errorf("fsync temp file: %w", serr)
		return err
	}
	if cerr := f.Close(); cerr != nil {
		err = fmt.Errorf("close temp file: %w", cerr)
		return err
	}
	if rerr := os.Rename(tmp, abs); rerr != nil {
		err = fmt.Errorf("atomic rename to %s: %w", abs, rerr)
		return err
	}
	return nil
}

// maxStreamBytes caps a single artifact stream at 500 MiB.
const maxStreamBytes int64 = 500 << 20
