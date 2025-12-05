package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
)

// mediaWorkFile represents a usable local file for ffmpeg/ffprobe.
type mediaWorkFile struct {
	LogicalPath string
	LocalPath   string
	IsMinio     bool
}

func (srv *server) isMinioPath(p string) bool {
	if !srv.minioEnabled() {
		return false
	}
	p = filepath.ToSlash(p)
	prefix := filepath.ToSlash(srv.MinioPrefix)
	if prefix == "" {
		prefix = "/users"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	prefix = filepath.ToSlash(prefix)
	base := strings.TrimSuffix(prefix, "/")
	if base == "" {
		return false
	}
	if p == base || strings.HasPrefix(p, base+"/") {
		return true
	}
	return false
}

func (srv *server) minioKeyFromPath(p string) string {
	p = filepath.ToSlash(p)
	prefix := filepath.ToSlash(srv.MinioPrefix)
	p = strings.TrimPrefix(p, prefix)
	p = strings.TrimPrefix(p, "/")
	return p
}

func (srv *server) minioDownloadToTemp(ctx context.Context, logicalPath string) (string, func(), error) {
	if err := srv.ensureMinioClient(); err != nil {
		return "", func() {}, err
	}
	key := srv.minioKeyFromPath(logicalPath)
	ext := filepath.Ext(logicalPath)
	tmp := filepath.ToSlash(filepath.Join(os.TempDir(), Utility.GenerateUUID("media")+ext))
	if err := srv.minioClient.FGetObject(ctx, srv.MinioBucket, key, tmp, minio.GetObjectOptions{}); err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		_ = os.Remove(tmp)
	}
	return tmp, cleanup, nil
}

func (srv *server) minioUploadFile(ctx context.Context, logicalPath, localPath, contentType string) error {
	if err := srv.ensureMinioClient(); err != nil {
		return err
	}
	key := srv.minioKeyFromPath(logicalPath)
	_, err := srv.minioClient.FPutObject(ctx, srv.MinioBucket, key, localPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (srv *server) minioUploadDir(ctx context.Context, logicalDir, localDir string) error {
	if err := srv.ensureMinioClient(); err != nil {
		return err
	}
	logicalDir = filepath.ToSlash(logicalDir)
	logicalDir = strings.TrimSuffix(logicalDir, "/")

	return filepath.Walk(localDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		logicalPath := logicalDir + "/" + rel
		ct := detectContentType(path)
		return srv.minioUploadFile(ctx, logicalPath, path, ct)
	})
}

func detectContentType(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".m3u8"):
		return "application/x-mpegURL"
	case strings.HasSuffix(lower, ".vtt"):
		return "text/vtt"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".txt"):
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

func (srv *server) minioObjectExists(ctx context.Context, logicalPath string) bool {
	if err := srv.ensureMinioClient(); err != nil {
		return false
	}
	key := srv.minioKeyFromPath(logicalPath)
	_, err := srv.minioClient.StatObject(ctx, srv.MinioBucket, key, minio.StatObjectOptions{})
	return err == nil
}

func (srv *server) withWorkFile(path string, fn func(wf mediaWorkFile) error) error {
	logical := filepath.ToSlash(path)
	if srv.isMinioPath(logical) {
		ctx := context.Background()
		tmp, cleanup, err := srv.minioDownloadToTemp(ctx, logical)
		if err != nil {
			return err
		}
		defer cleanup()
		return fn(mediaWorkFile{
			LogicalPath: logical,
			LocalPath:   filepath.ToSlash(tmp),
			IsMinio:     true,
		})
	}

	local := srv.formatPath(logical)
	return fn(mediaWorkFile{
		LogicalPath: logical,
		LocalPath:   filepath.ToSlash(local),
		IsMinio:     false,
	})
}

