// --- indexing_text.go ---
package main

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/file/filepb"
	Utility "github.com/globulario/utility"
)

func (srv *server) indexTextFile(path string, fileInfos *filepb.FileInfo) error {
	if fileInfos.Mime != "text/plain" {
		return errors.New("file is not a text file")
	}
	ctx := context.Background()
	if !srv.storageForPath(path).Exists(ctx, path) {
		return errors.New("file not found")
	}

	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	hidden := filepath.Join(dir, ".hidden", base)
	thumbDir := filepath.Join(hidden, "__thumbnail__")
	indexDir := filepath.Join(hidden, "__index_db__")
	_ = srv.storageMkdirAll(ctx, thumbDir, 0o755)
	_ = srv.storageMkdirAll(ctx, indexDir, 0o755)
	if srv.storageForPath(filepath.Join(thumbDir, "data_url.txt")).Exists(ctx, filepath.Join(thumbDir, "data_url.txt")) {
		return errors.New("info already exist")
	}

	metadata, _ := srv.ExtractMetada(path)
	metaJSON, _ := Utility.ToJson(metadata)
	if err := srv.IndexJsonObject(indexDir, metaJSON, "english", "SourceFile", []string{"FileName", "Author", "Producer", "Title"}, ""); err != nil {
		slog.Warn("metadata index failed", "err", err)
	}

	doc := map[string]interface{}{"Metadata": metadata}
	if b, err := srv.storageReadFile(ctx, path); err == nil {
		doc["Text"] = b
	}
	if docJSON, err := Utility.ToJson(doc); err == nil {
		if err := srv.IndexJsonObject(indexDir, docJSON, "english", "SourceFile", []string{"Text"}, ""); err != nil {
			slog.Warn("text index failed", "err", err)
		}
	}
	return nil
}
