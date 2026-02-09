package storage_backend

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// MinioStorage implements Storage using a MinIO/S3 bucket as root.
//
// This implementation provides a filesystem-like interface on top of object storage,
// emulating directories through key prefixes. It supports all standard file operations
// by translating them to appropriate S3/MinIO API calls.
//
// Example:
//
//	minioClient, _ := minio.New("localhost:9000", &minio.Options{...})
//	storage, _ := storage_backend.NewMinioStorage(minioClient, "my-bucket", "files/")
//	data, err := storage.ReadFile(ctx, "users/alice/doc.txt")
//	// Accesses S3 object: my-bucket/files/users/alice/doc.txt
type MinioStorage struct {
	client *minio.Client
	bucket string
	prefix string // optional path prefix inside bucket (e.g. "root/")
}

// NewMinioStorage creates a MinioStorage backed by the provided MinIO client, bucket, and prefix.
//
// The prefix parameter is optional and allows you to scope all operations to a specific
// "directory" within the bucket. For example, prefix="app/data/" means all paths will
// be prefixed with "app/data/" in the bucket.
func NewMinioStorage(client *minio.Client, bucket, prefix string) (*MinioStorage, error) {
	prefix = strings.Trim(prefix, "/")
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return &MinioStorage{
		client: client,
		bucket: bucket,
		prefix: prefix,
	}, nil
}

// pathToKey converts a "virtual" filesystem path into a MinIO object key.
func (s *MinioStorage) pathToKey(path string) string {
	clean := strings.TrimLeft(filepath.ToSlash(filepath.Clean(path)), "/")
	return s.prefix + clean
}

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

// readSeekCloser wraps an io.ReadCloser with seek support via re-opening.
type readSeekCloser struct {
	reader io.ReadCloser
	seekFn func(offset int64, whence int) (int64, error)
}

func (r *readSeekCloser) Read(p []byte) (int, error) { return r.reader.Read(p) }
func (r *readSeekCloser) Close() error               { return r.reader.Close() }
func (r *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return r.seekFn(offset, whence)
}

// Exists checks for object existence in the bucket.
func (s *MinioStorage) Exists(ctx context.Context, path string) bool {
	_, err := s.Stat(ctx, path)
	return err == nil
}

// Stat retrieves metadata for the given path, emulating directories when needed.
// MinIO doesn't have real directories, so we check if any objects exist with the given prefix.
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
	// Check if there are any objects with this prefix.
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
		break // Found at least one object, it's a directory
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

	// Group by first-level children under prefix
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
			// Already present; just OR the dir flag
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
		entries = append(entries, fs.FileInfoToDirEntry(agg.info))
	}
	return entries, nil
}

// Open returns an io.ReadSeekCloser backed by ranged GET requests to MinIO.
// Seek is implemented by re-opening with appropriate Range headers.
func (s *MinioStorage) Open(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	key := s.pathToKey(path)
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	// Implement Seek by re-opening with Range headers
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

		// Open at new offset using Range header
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
// Uses a pipe to stream data as it's written.
func (s *MinioStorage) Create(ctx context.Context, path string) (io.WriteCloser, error) {
	pr, pw := io.Pipe()
	key := s.pathToKey(path)

	// Background upload goroutine
	go func() {
		defer pr.Close()
		_, err := s.client.PutObject(ctx, s.bucket, key, pr, -1, minio.PutObjectOptions{})
		if err != nil {
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

	// Delete any "children" under prefix
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
// Note: This is not atomic in S3/MinIO.
func (s *MinioStorage) Rename(ctx context.Context, oldPath, newPath string) error {
	info, err := s.Stat(ctx, oldPath)
	if err != nil {
		return err
	}

	srcKey := s.pathToKey(oldPath)
	dstKey := s.pathToKey(newPath)

	if info.IsDir() {
		// Rename all objects under the directory prefix
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

	// Single file rename
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
// This is optional in S3/MinIO but can be useful for compatibility.
func (s *MinioStorage) MkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	key := s.pathToKey(path)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, strings.NewReader(""), 0, minio.PutObjectOptions{})
	return err
}

// TempDir returns the local temp directory (for scratch files).
func (s *MinioStorage) TempDir() string {
	return os.TempDir()
}

// Getwd exposes the current working directory for compatibility.
func (s *MinioStorage) Getwd() (string, error) {
	return os.Getwd()
}