func (srv *server) prepareOutputDir(logicalDir string, wf mediaWorkFile) (string, func(), error) {
	logicalDir = filepath.ToSlash(logicalDir)
	if wf.IsMinio {
		tmp, err := os.MkdirTemp("", "media-out-*")
		if err != nil {
			return "", nil, err
		}
		cleanup := func() { _ = os.RemoveAll(tmp) }
		return filepath.ToSlash(tmp), cleanup, nil
	}
	local := srv.formatPath(logicalDir)
	if err := Utility.CreateDirIfNotExist(local); err != nil {
		return "", nil, err
	}
	return filepath.ToSlash(local), func() {}, nil
}

func (srv *server) localPathExists(path string) bool {
	if path == "" {
		return false
	}
	return Utility.Exists(filepath.ToSlash(path))
}

func (srv *server) pathExists(path string) bool {
	if path == "" {
		return false
	}
	p := filepath.ToSlash(path)
	if srv.isMinioPath(p) {
		return srv.minioObjectExists(context.Background(), p)
	}
	if srv.localPathExists(p) {
		return true
	}
	formatted := srv.formatPath(p)
	if formatted != p {
		return srv.localPathExists(formatted)
	}
	return false
}

func (srv *server) resolveIOPath(path string) (string, bool) {
	p := filepath.ToSlash(path)
	if p == "" {
		return "", false
	}
	if srv.isMinioPath(p) {
		return p, true
	}
	if filepath.IsAbs(p) {
		return p, false
	}
	return filepath.ToSlash(srv.formatPath(p)), false
}

func (srv *server) ensureDirExists(path string) error {
	resolved, isMinio := srv.resolveIOPath(path)
	if resolved == "" {
		return fmt.Errorf("ensureDirExists: empty path")
	}
	if isMinio {
		return nil
	}
	return os.MkdirAll(resolved, 0o755)
}

func (srv *server) ensureParentDir(path string) error {
	return srv.ensureDirExists(filepath.Dir(path))
}

func (srv *server) writeFile(path string, data []byte, perm fs.FileMode) error {
	resolved, isMinio := srv.resolveIOPath(path)
	if resolved == "" {
		return fmt.Errorf("writeFile: empty path")
	}
	if perm == 0 {
		perm = 0o664
	}
	if isMinio {
		tmp, err := os.CreateTemp("", "globular-write-*")
		if err != nil {
			return err
		}
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			_ = os.Remove(tmp.Name())
			return err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmp.Name())
			return err
		}
		defer os.Remove(tmp.Name())
		return srv.minioUploadFile(context.Background(), resolved, tmp.Name(), detectContentType(resolved))
	}
	if err := srv.ensureParentDir(resolved); err != nil {
		return err
	}
	return os.WriteFile(resolved, data, perm)
}

func (srv *server) readFile(path string) ([]byte, error) {
	resolved, isMinio := srv.resolveIOPath(path)
	if resolved == "" {
		return nil, fmt.Errorf("readFile: empty path")
	}
	if isMinio {
		tmp, cleanup, err := srv.minioDownloadToTemp(context.Background(), resolved)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return os.ReadFile(tmp)
	}
	return os.ReadFile(resolved)
}

func (srv *server) removePath(path string) error {
	resolved, isMinio := srv.resolveIOPath(path)
	if resolved == "" {
		return nil
	}
	if isMinio {
		if err := srv.ensureMinioClient(); err != nil {
			return err
		}
		return srv.minioClient.RemoveObject(context.Background(), srv.MinioBucket, srv.minioKeyFromPath(resolved), minio.RemoveObjectOptions{})
	}
	return os.Remove(resolved)
}

