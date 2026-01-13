package pkgpack

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type archiveEntry struct {
	source string
	tarRel string
	isDir  bool
	info   os.FileInfo
}

// WriteTgz writes a deterministic tar.gz archive from rootDir.
func WriteTgz(outputPath, rootDir string) error {
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	gz.Name = ""
	gz.ModTime = time.Unix(0, 0)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	entries, err := collectEntries(rootDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.tarRel == "." {
			continue
		}

		mode := entryMode(entry.tarRel, entry.isDir)
		hdr, err := tar.FileInfoHeader(entry.info, "")
		if err != nil {
			return err
		}
		hdr.Name = entry.tarRel
		if entry.isDir && !strings.HasSuffix(hdr.Name, "/") {
			hdr.Name += "/"
		}
		hdr.ModTime = time.Unix(0, 0)
		hdr.AccessTime = time.Unix(0, 0)
		hdr.ChangeTime = time.Unix(0, 0)
		hdr.Uid = 0
		hdr.Gid = 0
		hdr.Uname = ""
		hdr.Gname = ""
		hdr.Mode = mode

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if entry.isDir {
			continue
		}
		f, err := os.Open(entry.source)
		if err != nil {
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}

	return nil
}

func entryMode(tarRel string, isDir bool) int64 {
	if isDir {
		return 0755
	}
	if strings.HasPrefix(tarRel, "bin/") {
		return 0755
	}
	return 0644
}

func collectEntries(root string) ([]archiveEntry, error) {
	var entries []archiveEntry
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		tarRel := toTarPath(rel)
		entries = append(entries, archiveEntry{
			source: p,
			tarRel: tarRel,
			isDir:  info.IsDir(),
			info:   info,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].tarRel < entries[j].tarRel
	})
	return entries, nil
}

func toTarPath(relPath string) string {
	rel := filepath.ToSlash(relPath)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "" {
		return "."
	}
	return rel
}
