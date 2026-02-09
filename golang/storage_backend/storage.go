package storage_backend

import (
	"context"
	"io"
	"io/fs"
)

// Storage is a unified abstraction for file/object storage operations.
//
// It provides a consistent interface for both POSIX filesystems (via OSStorage)
// and object storage systems like MinIO/S3 (via MinioStorage).
//
// This abstraction allows services to work with files without knowing whether
// they're stored locally or in object storage, making it easy to switch
// backends or support multiple storage types.
//
// Usage:
//
//	// POSIX filesystem
//	storage := storage_backend.NewOSStorage("/var/lib/globular/data")
//
//	// MinIO/S3
//	storage, _ := storage_backend.NewMinioStorage(minioClient, "bucket", "prefix/")
//
//	// Use the same interface for both
//	data, err := storage.ReadFile(ctx, "path/to/file.txt")
type Storage interface {
	// Basic file/directory metadata
	Stat(ctx context.Context, path string) (fs.FileInfo, error)
	ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error)
	Exists(ctx context.Context, path string) bool
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// File reading/writing
	Open(ctx context.Context, path string) (io.ReadSeekCloser, error) // for streaming + range support
	Create(ctx context.Context, path string) (io.WriteCloser, error)
	WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error

	// Mutations
	RemoveAll(ctx context.Context, path string) error
	Remove(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error
	MkdirAll(ctx context.Context, path string, perm fs.FileMode) error

	// Environment helpers
	TempDir() string
	Getwd() (string, error)
}
