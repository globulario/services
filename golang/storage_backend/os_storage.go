package storage_backend

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	Utility "github.com/globulario/utility"
)

// OSStorage implements Storage using the local POSIX filesystem, optionally under a root directory.
//
// If Root is empty, paths are treated as absolute filesystem paths.
// If Root is set, all paths are resolved relative to that root directory.
//
// Example:
//
//	storage := NewOSStorage("/var/lib/globular/data")
//	data, err := storage.ReadFile(ctx, "users/alice/file.txt")
//	// Reads from: /var/lib/globular/data/users/alice/file.txt
type OSStorage struct {
	Root string // Empty = use absolute paths as-is
}

// NewOSStorage returns an OSStorage rooted at the provided directory.
// An empty root means all paths are treated as absolute filesystem paths.
func NewOSStorage(root string) *OSStorage {
	// Normalize root
	root = strings.TrimRight(root, "/")
	return &OSStorage{Root: root}
}

// resolve converts a logical path to an absolute filesystem path.
func (s *OSStorage) resolve(path string) string {
	clean := filepath.Clean(path)
	if s.Root == "" {
		return clean
	}

	if filepath.IsAbs(clean) {
		if strings.HasPrefix(clean, s.Root) {
			clean = strings.TrimPrefix(clean, s.Root)
		}
		clean = strings.TrimPrefix(clean, string(filepath.Separator))
	}
	return filepath.Join(s.Root, clean)
}

// Stat resolves path relative to the configured root and proxies to os.Stat.
func (s *OSStorage) Stat(ctx context.Context, path string) (fs.FileInfo, error) {
	return os.Stat(s.resolve(path))
}

// Exists checks for file existence under the storage root.
func (s *OSStorage) Exists(ctx context.Context, path string) bool {
	return Utility.Exists(s.resolve(path))
}

// ReadDir mirrors os.ReadDir within the storage root.
func (s *OSStorage) ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error) {
	return os.ReadDir(s.resolve(path))
}

// ReadFile reads the file content into memory.
func (s *OSStorage) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return os.ReadFile(s.resolve(path))
}

// Open returns a read/seek capable handle for the requested file path.
// *os.File already implements io.ReadSeekCloser.
func (s *OSStorage) Open(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	return os.Open(s.resolve(path)) // #nosec G304
}

// Create ensures parent directories exist then opens the file for writing.
func (s *OSStorage) Create(ctx context.Context, path string) (io.WriteCloser, error) {
	abs := s.resolve(path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, err
	}
	return os.Create(abs) // #nosec G304
}

// WriteFile writes the provided bytes to path, creating parent directories if needed.
func (s *OSStorage) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	abs := s.resolve(path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, perm) // #nosec G306
}

// RemoveAll delegates to os.RemoveAll within the storage root.
func (s *OSStorage) RemoveAll(ctx context.Context, path string) error {
	return os.RemoveAll(s.resolve(path))
}

// Remove deletes a single file.
func (s *OSStorage) Remove(ctx context.Context, path string) error {
	return os.Remove(s.resolve(path))
}

// Rename renames/moves a file between two rooted paths.
func (s *OSStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	return os.Rename(s.resolve(oldPath), s.resolve(newPath))
}

// MkdirAll mirrors os.MkdirAll relative to the storage root.
func (s *OSStorage) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	return os.MkdirAll(s.resolve(path), perm)
}

// TempDir returns the host temp directory (used by callers to place scratch files).
func (s *OSStorage) TempDir() string {
	return os.TempDir()
}

// Getwd exposes the current working directory so callers can resolve relative paths.
func (s *OSStorage) Getwd() (string, error) {
	return os.Getwd()
}
