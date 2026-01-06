package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (srv *server) minioEnabled() bool {
	return srv.MinioConfig != nil && srv.MinioConfig.Endpoint != "" && srv.MinioConfig.Bucket != ""
}

func (srv *server) ensureMinioClient() error {
	if srv.minioClient != nil {
		return nil
	}
	if !srv.minioEnabled() {
		return fmt.Errorf("minio is not enabled")
	}

	auth := srv.MinioConfig.Auth
	if auth == nil {
		auth = &config.MinioProxyAuth{Mode: config.MinioProxyAuthModeNone}
	}

	var creds *credentials.Credentials
	switch auth.Mode {
	case config.MinioProxyAuthModeAccessKey:
		creds = credentials.NewStaticV4(auth.AccessKey, auth.SecretKey, "")
	case config.MinioProxyAuthModeFile:
		data, err := os.ReadFile(auth.CredFile)
		if err != nil {
			return fmt.Errorf("read minio credentials file: %w", err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid minio credentials file format")
		}
		creds = credentials.NewStaticV4(parts[0], parts[1], "")
	case config.MinioProxyAuthModeNone:
		creds = credentials.NewStaticV4("", "", "")
	default:
		return fmt.Errorf("unknown minio auth mode: %s", auth.Mode)
	}

	client, err := minio.New(srv.MinioConfig.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: srv.MinioConfig.Secure,
	})
	if err != nil {
		return err
	}

	srv.minioClient = client
	return nil
}

func (srv *server) normalizedMinioPrefix() string {
	if srv.MinioConfig == nil {
		return "/users"
	}
	prefix := filepath.ToSlash(strings.TrimSpace(srv.MinioConfig.Prefix))
	if prefix == "" {
		prefix = "/users"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return filepath.ToSlash(filepath.Clean(prefix))
}

func (srv *server) isMinioPath(path string) bool {
	if path == "" || !srv.minioEnabled() {
		return false
	}
	p := filepath.ToSlash(strings.TrimSpace(path))
	prefix := srv.normalizedMinioPrefix()
	base := strings.TrimSuffix(prefix, "/")
	if base != "" && (p == base || strings.HasPrefix(p, base+"/")) {
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
	p = strings.TrimPrefix(p, prefix)
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
		_, err := srv.minioClient.StatObject(ctx, srv.MinioConfig.Bucket, key, minio.StatObjectOptions{})
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
		info, err := srv.minioClient.StatObject(ctx, srv.MinioConfig.Bucket, key, minio.StatObjectOptions{})
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
