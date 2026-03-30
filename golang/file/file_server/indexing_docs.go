// --- indexing_docs.go ---
// Text extraction for common document formats: DOCX, XLSX, ODT, EPUB, HTML, RTF, CSV, Markdown.
// Uses mostly Go stdlib — no heavy external dependencies.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/file/filepb"
	Utility "github.com/globulario/utility"
	"github.com/tealeg/xlsx"
)

// indexDocumentFile indexes a document file (DOCX, XLSX, ODT, EPUB, HTML, RTF, CSV, MD)
// by extracting its text content and feeding it into the bleve full-text index.
func (srv *server) indexDocumentFile(path string, fileInfos *filepb.FileInfo, force bool) error {
	ctx := context.Background()
	if !srv.storageForPath(path).Exists(ctx, path) {
		return fmt.Errorf("file not found: %s", path)
	}

	ext := strings.ToLower(filepath.Ext(path))
	text, err := srv.extractDocumentText(ctx, path, ext)
	if err != nil {
		return fmt.Errorf("text extraction failed for %s: %w", ext, err)
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("no text content extracted from %s", path)
	}

	// Prepare index directories (same pattern as indexTextFile / indexPdfFile)
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	hidden := filepath.Join(dir, ".hidden", base)
	thumbDir := filepath.Join(hidden, "__thumbnail__")
	indexDir := filepath.Join(hidden, "__index_db__")

	if force {
		_ = os.RemoveAll(thumbDir)
		_ = os.RemoveAll(indexDir)
	}

	_ = srv.storageMkdirAll(ctx, thumbDir, 0o755)
	_ = srv.storageMkdirAll(ctx, indexDir, 0o755)

	if !force && srv.storageForPath(filepath.Join(thumbDir, "data_url.txt")).Exists(ctx, filepath.Join(thumbDir, "data_url.txt")) {
		return fmt.Errorf("indexing info already exists")
	}

	// Index metadata
	metadata, _ := srv.ExtractMetada(path)
	metaJSON, _ := Utility.ToJson(metadata)
	if err := srv.IndexJsonObject(indexDir, metaJSON, "english", "SourceFile", []string{"FileName", "Author", "Producer", "Title"}, ""); err != nil {
		slog.Warn("metadata index failed", "path", path, "err", err)
	}

	// Index extracted text
	doc := map[string]interface{}{
		"SourceFile": path,
		"Text":       text,
	}
	docJSON, err := Utility.ToJson(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}
	if err := srv.IndexJsonObject(indexDir, docJSON, "english", "SourceFile", []string{"Text"}, ""); err != nil {
		return fmt.Errorf("index failed: %w", err)
	}

	slog.Info("indexed document", "path", path, "ext", ext, "textLen", len(text))
	return nil
}

// extractDocumentText dispatches to the right extractor based on file extension.
func (srv *server) extractDocumentText(ctx context.Context, path, ext string) (string, error) {
	switch ext {
	case ".docx":
		return srv.extractDocxText(ctx, path)
	case ".xlsx":
		return srv.extractXlsxText(ctx, path)
	case ".odt", ".ods", ".odp":
		return srv.extractOdtText(ctx, path)
	case ".epub":
		return srv.extractEpubText(ctx, path)
	case ".html", ".htm", ".xhtml":
		return srv.extractHtmlText(ctx, path)
	case ".rtf":
		return srv.extractRtfText(ctx, path)
	case ".csv", ".tsv":
		return srv.extractCsvText(ctx, path)
	case ".md", ".markdown":
		return srv.extractPlainText(ctx, path)
	default:
		return "", fmt.Errorf("unsupported document type: %s", ext)
	}
}

// ── DOCX ─────────────────────────────────────────────────────────
// DOCX is a ZIP archive; word/document.xml contains the body text in <w:t> elements.
func (srv *server) extractDocxText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid ZIP/DOCX: %w", err)
	}

	var parts []string
	for _, f := range r.File {
		// word/document.xml is the main body; also check headers/footers
		if f.Name == "word/document.xml" || strings.HasPrefix(f.Name, "word/header") || strings.HasPrefix(f.Name, "word/footer") {
			text, err := extractTextFromZipXML(f, "t")
			if err == nil && text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n"), nil
}

// ── XLSX ─────────────────────────────────────────────────────────
// Uses the existing tealeg/xlsx library.
func (srv *server) extractXlsxText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	wb, err := xlsx.OpenBinary(data)
	if err != nil {
		return "", fmt.Errorf("not a valid XLSX: %w", err)
	}

	var buf strings.Builder
	for _, sheet := range wb.Sheets {
		buf.WriteString(sheet.Name)
		buf.WriteString("\n")
		for _, row := range sheet.Rows {
			var cells []string
			for _, cell := range row.Cells {
				v := cell.String()
				if v != "" {
					cells = append(cells, v)
				}
			}
			if len(cells) > 0 {
				buf.WriteString(strings.Join(cells, " "))
				buf.WriteString("\n")
			}
		}
		buf.WriteString("\n")
	}
	return buf.String(), nil
}

