package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/StalkR/imdb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/media/mediapb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/title_client"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"github.com/mitchellh/go-ps"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// NOTE: This file has been refactored to use slog for logging, clearer error messages, and
// comments for public functions. Public function prototypes are preserved.

func (srv *server) startProcessAudios() {
	ticker := time.NewTicker(4 * time.Hour)
	dirs := append([]string{}, config.GetPublicDirs()...)
	dirs = append(dirs, config.GetDataDir()+"/files/users")
	dirs = append(dirs, config.GetDataDir()+"/files/applications")

	go func() {
		processAudios(srv, dirs)
		for range ticker.C {
			processAudios(srv, dirs)
		}
	}()
}

// processAudios scans for audio files and ensures playlists exist.
func processAudios(srv *server, dirs []string) {
	if srv.isProcessingAudio {
		return
	}
	srv.isProcessingAudio = true
	defer func() { srv.isProcessingAudio = false }()

	for _, audio := range getAudioPaths(dirs) {
		dir := filepath.Dir(audio)
		if !Utility.Exists(dir + "/audio.m3u") {
			if err := srv.generatePlaylist(dir, ""); err != nil {
				logger.Error("generate audio playlist failed", "dir", dir, "err", err)
			}
		}
	}
}

// restoreVideoInfos ensures that a media file at videoPath has its metadata restored
// and (re)associated in the Title service.
//
// It looks for an embedded JSON blob (often base64-encoded) in the file's metadata
// under format.tags.comment. The blob is expected to represent either a titlepb.Title
// or a titlepb.Video. If the entity does not yet exist in the index, it will be created,
// thumbnails/posters will be fetched/derived when possible, and the file path will be
// associated. If the entity already exists, the function will (re)associate the path.
//
// Params:
//   - client: optional Title client; if nil, a local client is obtained.
//   - token:  unused in this function but kept to preserve the public signature.
//   - videoPath: absolute path to the file or HLS playlist.
//   - domain: domain used to scope cache keys and lookups.
//
// Returns:
//   - error describing the first hard failure encountered; nil if everything
//     succeeded or no actionable metadata was present.
func restoreVideoInfos(client *title_client.Title_Client, token, videoPath, domain string) error {
	p := filepath.ToSlash(videoPath)

	// Probe metadata from the file or its folder (HLS).
	infos, err := getVideoInfos(p, domain)
	if err != nil {
		logger.Error("restoreVideoInfos: getVideoInfos failed", "path", p, "err", err)
		return err
	}

	// Ensure we have a client.
	if client == nil {
		client, err = getTitleClient()
		if err != nil {
			logger.Error("restoreVideoInfos: getTitleClient failed", "err", err)
			return err
		}
	}

	// Bust any related cache entry for that path so future reads see updates.
	cache.RemoveItem(p)

	// Navigate to format.tags.comment safely.
	format, ok := infos["format"].(map[string]interface{})
	if !ok || format == nil {
		// Nothing we can restore from.
		return nil
	}
	tags, ok := format["tags"].(map[string]interface{})
	if !ok || tags == nil {
		return nil
	}
	rawComment, _ := tags["comment"].(string)
	comment := strings.TrimSpace(rawComment)
	if comment == "" {
		return nil
	}

	// Decode base64 if needed; fall back to the raw JSON string.
	jsonBytes, err := base64.StdEncoding.DecodeString(comment)
	if err != nil {
		jsonBytes = []byte(comment)
	}
	// Quick sanity check.
	if !strings.Contains(string(jsonBytes), "{") {
		return nil
	}

	// Try Title first.
	{
		title := new(titlepb.Title)
		if err := protojson.Unmarshal(jsonBytes, title); err == nil && title.ID != "" {
			return restoreAsTitle(client, title, p)
		}
	}

	// Otherwise try Video.
	{
		video := new(titlepb.Video)
		if err := protojson.Unmarshal(jsonBytes, video); err == nil && video.ID != "" {
			return restoreAsVideo(client, video, p)
		}
	}

	// If we get here, the JSON wasn't a Title nor a Video we recognize.
	logger.Warn("restoreVideoInfos: unsupported embedded JSON", "path", p)
	return nil
}

// restoreAsTitle creates or links a Title and associates the file path.
func restoreAsTitle(client *title_client.Title_Client, title *titlepb.Title, videoPath string) error {
	indexPath := config.GetDataDir() + "/search/titles"
	rel := strings.ReplaceAll(strings.ReplaceAll(videoPath, config.GetDataDir()+"/files", ""), "/playlist.m3u8", "")

	// Check if Title already exists.
	existing, _, err := client.GetTitleById(indexPath, title.ID)
	if err != nil && existing == nil {
		// Not found: try to enrich from IMDB and create it.
		if err := enrichTitleFromIMDB(title, videoPath); err != nil {
			logger.Warn("restoreAsTitle: enrich from IMDB failed", "id", title.ID, "err", err)
		}
		if err := client.CreateTitle("", indexPath, title); err != nil {
			logger.Error("restoreAsTitle: CreateTitle failed", "id", title.ID, "err", err)
			return err
		}
		logger.Info("restoreAsTitle: created title", "id", title.ID)
	}
	// (Re)associate file path.
	if err := client.AssociateFileWithTitle(indexPath, title.ID, rel); err != nil {
		logger.Error("restoreAsTitle: AssociateFileWithTitle failed", "id", title.ID, "path", rel, "err", err)
		return err
	}
	return nil
}

// restoreAsVideo creates or links a Video and associates the file path.
func restoreAsVideo(client *title_client.Title_Client, video *titlepb.Video, videoPath string) error {
	indexPath := config.GetDataDir() + "/search/videos"
	rel := strings.ReplaceAll(strings.ReplaceAll(videoPath, config.GetDataDir()+"/files", ""), "/playlist.m3u8", "")

	// Check if Video already exists.
	existing, _, err := client.GetVideoById(indexPath, video.ID)
	if err != nil && existing == nil {
		// Prepare poster/thumbnail if missing; compute duration.
		if video.Poster == nil {
			video.Poster = &titlepb.Poster{ID: video.ID}
		}
		if video.Poster.ContentUrl == "" {
			if url, _ := downloadThumbnail(video.ID, video.URL, videoPath); url != "" {
				video.Poster.ContentUrl = url
			}
		}
		video.Duration = int32(getVideoDuration(videoPath))

		if err := client.CreateVideo("", indexPath, video); err != nil {
			logger.Error("restoreAsVideo: CreateVideo failed", "id", video.ID, "err", err)
			return err
		}
		logger.Info("restoreAsVideo: created video", "id", video.ID)
		// Associate the file path.
		if err := client.AssociateFileWithTitle(indexPath, video.ID, rel); err != nil {
			logger.Error("restoreAsVideo: AssociateFileWithTitle failed", "id", video.ID, "path", rel, "err", err)
			return err
		}
		return nil
	}

	// Already exists: (re)associate path, best-effort.
	if err := client.AssociateFileWithTitle(indexPath, existing.ID, rel); err != nil {
		logger.Error("restoreAsVideo: AssociateFileWithTitle failed", "id", existing.ID, "path", rel, "err", err)
		return err
	}
	return nil
}

// enrichTitleFromIMDB populates Poster/ratings/cast from IMDB and writes a local thumbnail.
func enrichTitleFromIMDB(t *titlepb.Title, videoPath string) error {
	httpCli := getHTTPClient()
	it, err := imdb.NewTitle(httpCli, t.ID)
	if err != nil {
		return err
	}

	// Poster URL (remote) and local thumbnail.
	if posterURL, err := GetIMDBPoster(t.ID); err == nil && posterURL != "" {
		if t.Poster == nil {
			t.Poster = &titlepb.Poster{ID: t.ID}
		}
		t.Poster.URL = posterURL
		t.Poster.ContentUrl = posterURL

		// Build a local thumbnail beside the video (under .hidden/<name>/__thumbnail__).
		thumbDir := thumbnailDirFor(videoPath)
		if err := Utility.CreateIfNotExists(thumbDir, 0644); err == nil {
			dst := filepath.Join(thumbDir, posterURL[strings.LastIndex(posterURL, "/")+1:])
			if err := Utility.DownloadFile(posterURL, dst); err == nil {
				if dataURL, err := Utility.CreateThumbnail(dst, 300, 180); err == nil {
					_ = os.WriteFile(filepath.Join(thumbDir, "data_url.txt"), []byte(dataURL), 0664)
					t.Poster.ContentUrl = dataURL
				}
			}
		}
	}

	// Ratings.
	t.Rating = float32(Utility.ToNumeric(it.Rating))
	t.RatingCount = int32(it.RatingCount)

	// Cast/crew.
	t.Actors = make([]*titlepb.Person, 0, len(it.Actors))
	for _, a := range it.Actors {
		t.Actors = append(t.Actors, &titlepb.Person{ID: a.ID, FullName: a.FullName, URL: a.URL})
	}
	t.Directors = make([]*titlepb.Person, 0, len(it.Directors))
	for _, d := range it.Directors {
		t.Directors = append(t.Directors, &titlepb.Person{ID: d.ID, FullName: d.FullName, URL: d.URL})
	}
	t.Writers = make([]*titlepb.Person, 0, len(it.Writers))
	for _, w := range it.Writers {
		t.Writers = append(t.Writers, &titlepb.Person{ID: w.ID, FullName: w.FullName, URL: w.URL})
	}

	return nil
}

// thumbnailDirFor returns the .hidden thumbnail directory for a video path.
func thumbnailDirFor(videoPath string) string {
	p := filepath.ToSlash(videoPath)
	base := p[:strings.LastIndex(p, "/")]
	name := p[strings.LastIndex(p, "/")+1:]
	if i := strings.LastIndex(name, "."); i != -1 && strings.HasSuffix(strings.ToLower(name), ".mp4") {
		name = name[:i]
	}
	return filepath.Join(base, ".hidden", name, "__thumbnail__")
}