func (srv *server) removeAll(path string) error {
	resolved, isMinio := srv.resolveIOPath(path)
	if resolved == "" {
		return nil
	}
	if isMinio {
		if err := srv.ensureMinioClient(); err != nil {
			return err
		}
		ctx := context.Background()
		key := srv.minioKeyFromPath(resolved)
		if key != "" {
			if err := srv.minioClient.RemoveObject(ctx, srv.MinioBucket, key, minio.RemoveObjectOptions{}); err != nil {
				return err
			}
		}
		prefix := strings.TrimSuffix(key, "/")
		if prefix == "" {
			return nil
		}
		prefix += "/"
		objs := srv.minioClient.ListObjects(ctx, srv.MinioBucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
		for obj := range objs {
			if obj.Err != nil {
				return obj.Err
			}
			if err := srv.minioClient.RemoveObject(ctx, srv.MinioBucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
				return err
			}
		}
		return nil
	}
	return os.RemoveAll(resolved)
}

func (srv *server) createDirIfNotExist(path string) error {
	return srv.ensureDirExists(path)
}

func (srv *server) moveLocalFileToPath(localPath, dest string) error {
	resolved, isMinio := srv.resolveIOPath(dest)
	if resolved == "" {
		return fmt.Errorf("moveLocalFileToPath: empty destination")
	}
	if isMinio {
		return srv.minioUploadFile(context.Background(), resolved, localPath, detectContentType(localPath))
	}
	if err := srv.ensureParentDir(resolved); err != nil {
		return err
	}
	if err := os.Rename(localPath, resolved); err == nil {
		return nil
	}
	if err := Utility.CopyFile(localPath, resolved); err != nil {
		return err
	}
	return os.Remove(localPath)
}

// walkDirEntry implements fs.DirEntry for synthesized MinIO directories/files.
type walkDirEntry struct {
	name string
	dir  bool
}

func (e walkDirEntry) Name() string { return e.name }
func (e walkDirEntry) IsDir() bool  { return e.dir }
func (e walkDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	} else {
		return 0
	}
}
func (e walkDirEntry) Info() (fs.FileInfo, error) {
	return virtualFileInfo{name: e.name, dir: e.dir}, nil
}

type virtualFileInfo struct {
	name string
	dir  bool
}

func (fi virtualFileInfo) Name() string { return fi.name }
func (fi virtualFileInfo) Size() int64  { return 0 }
func (fi virtualFileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0o755
	} else {
		return 0o644
	}
}
func (fi virtualFileInfo) ModTime() time.Time { return time.Time{} }
func (fi virtualFileInfo) IsDir() bool        { return fi.dir }
func (fi virtualFileInfo) Sys() interface{}   { return nil }

// walkDir traverses a logical path, handling both local and MinIO-backed trees.
func (srv *server) walkDir(root string, fn fs.WalkDirFunc) error {
	logicalRoot := filepath.ToSlash(root)
	if logicalRoot == "" {
		return nil
	}
	if srv.isMinioPath(logicalRoot) {
		return srv.walkMinioDir(logicalRoot, fn)
	}

	actualRoot := logicalRoot
	if !srv.localPathExists(actualRoot) {
		actualRoot = srv.formatPath(logicalRoot)
	}
	if !srv.localPathExists(actualRoot) {
		return nil
	}

	return filepath.WalkDir(actualRoot, func(actual string, d fs.DirEntry, err error) error {
		if err != nil {
			return fn(actual, d, err)
		}
		logicalPath := logicalRoot
		if actual != actualRoot {
			if rel, relErr := filepath.Rel(actualRoot, actual); relErr == nil && rel != "." {
				logicalPath = filepath.ToSlash(filepath.Join(logicalRoot, rel))
			}
		}
		return fn(logicalPath, d, nil)
	})
}

