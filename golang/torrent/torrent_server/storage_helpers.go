package main

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	Utility "github.com/globulario/utility"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func (srv *server) minioEnabled() bool {
	return srv.UseMinio && srv.MinioEndpoint != "" && srv.MinioBucket != ""
}

func (srv *server) ensureMinioClient() error {
	if !srv.minioEnabled() {
		return fmt.Errorf("minio is not enabled")
	}
	if srv.minioClient != nil {
		return nil
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
		prefix = "/users"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if prefix != "/" {
		prefix = "/" + strings.Trim(filepath.Clean(prefix), "/")
	}
	return filepath.ToSlash(prefix)
}

func (srv *server) normalizeLogicalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "file://") {
		path = strings.TrimPrefix(path, "file://")
	}
	if strings.HasPrefix(path, "\\\\") {
		return filepath.Clean(path)
	}
	if len(path) >= 2 && path[1] == ':' {
		return filepath.Clean(path)
	}
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") && filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, ".") {
		path = "/" + path
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func (srv *server) isMinioPath(path string) bool {
	normalized := srv.normalizeLogicalPath(path)
	if normalized == "" {
		return false
	}
	prefix := filepath.ToSlash(strings.TrimSuffix(srv.normalizedMinioPrefix(), "/"))
	if prefix == "" || prefix == "." {
		return true
	}
	normalized = strings.TrimSuffix(normalized, "/")
	return normalized == prefix || strings.HasPrefix(normalized, prefix+"/")
}

func (srv *server) minioKeyFromPath(path string) string {
	path = srv.normalizeLogicalPath(path)
	return strings.TrimPrefix(path, "/")
}

func (srv *server) pathExists(path string) bool {
	if path == "" {
		return false
	}
	if srv.isMinioPath(path) {
		if err := srv.ensureMinioClient(); err != nil {
			return false
		}
		key := srv.minioKeyFromPath(path)
		_, err := srv.minioClient.StatObject(context.Background(), srv.MinioBucket, key, minio.StatObjectOptions{})
		return err == nil
	}
	actual := filepath.Clean(path)
	return Utility.Exists(actual)
}

func (srv *server) copyToDestination(src, dest string) error {
	dest = srv.normalizeLogicalPath(dest)
	if dest == "" {
		return fmt.Errorf("copyToDestination: empty destination")
	}
	if srv.isMinioPath(dest) {
		if err := srv.ensureMinioClient(); err != nil {
			return err
		}
		key := srv.minioKeyFromPath(dest)
		_, err := srv.minioClient.FPutObject(context.Background(), srv.MinioBucket, key, src, minio.PutObjectOptions{
			ContentType: guessContentType(dest),
		})
		return err
	}
	actual := filepath.Clean(dest)
	if err := Utility.CreateDirIfNotExist(filepath.Dir(actual)); err != nil {
		return err
	}
	return Utility.CopyFile(src, actual)
}

func guessContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return "application/octet-stream"
	}
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	switch ext {
	case ".mkv":
		return "video/x-matroska"
	default:
		return "application/octet-stream"
	}
}
