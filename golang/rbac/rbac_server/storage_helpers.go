package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (srv *server) minioEnabled() bool {
	return srv.UseMinio && srv.MinioEndpoint != "" && srv.MinioBucket != ""
}

func (srv *server) ensureMinioClient() error {
	if srv.minioClient != nil {
		return nil
	}
	if !srv.minioEnabled() {
		return fmt.Errorf("minio is not enabled")
	}
	client, err := minio.New(srv.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(srv.MinioAccessKey, srv.MinioSecretKey, ""),
		Secure: srv.MinioUseSSL,
	})
	if err != nil {
		return err
	}
	srv.minioClient = client
	return nil
}

func (srv *server) normalizedMinioPrefix() string {
	prefix := filepath.ToSlash(strings.TrimSpace(srv.MinioPrefix))
	if prefix == "" {
		return ""
	}
	prefix = strings.TrimSuffix(prefix, "/")
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return prefix
}

func (srv *server) isMinioPath(path string) bool {
	if path == "" || !srv.minioEnabled() {
		return false
	}
	p := filepath.ToSlash(strings.TrimSpace(path))
	prefix := srv.normalizedMinioPrefix()
	if prefix != "" && strings.HasPrefix(p, prefix+"/") {
		return true
	}
	if prefix != "" && p == prefix {
		return true
	}
	if strings.HasPrefix(p, "/users/") || p == "/users" || strings.HasPrefix(p, "/applications/") || p == "/applications" {
		return true
	}
	return false
}

func (srv *server) minioKeyFromPath(path string) string {
	p := filepath.ToSlash(strings.TrimSpace(path))
	prefix := srv.normalizedMinioPrefix()
	if prefix != "" {
		if strings.HasPrefix(p, prefix) {
			p = strings.TrimPrefix(p, prefix)
		}
	}
	p = strings.TrimPrefix(p, "/")
	return p
}

type minioFileInfo struct {
	name string
	size int64
	dir  bool
}

func (fi minioFileInfo) Name() string { return fi.name }
func (fi minioFileInfo) Size() int64  { return fi.size }
func (fi minioFileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (fi minioFileInfo) ModTime() time.Time { return time.Time{} }
func (fi minioFileInfo) IsDir() bool        { return fi.dir }
func (fi minioFileInfo) Sys() interface{}   { return nil }

func (srv *server) storageExists(path string) bool {
	if path == "" {
		return false
	}
	if Utility.Exists(path) {
		return true
	}
	if srv.isMinioPath(path) {
		if err := srv.ensureMinioClient(); err != nil {
			return false
		}
		key := srv.minioKeyFromPath(path)
		ctx := context.Background()
		if key == "" {
			return false
		}
		_, err := srv.minioClient.StatObject(ctx, srv.MinioBucket, key, minio.StatObjectOptions{})
		return err == nil
	}
	return false
}

func (srv *server) storageStat(path string) (fs.FileInfo, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	if Utility.Exists(path) {
		return os.Stat(path)
	}
	if srv.isMinioPath(path) {
		if err := srv.ensureMinioClient(); err != nil {
			return nil, err
		}
		key := srv.minioKeyFromPath(path)
		ctx := context.Background()
		info, err := srv.minioClient.StatObject(ctx, srv.MinioBucket, key, minio.StatObjectOptions{})
		if err != nil {
			return nil, err
		}
		dir := strings.HasSuffix(key, "/")
		if !dir && strings.HasSuffix(path, "/") {
			dir = true
		}
		name := filepath.Base(strings.TrimSuffix(key, "/"))
		if name == "" {
			name = filepath.Base(strings.TrimSuffix(path, "/"))
		}
		return minioFileInfo{name: name, size: info.Size, dir: dir}, nil
	}
	return nil, os.ErrNotExist
}