func (srv *server) walkMinioDir(root string, fn fs.WalkDirFunc) error {
	if err := srv.ensureMinioClient(); err != nil {
		return err
	}
	root = filepath.ToSlash(strings.TrimSuffix(root, "/"))
	if root == "" {
		root = "/"
	}

	rootEntry := walkDirEntry{name: filepath.Base(root), dir: true}
	if root == "/" {
		rootEntry.name = "/"
	}
	if err := fn(root, rootEntry, nil); err != nil {
		if errors.Is(err, fs.SkipDir) {
			return nil
		}
		return err
	}

	ctx := context.Background()
	prefix := strings.TrimSuffix(srv.minioKeyFromPath(root), "/")
	if prefix != "" {
		prefix += "/"
	}

	visited := map[string]bool{root: true}
	skipped := map[string]bool{}

	list := srv.minioClient.ListObjects(ctx, srv.MinioBucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	for obj := range list {
		if obj.Err != nil {
			return obj.Err
		}
		key := obj.Key
		if prefix != "" {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			key = strings.TrimPrefix(key, prefix)
		}
		key = strings.Trim(key, "/")
		if key == "" {
			continue
		}
		logicalPath := filepath.ToSlash(filepath.Join(root, key))

		skip, err := srv.ensureMinioDirs(root, logicalPath, visited, skipped, fn)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		if srv.isSkippedPath(logicalPath, skipped) {
			continue
		}
		if strings.HasSuffix(obj.Key, "/") {
			// directory placeholder, already handled
			continue
		}
		entry := walkDirEntry{name: filepath.Base(logicalPath), dir: false}
		if err := fn(logicalPath, entry, nil); err != nil {
			if errors.Is(err, fs.SkipDir) {
				continue
			}
			return err
		}
	}
	return nil
}

func (srv *server) ensureMinioDirs(root, path string, visited, skipped map[string]bool, fn fs.WalkDirFunc) (bool, error) {
	if path == root {
		return false, nil
	}
	rel := strings.TrimPrefix(path, root)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return false, nil
	}
	parts := strings.Split(rel, "/")
	current := root
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		current = filepath.ToSlash(filepath.Join(current, part))
		if visited[current] {
			if skipped[current] {
				return true, nil
			}
			continue
		}
		visited[current] = true
		entry := walkDirEntry{name: filepath.Base(current), dir: true}
		if err := fn(current, entry, nil); err != nil {
			if errors.Is(err, fs.SkipDir) {
				skipped[current] = true
				return true, nil
			}
			return false, err
		}
	}
	if srv.isSkippedPath(path, skipped) {
		return true, nil
	}
	return false, nil
}

func (srv *server) isSkippedPath(path string, skipped map[string]bool) bool {
	for skip := range skipped {
		if path == skip || strings.HasPrefix(path, skip+"/") {
			return true
		}
	}
	return false
}

func (srv *server) readDirEntries(path string) ([]fs.DirEntry, error) {
	resolved, isMinio := srv.resolveIOPath(path)
	if resolved == "" {
		return nil, fmt.Errorf("readDirEntries: empty path")
	}
	if isMinio {
		if err := srv.ensureMinioClient(); err != nil {
			return nil, err
		}
		ctx := context.Background()
		prefix := srv.minioKeyFromPath(path)
		prefix = strings.TrimPrefix(prefix, "/")
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		opts := minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: false,
		}
		var entries []fs.DirEntry
		seenDirs := make(map[string]bool)
		for obj := range srv.minioClient.ListObjects(ctx, srv.MinioBucket, opts) {
			if obj.Err != nil {
				return nil, obj.Err
			}
			key := obj.Key
			if prefix != "" && strings.HasPrefix(key, prefix) {
				key = strings.TrimPrefix(key, prefix)
			}
			key = strings.Trim(key, "/")
			if key == "" {
				continue
			}
			parts := strings.Split(key, "/")
			name := parts[0]
			if len(parts) > 1 || strings.HasSuffix(obj.Key, "/") || obj.Size == 0 {
				if seenDirs[name] {
					continue
				}
				seenDirs[name] = true
				entries = append(entries, walkDirEntry{name: name, dir: true})
				continue
			}
			entries = append(entries, walkDirEntry{name: name, dir: false})
		}
		return entries, nil
	}
	return os.ReadDir(resolved)
}