// processVideoInfo consumes a .info.json (yt-dlp) to create Title/Video and local artifacts.
func (srv *server) processVideoInfo(token, infoPath string) error {
	mediaInfo := map[string]any{}
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &mediaInfo); err != nil {
		return err
	}
	ext, _ := mediaInfo["ext"].(string)
	if ext == "" {
		return errors.New("info.json has no ext field")
	}

	dir := strings.ReplaceAll(filepath.Dir(infoPath), "\\", "/")
	file := filepath.Base(infoPath)
	mediaPath := dir + "/" + file[:strings.Index(file, ".")] + "." + ext
	mediaPath = strings.ReplaceAll(mediaPath, "\\", "/")

	// create playlists & previews
	switch ext {
	case "mp4":
		if Utility.Exists(mediaPath) {
			if err := srv.createVideoInfo(token, strings.ReplaceAll(dir, config.GetDataDir()+"/files/", "/"), mediaPath, infoPath); err != nil {
				return err
			}
			go func(p string) {
				srv.createVideoPreview(p, 20, 128, false)
				srv.generateVideoPreview(p, 10, 320, 30, true)
				srv.createVideoTimeLine(p, 180, .2, false)
			}(strings.ReplaceAll(mediaPath, "/.hidden/", "/"))
		}
	case "mp3":
		if Utility.Exists(mediaPath) {
			if err := srv.generatePlaylist(filepath.Dir(mediaPath), ""); err != nil {
				return err
			}
		}
	}

	if err := srv.setOwner(token, strings.ReplaceAll(dir, config.GetDataDir()+"/files/", "/")+"/"+filepath.Base(mediaPath)); err != nil {
		return err
	}
	if rmErr := os.Remove(infoPath); rmErr != nil {
		logger.Warn("remove info.json failed", "path", infoPath, "err", rmErr)
	}
	return nil
}

func processVideos(srv *server, token string, dirs []string) {
	videoInfos := getVideoInfoPaths(dirs)
	if srv.isProcessing {
		return
	}
	srv.isProcessing = true
	defer func() { srv.isProcessing = false }()

	// Step 1: consume pending info.json files
	for _, info := range videoInfos {
		if err := srv.processVideoInfo(token, info); err != nil {
			logger.Error("processVideoInfo failed", "info", info, "err", err)
		}
	}

	videoPaths := getVideoPaths(dirs)
	client, err := getTitleClient()
	if err != nil {
		logger.Error("connect title client failed", "err", err)
	} else {
		// Restore series from infos.json
		for _, d := range dirs {
			infos := Utility.GetFilePathsByExtension(d, "infos.json")
			for _, p := range infos {
				data, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				m := map[string]any{}
				if json.Unmarshal(data, &m) != nil {
					continue
				}
				if t, _ := m["Type"].(string); t != "TVSeries" {
					continue
				}
				title := new(titlepb.Title)
				if err := protojson.Unmarshal(data, title); err != nil {
					continue
				}
				if _, _, err := client.GetTitleById(config.GetDataDir()+"/search/titles", title.ID); err == nil {
					continue
				}
				if poster, err := GetIMDBPoster(title.ID); err == nil {
					title.Poster.URL, title.Poster.ContentUrl, title.Poster.ID = poster, poster, title.ID
				}
				if err := client.CreateTitle("", config.GetDataDir()+"/search/titles", title); err == nil {
					client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, d)
				}
			}
		}
		for _, vp := range videoPaths {
			if err := restoreVideoInfos(client, token, vp, srv.Domain); err != nil {
				logger.Warn("restoreVideoInfos failed", "path", vp, "err", err)
			}
		}
	}

	// Step 2: previews & timelines
	for _, video := range videoPaths {
		log := &mediapb.VideoConversionLog{LogTime: time.Now().Unix(), Msg: "Create video preview", Path: strings.ReplaceAll(video, config.GetDataDir()+"/files", ""), Status: "running"}
		srv.videoConversionLogs.Store(log.LogTime, log)
		srv.publishConvertionLogEvent(log)
		if err := srv.createVideoPreview(video, 20, 128, false); err != nil {
			log.Status = "fail"
			srv.publishConvertionLogEvent(log)
			srv.publishConvertionLogError(log.Path, err)
		} else {
			log.Status = "done"
			srv.publishConvertionLogEvent(log)
		}

		g := &mediapb.VideoConversionLog{LogTime: time.Now().Unix(), Msg: "Generate video Gif image", Path: strings.ReplaceAll(video, config.GetDataDir()+"/files", ""), Status: "running"}
		srv.videoConversionLogs.Store(g.LogTime, g)
		srv.publishConvertionLogEvent(g)
		if err := srv.generateVideoPreview(video, 10, 320, 30, false); err != nil {
			g.Status = "fail"
			srv.publishConvertionLogEvent(g)
			srv.publishConvertionLogError(g.Path, err)
		} else {
			g.Status = "done"
			srv.publishConvertionLogEvent(g)
		}

		t := &mediapb.VideoConversionLog{LogTime: time.Now().Unix(), Msg: "Generate video time line", Path: strings.ReplaceAll(video, config.GetDataDir()+"/files", ""), Status: "running"}
		srv.videoConversionLogs.Store(t.LogTime, t)
		srv.publishConvertionLogEvent(t)
		if err := srv.createVideoTimeLine(video, 180, .2, false); err != nil {
			t.Status = "fail"
			srv.publishConvertionLogEvent(t)
			srv.publishConvertionLogError(t.Path, err)
		} else {
			t.Status = "done"
			srv.publishConvertionLogEvent(t)
		}
		if !srv.isProcessing || srv.isExpired() {
			break
		}
	}

	// Step 3: conversions & HLS
	for _, video := range videoPaths {
		if strings.HasSuffix(video, ".m3u8") || !strings.Contains(video, ".") {
			continue
		}

		dir := video[:strings.LastIndex(video, ".")]
		if Utility.Exists(dir+"/playlist.m3u8") && Utility.Exists(video) {
			continue
		}

		if _, hasFail := srv.videoConversionErrors.Load(video); hasFail {
			continue
		}

		// Transcode to mp4/h264 when needed
		if strings.HasSuffix(strings.ToLower(video), ".mkv") || strings.HasSuffix(strings.ToLower(video), ".avi") || getCodec(video) == "hevc" {
			log := &mediapb.VideoConversionLog{LogTime: time.Now().Unix(), Msg: "Convert video to mp4 h.264", Path: strings.ReplaceAll(video, config.GetDataDir()+"/files", ""), Status: "running"}
			srv.videoConversionLogs.Store(log.LogTime, log)
			srv.publishConvertionLogEvent(log)
			out, err := srv.createVideoMpeg4H264(video)
			if err != nil {
				log.Status = "fail"
				srv.publishConvertionLogEvent(log)
				srv.publishConvertionLogError(video, err)
			} else {
				video = out
				log.Status = "done"
				srv.publishConvertionLogEvent(log)
			}
		}

		// Ensure AAC/default audio
		if strings.HasSuffix(strings.ToLower(video), ".mp4") {
			if err := ensureAACDefault(video); err != nil {
				logger.Warn("ensure AAC failed", "path", video, "err", err)
			}
		}

		if srv.AutomaticStreamConversion {
			log := &mediapb.VideoConversionLog{LogTime: time.Now().Unix(), Msg: "Convert video to mp4", Path: strings.ReplaceAll(video, config.GetDataDir()+"/files", ""), Status: "running"}
			srv.videoConversionLogs.Store(log.LogTime, log)
			srv.publishConvertionLogEvent(log)
			if err := srv.createHlsStreamFromMpeg4H264(video); err != nil {
				log.Status = "fail"
				srv.publishConvertionLogEvent(log)
				srv.publishConvertionLogError(video, err)
			} else {
				log.Status = "done"
				srv.publishConvertionLogEvent(log)
			}
		}
		if !srv.isProcessing || srv.isExpired() {
			break
		}
	}
}

// getAudioPaths returns all audio file paths under the given directories.
func getAudioPaths(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info == nil {
				return fmt.Errorf("stat missing for %s", path)
			}
			if info.IsDir() {
				if empty, err := Utility.IsEmpty(filepath.Join(path, info.Name())); err == nil && empty {
					_ = os.RemoveAll(filepath.Join(path, info.Name()))
				}
				return nil
			}
			p := filepath.ToSlash(path)
			if strings.Contains(p, ".hidden") || strings.Contains(p, ".temp") {
				return nil
			}
			if strings.HasSuffix(p, ".mp3") || strings.HasSuffix(p, ".wav") || strings.HasSuffix(p, ".flac") || strings.HasSuffix(p, ".flc") || strings.HasSuffix(p, ".acc") || strings.HasSuffix(p, ".ogg") {
				out = append(out, p)
			}
			return nil
		})
	}
	return out
}

// getVideoPaths returns video & HLS playlist file paths under the given directories.
func getVideoPaths(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.Contains(path, ".hidden") {
				return nil
			}
			if info == nil {
				return fmt.Errorf("stat missing for %s", path)
			}
			if info.IsDir() {
				if empty, err := Utility.IsEmpty(filepath.Join(path, info.Name())); err == nil && empty {
					_ = os.RemoveAll(filepath.Join(path, info.Name()))
				}
				return nil
			}
			p := filepath.ToSlash(path)
			if strings.Contains(p, ".temp") {
				return nil
			}
			if strings.HasSuffix(p, "playlist.m3u8") || strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".mkv") || strings.HasSuffix(p, ".avi") || strings.HasSuffix(p, ".mov") || strings.HasSuffix(p, ".wmv") {
				out = append(out, p)
			}
			return nil
		})
	}
	return out
}

