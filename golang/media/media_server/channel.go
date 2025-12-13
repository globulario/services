package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/media/mediapb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// channelStorageDir returns the ".hidden/__channels__" dir for a given root path.
func (srv *server) channelStorageDir(root string) string {
	root = filepath.ToSlash(strings.TrimSpace(root))
	if root == "" {
		return ""
	}
	root = srv.formatPath(root)
	return filepath.Join(root, ".hidden", "__channels__")
}

func (srv *server) channelFilePath(root, id string) string {
	dir := srv.channelStorageDir(root)
	if dir == "" || strings.TrimSpace(id) == "" {
		return ""
	}
	return filepath.Join(dir, id+".json")
}

// channelThumbnailDir returns the ".hidden/__thumbnails__" directory for a root.
func (srv *server) channelThumbnailDir(root string) string {
	root = filepath.ToSlash(strings.TrimSpace(root))
	if root == "" {
		return ""
	}
	root = srv.formatPath(root)
	return filepath.Join(root, ".hidden", "__thumbnails__")
}

func (srv *server) channelThumbnailPath(root, id string) string {
	dir := srv.channelThumbnailDir(root)
	if dir == "" || strings.TrimSpace(id) == "" {
		return ""
	}
	return filepath.Join(dir, id+".jpg")
}

func (srv *server) saveChannel(ch *mediapb.Channel) error {
	if ch == nil {
		return errors.New("nil channel")
	}
	if strings.TrimSpace(ch.Id) == "" {
		return errors.New("channel has empty id")
	}
	if strings.TrimSpace(ch.Path) == "" {
		return errors.New("channel has empty path")
	}

	dest := srv.channelFilePath(ch.Path, ch.Id)
	if dest == "" {
		return fmt.Errorf("cannot compute channel file path for id=%s", ch.Id)
	}

	if err := srv.createDirIfNotExist(filepath.Dir(dest)); err != nil {
		return err
	}

	data, err := protojson.Marshal(ch)
	if err != nil {
		return err
	}

	return srv.writeFile(dest, data, 0o664)
}

func (srv *server) loadChannel(root, id string) (*mediapb.Channel, error) {
	path := srv.channelFilePath(root, id)
	if path == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid root or channel id")
	}
	data, err := srv.readFile(path)
	if err != nil {
		return nil, err
	}
	ch := new(mediapb.Channel)
	if err := protojson.Unmarshal(data, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

type playlistEnvelope struct {
	Format string        `json:"format"`
	Path   string        `json:"path"`
	URL    string        `json:"url"`
	Items  []interface{} `json:"items"`
}

// buildChannelFromPlaylistJSON converts playlist.json (yt-dlp output + wrapper)
// into a Channel message (no disk I/O here).
func (srv *server) buildChannelFromPlaylistJSON(raw []byte) (*mediapb.Channel, error) {
	var env playlistEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid playlist_json: %v", err)
	}
	if len(env.Items) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "playlist_json contains no items")
	}

	firstMap, ok := env.Items[0].(map[string]interface{})
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "playlist_json items[0] is not an object")
	}

	// Use yt-dlp playlist metadata embedded in the first item.
	playlistID := asString(firstMap, "playlist_id")
	if playlistID == "" {
		// fallback: derive from URL
		playlistID = env.URL
	}

	ch := &mediapb.Channel{
		Id:                 playlistID,
		Url:                env.URL,
		Path:               env.Path,
		Format:             env.Format,
		Extractor:          asString(firstMap, "extractor"),
		ExtractorKey:       asString(firstMap, "extractor_key"),
		PlaylistTitle:      asString(firstMap, "playlist_title"),
		PlaylistUploader:   asString(firstMap, "playlist_uploader"),
		PlaylistUploaderId: asString(firstMap, "playlist_uploader_id"),
		PlaylistCount:      asInt32(firstMap, "playlist_count"),
		LastSyncEpoch:      time.Now().Unix(),
		Items:              make([]*mediapb.ChannelItem, 0, len(env.Items)),
	}

	for _, it := range env.Items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		item := &mediapb.ChannelItem{
			VideoId:       asString(m, "id"),
			Title:         asString(m, "title"),
			Url:           asString(m, "url"),
			WebpageUrl:    asString(m, "webpage_url"),
			Epoch:         asInt64(m, "epoch"),
			PlaylistIndex: asInt32(m, "playlist_index"),
			Extractor:     asString(m, "extractor"),
			ExtractorKey:  asString(m, "extractor_key"),
			OriginalUrl:   asString(m, "original_url"),
			Filename:      asString(m, "filename"),
		}

		// Optional: guess local_path using filename + format if you want.
		// This assumes that your yt-dlp output template matches UploadVideo.
		if env.Path != "" && item.Filename != "" && env.Format != "" {
			// Many yt-dlp templates already append extension to filename.
			// In your sample, filename ends with ".NA", so we avoid double-ext.
			base := item.Filename
			if !strings.Contains(base, ".") {
				base = base + "." + env.Format
			}
			item.LocalPath = filepath.ToSlash(filepath.Join(env.Path, base))
		}

		ch.Items = append(ch.Items, item)
	}

	return ch, nil
}

