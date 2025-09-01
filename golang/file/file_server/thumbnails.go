// --- thumbnails.go ---
package main

import (
	"fmt"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/file/filepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/encoding/protojson"
)

func (s *server) getThumbnail(path string, h, w int) (string, error) {
	id := fmt.Sprintf("%s_%dx%d@%s", toSlash(path), h, w, s.Domain)

	if data, err := cache.GetItem(id); err == nil {
		return string(data), nil
	}
	t, err := Utility.CreateThumbnail(path, h, w)
	if err != nil { return "", err }
	_ = cache.SetItem(id, []byte(t))
	return t, nil
}

// getFileInfo returns metadata & thumbnail info for a given path.
func getFileInfo(s *server, path string, thumbnailMaxHeight, thumbnailMaxWidth int) (*filepb.FileInfo, error) {
	p := toSlash(path)
	info := new(filepb.FileInfo)
	info.Path = p

	st, err := os.Stat(p)
	if err != nil { return nil, err }

	// Make paths relative to data/files for external consumers
	rootPrefix := toSlash(config.GetDataDir() + "/files")
	if strings.HasPrefix(info.Path, rootPrefix) {
		info.Path = info.Path[len(rootPrefix):]
	}

	// Short-circuit hidden special dirs
	if strings.Contains(p, "/.hidden") {
		if strings.HasSuffix(p, "__preview__") || strings.HasSuffix(p, "__timeline__") || strings.HasSuffix(p, "__thumbnail__") {
			info.Mime = "inode/directory"
			info.IsDir = true
			return info, nil
		}
	}

	// Try cache first
	if data, err := cache.GetItem(p + "@" + s.Domain); err == nil && data != nil {
		if err := protojson.Unmarshal(data, info); err == nil {
			if info.IsDir && Utility.Exists(filepath.Join(p, "playlist.m3u8")) { info.Mime = "video/hls-stream" }
			return info, nil
		}
		cache.RemoveItem(p)
	}

	info.IsDir = st.IsDir()
	if info.IsDir {
		info.Mime = "inode/directory"
		if cwd, err := os.Getwd(); err == nil {
			icon := toSlash(filepath.Join(cwd, "mimetypes", "inode-directory.png"))
			info.Thumbnail, _ = s.getMimeTypesUrl(icon)
		}
	} else {
		info.Checksum = Utility.CreateFileChecksum(p)
	}
	info.Size = st.Size()
	info.Name = st.Name()
	info.ModeTime = st.ModTime().Unix()

	if !info.IsDir {
		// mime sniffing
		if dot := strings.LastIndex(st.Name(), "."); dot != -1 {
			info.Mime = mime.TypeByExtension(st.Name()[dot:])
		} else if f, err := os.Open(p); err == nil {
			info.Mime, _ = Utility.GetFileContentType(f); _ = f.Close()
		}

		// Default thumbnail placeholder
		if cwd, err := os.Getwd(); err == nil && !strings.Contains(p, "/.hidden/") {
			icon := toSlash(filepath.Join(cwd, "mimetypes", "unknown.png"))
			info.Thumbnail, _ = s.getMimeTypesUrl(icon)
		}

		// per-type thumbnail logic
		dir := filepath.Dir(p)
		base := filepath.Base(p)
		nameNoExt := strings.TrimSuffix(base, filepath.Ext(base))
		hidden := toSlash(filepath.Join(dir, ".hidden", nameNoExt))

		switch {
		case strings.HasPrefix(info.Mime, "image/"):
			mh, mw := 80, 80
			if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 { mh, mw = thumbnailMaxHeight, thumbnailMaxWidth }
			info.Thumbnail, _ = s.getThumbnail(p, mh, mw)

		case strings.HasPrefix(info.Mime, "video/"):
			if Utility.Exists(hidden) {
				prev := filepath.Join(hidden, "__preview__", "preview_00001.jpg")
				if !Utility.Exists(prev) {
					_ = os.RemoveAll(filepath.Join(hidden, "__preview__"))
					go s.createVideoPreview(info.Path, 20, 128)
					_ = os.RemoveAll(filepath.Join(hidden, "__timeline__"))
					go s.createVideoTimeLine(info.Path, 180, .2)
				}
				if Utility.Exists(prev) {
					thumb, err := s.getThumbnail(prev, -1, -1)
					if err != nil { slog.Error("video preview thumb failed", "path", p, "err", err) }
					info.Thumbnail = thumb
				} else if Utility.Exists(filepath.Join(hidden, "__thumbnail__", "data_url.txt")) {
					if b, err := os.ReadFile(filepath.Join(hidden, "__thumbnail__", "data_url.txt")); err == nil { info.Thumbnail = string(b) }
				} else if Utility.Exists(filepath.Join(hidden, "__thumbnail__")) {
					if files, err := Utility.ReadDir(filepath.Join(hidden, "__thumbnail__")); err == nil {
						for _, f := range files {
							if thumb, err := s.getThumbnail(filepath.Join(hidden, "__thumbnail__", f.Name()), 72, 128); err == nil {
								_ = os.WriteFile(filepath.Join(hidden, "__thumbnail__", "data_url.txt"), []byte(thumb), 0644)
								info.Thumbnail = thumb
								break
							}
						}
					}
				}
			} else if cwd, err := os.Getwd(); err == nil {
				icon := toSlash(filepath.Join(cwd, "mimetypes", "video-x-generic.png"))
				info.Thumbnail, _ = s.getMimeTypesUrl(icon)
			}

		case strings.HasPrefix(info.Mime, "audio/") || strings.HasSuffix(p, ".flac") || strings.HasSuffix(p, ".mp3"):
			if meta, err := Utility.ReadAudioMetadata(p, thumbnailMaxHeight, thumbnailMaxWidth); err == nil {
				if v, ok := meta["ImageUrl"].(string); ok { info.Thumbnail = v }
			}

		default:
			if Utility.Exists(filepath.Join(hidden, "__thumbnail__", "data_url.txt")) {
				if b, err := os.ReadFile(filepath.Join(hidden, "__thumbnail__", "data_url.txt")); err == nil { info.Thumbnail = string(b) }
			} else if strings.Contains(info.Mime, "/") {
				if cwd, err := os.Getwd(); err == nil {
					icon := toSlash(filepath.Join(cwd, "mimetypes", strings.ReplaceAll(strings.Split(info.Mime, ";")[0], "/", "-")+".png"))
					info.Thumbnail, _ = s.getMimeTypesUrl(icon)
				}
			}
		}
	} else {
		// Dir with HLS playlist thumbnail
		if Utility.Exists(filepath.Join(p, "playlist.m3u8")) {
			d := filepath.Dir(p)
			bn := filepath.Base(p)
			h := filepath.Join(d, ".hidden", bn, "__preview__", "preview_00001.jpg")
			if Utility.Exists(h) {
				if thumb, err := s.getThumbnail(h, -1, -1); err == nil { info.Thumbnail = thumb } else { slog.Error("hls preview thumb failed", "path", p, "err", err) }
			} else if cwd, err := os.Getwd(); err == nil {
				icon := toSlash(filepath.Join(cwd, "mimetypes", "video-x-generic.png"))
				info.Thumbnail, _ = s.getMimeTypesUrl(icon)
			}
		}
	}

	if b, err := protojson.Marshal(info); err == nil { _ = cache.SetItem(p, b) }
	return info, nil
}