func getVideoInfoPaths(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if strings.Contains(path, "__timeline__") || strings.Contains(path, "__preview__") || strings.Contains(path, "__thumbnail__") {
				return nil
			}
			if err != nil {
				return err
			}
			if info == nil {
				return fmt.Errorf("stat missing for %s", path)
			}
			if info.IsDir() {
				if empty, err := Utility.IsEmpty(filepath.Join(path, info.Name())); err == nil && empty {
					_ = os.RemoveAll(filepath.Join(path, info.Name()))
				}
				return nil
			}
			p := filepath.ToSlash(path)
			if strings.HasSuffix(p, ".info.json") {
				out = append(out, p)
			}
			return nil
		})
	}
	logger.Info("pending info.json files", "count", len(out))
	return out
}

// Dissociate file, if the if is deleted...
func dissociateFileWithTitle(path string, domain string) error {

	path = strings.ReplaceAll(path, "\\", "/")

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return err
	}

	titles, err := client.GetFileTitles(config.GetDataDir()+"/search/titles", path)
	if err == nil {
		// Here I will asscociate the path
		for _, title := range titles {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, path)
		}
	}

	// Look for videos
	videos, err := getFileVideos(path, domain)
	if err == nil {
		// Here I will asscociate the path
		for _, video := range videos {
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, path)
		}
	}

	return nil
}

func getFileVideos(path string, domain string) ([]*titlepb.Video, error) {

	id := path + "@" + domain + ":videos"
	data, err := cache.GetItem(id)
	videos := new(titlepb.Videos)

	if err == nil && data != nil {
		err = protojson.Unmarshal(data, videos)
		if err == nil {
			return videos.Videos, err
		}
		cache.RemoveItem(id)
	}

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	// get from the title srv.
	videos.Videos, err = client.GetFileVideos(config.GetDataDir()+"/search/videos", path)
	if err != nil {
		return nil, err
	}

	// keep to cache...
	str, _ := protojson.Marshal(videos)
	cache.SetItem(id, str)

	return videos.Videos, nil

}

func getFileTitles(path string) ([]*titlepb.Title, error) {

	id := path + ":titles"

	data, err := cache.GetItem(id)
	titles := new(titlepb.Titles)

	if err == nil && data != nil {
		err = protojson.Unmarshal(data, titles)
		if err == nil {
			return titles.Titles, err
		}
		cache.RemoveItem(id)
	}

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return nil, err
	}

	titles.Titles, err = client.GetFileTitles(config.GetDataDir()+"/search/titles", path)
	if err != nil {
		return nil, err
	}
	// keep to cache...
	str, _ := protojson.Marshal(titles)
	cache.SetItem(id, str)

	return titles.Titles, nil
}

// Reassociate a path when it name was change...
func reassociatePath(path, new_path, domain string) error {
	path = strings.ReplaceAll(path, "\\", "/")

	// So here I will try to retreive indexation for the file...
	client, err := getTitleClient()
	if err != nil {
		return err
	}

	// Now I will asscociate the title.
	titles, err := getFileTitles(path)
	if err == nil {
		// Here I will asscociate the path
		for _, title := range titles {
			client.AssociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, new_path)
			client.DissociateFileWithTitle(config.GetDataDir()+"/search/titles", title.ID, path)
		}
	}

	// Look for videos
	videos, err := getFileVideos(path, domain)

	if err == nil {
		// Here I will asscociate the path
		for _, video := range videos {
			err_0 := client.AssociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, new_path)
			if err_0 != nil {
				fmt.Println("fail to associte file ", err)
			}
			err_1 := client.DissociateFileWithTitle(config.GetDataDir()+"/search/videos", video.ID, path)
			if err_1 != nil {
				fmt.Println("fail to dissocite file ", err_1)
			}
		}
	}

	return nil
}

// formatDuration renders a duration as HH:MM:SS.000 (WebVTT-style).
func formatDuration(d time.Duration) string {
	totalMs := d.Milliseconds()

	h := totalMs / 3_600_000
	totalMs -= h * 3_600_000

	m := totalMs / 60_000
	totalMs -= m * 60_000

	s := totalMs / 1_000

	// Always .000 ms
	return fmt.Sprintf("%02d:%02d:%02d.000", h, m, s)
}

// createVttFile generates a WEBVTT file (thumbnails.vtt) inside the given output
// directory using the JPG thumbnails present there. Each thumbnail is assumed to
// represent a frame covering a window of 1/fps seconds.
//
// Parameters:
//   - output: absolute or relative directory path containing JPG thumbnails
//   - fps: frames per second used to space cues (must be > 0)
//
// Returns:
//   - error if the directory can't be read, fps is invalid, or writing the VTT fails.
func createVttFile(output string, fps float32) error {
	// Validate inputs early.
	if fps <= 0 {
		return fmt.Errorf("createVttFile: fps must be > 0 (got %.3f)", fps)
	}

	// Normalize path separators.
	output = filepath.ToSlash(output)

	entries, err := Utility.ReadDir(output)
	if err != nil {
		return fmt.Errorf("createVttFile: read dir %q: %w", output, err)
	}

	// Derive per-image duration (seconds). Ensure at least 1 second.
	delaySec := int(math.Ceil(1.0 / float64(fps)))
	if delaySec < 1 {
		delaySec = 1
	}

	address, _ := config.GetAddress()
	localCfg, _ := config.GetLocalConfig(true)

	// Build base URL (best-effort; if protocol missing, fallback to http).
	proto, _ := localCfg["Protocol"].(string)
	if proto == "" {
		proto = "http"
	}

	// Build the WEBVTT content.
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")

	elapsed := 0
	index := 1

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".jpg") {
			continue
		}

		// Cue index
		b.WriteString(strconv.Itoa(index))
		b.WriteByte('\n')

		// Time window
		start := time.Duration(elapsed) * time.Second
		elapsed += delaySec
		end := time.Duration(elapsed) * time.Second
		b.WriteString(formatDuration(start))
		b.WriteString(" --> ")
		b.WriteString(formatDuration(end))
		b.WriteByte('\n')

		// Resource URL: /<trimmed-output>/<file>.jpg
		trimmed := strings.TrimPrefix(strings.ReplaceAll(output, filepath.ToSlash(config.GetDataDir())+"/files/", ""), "/")
		b.WriteString(proto)
		b.WriteString("://")
		b.WriteString(address)
		b.WriteByte('/')
		if trimmed != "" {
			b.WriteString(trimmed)
			if !strings.HasSuffix(trimmed, "/") {
				b.WriteByte('/')
			}
		}
		b.WriteString(name)
		b.WriteString("\n\n")

		index++
	}

	// If no JPGs found, return a clear error.
	if index == 1 {
		return fmt.Errorf("createVttFile: no JPG thumbnails found in %q", output)
	}

	// Best-effort removal of previous file.
	target := filepath.ToSlash(filepath.Join(output, "thumbnails.vtt"))
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		logger.Warn("createVttFile: remove existing VTT failed", "path", target, "err", err)
	}

	// Write the new VTT.
	if err := os.WriteFile(target, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("createVttFile: write VTT %q: %w", target, err)
	}

	logger.Info("WEBVTT generated",
		"dir", output,
		"file", target,
		"fps", fps,
		"delay_sec", delaySec,
		"cues", index-1,
	)
	return nil
}

// getVideoInfos returns metadata for a media path in the same shape that ffprobe
// would produce for "format:tags:comment" (base64-encoded JSON blob).
//
// Behavior:
//   - For HLS inputs (.../playlist.m3u8): tries <dir>/infos.json first. If absent,
//     attempts to derive info from previously indexed Video/Title entities, then
//     writes <dir>/infos.json for future reads.
//   - For regular media files: defers to Utility.ReadMetadata(path).
//
// The returned map is of the form:
//
//	{"format": {"tags": {"comment": "<base64 JSON of titlepb.Title or titlepb.Video>"}}}
func getVideoInfos(path, domain string) (map[string]interface{}, error) {
	p := filepath.ToSlash(path)

	if strings.Contains(p, ".hidden") {
		err := errors.New("metadata unavailable for files in .hidden")
		logger.Warn("getVideoInfos: hidden path rejected", "path", p, "err", err)
		return nil, err
	}

	// HLS directory case: /.../<name>/playlist.m3u8
	if strings.HasSuffix(p, "playlist.m3u8") {
		dir := p[:strings.LastIndex(p, "/")]
		const infoName = "infos.json"
		infoPath := filepath.ToSlash(filepath.Join(dir, infoName))

		// 1) If a local infos.json exists, trust it.
		if Utility.Exists(infoPath) {
			data, err := os.ReadFile(infoPath)
			if err != nil {
				logger.Error("getVideoInfos: read infos.json failed", "path", infoPath, "err", err)
				return nil, err
			}

			var t titlepb.Title
			if err := protojson.Unmarshal(data, &t); err != nil {
				logger.Error("getVideoInfos: decode infos.json failed", "path", infoPath, "err", err)
				return nil, err
			}
			return buildInfoMapFromJSON(data), nil
		}

		// 2) Try to reconstruct from indexed Video first.
		videos, err := getFileVideos(dir, domain)
		if err == nil && len(videos) > 0 {
			data, mErr := protojson.Marshal(videos[0])
			if mErr != nil {
				logger.Error("getVideoInfos: marshal video failed", "path", dir, "err", mErr)
				return nil, mErr
			}
			if wErr := os.WriteFile(infoPath, data, 0664); wErr != nil {
				logger.Warn("getVideoInfos: write infos.json failed (continuing)", "path", infoPath, "err", wErr)
			}
			return buildInfoMapFromJSON(data), nil
		}

		// 3) Otherwise try Title association.
		client, err := getTitleClient()
		if err != nil {
			logger.Error("getVideoInfos: getTitleClient failed", "err", err)
			return nil, err
		}

		titles, tErr := client.GetFileTitles(config.GetDataDir()+"/search/titles", dir)
		if tErr == nil && len(titles) > 0 {
			data, mErr := protojson.Marshal(titles[0])
			if mErr != nil {
				logger.Error("getVideoInfos: marshal title failed", "path", dir, "err", mErr)
				return nil, mErr
			}
			if wErr := os.WriteFile(infoPath, data, 0664); wErr != nil {
				logger.Warn("getVideoInfos: write infos.json failed (continuing)", "path", infoPath, "err", wErr)
			}
			return buildInfoMapFromJSON(data), nil
		}

		errNoInfo := errors.New("no metadata available for HLS stream; neither infos.json nor index entries found")
		logger.Info("getVideoInfos: no info for HLS", "path", p, "err", errNoInfo)
		return nil, errNoInfo
	}

	// Regular file: let Utility extract metadata (ffprobe wrapper).
	infos, err := Utility.ReadMetadata(p)
	if err != nil {
		logger.Error("getVideoInfos: Utility.ReadMetadata failed", "path", p, "err", err)
		return nil, err
	}
	return infos, nil
}

