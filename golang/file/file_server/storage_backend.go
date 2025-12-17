package main

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage is the abstraction used by FileService + HTTP handlers.
// It covers exactly the os.* and fs ops you listed.
type Storage interface {
	// Basic file / dir metadata
	Stat(ctx context.Context, path string) (fs.FileInfo, error)
	ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error)
	Exists(ctx context.Context, path string) bool
	ReadFile(ctx context.Context, path string) ([]byte, error)

	// File reading / writing
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

// -----------------------------------------------------------------------------
// OS implementation (current behaviour)
// -----------------------------------------------------------------------------

// OSStorage implements Storage using the local filesystem, optionally under a root.
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

// We'll use *os.File which already implements io.ReadSeekCloser.
// Open returns a read/seek capable handle for the requested file path.
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

// -----------------------------------------------------------------------------
// MinIO implementation
// -----------------------------------------------------------------------------

// minioFileInfo adapts minio.ObjectInfo to fs.FileInfo.
type minioFileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
	etag    string
}

func (i *minioFileInfo) Name() string { return i.name }
func (i *minioFileInfo) Size() int64  { return i.size }
func (i *minioFileInfo) Mode() fs.FileMode {
	if i.isDir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (i *minioFileInfo) ModTime() time.Time { return i.modTime }
func (i *minioFileInfo) IsDir() bool        { return i.isDir }
func (i *minioFileInfo) Sys() any           { return map[string]any{"etag": i.etag} }

// readSeekCloser wraps an io.ReadCloser + a replenisher function for Seek.
// For OS we just use *os.File directly; for MinIO we re-open with a range.
type readSeekCloser struct {
	reader io.ReadCloser
	seekFn func(offset int64, whence int) (int64, error)
}

func (r *readSeekCloser) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *readSeekCloser) Close() error               { return r.reader.Close() }
func (r *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return r.seekFn(offset, whence)
}

// MinioStorage implements Storage using a MinIO bucket as root.
type MinioStorage struct {
	client *minio.Client
	bucket string
	prefix string // optional path prefix inside bucket (e.g. "root/")
}

// NewMinioStorage creates a MinioStorage backed by the provided MinIO endpoint/bucket.
// Endpoint should be host:port, credentials map to an access/secret key pair.
func NewMinioStorage(endpoint, accessKey, secretKey, bucket, prefix string, useSSL bool) (*MinioStorage, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	prefix = strings.Trim(prefix, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return &MinioStorage{
		client: mc,
		bucket: bucket,
		prefix: prefix,
	}, nil
}

// pathToKey converts a "virtual" path into a MinIO object key.
func (s *MinioStorage) pathToKey(path string) string {
	clean := strings.TrimLeft(filepath.ToSlash(filepath.Clean(path)), "/")
	return s.prefix + clean
}

// ---- Metadata ----

// Exists checks for file existence under the storage root.
func (s *MinioStorage) Exists(ctx context.Context, path string) bool {
	_, err := s.Stat(ctx, path)
	return err == nil
}

// Stat retrieves metadata for the given path, emulating directories when needed.
func (s *MinioStorage) Stat(ctx context.Context, path string) (fs.FileInfo, error) {
	key := s.pathToKey(path)
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return &minioFileInfo{
			name:    filepath.Base(path),
			size:    info.Size,
			modTime: info.LastModified,
			isDir:   false,
			etag:    info.ETag,
		}, nil
	}

	// If Stat fails, we might be dealing with a "directory".
	// MinIO has no real dirs; we emulate dir existence if there is at least one object with that prefix.
	prefix := key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	})
	var dirInfo *minioFileInfo
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		dirInfo = &minioFileInfo{
			name:    filepath.Base(path),
			size:    0,
			modTime: obj.LastModified,
			isDir:   true,
			etag:    "",
		}
	}

	if dirInfo != nil {
		return dirInfo, nil
	}
	return nil, os.ErrNotExist
}

// ReadDir lists first-level children by scanning objects with the requested prefix.
func (s *MinioStorage) ReadDir(ctx context.Context, path string) ([]fs.DirEntry, error) {
	keyPrefix := s.pathToKey(path)
	if keyPrefix != "" && !strings.HasSuffix(keyPrefix, "/") {
		keyPrefix += "/"
	}

	// We'll group by first-level children under prefix.
	type entryAgg struct {
		isDir bool
		info  *minioFileInfo
	}
	children := make(map[string]*entryAgg)

	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    keyPrefix,
		Recursive: false,
	})
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		rel := strings.TrimPrefix(obj.Key, keyPrefix)
		if rel == "" {
			continue
		}
		parts := strings.SplitN(rel, "/", 2)
		name := parts[0]
		isDir := len(parts) > 1
		if agg, ok := children[name]; ok {
			// Already present; just OR the dir flag.
			agg.isDir = agg.isDir || isDir
			continue
		}
		children[name] = &entryAgg{
			isDir: isDir,
			info: &minioFileInfo{
				name:    name,
				size:    obj.Size,
				modTime: obj.LastModified,
				isDir:   isDir,
				etag:    obj.ETag,
			},
		}
	}

	entries := make([]fs.DirEntry, 0, len(children))
	for _, agg := range children {
		agg := agg
		entries = append(entries, fs.FileInfoToDirEntry(agg.info))
	}
	return entries, nil
}