// ── ODT / ODS / ODP ─────────────────────────────────────────────
// OpenDocument formats are ZIP archives; content.xml has the text in <text:p> elements.
func (srv *server) extractOdtText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid ZIP/ODT: %w", err)
	}

	for _, f := range r.File {
		if f.Name == "content.xml" {
			return extractTextFromZipXML(f, "")
		}
	}
	return "", fmt.Errorf("content.xml not found in ODT")
}

// ── EPUB ─────────────────────────────────────────────────────────
// EPUB is a ZIP of XHTML files. Extract text from all .xhtml/.html files.
func (srv *server) extractEpubText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("not a valid ZIP/EPUB: %w", err)
	}

	var parts []string
	for _, f := range r.File {
		lower := strings.ToLower(f.Name)
		if strings.HasSuffix(lower, ".xhtml") || strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") {
			text, err := extractTextFromZipXML(f, "")
			if err == nil && strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n"), nil
}

// ── HTML ─────────────────────────────────────────────────────────
func (srv *server) extractHtmlText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	return stripXMLTags(string(data)), nil
}

// ── RTF ──────────────────────────────────────────────────────────
// Simple RTF text extraction: strip control words and groups.
var rtfControlWord = regexp.MustCompile(`\\[a-z]+[-]?\d*\s?`)
var rtfSpecial = regexp.MustCompile(`[{}\\]`)

func (srv *server) extractRtfText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	text := string(data)
	// Remove RTF header/font tables (everything between { and } at depth)
	// Simple approach: strip control words, then braces
	text = strings.ReplaceAll(text, "\\par", "\n")
	text = strings.ReplaceAll(text, "\\line", "\n")
	text = strings.ReplaceAll(text, "\\tab", " ")
	text = rtfControlWord.ReplaceAllString(text, "")
	text = rtfSpecial.ReplaceAllString(text, "")
	// Clean up whitespace
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n"), nil
}

// ── CSV / TSV ────────────────────────────────────────────────────
func (srv *server) extractCsvText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // variable columns
	if strings.HasSuffix(strings.ToLower(path), ".tsv") {
		reader.Comma = '\t'
	}

	var buf strings.Builder
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		buf.WriteString(strings.Join(record, " "))
		buf.WriteString("\n")
	}
	return buf.String(), nil
}

// ── Plain text (Markdown, etc.) ──────────────────────────────────
func (srv *server) extractPlainText(ctx context.Context, path string) (string, error) {
	data, err := srv.storageReadFile(ctx, path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ── Helpers ──────────────────────────────────────────────────────

// extractTextFromZipXML opens a zip entry, parses its XML, and extracts all character data.
// If localName is non-empty, only text inside elements with that local name is collected
// (e.g., "t" for DOCX <w:t> elements). If empty, all character data is collected.
func extractTextFromZipXML(f *zip.File, localName string) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}

	decoder := xml.NewDecoder(bytes.NewReader(raw))
	decoder.Strict = false
	decoder.AutoClose = xml.HTMLAutoClose

	var buf strings.Builder
	var inside bool
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if localName == "" {
				inside = true
			} else if t.Name.Local == localName {
				inside = true
			}
		case xml.EndElement:
			if localName != "" && t.Name.Local == localName {
				inside = false
			}
			// Add space after block-level elements for readability
			switch t.Name.Local {
			case "p", "br", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr":
				buf.WriteString("\n")
			}
		case xml.CharData:
			if localName == "" || inside {
				text := strings.TrimSpace(string(t))
				if text != "" {
					buf.WriteString(text)
					buf.WriteString(" ")
				}
			}
		}
	}
	return buf.String(), nil
}

// stripXMLTags removes all XML/HTML tags and returns just the text content.
func stripXMLTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			buf.WriteRune(' ')
			continue
		}
		if !inTag {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}