// buildInfoMapFromJSON wraps a raw JSON blob (Title or Video) into the ffprobe-like
// structure expected elsewhere in the pipeline:
//
//	{"format": {"tags": {"comment": "<base64(JSON)>"}}}
//
// The JSON is base64-encoded and placed under "format.tags.comment".
func buildInfoMapFromJSON(jsonBlob []byte) map[string]interface{} {
	encoded := base64.StdEncoding.EncodeToString(jsonBlob)
	return map[string]interface{}{
		"format": map[string]interface{}{
			"tags": map[string]interface{}{
				"comment": encoded,
			},
		},
	}
}

// publishConvertionLogError records and publishes a video conversion error event.
// The error is stored in videoConversionErrors and broadcast to interested clients.
func (srv *server) publishConvertionLogError(path string, convErr error) {
	// Keep the error in memory for later inspection.
	srv.videoConversionErrors.Store(path, convErr.Error())

	// Try to get an event client.
	client, err := getEventClient()
	if err != nil {
		logger.Error("publishConvertionLogError: getEventClient failed", "path", path, "err", err)
		return
	}

	// Marshal the error payload.
	payload := &mediapb.VideoConversionError{
		Path:  path,
		Error: convErr.Error(),
	}
	data, mErr := protojson.Marshal(payload)
	if mErr != nil {
		logger.Error("publishConvertionLogError: marshal failed", "path", path, "err", mErr)
		return
	}

	// Publish the event.
	if pErr := client.Publish("conversion_error_event", data); pErr != nil {
		logger.Error("publishConvertionLogError: publish failed", "path", path, "err", pErr)
	}
}

// --- Events ---------------------------------------------------------------

// publishConvertionLogEvent publishes a conversion log to subscribers.
func (srv *server) publishConvertionLogEvent(convertionLog *mediapb.VideoConversionLog) {
	client, err := getEventClient()
	if err != nil {
		logger.Error("publishConvertionLogEvent: getEventClient failed", "err", err)
		return
	}

	data, mErr := protojson.Marshal(convertionLog)
	if mErr != nil {
		logger.Error("publishConvertionLogEvent: marshal failed", "err", mErr)
		return
	}

	if pErr := client.Publish("conversion_log_event", data); pErr != nil {
		logger.Error("publishConvertionLogEvent: publish failed", "err", pErr)
	}
}

// --- Audio indexing / association ----------------------------------------

/**
 * return the audios and file associations.
 */
func (srv *server) getFileAudiosAssociation(client *title_client.Title_Client, path string, audios map[string][]*titlepb.Audio) error {
	pathNorm := srv.formatPath(path)
	auds, err := client.GetFileAudios(config.GetDataDir()+"/search/audios", pathNorm)
	if err == nil {
		// Store under the original caller key to match their lookup.
		audios[path] = auds
	}
	return err
}

// Create an audio info if not exist and reassociate path with the title.
func (srv *server) createAudio(client *title_client.Title_Client, path string, duration int, metadata map[string]interface{}) error {
	// Already have associations?
	audiosByPath := make(map[string][]*titlepb.Audio)
	if err := srv.getFileAudiosAssociation(client, path, audiosByPath); err != nil {
		// If none exist, create from metadata.
		if err.Error() == "no audios found" {
			album := mdStr(metadata, "Album")
			title := mdStr(metadata, "Title")
			albumArtist := mdStr(metadata, "AlbumArtist")

			track := &titlepb.Audio{
				ID:          Utility.GenerateUUID(album + ":" + title + ":" + albumArtist),
				Album:       album,
				AlbumArtist: albumArtist,
				Artist:      mdStr(metadata, "Artist"),
				Comment:     mdStr(metadata, "Comment"),
				Composer:    mdStr(metadata, "Composer"),
				Genres:      strings.Split(mdStr(metadata, "Genre"), " / "),
				Lyrics:      mdStr(metadata, "Lyrics"),
				Title:       title,
				Year:        int32(mdInt(metadata, "Year")),
				DiscNumber:  int32(mdInt(metadata, "DiscNumber")),
				DiscTotal:   int32(mdInt(metadata, "DiscTotal")),
				TrackNumber: int32(mdInt(metadata, "TrackNumber")),
				TrackTotal:  int32(mdInt(metadata, "TrackTotal")),
				Duration:    int32(duration),
				Poster: &titlepb.Poster{
					ID:         "", // will set below
					URL:        "",
					TitleId:    "",
					ContentUrl: mdStr(metadata, "ImageUrl"),
				},
			}
			track.Poster.ID = track.ID
			track.Poster.TitleId = track.ID

			if cErr := client.CreateAudio("", config.GetDataDir()+"/search/audios", track); cErr != nil {
				logger.Error("createAudio: CreateAudio failed", "path", path, "err", cErr)
				return cErr
			}
			if aErr := client.AssociateFileWithTitle(config.GetDataDir()+"/search/audios", track.ID, path); aErr != nil {
				logger.Error("createAudio: AssociateFileWithTitle failed", "path", path, "id", track.ID, "err", aErr)
				return aErr
			}
			return nil
		}
		// Unexpected error
		return err
	}

	// Force re-associations for already-known tracks.
	for _, a := range audiosByPath[path] {
		if aErr := client.AssociateFileWithTitle(config.GetDataDir()+"/search/audios", a.ID, path); aErr != nil {
			logger.Warn("createAudio: reassociate failed", "path", path, "id", a.ID, "err", aErr)
		}
	}
	return nil
}

// --- Playlist ordering (optional .hidden/playlist.json) -------------------

func (srv *server) orderedPlayList(path string, files []string) []string {
	conf := filepath.ToSlash(filepath.Join(path, ".hidden", "playlist.json"))
	if !Utility.Exists(conf) {
		return files
	}

	data, err := os.ReadFile(conf)
	if err != nil {
		logger.Warn("orderedPlayList: read failed", "path", conf, "err", err)
		return files
	}

	var playlist map[string]interface{}
	if err := json.Unmarshal(data, &playlist); err != nil {
		logger.Warn("orderedPlayList: unmarshal failed", "path", conf, "err", err)
		return files
	}

	items, ok := playlist["items"].([]interface{})
	if !ok {
		return files
	}
	format := mdStr(playlist, "format")

	ordered := make([]string, 0, len(items))
	for _, it := range items {
		m, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		id := mdStr(m, "id")
		if id == "" {
			continue
		}
		fp := filepath.ToSlash(filepath.Join(path, id+"."+format))
		ordered = append(ordered, fp)
		files = Utility.RemoveString(files, fp)
	}

	return append(ordered, files...)
}

// --- Directory playlist generation ---------------------------------------

func (srv *server) generatePlaylist(path, token string) error {
	entries, err := Utility.ReadDir(path)
	if err != nil {
		return err
	}

	var videos []string
	var audios []string

	for _, e := range entries {
		filename := filepath.Join(path, e.Name())
		info, gErr := srv.getFileInfo(token, filename)
		if gErr != nil {
			continue
		}

		// Resolve Windows-style link (.lnk) pointing to real path we own.
		if strings.HasSuffix(e.Name(), ".lnk") {
			if data, rErr := os.ReadFile(filename); rErr == nil {
				var lnk map[string]interface{}
				if json.Unmarshal(data, &lnk) == nil {
					target := srv.formatPath(mdStr(lnk, "path"))
					if Utility.Exists(target) {
						info, _ = srv.getFileInfo(token, target)
						filename = target
					}
				}
			}
		}

		if info.IsDir {
			if Utility.Exists(info.Path + "/playlist.m3u8") {
				videos = append(videos, info.Path+"/playlist.m3u8")
			}
			continue
		}

		// Skip playlists themselves.
		if strings.HasSuffix(filename, ".m3u") {
			continue
		}

		if strings.HasPrefix(info.Mime, "audio/") {
			audios = append(audios, filename)
		} else if strings.HasPrefix(info.Mime, "video/") && !strings.HasSuffix(info.Name, ".temp.mp4") {
			videos = append(videos, filename)
		}
	}

	if len(audios) > 0 {
		_ = srv.generateAudioPlaylist(path, token, srv.orderedPlayList(path, audios))
	}
	if len(videos) > 0 {
		_ = srv.generateVideoPlaylist(path, token, srv.orderedPlayList(path, videos))
	}

	srv.publishReloadDirEvent(path)
	return nil
}