func (srv *server) fetchChannelThumbnailURL(urlStr string) (string, error) {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		return "", errors.New("empty url")
	}

	cmd := exec.Command("yt-dlp", "--dump-single-json", "--skip-download", urlStr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(out, &obj); err != nil {
		return "", err
	}

	if thumbs, ok := obj["thumbnails"].([]interface{}); ok && len(thumbs) > 0 {
		if u := firstThumbURL(thumbs); u != "" {
			return u, nil
		}
	}

	if thumbs, ok := obj["uploader_thumbnails"].([]interface{}); ok && len(thumbs) > 0 {
		if u := firstThumbURL(thumbs); u != "" {
			return u, nil
		}
	}

	if entries, ok := obj["entries"].([]interface{}); ok && len(entries) > 0 {
		if m, ok := entries[0].(map[string]interface{}); ok {
			if u, ok := m["thumbnail"].(string); ok && strings.TrimSpace(u) != "" {
				return u, nil
			}
		}
	}

	return "", errors.New("no thumbnail found in yt-dlp json")
}

func firstThumbURL(arr []interface{}) string {
	for _, it := range arr {
		if m, ok := it.(map[string]interface{}); ok {
			if u, ok := m["url"].(string); ok && strings.TrimSpace(u) != "" {
				return u
			}
		}
	}
	return ""
}

func (srv *server) ensureChannelThumbnail(ch *mediapb.Channel) (string, error) {
	if ch == nil {
		return "", errors.New("nil channel")
	}
	root := strings.TrimSpace(ch.Path)
	id := strings.TrimSpace(ch.Id)
	if root == "" || id == "" {
		return "", errors.New("channel missing path or id")
	}

	dst := srv.channelThumbnailPath(root, id)
	if dst == "" {
		return "", errors.New("cannot compute thumbnail path")
	}
	if srv.pathExists(dst) {
		return dst, nil
	}

	thumbURL, err := srv.fetchChannelThumbnailURL(ch.Url)
	if err != nil {
		slog.Warn("ensureChannelThumbnail: fetch thumbnail url failed",
			"channel_id", id, "url", ch.Url, "err", err)
		return "", err
	}

	if err := srv.createDirIfNotExist(filepath.Dir(dst)); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp("", "channel-thumb-*")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := Utility.DownloadFile(thumbURL, tmpPath); err != nil {
		return "", err
	}
	if err := srv.moveLocalFileToPath(tmpPath, dst); err != nil {
		return "", err
	}

	slog.Info("channel thumbnail saved",
		"channel_id", id,
		"url", thumbURL,
		"thumb", dst)

	return dst, nil
}

func (srv *server) SyncChannelFromPlaylist(ctx context.Context, rq *mediapb.SyncChannelFromPlaylistRequest) (*mediapb.SyncChannelFromPlaylistResponse, error) {
	// Optional: auth – similar to UploadVideo
	if _, _, err := security.GetClientId(ctx); err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(rq.GetPlaylistJson())
	if raw == "" {
		return nil, status.Errorf(codes.InvalidArgument, "playlist_json is required")
	}

	ch, err := srv.buildChannelFromPlaylistJSON([]byte(raw))
	if err != nil {
		return nil, err
	}

	// Persist to ".hidden/__channels__/<id>.json"
	if err := srv.saveChannel(ch); err != nil {
		return nil, status.Errorf(codes.Internal, "save channel failed: %v", err)
	}

	if _, err := srv.ensureChannelThumbnail(ch); err != nil {
		slog.Warn("SyncChannelFromPlaylist: ensure channel thumbnail failed",
			"id", ch.Id, "err", err)
	}

	slog.Info("channel synced from playlist", "id", ch.Id, "url", ch.Url, "path", ch.Path, "count", len(ch.Items))
	return &mediapb.SyncChannelFromPlaylistResponse{Channel: ch}, nil
}

