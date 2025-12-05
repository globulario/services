// --- indexing_pdf.go ---
package main

import (
	"context"
	"fmt"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/file/filepb"
	Utility "github.com/globulario/utility"
	"github.com/karmdip-mi/go-fitz"
)

// indexPdfFile indexes the content of a PDF file into a search engine.
func (srv *server) indexPdfFile(path string, fileInfos *filepb.FileInfo) error {
	slog.Info("index pdf", "path", path)
	if fileInfos.Mime != "application/pdf" {
		return fmt.Errorf("file is not a PDF: %s", path)
	}

	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	hidden := filepath.Join(dir, ".hidden", base)
	thumbDir := filepath.Join(hidden, "__thumbnail__")
	indexDir := filepath.Join(hidden, "__index_db__")
	_ = srv.storageMkdirAll(context.Background(), thumbDir, 0o755)
	_ = srv.storageMkdirAll(context.Background(), indexDir, 0o755)

	if srv.storageForPath(filepath.Join(thumbDir, "data_url.txt")).Exists(context.Background(), filepath.Join(thumbDir, "data_url.txt")) {
		return fmt.Errorf("indexing info already exists")
	}

	doc, err := fitz.New(path)
	if err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	metadata, _ := ExtractMetada(path)
	metaJSON, _ := Utility.ToJson(metadata)
	docId := Utility.GenerateUUID(path)
	metadata["DocId"] = docId

	if err := srv.IndexJsonObject(indexDir, metaJSON, "english", "SourceFile", []string{"FileName", "Author", "Producer", "Title"}, ""); err != nil {
		slog.Warn("metadata index failed", "err", err)
	}

	for i := 0; i < doc.NumPage(); i++ {
		pageMap := map[string]interface{}{"Id": fmt.Sprintf("%s_page_%d", docId, i), "Number": i, "Path": path, "DocId": docId}
		if i == 0 {
			if err := srv.processThumbnail(doc, thumbDir); err != nil {
				slog.Warn("thumbnail failed", "err", err)
			}
		}
		text, err := doc.Text(i)
		if err != nil || len(text) == 0 {
			if text, err = extractTextFromImage(doc, i); err != nil {
				slog.Warn("ocr extract failed", "page", i, "err", err)
			}
		}
		pageMap["Text"] = text
		if pageJSON, err := Utility.ToJson(pageMap); err == nil {
			if err := srv.IndexJsonObject(indexDir, pageJSON, "english", "Id", []string{"Text"}, ""); err != nil {
				slog.Warn("page index failed", "page", i, "err", err)
			}
		}
	}
	return nil
}

func (srv *server) processThumbnail(doc *fitz.Document, thumbnailPath string) error {
	img, err := doc.Image(0)
	if err != nil || img == nil {
		return fmt.Errorf("no image found on first page")
	}
	tmp := filepath.Join(os.TempDir(), Utility.RandomUUID()+".jpg")
	defer os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		return fmt.Errorf("encode thumb: %w", err)
	}
	dataURL, err := Utility.CreateThumbnail(tmp, 256, 256)
	if err != nil {
		return fmt.Errorf("create thumb: %w", err)
	}
	return srv.storageWriteFile(context.Background(), filepath.Join(thumbnailPath, "data_url.txt"), []byte(dataURL), 0o755)
}

func extractTextFromImage(doc *fitz.Document, page int) (string, error) {
	img, err := doc.Image(page)
	if err != nil || img == nil {
		return "", fmt.Errorf("no image found on page %d", page)
	}
	tmp := filepath.Join(os.TempDir(), Utility.RandomUUID()+".jpg")
	defer os.Remove(tmp)
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: jpeg.DefaultQuality}); err != nil {
		return "", fmt.Errorf("encode: %w", err)
	}
	return Utility.ExtractTextFromJpeg(tmp)
}