// --- Video playlist -------------------------------------------------------
func (srv *server) generateVideoPlaylist(path, token string, paths []string) error {
	if len(paths) == 0 {
		return errors.New("no paths were given")
	}

	_, err := getTitleClient()
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n\n")
	b.WriteString("#PLAYLIST: " + strings.ReplaceAll(path, config.GetDataDir()+"/files/", "/") + "\n\n")

	localCfg, _ := config.GetLocalConfig(true)
	proto := fmt.Sprintf("%v", localCfg["Protocol"])
	domain, _ := config.GetDomain()
	port := ""
	if proto == "https" {
		port = Utility.ToString(localCfg["PortHTTPS"])
	} else {
		port = Utility.ToString(localCfg["PortHTTP"])
	}

	for _, p := range paths {
		queryKey := p
		if strings.HasSuffix(p, ".m3u8") {
			queryKey = filepath.Dir(p)
		}

		videos := make(map[string][]*titlepb.Video)
		_ = srv.getFileVideosAssociation(nil, queryKey, videos) // getFileVideosAssociation ignores client when leaf

		if len(videos[queryKey]) == 0 {
			continue
		}

		v := videos[queryKey][0]
		b.WriteString("#EXTINF:" + Utility.ToString(v.GetDuration()))
		b.WriteString(` tvg-id="` + v.ID + `"` + ` tvg-url="` + v.URL + `"` + "," + v.Description + "\n")

		// Build URL with percent-encoding per path segment.
		pNorm := strings.ReplaceAll(srv.formatPath(p), config.GetDataDir()+"/files/", "/")
		if !strings.HasPrefix(pNorm, "/") {
			pNorm = "/" + pNorm
		}
		parts := strings.Split(pNorm, "/")
		escaped := make([]string, 0, len(parts))
		for _, seg := range parts {
			if seg == "" {
				continue
			}
			escaped = append(escaped, url.PathEscape(seg))
		}
		fullURL := proto + "://" + domain + ":" + port + "/" + strings.Join(escaped, "/")
		b.WriteString(fullURL + "\n\n")
	}

	cache.RemoveItem(path + "/video.m3u")
	Utility.WriteStringToFile(path+"/video.m3u", b.String())
	return nil
}

// --- Video association lookup --------------------------------------------

/**
 * Return the list of videos description and file association
 */
func (srv *server) getFileVideosAssociation(client *title_client.Title_Client, path string, videos map[string][]*titlepb.Video) error {
	p := srv.formatPath(path)
	info, err := os.Stat(p)
	if err != nil {
		return err
	}

	// Recurse into directories (skip .hidden) unless they already contain an HLS playlist.
	if info.IsDir() && !Utility.Exists(filepath.ToSlash(filepath.Join(p, "playlist.m3u8"))) {
		ents, rErr := os.ReadDir(p)
		if rErr != nil {
			return rErr
		}
		for _, e := range ents {
			child := filepath.ToSlash(filepath.Join(path, e.Name()))
			if strings.Contains(child, ".hidden/") {
				continue
			}
			_ = srv.getFileVideosAssociation(client, child, videos)
		}
		return nil
	}

	// Leaf: resolve videos for the file/dir.
	v, gErr := getFileVideos(p, srv.Domain)
	if gErr == nil {
		videos[path] = v
	}
	return gErr
}

// helpers to safely read from metadata map
func mdStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func mdInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok && v != nil {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			return Utility.ToInt(t)
		}
	}
	return 0
}

// CreateVttFile handles the gRPC request to create a VTT (WebVTT) subtitle file.
// It receives the file path and frames per second (FPS) from the request,
// calls the createVttFile helper function to generate the VTT file,
// and returns an appropriate response or error.
//
// Parameters:
//
//	ctx - The context for the request, used for cancellation and deadlines.
//	rqst - The CreateVttFileRequest containing the path and FPS for the VTT file.
//
// Returns:
//
//	*mediapb.CreateVttFileResponse - The response indicating success.
//	error - An error if the VTT file creation fails.
func (srv *server) CreateVideoTimeLine(ctx context.Context, rqst *mediapb.CreateVideoTimeLineRequest) (*mediapb.CreateVideoTimeLineResponse, error) {
	p := srv.formatPath(rqst.Path)
	if !Utility.Exists(p) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	log := new(mediapb.VideoConversionLog)
	log.LogTime = time.Now().Unix()
	log.Msg = "Create Video time line"
	log.Path = rqst.Path
	log.Status = "running"

	srv.videoConversionLogs.Store(log.LogTime, log)
	srv.publishConvertionLogEvent(log)

	if err := srv.createVideoTimeLine(p, int(rqst.Width), rqst.Fps, true); err != nil {
		log.Status = "fail"
		srv.publishConvertionLogEvent(log)
		srv.publishConvertionLogError(rqst.Path, err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	log.Status = "done"
	srv.publishConvertionLogEvent(log)
	return &mediapb.CreateVideoTimeLineResponse{}, nil
}

// ConvertVideoToMpeg4H264 converts a video file or all video files in a directory to MPEG-4 format with H.264 encoding.
// It accepts a context and a ConvertVideoToMpeg4H264Request containing the path to the source file or directory.
// The function checks for file existence, retrieves file information, and performs the conversion for supported formats (.mkv, .avi).
// Conversion progress and errors are logged and published as events.
// Returns a ConvertVideoToMpeg4H264Response on success, or an error if the operation fails.
func (srv *server) ConvertVideoToMpeg4H264(ctx context.Context, rqst *mediapb.ConvertVideoToMpeg4H264Request) (*mediapb.ConvertVideoToMpeg4H264Response, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	p := srv.formatPath(rqst.Path)
	if !Utility.Exists(p) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found at path "+rqst.Path)))
	}

	info, err := srv.getFileInfo(token, p)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	convert := func(path string) error {
		log := &mediapb.VideoConversionLog{
			LogTime: time.Now().Unix(),
			Msg:     "Convert video to mp4",
			Path:    path,
			Status:  "running",
		}
		srv.videoConversionLogs.Store(log.LogTime, log)
		srv.publishConvertionLogEvent(log)

		_, err := srv.createVideoMpeg4H264(path)
		if err != nil {
			srv.publishConvertionLogError(path, err)
			log.Status = "fail"
			srv.publishConvertionLogEvent(log)
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		log.Status = "done"
		srv.publishConvertionLogEvent(log)
		return nil
	}

	if !info.IsDir {
		if err := convert(p); err != nil {
			return nil, err
		}
	} else {
		files := append(Utility.GetFilePathsByExtension(p, ".mkv"), Utility.GetFilePathsByExtension(p, ".avi")...)
		for _, f := range files {
			if err := convert(f); err != nil {
				return nil, err
			}
		}
	}
	return &mediapb.ConvertVideoToMpeg4H264Response{}, nil
}

// ConvertVideoToHls converts a video file or all supported video files in a directory to HLS (HTTP Live Streaming) format.
// If the input video is in AVI, MKV, or uses the HEVC codec, it is first pre-converted to MP4/H.264 format.
// The function logs conversion progress and errors, and publishes conversion events.
//
// Parameters:
//
//	ctx  - The context for request-scoped values, cancellation, and deadlines.
//	rqst - The request containing the path to the video file or directory.
//
// Returns:
//
//	*mediapb.ConvertVideoToHlsResponse - The response indicating successful conversion.
//	error                              - An error if the conversion fails or the file is not found.
func (srv *server) ConvertVideoToHls(ctx context.Context, rqst *mediapb.ConvertVideoToHlsRequest) (*mediapb.ConvertVideoToHlsResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	p := srv.formatPath(rqst.Path)
	if !Utility.Exists(p) {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no file found at path "+rqst.Path)))
	}

	info, err := srv.getFileInfo(token, p)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	needsPreconversion := func(path string) bool {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".avi" || ext == ".mkv" || getCodec(path) == "hevc"
	}

	convertAndStream := func(path string) error {
		// 1) Optional pre-conversion to MP4/H.264
		if needsPreconversion(path) {
			log := &mediapb.VideoConversionLog{
				LogTime: time.Now().Unix(),
				Msg:     "Convert video to mp4",
				Path:    path,
				Status:  "running",
			}
			srv.videoConversionLogs.Store(log.LogTime, log)
			srv.publishConvertionLogEvent(log)

			newPath, err := srv.createVideoMpeg4H264(path)
			if err != nil {
				srv.publishConvertionLogError(path, err)
				log.Status = "fail"
				srv.publishConvertionLogEvent(log)
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			log.Status = "done"
			srv.publishConvertionLogEvent(log)
			path = newPath
		}

		// 2) HLS packaging
		log := &mediapb.VideoConversionLog{
			LogTime: time.Now().Unix(),
			Msg:     "Convert video to stream",
			Path:    path,
			Status:  "running",
		}
		srv.videoConversionLogs.Store(log.LogTime, log)
		srv.publishConvertionLogEvent(log)

		if err := srv.createHlsStreamFromMpeg4H264(path); err != nil {
			srv.publishConvertionLogError(path, err)
			log.Status = "fail"
			srv.publishConvertionLogEvent(log)
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		log.Status = "done"
		srv.publishConvertionLogEvent(log)
		return nil
	}

	if !info.IsDir {
		if err := convertAndStream(p); err != nil {
			return nil, err
		}
	} else {
		files := append(Utility.GetFilePathsByExtension(p, ".mkv"), Utility.GetFilePathsByExtension(p, ".avi")...)
		for _, f := range files {
			if err := convertAndStream(f); err != nil {
				return nil, err
			}
		}
	}

	return &mediapb.ConvertVideoToHlsResponse{}, nil
}

// Create & index a Video entity from a yt-dlp info.json and associate with file.
func (srv *server) createVideoInfo(token, dirPath, filePath, infoJSON string) error {
	if strings.Contains(dirPath, ".hidden") {
		return nil
	}

	data, err := os.ReadFile(infoJSON)
	if err != nil {
		return err
	}
	info := map[string]interface{}{}
	if err := json.Unmarshal(data, &info); err != nil {
		return err
	}

	videoURL, _ := info["webpage_url"].(string)
	videoID, _ := info["id"].(string)
	if videoID == "" {
		return errors.New("missing video id in info.json")
	}

	videoPath := filepath.ToSlash(filepath.Join(dirPath, videoID+".mp4"))
	indexPath := filepath.ToSlash(config.GetDataDir() + "/search/videos")

	var v *titlepb.Video

	switch {
	case strings.Contains(videoURL, "pornhub"):
		v, err = indexPornhubVideo(token, videoID, videoURL, indexPath, videoPath, strings.ReplaceAll(filePath, "/.hidden/", ""))
	case strings.Contains(videoURL, "xnxx"):
		v, err = indexXnxxVideo(token, videoID, videoURL, indexPath, videoPath, strings.ReplaceAll(filePath, "/.hidden/", ""))
	case strings.Contains(videoURL, "xvideo"):
		v, err = indexXvideosVideo(token, videoID, videoURL, indexPath, videoPath, strings.ReplaceAll(filePath, "/.hidden/", ""))
	case strings.Contains(videoURL, "xhamster"):
		v, err = indexXhamsterVideo(token, videoID, videoURL, indexPath, videoPath, strings.ReplaceAll(filePath, "/.hidden/", ""))
	case strings.Contains(videoURL, "youtube"):
		v, err = indexYoutubeVideo(token, videoID, videoURL, indexPath, videoPath, strings.ReplaceAll(filePath, "/.hidden/", ""))
		// fallback poster from thumbnails if present
		if err == nil && v != nil && info["thumbnails"] != nil {
			if arr, ok := info["thumbnails"].([]interface{}); ok && len(arr) > 0 {
				if first, ok := arr[0].(map[string]interface{}); ok {
					if u, ok := first["url"].(string); ok {
						if v.Poster == nil {
							v.Poster = new(titlepb.Poster)
						}
						v.Poster.URL = u
					}
				}
			}
		}
	default:
		// Unknown site — try to proceed with generic fields
		v = &titlepb.Video{ID: videoID, URL: videoURL}
	}

	if err != nil || v == nil {
		if err == nil {
			err = errors.New("failed to build video from info.json")
		}
		return err
	}

	// Title/description/poster
	if full, ok := info["fulltitle"].(string); ok && full != "" {
		v.Description = full
		if thumb, ok := info["thumbnail"].(string); ok && thumb != "" {
			if v.Poster == nil {
				v.Poster = new(titlepb.Poster)
			}
			v.Poster.URL = thumb
		}
	}

	// Genres (categories)
	if cats, ok := info["categories"].([]interface{}); ok {
		for _, c := range cats {
			if s, ok := c.(string); ok {
				v.Genres = append(v.Genres, s)
			}
		}
	}

	// Tags
	if tags, ok := info["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				v.Tags = append(v.Tags, s)
			}
		}
	}

	// Likes / views / rating
	if lc, ok := numeric(info["like_count"]); ok {
		v.Likes = int64(lc)
	}
	if vc, ok := numeric(info["view_count"]); ok {
		v.Count = int64(vc)
	}
	if d, ok := numeric(info["duration"]); ok {
		v.Duration = int32(d)
	}
	if ld, lok := asFloat(info["like_count"]); lok {
		if dd, dok := asFloat(info["dislike_count"]); dok && (ld+dd) > 0 {
			v.Rating = float32(ld/(ld+dd)) * 10
		}
	}

	tcli, err := getTitleClient()
	if err != nil {
		return err
	}
	if err := tcli.CreateVideo(token, indexPath, v); err != nil {
		return err
	}
	if err := tcli.AssociateFileWithTitle(indexPath, v.ID, videoPath); err != nil {
		return err
	}
	return nil
}

func numeric(v interface{}) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case string:
		return Utility.ToInt(x), true
	default:
		return 0, false
	}
}
func asFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		return float64(Utility.ToInt(x)), true
	default:
		return 0, false
	}
}

