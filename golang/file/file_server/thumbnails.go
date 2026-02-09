// --- thumbnails.go ---
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/storage_backend"
	//"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/file/filepb"
	Utility "github.com/globulario/utility"
	_ "golang.org/x/image/webp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

func (s *server) getThumbnail(path string, h, w int) (string, error) {
	checksum := ""
	if sum, err := s.computeChecksum(context.Background(), path); err == nil {
		checksum = sum
	}
	id := fmt.Sprintf("%s_%s_%dx%d@%s", path, checksum, h, w, s.Domain)

	if data, err := cache.GetItem(id); err == nil {
		return string(data), nil
	}

	thumbPath := path
	storage := s.storageForPath(path)
	if _, ok := storage.(*storage_backend.OSStorage); !ok {
		tmp, err := os.CreateTemp("", "thumb-*"+filepath.Ext(path))
		if err != nil {
			return "", err
		}
		if err := func() error {
			defer tmp.Close()
			r, err := storage.Open(context.Background(), path)
			if err != nil {
				return err
			}
			defer r.Close()
			if _, err := io.Copy(tmp, r); err != nil {
				return err
			}
			return nil
		}(); err != nil {
			_ = os.Remove(tmp.Name())
			return "", err
		}
		thumbPath = tmp.Name()
		defer os.Remove(tmp.Name())
	}

	t, err := Utility.CreateThumbnail(thumbPath, h, w)
	if err != nil {
		return "", err
	}
	_ = cache.SetItem(id, []byte(t))
	return t, nil
}

// getFileInfo returns metadata & thumbnail info for a given path.
func getFileInfo(s *server, path string, thumbnailMaxHeight, thumbnailMaxWidth int) (*filepb.FileInfo, error) {

	info := new(filepb.FileInfo)
	info.Path = path

	st, err := s.storageStat(context.Background(), path)
	if err != nil {
		return nil, err
	}

	// Short-circuit hidden special dirs
	if strings.Contains(path, "/.hidden") {
		if strings.HasSuffix(path, "__preview__") || strings.HasSuffix(path, "__timeline__") || strings.HasSuffix(path, "__thumbnail__") {
			info.Mime = "inode/directory"
			info.IsDir = true
			return info, nil
		}
	}

	// Try cache first
	if data, err := s.cacheGet(path); err == nil && data != nil {
		if err := protojson.Unmarshal(data, info); err == nil {
			if info.IsDir && s.storageForPath(filepath.Join(path, "playlist.m3u8")).Exists(context.Background(), filepath.Join(path, "playlist.m3u8")) {
				info.Mime = "video/hls-stream"
			}
			if !info.IsDir && info.Checksum == "" {
				if sum, err := s.computeChecksum(context.Background(), path); err == nil {
					info.Checksum = sum
					if b, marshalErr := protojson.Marshal(info); marshalErr == nil {
						s.cacheSet(path, b)
					}
				}
			}
			return info, nil
		}
		s.cacheRemove(path)
	}

	info.IsDir = st.IsDir()
	if info.IsDir {
		info.Mime = "inode/directory"
		if cwd, err := s.Storage().Getwd(); err == nil {
			icon := filepath.Join(cwd, "mimetypes", "inode-directory.png")
			info.Thumbnail, _ = s.getMimeTypesUrl(icon)
		}
	} else {
		if sum, err := s.computeChecksum(context.Background(), path); err == nil {
			info.Checksum = sum
		}
	}
	info.Size = st.Size()
	info.Name = st.Name()
	info.ModeTime = st.ModTime().Unix()

	if !info.IsDir {
		// mime sniffing
		if dot := strings.LastIndex(st.Name(), "."); dot != -1 {
			info.Mime = mime.TypeByExtension(st.Name()[dot:])
		} else if f, err := s.storageOpen(context.Background(), path); err == nil {
			if mime, err := detectContentTypeFromReader(f); err == nil {
				info.Mime = mime
			}
			_ = f.Close()
		}

		// Default thumbnail placeholder
		if cwd, err := s.Storage().Getwd(); err == nil && !strings.Contains(path, "/.hidden/") {
			icon := filepath.Join(cwd, "mimetypes", "unknown.png")
			info.Thumbnail, _ = s.getMimeTypesUrl(icon)
		}

		// per-type thumbnail logic
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		nameNoExt := strings.TrimSuffix(base, filepath.Ext(base))
		hidden := filepath.Join(dir, ".hidden", nameNoExt)

		switch {
		case strings.HasPrefix(info.Mime, "image/"):
			mh, mw := 80, 80
			if thumbnailMaxHeight > 0 && thumbnailMaxWidth > 0 {
				mh, mw = thumbnailMaxHeight, thumbnailMaxWidth
			}
			info.Thumbnail, _ = s.getThumbnail(path, mh, mw)
			annotateImageMetadata(s, info, path, info.Thumbnail, mw, mh)

		case strings.HasPrefix(info.Mime, "video/"):
			if s.storageForPath(hidden).Exists(context.Background(), hidden) {
				prev := filepath.Join(hidden, "__preview__", "preview_00001.jpg")
				if !s.storageForPath(prev).Exists(context.Background(), prev) {
					_ = s.storageRemoveAll(context.Background(), filepath.Join(hidden, "__preview__"))
					go s.createVideoPreview(info.Path, 20, 128)
					_ = s.storageRemoveAll(context.Background(), filepath.Join(hidden, "__timeline__"))
					go s.createVideoTimeLine(info.Path, 180, .2)
				}
				if s.storageForPath(prev).Exists(context.Background(), prev) {
					thumb, err := s.getThumbnail(prev, -1, -1)
					if err != nil {
						slog.Error("video preview thumb failed", "path", path, "err", err)
					}
					info.Thumbnail = thumb
				} else if s.storageForPath(filepath.Join(hidden, "__thumbnail__", "data_url.txt")).Exists(context.Background(), filepath.Join(hidden, "__thumbnail__", "data_url.txt")) {
					if b, err := s.storageReadFile(context.Background(), filepath.Join(hidden, "__thumbnail__", "data_url.txt")); err == nil {
						info.Thumbnail = string(b)
					}
				} else if s.storageForPath(filepath.Join(hidden, "__thumbnail__")).Exists(context.Background(), filepath.Join(hidden, "__thumbnail__")) {
					if files, err := s.storageReadDir(context.Background(), filepath.Join(hidden, "__thumbnail__")); err == nil {
						for _, f := range files {
							if thumb, err := s.getThumbnail(filepath.Join(hidden, "__thumbnail__", f.Name()), 72, 128); err == nil {
								_ = s.storageWriteFile(context.Background(), filepath.Join(hidden, "__thumbnail__", "data_url.txt"), []byte(thumb), 0o644)
								info.Thumbnail = thumb
								break
							}
						}
					}
				}
			} else if cwd, err := s.Storage().Getwd(); err == nil {
				icon := filepath.Join(cwd, "mimetypes", "video-x-generic.png")
				info.Thumbnail, _ = s.getMimeTypesUrl(icon)
			}

		case strings.HasPrefix(info.Mime, "audio/") || strings.HasSuffix(path, ".flac") || strings.HasSuffix(path, ".mp3"):
			if meta, err := s.readAudioMetadata(path, thumbnailMaxHeight, thumbnailMaxWidth); err == nil {
				if v, ok := meta["ImageUrl"].(string); ok {
					info.Thumbnail = v
				}
			}

		default:
			if s.storageForPath(filepath.Join(hidden, "__thumbnail__", "data_url.txt")).Exists(context.Background(), filepath.Join(hidden, "__thumbnail__", "data_url.txt")) {
				if b, err := s.storageReadFile(context.Background(), filepath.Join(hidden, "__thumbnail__", "data_url.txt")); err == nil {
					info.Thumbnail = string(b)
				}
			} else if strings.Contains(info.Mime, "/") {
				if cwd, err := s.Storage().Getwd(); err == nil {
					icon := filepath.Join(cwd, "mimetypes", strings.ReplaceAll(strings.Split(info.Mime, ";")[0], "/", "-")+".png")
					info.Thumbnail, _ = s.getMimeTypesUrl(icon)
				}
			}
		}
	} else {
		// Dir with HLS playlist thumbnail
		if s.storageForPath(filepath.Join(path, "playlist.m3u8")).Exists(context.Background(), filepath.Join(path, "playlist.m3u8")) {
			d := filepath.Dir(path)
			bn := filepath.Base(path)
			h := filepath.Join(d, ".hidden", bn, "__preview__", "preview_00001.jpg")
			if s.storageForPath(h).Exists(context.Background(), h) {
				if thumb, err := s.getThumbnail(h, -1, -1); err == nil {
					info.Thumbnail = thumb
				} else {
					slog.Error("hls preview thumb failed", "path", path, "err", err)
				}
			} else if cwd, err := s.Storage().Getwd(); err == nil {
				icon := filepath.Join(cwd, "mimetypes", "video-x-generic.png")
				info.Thumbnail, _ = s.getMimeTypesUrl(icon)
			}
		}
	}

	if b, err := protojson.Marshal(info); err == nil {
		s.cacheSet(path, b)
	}
	return info, nil
}

