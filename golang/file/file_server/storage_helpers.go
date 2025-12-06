package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"

	Utility "github.com/globulario/utility"
)

type checksumEntry struct {
	size    int64
	modTime int64
	sum     string
}

func (srv *server) cacheKey(path string) string {
	return path + "@" + srv.Domain
}

func (srv *server) cacheGet(path string) ([]byte, error) {
	if data, err := cache.GetItem(srv.cacheKey(path)); err == nil {
		return data, nil
	}
	return cache.GetItem(path)
}

func (srv *server) cacheSet(path string, data []byte) {
	_ = cache.SetItem(srv.cacheKey(path), data)
	_ = cache.SetItem(path, data)
}

func (srv *server) cacheRemove(path string) {
	cache.RemoveItem(srv.cacheKey(path))
	cache.RemoveItem(path)
	srv.checksumCache.Delete(filepath.ToSlash(path))
}

func (srv *server) storageStat(ctx context.Context, path string) (fs.FileInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).Stat(ctx, path)
}

func (srv *server) storageReadDir(ctx context.Context, path string) ([]fs.DirEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).ReadDir(ctx, path)
}

func (srv *server) storageOpen(ctx context.Context, path string) (io.ReadSeekCloser, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).Open(ctx, path)
}

func (srv *server) storageCreate(ctx context.Context, path string) (io.WriteCloser, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).Create(ctx, path)
}

func (srv *server) storageReadFile(ctx context.Context, path string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).ReadFile(ctx, path)
}

func (srv *server) storageRemoveAll(ctx context.Context, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).RemoveAll(ctx, path)
}

func (srv *server) storageRemove(ctx context.Context, path string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).Remove(ctx, path)
}

func (srv *server) storageRename(ctx context.Context, oldPath, newPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(oldPath).Rename(ctx, oldPath, newPath)
}

func (srv *server) storageWriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).WriteFile(ctx, path, data, perm)
}

func (srv *server) storageMkdirAll(ctx context.Context, path string, perm fs.FileMode) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return srv.storageForPath(path).MkdirAll(ctx, path, perm)
}

func (srv *server) storageCopyFile(ctx context.Context, src, dst string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	reader, err := srv.storageOpen(ctx, src)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := srv.storageMkdirAll(ctx, filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	writer, err := srv.storageCreate(ctx, dst)
	if err != nil {
		return err
	}
	defer writer.Close()

	if _, err := io.Copy(writer, reader); err != nil {
		return err
	}
	return nil
}

func (srv *server) storageCopyDir(ctx context.Context, src, dst string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := srv.storageMkdirAll(ctx, dst, 0o755); err != nil {
		return err
	}

	entries, err := srv.storageReadDir(ctx, src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		srcChild := filepath.Join(src, name)
		dstChild := filepath.Join(dst, name)
		if entry.IsDir() {
			if err := srv.storageCopyDir(ctx, srcChild, dstChild); err != nil {
				return err
			}
		} else {
			if err := srv.storageCopyFile(ctx, srcChild, dstChild); err != nil {
				return err
			}
		}
	}
	return nil
}

func detectContentTypeFromReader(r io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if n == 0 {
		return http.DetectContentType([]byte{}), nil
	}
	return http.DetectContentType(buf[:n]), nil
}

func (srv *server) storageMove(ctx context.Context, src, dst string) error {
	if err := srv.storageRename(context.Background(), src, dst); err == nil {
		return nil
	}
	// Fallback copy+remove
	info, err := srv.storageStat(context.Background(), src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := srv.storageCopyDir(context.Background(), src, dst); err != nil {
			return err
		}
	} else {
		if err := srv.storageCopyFile(context.Background(), src, dst); err != nil {
			return err
		}
	}
	return srv.storageRemoveAll(context.Background(), src)
}

func (srv *server) computeChecksum(ctx context.Context, path string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	info, err := srv.storageStat(ctx, path)
	if err != nil {
		return "", err
	}
	key := filepath.ToSlash(path)
	if entryRaw, ok := srv.checksumCache.Load(key); ok {
		if entry, ok := entryRaw.(checksumEntry); ok {
			if entry.size == info.Size() && entry.modTime == info.ModTime().UnixNano() {
				return entry.sum, nil
			}
		}
	}

	storage := srv.storageForPath(path)
	if _, ok := storage.(*OSStorage); ok {
		localPath := srv.formatPath(path)
		sum := Utility.CreateFileChecksum(localPath)
		srv.checksumCache.Store(key, checksumEntry{
			size:    info.Size(),
			modTime: info.ModTime().UnixNano(),
			sum:     sum,
		})
		return sum, nil
	}

	reader, err := storage.Open(ctx, path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	srv.checksumCache.Store(key, checksumEntry{
		size:    info.Size(),
		modTime: info.ModTime().UnixNano(),
		sum:     sum,
	})
	return sum, nil
}