// ---- File reading / writing ----

// Open returns an io.ReadSeekCloser backed by ranged GET requests to MinIO.
func (s *MinioStorage) Open(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	key := s.pathToKey(path)
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	// We implement Seek by re-opening new ranged readers.
	var curOffset int64
	seekFn := func(offset int64, whence int) (int64, error) {
		switch whence {
		case io.SeekStart:
			curOffset = offset
		case io.SeekCurrent:
			curOffset += offset
		case io.SeekEnd:
			// Need total size
			info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
			if err != nil {
				return 0, err
			}
			curOffset = info.Size + offset
		default:
			return 0, os.ErrInvalid
		}

		if err := obj.Close(); err != nil {
			return 0, err
		}

		// Open at new offset using Range header.
		opts := minio.GetObjectOptions{}
		if err := opts.SetRange(curOffset, 0); err != nil {
			return 0, err
		}
		obj, err = s.client.GetObject(ctx, s.bucket, key, opts)
		if err != nil {
			return 0, err
		}
		return curOffset, nil
	}

	return &readSeekCloser{
		reader: obj,
		seekFn: seekFn,
	}, nil
}

// ReadFile loads the full object into memory.
func (s *MinioStorage) ReadFile(ctx context.Context, path string) ([]byte, error) {
	rc, err := s.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// Create returns a writer that uploads data to MinIO when closed.
func (s *MinioStorage) Create(ctx context.Context, path string) (io.WriteCloser, error) {
	// For S3/MinIO we don't "create then write". We just buffer and PutObject on Close.
	// For big files you might want multipart upload; here we go with a simple buffered implementation.

	pr, pw := io.Pipe()
	key := s.pathToKey(path)

	// Background upload goroutine.
	go func() {
		defer pr.Close()
		_, err := s.client.PutObject(ctx, s.bucket, key, pr, -1, minio.PutObjectOptions{})
		if err != nil {
			// Signal error to writer side
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	return pw, nil
}

// WriteFile uploads the provided bytes to the object represented by path.
func (s *MinioStorage) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	key := s.pathToKey(path)
	r := strings.NewReader(string(data))
	_, err := s.client.PutObject(ctx, s.bucket, key, r, int64(len(data)), minio.PutObjectOptions{})
	return err
}

// RemoveAll deletes the specified object and any keys beneath its prefix.
func (s *MinioStorage) RemoveAll(ctx context.Context, path string) error {
	key := s.pathToKey(path)
	// Delete object if exists
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Delete any "children" under prefix.
	prefix := key
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	ch := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	for obj := range ch {
		if obj.Err != nil {
			return obj.Err
		}
		if err := s.client.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// Remove deletes a single object.
func (s *MinioStorage) Remove(ctx context.Context, path string) error {
	key := s.pathToKey(path)
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// Rename copies an object to the new path and removes the original key.
func (s *MinioStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	// S3/MinIO doesn't have an atomic rename. We copy then delete.
	info, err := s.Stat(ctx, oldPath)
	if err != nil {
		return err
	}

	srcKey := s.pathToKey(oldPath)
	dstKey := s.pathToKey(newPath)

	if info.IsDir() {
		srcPrefix := srcKey
		if !strings.HasSuffix(srcPrefix, "/") {
			srcPrefix += "/"
		}
		dstPrefix := dstKey
		if !strings.HasSuffix(dstPrefix, "/") {
			dstPrefix += "/"
		}

		var moved int
		objs := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
			Prefix:    srcPrefix,
			Recursive: true,
		})
		for obj := range objs {
			if obj.Err != nil {
				return obj.Err
			}
			rel := strings.TrimPrefix(obj.Key, srcPrefix)
			targetKey := dstPrefix + rel

			if _, err := s.client.CopyObject(ctx, minio.CopyDestOptions{
				Bucket: s.bucket,
				Object: targetKey,
			}, minio.CopySrcOptions{
				Bucket: s.bucket,
				Object: obj.Key,
			}); err != nil {
				return err
			}
			if err := s.client.RemoveObject(ctx, s.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				return err
			}
			moved++
		}

		if moved == 0 {
			return os.ErrNotExist
		}
		return nil
	}

	if _, err := s.client.CopyObject(ctx, minio.CopyDestOptions{
		Bucket: s.bucket,
		Object: dstKey,
	}, minio.CopySrcOptions{
		Bucket: s.bucket,
		Object: srcKey,
	}); err != nil {
		return err
	}

	return s.client.RemoveObject(ctx, s.bucket, srcKey, minio.RemoveObjectOptions{})
}

// MkdirAll creates an empty "directory marker" object so prefixes appear in listings.
func (s *MinioStorage) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	// Directories are virtual; we can optionally create a "marker" object.
	key := s.pathToKey(path)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, strings.NewReader(""), 0, minio.PutObjectOptions{})
	return err
}

// Environment helpers: just delegate to local OS for now.
// TempDir returns the local temp directory to mirror the OS storage helpers.
func (s *MinioStorage) TempDir() string {
	return os.TempDir()
}

// Getwd exposes the current working directory for compatibility with OS storage.
func (s *MinioStorage) Getwd() (string, error) {
	return os.Getwd()
}