// annotateImageMetadata stores useful image sizing details in the FileInfo metadata.
func annotateImageMetadata(s *server, info *filepb.FileInfo, imagePath, thumbnailData string, requestedThumbWidth, requestedThumbHeight int) {
	origW, origH, err := imageDimensions(s, imagePath)
	if err == nil && origW > 0 && origH > 0 {
		setMetadataNumber(info, "OriginalWidth", float64(origW))
		setMetadataNumber(info, "OriginalHeight", float64(origH))
	}

	thumbW, thumbH, thumbErr := thumbnailDimensions(thumbnailData)
	if thumbErr != nil && requestedThumbWidth > 0 && requestedThumbHeight > 0 {
		thumbW, thumbH = requestedThumbWidth, requestedThumbHeight
	}
	if thumbW > 0 && thumbH > 0 {
		setMetadataNumber(info, "ThumbnailWidth", float64(thumbW))
		setMetadataNumber(info, "ThumbnailHeight", float64(thumbH))
	}
}

func imageDimensions(s *server, path string) (int, int, error) {
	f, err := s.storageOpen(context.Background(), path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func thumbnailDimensions(dataURL string) (int, int, error) {
	if dataURL == "" {
		return 0, 0, fmt.Errorf("empty thumbnail data")
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid data url")
	}
	payload := parts[1]
	buf, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func setMetadataNumber(info *filepb.FileInfo, key string, value float64) {
	if value <= 0 {
		return
	}
	if info.Metadata == nil {
		info.Metadata = &structpb.Struct{Fields: map[string]*structpb.Value{}}
	} else if info.Metadata.Fields == nil {
		info.Metadata.Fields = map[string]*structpb.Value{}
	}
	info.Metadata.Fields[key] = structpb.NewNumberValue(value)
}