func (srv *server) GetChannel(ctx context.Context, rq *mediapb.GetChannelRequest) (*mediapb.GetChannelResponse, error) {
	if strings.TrimSpace(rq.GetId()) == "" {
		return nil, status.Errorf(codes.InvalidArgument, "channel id is required")
	}

	// If path is specified, only use that root.
	root := rq.GetPath()
	if strings.TrimSpace(root) != "" {
		ch, err := srv.loadChannel(root, rq.GetId())
		if err != nil {
			return nil, err
		}
		if _, err := srv.ensureChannelThumbnail(ch); err != nil {
			slog.Warn("GetChannel: ensure channel thumbnail failed",
				"root", root, "id", ch.Id, "err", err)
		}
		return &mediapb.GetChannelResponse{Channel: ch}, nil
	}

	// Otherwise, search known roots (public dirs, /users, /applications),
	// same pattern you use for processVideos / processAudios.:contentReference[oaicite:2]{index=2}
	roots := srv.normalizeDirList(config.GetPublicDirs())
	roots = append(roots, "/users", "/applications")

	for _, r := range roots {
		ch, err := srv.loadChannel(r, rq.GetId())
		if err == nil && ch != nil {
			if _, err := srv.ensureChannelThumbnail(ch); err != nil {
				slog.Warn("GetChannel: ensure channel thumbnail failed",
					"root", r, "id", ch.Id, "err", err)
			}
			return &mediapb.GetChannelResponse{Channel: ch}, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "channel %s not found", rq.GetId())
}

func (srv *server) ListChannels(ctx context.Context, rq *mediapb.ListChannelsRequest) (*mediapb.ListChannelsResponse, error) {
	root := strings.TrimSpace(rq.GetPath())
	if root == "" {
		return nil, status.Errorf(codes.InvalidArgument, "path is required")
	}

	dir := srv.channelStorageDir(root)
	dir = filepath.ToSlash(dir)
	if dir == "" || !srv.pathExists(dir) {
		// No channels yet is not an error; just return empty list.
		return &mediapb.ListChannelsResponse{Channels: []*mediapb.Channel{}}, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list channels failed: %v", err)
	}

	out := &mediapb.ListChannelsResponse{
		Channels: make([]*mediapb.Channel, 0, len(entries)),
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}

		// ID derived from filename
		id := strings.TrimSuffix(e.Name(), ".json")
		if strings.TrimSpace(id) == "" {
			slog.Warn("ListChannels: skip file with empty id", "file", e.Name())
			continue
		}

		data, err := srv.readFile(filepath.Join(dir, e.Name()))
		if err != nil {
			slog.Warn("ListChannels: read channel failed", "file", e.Name(), "err", err)
			continue
		}

		ch := new(mediapb.Channel)
		if err := protojson.Unmarshal(data, ch); err != nil {
			slog.Warn("ListChannels: unmarshal channel failed", "file", e.Name(), "err", err)
			continue
		}

		// Ensure ID consistency between filename and JSON
		jsonID := strings.TrimSpace(ch.Id)
		switch {
		case jsonID == "":
			// JSON missing id: use filename
			ch.Id = id

		case jsonID != id:
			// Mismatch: log and keep JSON ID (or you could choose to override)
			slog.Warn("ListChannels: channel id mismatch",
				"filename_id", id,
				"json_id", jsonID,
				"file", e.Name(),
			)
		}

		// Optional extractor filter
		if filter := strings.TrimSpace(rq.GetExtractor()); filter != "" {
			if !strings.EqualFold(filter, ch.Extractor) {
				continue
			}
		}

		out.Channels = append(out.Channels, ch)
	}

	return out, nil
}