// Use yt-dlp to get channel or video information...
// https://github.com/yt-dlp/yt-dlp/blob/master/supportedsites.md
func (srv *server) getVideoInfos(url, path, format string) (string, []map[string]interface{}, map[string]interface{}, error) {

	// wait := make(chan error)
	//Utility.RunCmd("yt-dlp", path, []string{"-j", "--flat-playlist", "--skip-download", url},  wait)
	cmd := exec.Command("yt-dlp", "-j", "--flat-playlist", "--skip-download", url)

	cmd.Dir = filepath.Dir(path)
	out, err := cmd.Output()
	if err != nil {
		return "", nil, nil, err
	}

	playlist := make([]map[string]interface{}, 0)
	jsonStr := `[` + strings.ReplaceAll(string(out), "}\n{", "},\n{") + `]`

	err = json.Unmarshal([]byte(jsonStr), &playlist)
	if err != nil {
		return "", nil, nil, err
	}

	if len(playlist) == 0 {
		return "", nil, nil, errors.New("playlist at " + url + " is empty")
	}

	if playlist[0]["playlist"] != nil {
		path_ := path + "/" + playlist[0]["playlist"].(string)
		Utility.CreateDirIfNotExist(path_)
		Utility.CreateDirIfNotExist(path_ + "/.hidden")

		// I will save the playlist in the  .hidden directory.
		playlist_ := map[string]interface{}{"url": url, "path": path, "format": format, "items": playlist}
		jsonStr, _ = Utility.ToJson(playlist_)

		err = os.WriteFile(path_+"/.hidden/playlist.json", []byte(jsonStr), 0644)
		if err != nil {
			return "", nil, nil, err
		}

		return path_, playlist, nil, nil

	} else {
		return "", nil, playlist[0], nil
	}

}

// Registerable handler to cancel an ongoing yt-dlp upload by PID and cleanup leftovers.
func cancelUploadVideoHandeler(srv *server, titleClient *title_client.Title_Client) func(evt *eventpb.Event) {
	return func(evt *eventpb.Event) {
		var data map[string]interface{}
		if err := json.Unmarshal(evt.Data, &data); err != nil {
			return
		}
		pid := Utility.ToInt(data["pid"])
		dir := srv.formatPath(asString(data["path"]))

		proc, err := os.FindProcess(pid)
		if err != nil {
			return
		}

		pinfo, err := ps.FindProcess(pid)
		if err != nil || pinfo == nil || !strings.Contains(strings.ToLower(pinfo.Executable()), "yt-dlp") {
			return // only manage yt-dlp processes
		}

		_ = proc.Signal(syscall.SIGTERM)
		time.Sleep(1 * time.Second)

		files, _ := Utility.ReadDir(dir)
		for _, f := range files {
			name := f.Name()
			full := filepath.ToSlash(filepath.Join(dir, name))

			// Remove incomplete/temp artifacts
			if strings.Contains(name, ".temp.") ||
				strings.HasSuffix(name, ".ytdl") ||
				strings.HasSuffix(name, ".webp") ||
				strings.HasSuffix(name, ".png") ||
				strings.HasSuffix(name, ".jpg") ||
				strings.HasSuffix(name, ".info.json") ||
				strings.Contains(name, ".part") {
				_ = os.Remove(full)
				continue
			}

			// Remove orphan mp4s that have no association
			if strings.HasSuffix(strings.ToLower(name), ".mp4") {
				token, _ := security.GetLocalToken(srv.Mac)
				if err := restoreVideoInfos(titleClient, token, full, srv.Domain); err != nil {
					videos := map[string][]*titlepb.Video{}
					if err := srv.getFileVideosAssociation(titleClient, strings.ReplaceAll(dir, config.GetDataDir()+"/files", "/")+"/"+name, videos); err != nil || len(videos) == 0 {
						_ = os.Remove(full)
					}
				}
			}
		}
	}
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Download (via yt-dlp) a single video/audio item and process it.
// Returns the yt-dlp PID (if obtained) and an error.
func (srv *server) uploadedVideo(token, urlStr, dest, format, outFile string, stream mediapb.MediaService_UploadVideoServer) (int, error) {
	dirPath := srv.formatPath(dest)
	if !Utility.Exists(dirPath) {
		return -1, errors.New("destination does not exist: " + dirPath)
	}

	Utility.CreateDirIfNotExist(dirPath)

	baseCmd := "yt-dlp"
	var args []string
	if format == "mp3" {
		args = []string{
			"-f", "bestaudio",
			"--extract-audio", "--audio-format", "mp3", "--audio-quality", "0",
			"--embed-thumbnail", "--embed-metadata", "--write-info-json",
			"-o", `%(id)s.%(ext)s`, urlStr,
		}
	} else if format == "mp4" {
		args = []string{"--write-info-json", "--embed-metadata", "--embed-thumbnail", "-o", `%(id)s.%(ext)s`, urlStr}
	} else {
		return -1, errors.New("format " + format + " not supported")
	}

	cmd := exec.Command(baseCmd, args...)
	cmd.Dir = dirPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	defer stdout.Close()

	output := make(chan string)
	done := make(chan bool)
	var pid int = -1

	// Stream progress to client
	go func() {
		for {
			select {
			case <-done:
				return
			case line := <-output:
				if cmd.Process != nil {
					pid = cmd.Process.Pid
				}
				_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: line})
			}
		}
	}()

	go Utility.ReadOutput(output, stdout)
	if err := cmd.Run(); err != nil {
		done <- true
		return pid, err
	}
	done <- true

	// Post-processing
	switch format {
	case "mp4":
		infoPath := strings.ReplaceAll(outFile, ".mp4", ".info.json")
		if Utility.Exists(infoPath) {
			_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "create video info for " + outFile})
			if err := srv.createVideoInfo(token, dest, outFile, infoPath); err != nil {
				_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "fail to create video info with error " + err.Error()})
			}

			// Permissions
			if err := srv.setOwner(token, dest+"/"+filepath.Base(outFile)); err != nil {
				_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "fail to create video permission with error " + err.Error()})
			} else {
				_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "create permission " + outFile})
			}

			// Cleanup info.json
			_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "remove file " + infoPath})
			if err := os.Remove(infoPath); err != nil {
				_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "fail to remove file " + err.Error()})
			}

			// Regenerate playlists
			if Utility.Exists(dirPath + "/video.m3u") {
				_ = os.Remove(dirPath + "/video.m3u")
			}
			if err := srv.generatePlaylist(dirPath, ""); err != nil {
				fmt.Println("fail to generate playlist with error ", err)
			}

			// Kick off previews asynchronously (best-effort)
			go func() {
				fileName := strings.ReplaceAll(outFile, "/.hidden/", "/")
				_ = srv.createVideoPreview(fileName, 20, 128, false)
				_ = srv.generateVideoPreview(fileName, 10, 320, 30, true)
				_ = srv.createVideoTimeLine(fileName, 180, .2, false)
			}()
		}

	case "mp3":
		infoPath := strings.ReplaceAll(outFile, ".mp3", ".info.json")
		needRefresh := false
		if Utility.Exists(infoPath) {
			needRefresh = true
			if err := srv.setOwner(token, dest+"/"+filepath.Base(outFile)); err != nil {
				fmt.Println("fail to create audio permission with error ", err)
			}
			if err := os.Remove(infoPath); err != nil {
				fmt.Println("fail to remove file ", infoPath, err)
			}
		}
		if needRefresh {
			if Utility.Exists(dirPath + "/audio.m3u") {
				_ = os.Remove(dirPath + "/audio.m3u")
			}
			if err := srv.generatePlaylist(dirPath, ""); err != nil {
				fmt.Println("fail to generate playlist with error ", err)
			}
		}
	}

	_ = stream.Send(&mediapb.UploadVideoResponse{Pid: int32(pid), Result: "done"})
	srv.publishReloadDirEvent(dirPath)
	return pid, nil
}

