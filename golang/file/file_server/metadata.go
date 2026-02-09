// --- metadata.go ---
package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/barasher/go-exiftool"
	"github.com/globulario/services/golang/storage_backend"
	Utility "github.com/globulario/utility"
)

// ExtractMetada returns file metadata regardless of the underlying storage backend.
// For paths stored outside the local filesystem (e.g., MinIO), the file is streamed to
// a temporary location before invoking exiftool.
func (srv *server) ExtractMetada(path string) (map[string]interface{}, error) {
	localPath, cleanup, err := srv.prepareLocalFile(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	et, err := exiftool.NewExiftool()
	if err != nil {
		return nil, err
	}
	defer et.Close()

	infos := et.ExtractMetadata(localPath)
	if len(infos) > 0 {
		return infos[0].Fields, nil
	}
	return nil, errors.New("no metadata found for " + path)
}

func (srv *server) prepareLocalFile(path string) (string, func(), error) {
	p := filepath.ToSlash(path)
	ctx := context.Background()

	if _, ok := srv.storageForPath(p).(*storage_backend.OSStorage); ok {
		return filepath.ToSlash(p), func() {}, nil
	}

	reader, err := srv.storageOpen(ctx, p)
	if err != nil {
		return "", nil, err
	}
	defer reader.Close()

	tmp, err := os.CreateTemp("", "fs-cache-*"+filepath.Ext(p))
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tmp, reader); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}

	cleanup := func() { _ = os.Remove(tmp.Name()) }
	return tmp.Name(), cleanup, nil
}

func (srv *server) readAudioMetadata(path string, height, width int) (map[string]interface{}, error) {
	localPath, cleanup, err := srv.prepareLocalFile(path)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return Utility.ReadAudioMetadata(localPath, height, width)
}
