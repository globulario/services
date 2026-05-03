package storage_backend

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
)

// ResilientStorage implements Storage with a mandatory local POSIX store and an
// optional best-effort mirror (e.g. MinIO).
//
// Read path:  try local first; on miss, try mirror and populate local (cache-fill).
// Write path: write to local (mandatory); write to mirror (best-effort, logged on failure).
// Mutations (Remove, Rename, MkdirAll, etc.): local only.
// Mirror unavailability never causes an error visible to callers.
type ResilientStorage struct {
	local  Storage
	mirror Storage // may be nil
}

// NewResilientStorage creates a ResilientStorage.
// local must not be nil.
// mirror may be nil (MinIO optional mode).
func NewResilientStorage(local Storage, mirror Storage) *ResilientStorage {
	return &ResilientStorage{
		local:  local,
		mirror: mirror,
	}
}

// IsMirrorAvailable returns true if s is a *ResilientStorage and its mirror is non-nil.
func IsMirrorAvailable(s Storage) bool {
	rs, ok := s.(*ResilientStorage)
	return ok && rs.mirror != nil
}

// -----------------------------------------------------------------------------
// Storage interface — read operations
// -----------------------------------------------------------------------------

// ReadFile reads from local first. On error (e.g. cache miss), falls back to
// mirror and writes the result into local before returning.
func (r *ResilientStorage) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, err := r.local.ReadFile(ctx, path)
	if err == nil {
		return data, nil
	}
	if r.mirror == nil {
		return nil, err
	}
	// Mirror fallback — populate local on success.
	data, mirrorErr := r.mirror.ReadFile(ctx, path)
	if mirrorErr != nil {
		return nil, fmt.Errorf("local: %w; mirror: %v", err, mirrorErr)
	}
	// Best-effort populate local cache; ignore write errors.
	if writeErr := r.local.WriteFile(ctx, path, data, 0o644); writeErr != nil {
		slog.Warn("resilient_storage: failed to populate local cache from mirror",
			"path", path, "err", writeErr)
	}
	return data, nil
}

// Open opens the file for streaming read. Tries local first; falls back to
// mirror (downloading to local) if local is absent.
func (r *ResilientStorage) Open(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	rc, err := r.local.Open(ctx, path)
	if err == nil {
		return rc, nil
	}
	if r.mirror == nil {
		return nil, err
	}
	// Pull from mirror into local cache, then open local.
	data, mirrorErr := r.mirror.ReadFile(ctx, path)
	if mirrorErr != nil {
		return nil, fmt.Errorf("local: %w; mirror: %v", err, mirrorErr)
	}
	if writeErr := r.local.WriteFile(ctx, path, data, 0o644); writeErr != nil {
		slog.Warn("resilient_storage: failed to populate local cache from mirror (open)",
			"path", path, "err", writeErr)
		// Even if local write failed, return an in-memory reader so callers don't fail.
		return newBytesReadSeekCloser(data), nil
	}
	return r.local.Open(ctx, path)
}

// Stat returns metadata. Tries local first; falls back to mirror.
func (r *ResilientStorage) Stat(ctx context.Context, path string) (fs.FileInfo, error) {
	fi, err := r.local.Stat(ctx, path)
	if err == nil {
		return fi, nil
	}
	if r.mirror == nil {
		return nil, err
	}
	fi, mirrorErr := r.mirror.Stat(ctx, path)
	if mirrorErr != nil {
		return nil, fmt.Errorf("local: %w; mirror: %v", err, mirrorErr)
	}
	return fi, nil
}

// Exists returns true if the path exists in local or (if available) the mirror.
func (r *ResilientStorage) Exists(ctx context.Context, path string) bool {
	if r.local.Exists(ctx, path) {
		return true
	}
	if r.mirror == nil {
		return false
	}
	return r.mirror.Exists(ctx, path)
}

// ReadDir lists the directory from local only (directory structure is local).
func (r *ResilientStorage) ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error) {
	return r.local.ReadDir(ctx, path)
}

// -----------------------------------------------------------------------------
// Storage interface — write operations
// -----------------------------------------------------------------------------

// WriteFile writes to local (mandatory) then to mirror (best-effort).
// A mirror write failure is logged but never returned as an error.
func (r *ResilientStorage) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	if err := r.local.WriteFile(ctx, path, data, perm); err != nil {
		return err // local write failure is fatal
	}
	if r.mirror != nil {
		if err := r.mirror.WriteFile(ctx, path, data, perm); err != nil {
			slog.Warn("resilient_storage: mirror write failed (best-effort, local write succeeded)",
				"path", path, "err", err)
		}
	}
	return nil
}

// Create opens a file for writing. Writes to local; mirror is updated lazily
// via WriteFile calls (not via Create, as we cannot tee streams portably).
func (r *ResilientStorage) Create(ctx context.Context, path string) (io.WriteCloser, error) {
	return r.local.Create(ctx, path)
}

// -----------------------------------------------------------------------------
// Storage interface — mutation operations (local only)
// -----------------------------------------------------------------------------

// MkdirAll creates directories in the local store only.
func (r *ResilientStorage) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return r.local.MkdirAll(ctx, path, perm)
}

// RemoveAll removes a path tree from local only.
func (r *ResilientStorage) RemoveAll(ctx context.Context, path string) error {
	return r.local.RemoveAll(ctx, path)
}

// Remove removes a single file from local only.
func (r *ResilientStorage) Remove(ctx context.Context, path string) error {
	return r.local.Remove(ctx, path)
}

// Rename renames/moves a path in local only.
func (r *ResilientStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	return r.local.Rename(ctx, oldPath, newPath)
}

// -----------------------------------------------------------------------------
// Storage interface — health & environment
// -----------------------------------------------------------------------------

// Ping always returns local.Ping() — the service is healthy as long as the
// local POSIX store is reachable (mirror is informational only).
func (r *ResilientStorage) Ping(ctx context.Context) error {
	return r.local.Ping(ctx)
}

// TempDir delegates to the local store.
func (r *ResilientStorage) TempDir() string {
	return r.local.TempDir()
}

// Getwd delegates to the local store.
func (r *ResilientStorage) Getwd() (string, error) {
	return r.local.Getwd()
}

// PingMirror returns an error if the mirror is unreachable.
// Returns nil if mirror is nil (no mirror configured).
func (r *ResilientStorage) PingMirror(ctx context.Context) error {
	if r.mirror == nil {
		return nil
	}
	return r.mirror.Ping(ctx)
}

// -----------------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------------

// bytesReadSeekCloser wraps a []byte as io.ReadSeekCloser.
// Used when local cache write failed but we still have data from mirror.
type bytesReadSeekCloser struct {
	data []byte
	pos  int64
}

func newBytesReadSeekCloser(data []byte) *bytesReadSeekCloser {
	return &bytesReadSeekCloser{data: data}
}

func (b *bytesReadSeekCloser) Read(p []byte) (int, error) {
	if b.pos >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += int64(n)
	return n, nil
}

func (b *bytesReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = b.pos + offset
	case io.SeekEnd:
		abs = int64(len(b.data)) + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if abs < 0 {
		return 0, fmt.Errorf("negative seek position")
	}
	b.pos = abs
	return abs, nil
}

func (b *bytesReadSeekCloser) Close() error { return nil }