// --- Audio playlist -------------------------------------------------------

func (srv *server) generateAudioPlaylist(path, token string, paths []string) error {
	if len(paths) == 0 {
		return errors.New("no paths were given")
	}

	client, err := getTitleClient()
	if err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n\n")
	b.WriteString("#PLAYLIST: " + strings.ReplaceAll(path, config.GetDataDir()+"/files/", "/") + "\n\n")

	localCfg, _ := config.GetLocalConfig(true)
	proto := fmt.Sprintf("%v", localCfg["Protocol"])
	domain, _ := config.GetDomain()
	port := ""
	if proto == "https" {
		port = Utility.ToString(localCfg["PortHTTPS"])
	} else {
		port = Utility.ToString(localCfg["PortHTTP"])
	}

	for _, p := range paths {
		metadata, mErr := Utility.ReadAudioMetadata(p, 300, 300)
		dur := getVideoDuration(p)
		if mErr != nil || dur <= 0 {
			continue
		}

		id := Utility.GenerateUUID(mdStr(metadata, "Album") + ":" + mdStr(metadata, "Title") + ":" + mdStr(metadata, "AlbumArtist"))

		b.WriteString("#EXTINF:" + Utility.ToString(dur) + ",")
		b.WriteString(mdStr(metadata, "Title") + `, tvg-id="` + id + `"` + ` tvg-url=""` + "\n")

		// Build URL with percent-encoding per path segment.
		pNorm := strings.ReplaceAll(srv.formatPath(p), config.GetDataDir()+"/files/", "/")
		if !strings.HasPrefix(pNorm, "/") {
			pNorm = "/" + pNorm
		}
		parts := strings.Split(pNorm, "/")
		escaped := make([]string, 0, len(parts))
		for _, seg := range parts {
			if seg == "" {
				continue
			}
			escaped = append(escaped, url.PathEscape(seg))
		}
		fullURL := proto + "://" + domain + ":" + port + "/" + strings.Join(escaped, "/")
		b.WriteString(fullURL + "\n\n")

		// Ensure audio entity exists / associated.
		_ = srv.createAudio(client, p, dur, metadata)
	}

	cache.RemoveItem(path + "/audio.m3u")
	Utility.WriteStringToFile(path+"/audio.m3u", b.String())
	return nil
}

// StartProcessAudio processes audio files in the specified path by generating an audio playlist.
// It retrieves the client token from the context, formats the provided path, and collects audio files
// with ".mp3" and ".flac" extensions. The function then generates an audio playlist using the collected files.
// Returns a StartProcessAudioResponse on success, or an error if any step fails.
func (srv *server) StartProcessAudio(ctx context.Context, rqst *mediapb.StartProcessAudioRequest) (*mediapb.StartProcessAudioResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	path := srv.formatPath(rqst.Path)
	audios := append(Utility.GetFilePathsByExtension(path, ".mp3"), Utility.GetFilePathsByExtension(path, ".flac")...)
	if err := srv.generateAudioPlaylist(path, token, audios); err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &mediapb.StartProcessAudioResponse{}, nil
}

// IsProcessVideo returns the current status indicating whether a video is being processed.
// It responds with a boolean value encapsulated in IsProcessVideoResponse.
// This method implements the gRPC endpoint for checking video processing status.
func (srv *server) IsProcessVideo(ctx context.Context, _ *mediapb.IsProcessVideoRequest) (*mediapb.IsProcessVideoResponse, error) {
	return &mediapb.IsProcessVideoResponse{IsProcessVideo: srv.isProcessing}, nil
}

// StopProcessVideo stops the ongoing video processing by setting the isProcessing flag to false
// and terminating any running "ffmpeg" processes. It returns a StopProcessVideoResponse on success,
// or an error if the process termination fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	_   - The StopProcessVideoRequest (unused).
//
// Returns:
//
//	*mediapb.StopProcessVideoResponse - The response indicating the process has been stopped.
//	error - An error if the process could not be terminated.
func (srv *server) CreateVideoPreview(ctx context.Context, rqst *mediapb.CreateVideoPreviewRequest) (*mediapb.CreateVideoPreviewResponse, error) {
	path := srv.formatPath(rqst.Path)
	if !Utility.Exists(path) {
		return nil, errors.New("no file found at path " + rqst.Path)
	}

	log := slog.With("path", rqst.Path)
	log.Info("create video preview: start")

	if err := srv.createVideoPreview(path, int(rqst.Nb), int(rqst.Height), true); err != nil {
		log.Error("create video preview: failed", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.generateVideoPreview(path, 10, 320, 30, true); err != nil {
		log.Error("generate gif preview: failed", "err", err)
		srv.publishConvertionLogError(rqst.Path, err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	log.Info("create video preview: done")
	return &mediapb.CreateVideoPreviewResponse{}, nil
}

// GeneratePlaylist generates audio and video playlists (M3U files) for the specified directory.
// It first verifies the existence of the directory, removes any existing playlist files,
// and then calls the internal generatePlaylist method to create new playlists.
// Returns an error if the directory does not exist or playlist generation fails.
func (srv *server) GeneratePlaylist(ctx context.Context, rqst *mediapb.GeneratePlaylistRequest) (*mediapb.GeneratePlaylistResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	path := srv.formatPath(rqst.Dir)
	if !Utility.Exists(path) {
		return nil, errors.New("no file found at path " + rqst.Dir)
	}

	_ = os.Remove(path + "/audio.m3u")
	_ = os.Remove(path + "/video.m3u")

	if err := srv.generatePlaylist(path, token); err != nil {
		slog.With("path", path).Error("generate playlist failed", "err", err)
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	slog.With("path", path).Info("generate playlist: done")
	return &mediapb.GeneratePlaylistResponse{}, nil
}

// StartProcessVideo initiates the video processing workflow for the specified directories or path.
// If no path is provided in the request, it processes videos in the public, user, and application directories.
// The method checks if a video conversion is already running and returns an error if so.
// Video processing is performed asynchronously in a goroutine. After processing, it regenerates playlists
// and refreshes VTT (WebVTT) files for thumbnails in the affected directories.
// Returns an empty StartProcessVideoResponse on success, or an error if the operation cannot be started.
func (srv *server) StartProcessVideo(ctx context.Context, rqst *mediapb.StartProcessVideoRequest) (*mediapb.StartProcessVideoResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if srv.isProcessing {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("conversion is already running")))
	}

	dirs := make([]string, 0)
	if rqst.Path == "" {
		dirs = append(dirs, config.GetPublicDirs()...)
		dirs = append(dirs, config.GetDataDir()+"/files/users")
		dirs = append(dirs, config.GetDataDir()+"/files/applications")
	} else {
		dirs = append(dirs, srv.formatPath(rqst.Path))
	}
	slog.With("dirs", strings.Join(dirs, ",")).Info("start process video")

	go func() {
		processVideos(srv, token, dirs)

		// regenerate playlists + refresh VTT files
		for _, p := range dirs {
			for _, m3u := range Utility.GetFilePathsByExtension(p, "m3u") {
				cache.RemoveItem(m3u)
				_ = os.Remove(m3u)
			}
			_ = srv.generatePlaylist(p, token)

			for _, vtt := range Utility.GetFilePathsByExtension(p, ".vtt") {
				if filepath.Base(vtt) == "thumbnails.vtt" {
					_ = os.Remove(vtt)
					_ = createVttFile(filepath.Dir(vtt), 0.2)
				}
			}
		}
	}()

	return &mediapb.StartProcessVideoResponse{}, nil
}

// Use yt-dlp to get channel or single-video information.
// Returns:
//   - playlistDir (if a playlist), the raw playlist items, and nil "single" info
//   - OR "", nil, and the single "info" map
func (srv *server) getYTDLPInfos(urlStr, path, format string) (string, []map[string]interface{}, map[string]interface{}, error) {
	cmd := exec.Command("yt-dlp", "-j", "--flat-playlist", "--skip-download", urlStr)
	cmd.Dir = filepath.Dir(path)

	out, err := cmd.Output()
	if err != nil {
		return "", nil, nil, err
	}

	jsonStr := "[" + strings.ReplaceAll(string(out), "}\n{", "},\n{") + "]"
	var playlist []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &playlist); err != nil {
		return "", nil, nil, err
	}
	if len(playlist) == 0 {
		return "", nil, nil, errors.New("playlist at " + urlStr + " is empty")
	}

	// Channel/playlist case
	if playlist[0]["playlist"] != nil {
		plName := playlist[0]["playlist"].(string)
		dest := filepath.ToSlash(filepath.Join(path, plName))
		Utility.CreateDirIfNotExist(dest)
		Utility.CreateDirIfNotExist(dest + "/.hidden")

		payload := map[string]interface{}{"url": urlStr, "path": path, "format": format, "items": playlist}
		js, _ := Utility.ToJson(payload)

		if err := os.WriteFile(dest+"/.hidden/playlist.json", []byte(js), 0644); err != nil {
			return "", nil, nil, err
		}
		return dest, playlist, nil, nil
	}

	// Single-video case
	return "", nil, playlist[0], nil
}

// Upload a video (or playlist) from a URL using yt-dlp.
func (srv *server) UploadVideo(rqst *mediapb.UploadVideoRequest, stream mediapb.MediaService_UploadVideoServer) error {
	_, token, err := security.GetClientId(stream.Context())
	if err != nil {
		return err
	}

	dest := srv.formatPath(rqst.Dest)
	if !Utility.Exists(dest) {
		return status.Errorf(codes.Internal, Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no folder found with path "+dest)))
	}
	Utility.CreateDirIfNotExist(dest)

	playlistDir, playlist, info, err := srv.getYTDLPInfos(rqst.Url, dest, rqst.Format)
	if err != nil {
		return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	titleClient, err := getTitleClient()
	if err != nil {
		return err
	}

	// --- Playlist/Channel mode ---
	if len(playlist) > 0 {
		// Finish any partially downloaded items first
		files, _ := Utility.ReadDir(playlistDir)
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".info.json") {
				subDest := rqst.Dest + "/" + playlist[0]["playlist"].(string)
				infoPath := filepath.ToSlash(filepath.Join(playlistDir, f.Name()))
				mp4Path := strings.TrimSuffix(infoPath, ".info.json") + ".mp4"
				if Utility.Exists(mp4Path) {
					if err := srv.createVideoInfo(token, subDest, mp4Path, infoPath); err == nil {
						_ = srv.setOwner(token, subDest+"/"+filepath.Base(mp4Path))
					}
					_ = os.Remove(infoPath)
				}
			}
		}

		_ = srv.generatePlaylist(dest, "")

		authClient, err := getAuticationClient(srv.GetAddress())
		if err != nil {
			return err
		}

		// Cleanup temp artifacts & orphan mp4s
		for _, f := range files {
			name := f.Name()
			full := filepath.ToSlash(filepath.Join(playlistDir, name))
			if strings.Contains(name, ".temp.") ||
				strings.HasSuffix(name, ".ytdl") ||
				strings.HasSuffix(name, ".webp") ||
				strings.HasSuffix(name, ".png") ||
				strings.HasSuffix(name, ".jpg") ||
				strings.HasSuffix(name, ".info.json") ||
				strings.Contains(name, ".part-") {
				_ = os.Remove(full)
				continue
			}
			if strings.HasSuffix(strings.ToLower(name), ".mp4") {
				videos := make(map[string][]*titlepb.Video, 0)
				if err := restoreVideoInfos(titleClient, token, full, srv.Domain); err != nil {
					if err := srv.getFileVideosAssociation(titleClient, strings.ReplaceAll(playlistDir, config.GetDataDir()+"/files", "/")+"/"+name, videos); err != nil || len(videos) == 0 {
						_ = os.Remove(full)
					}
				}
			}
		}

		// Download items that are not present yet
		for _, item := range playlist {
			id := asString(item["id"])
			pl := asString(item["playlist"])
			targetMP4 := filepath.ToSlash(filepath.Join(playlistDir, id+"."+rqst.Format))

			if Utility.Exists(targetMP4) || Utility.Exists(filepath.Join(playlistDir, id)) {
				continue
			}

			// Ensure token validity
			if _, err := security.ValidateToken(token); err != nil {
				if token, err = authClient.RefreshToken(token); err != nil {
					return err
				}
			}

			pid, err := srv.uploadedVideo(token, asString(item["url"]), rqst.Dest+"/"+pl, rqst.Format, targetMP4, stream)
			if err != nil {
				_ = stream.Send(&mediapb.UploadVideoResponse{
					Pid:    int32(pid),
					Result: "fail to upload video " + id + " with error " + err.Error(),
				})
				if strings.Contains(err.Error(), "signal: killed") {
					return errors.New("fail to upload video " + id + " with error " + err.Error())
				}
			} else {
				srv.publishReloadDirEvent(playlistDir)
			}
		}

		return nil
	}

	// --- Single video mode ---
	if info != nil {
		id := asString(info["id"])
		target := filepath.ToSlash(filepath.Join(dest, id+"."+rqst.Format))

		pid, err := srv.uploadedVideo(token, rqst.Url, rqst.Dest, rqst.Format, target, stream)
		if err != nil {
			_ = stream.Send(&mediapb.UploadVideoResponse{
				Pid:    int32(pid),
				Result: "fail to upload video " + id + " with error " + err.Error(),
			})
			return errors.New("fail to upload video " + id + " with error " + err.Error())
		}
		srv.publishReloadDirEvent(dest)
	}

	return nil
}
// Clear the video conversion errors
func (srv *server) ClearVideoConversionErrors(ctx context.Context, rqst *mediapb.ClearVideoConversionErrorsRequest) (*mediapb.ClearVideoConversionErrorsResponse, error) {
	srv.videoConversionErrors.Range(func(key, value interface{}) bool {
		srv.videoConversionErrors.Delete(key)
		return true
	})

	return &mediapb.ClearVideoConversionErrorsResponse{}, nil
}

// Clear a specific video conversion error
func (srv *server) ClearVideoConversionError(ctx context.Context, rqst *mediapb.ClearVideoConversionErrorRequest) (*mediapb.ClearVideoConversionErrorResponse, error) {
	srv.videoConversionErrors.Delete(rqst.Path)
	return &mediapb.ClearVideoConversionErrorResponse{}, nil
}

// Clear a specific video conversion log
func (srv *server) ClearVideoConversionLogs(ctx context.Context, rqst *mediapb.ClearVideoConversionLogsRequest) (*mediapb.ClearVideoConversionLogsResponse, error) {

	srv.videoConversionLogs.Range(func(key, value interface{}) bool {
		srv.videoConversionLogs.Delete(key)
		return true
	})

	return &mediapb.ClearVideoConversionLogsResponse{}, nil
}


// Create a VTT file for a video.
func (s *server) CreateVttFile(ctx context.Context, rqst *mediapb.CreateVttFileRequest) (*mediapb.CreateVttFileResponse, error) {

	err := createVttFile(rqst.Path, rqst.Fps)
	if err != nil {
		return nil, err
	}

	return &mediapb.CreateVttFileResponse{}, nil
}

// Return the list of failed video conversion.
func (srv *server) GetVideoConversionErrors(ctx context.Context, rqst *mediapb.GetVideoConversionErrorsRequest) (*mediapb.GetVideoConversionErrorsResponse, error) {
	video_conversion_errors := make([]*mediapb.VideoConversionError, 0)

	srv.videoConversionErrors.Range(func(key, value interface{}) bool {
		video_conversion_errors = append(video_conversion_errors, &mediapb.VideoConversionError{Path: key.(string), Error: value.(string)})
		return true
	})

	return &mediapb.GetVideoConversionErrorsResponse{Errors: video_conversion_errors}, nil
}

// Return the list of log messages
func (srv *server) GetVideoConversionLogs(ctx context.Context, rqst *mediapb.GetVideoConversionLogsRequest) (*mediapb.GetVideoConversionLogsResponse, error) {
	logs := make([]*mediapb.VideoConversionLog, 0)

	srv.videoConversionLogs.Range(func(key, value interface{}) bool {
		logs = append(logs, value.(*mediapb.VideoConversionLog))
		return true
	})

	return &mediapb.GetVideoConversionLogsResponse{Logs: logs}, nil
}

// Set the maximum delay when conversion can run, it will finish actual conversion but it will not begin new conversion past this delay.
func (srv *server) SetMaximumVideoConversionDelay(ctx context.Context, rqst *mediapb.SetMaximumVideoConversionDelayRequest) (*mediapb.SetMaximumVideoConversionDelayResponse, error) {
	srv.MaximumVideoConversionDelay = rqst.Value
	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &mediapb.SetMaximumVideoConversionDelayResponse{}, nil
}

// Set the hour when the video conversion must start.
func (srv *server) SetStartVideoConversionHour(ctx context.Context, rqst *mediapb.SetStartVideoConversionHourRequest) (*mediapb.SetStartVideoConversionHourResponse, error) {
	srv.StartVideoConversionHour = rqst.Value

	// remove actual process video...
	srv.scheduler.Remove(processVideos)

	if srv.AutomaticVideoConversion {
		srv.scheduler.Every(1).Day().At(srv.StartVideoConversionHour).Do(processVideos, srv)
		srv.scheduler.Start()
	}

	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &mediapb.SetStartVideoConversionHourResponse{}, nil
}


// Set video processing.
func (srv *server) SetVideoConversion(ctx context.Context, rqst *mediapb.SetVideoConversionRequest) (*mediapb.SetVideoConversionResponse, error) {

	srv.AutomaticVideoConversion = rqst.Value
	// remove process video...
	srv.scheduler.Remove(processVideos)

	if srv.AutomaticVideoConversion {
		srv.scheduler.Every(1).Day().At(srv.StartVideoConversionHour).Do(processVideos, srv)
		srv.scheduler.Start()
	}

	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &mediapb.SetVideoConversionResponse{}, nil
}

// Set video stream conversion.
func (srv *server) SetVideoStreamConversion(ctx context.Context, rqst *mediapb.SetVideoStreamConversionRequest) (*mediapb.SetVideoStreamConversionResponse, error) {
	srv.AutomaticStreamConversion = rqst.Value
	err := srv.Save()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &mediapb.SetVideoStreamConversionResponse{}, nil
}


// Stop process video on the server.
func (srv *server) StopProcessVideo(ctx context.Context, rqst *mediapb.StopProcessVideoRequest) (*mediapb.StopProcessVideoResponse, error) {

	srv.isProcessing = false

	// kill current procession...
	err := Utility.KillProcessByName("ffmpeg")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &mediapb.StopProcessVideoResponse{}, nil
}