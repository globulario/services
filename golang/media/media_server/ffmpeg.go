package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

// getVideoDuration returns the duration of the given media file in seconds, rounded.
func getVideoDuration(path string) int {
	path = strings.ReplaceAll(path, "\\", "/")
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "compact=print_section=0:nokey=1:escape=csv",
		"-show_entries", "format=duration",
		path,
	)
	cmd.Dir = filepath.Dir(path)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logger.Error("ffprobe duration failed", "path", path, "stderr", strings.TrimSpace(stderr.String()), "err", err)
		return 0
	}

	dur, err := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)
	if err != nil {
		logger.Error("ffprobe duration parse failed", "path", path, "raw", strings.TrimSpace(out.String()), "err", err)
		return 0
	}
	return Utility.ToInt(dur + 0.5)
}

func (srv *server) getStartTime() time.Time {
	values := strings.Split(srv.StartVideoConversionHour, ":")
	now := time.Now()
	if len(values) == 2 {
		return time.Date(now.Year(), now.Month(), now.Day(), Utility.ToInt(values[0]), Utility.ToInt(values[1]), 0, 0, now.Location())
	}
	return now
}

func (srv *server) isExpired() bool {
	values := strings.Split(srv.MaximumVideoConversionDelay, ":")
	if len(values) != 2 {
		return false
	}
	delay := time.Duration(Utility.ToInt(values[0]))*time.Hour + time.Duration(Utility.ToInt(values[1]))*time.Minute
	if delay == 0 {
		return false
	}
	start := srv.getStartTime()
	end := start.Add(delay)
	return time.Now().After(end)
}

// getStreamFrameRateInterval returns FPS as an integer derived from r_frame_rate.
func getStreamFrameRateInterval(path string) (int, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v", "-of", "default=noprint_wrappers=1:nokey=1", "-show_entries", "stream=r_frame_rate", path)
	cmd.Dir = filepath.Dir(path)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("ffprobe r_frame_rate failed: %w", err)
	}
	parts := strings.Split(strings.TrimSpace(string(data)), "/")
	if len(parts) != 2 {
		return -1, fmt.Errorf("unexpected r_frame_rate: %q", strings.TrimSpace(string(data)))
	}
	fps := Utility.ToNumeric(parts[0]) / Utility.ToNumeric(parts[1])
	return int(fps + .5), nil
}

// getTrackInfos runs ffprobe to extract stream info of a given type (e.g. "a" for audio, "s" for subtitles).
//
// It returns a slice of stream metadata (as generic maps) or nil if none were found.
// Errors are logged but not returned, to keep the original function signature.
func getTrackInfos(path, streamType string) []interface{} {
	path = filepath.ToSlash(path)

	args := []string{
		"-v", "error",
		"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language",
		"-select_streams", streamType,
		"-of", "json",
		path,
	}

	cmd := exec.Command("ffprobe", args...)
	cmd.Dir = filepath.Dir(path)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("ffprobe getTrackInfos failed", "path", path, "streamType", streamType, "err", err, "stderr", string(output))
		return nil
	}

	var infos map[string]interface{}
	if err := json.Unmarshal(output, &infos); err != nil {
		logger.Error("ffprobe getTrackInfos: invalid JSON", "path", path, "err", err, "raw", string(output))
		return nil
	}

	streams, ok := infos["streams"].([]interface{})
	if !ok {
		logger.Warn("ffprobe getTrackInfos: no streams found", "path", path, "streamType", streamType)
		return nil
	}

	return streams
}

// generateVideoPreview creates preview.gif and preview.mp4 next to the source,
// under "<dir>/.hidden/<name>/".
//   - GIF: sampled window starting at ~10% into the video, duration = `duration` seconds,
//     palettegen/paletteuse pipeline for quality.
//   - MP4: short, silent H.264 clip using either NVENC or libx264.
//
// It will skip work if outputs already exist unless `force` is true.
func (s *server) generateVideoPreview(path string, fps, scale, duration int, force bool) error {
	path = s.formatPath(filepath.ToSlash(path))
	if !Utility.Exists(path) {
		return fmt.Errorf("generateVideoPreview: no file found at path %q", path)
	}

	// Limit concurrent ffmpeg
	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return errors.New("generateVideoPreview: maximum ffmpeg instances reached; try again later")
	}

	// Skip temp/hidden inputs
	if strings.Contains(path, ".hidden") || strings.Contains(path, ".temp") {
		logger.Info("generateVideoPreview: skipping hidden/temp path", "path", path)
		return nil
	}

	totalSec := getVideoDuration(path)
	if totalSec == 0 {
		return fmt.Errorf("generateVideoPreview: video length is 0 sec for %q", path)
	}

	// If the path is a directory containing HLS playlist, point to the .m3u8
	if Utility.Exists(path+"/playlist.m3u8") && !strings.HasSuffix(path, "playlist.m3u8") {
		path = filepath.ToSlash(filepath.Join(path, "playlist.m3u8"))
	}

	// Must have an extension or be a .m3u8
	if !strings.Contains(path, ".") {
		return fmt.Errorf("generateVideoPreview: %q has no file extension", path)
	}

	// Derive output folder: <dir>/.hidden/<basename>
	dir := path[:strings.LastIndex(path, "/")]
	name := ""
	if strings.HasSuffix(path, "playlist.m3u8") {
		// HLS: name is the parent folder name
		name = dir[strings.LastIndex(dir, "/")+1:]
		dir = dir[:strings.LastIndex(dir, "/")]
	} else {
		name = path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]
	}
	outDir := filepath.ToSlash(filepath.Join(dir, ".hidden", name))

	gifOut := filepath.ToSlash(filepath.Join(outDir, "preview.gif"))
	mp4Out := filepath.ToSlash(filepath.Join(outDir, "preview.mp4"))

	// Fast exit if already present and not forcing re-gen
	if Utility.Exists(gifOut) && Utility.Exists(mp4Out) && !force {
		logger.Info("generateVideoPreview: previews already exist; skipping", "path", path, "out", outDir)
		return nil
	}

	if err := Utility.CreateDirIfNotExist(outDir); err != nil {
		logger.Error("generateVideoPreview: mkdir failed", "dir", outDir, "err", err)
		return fmt.Errorf("generateVideoPreview: cannot create output dir %q: %w", outDir, err)
	}

	start := totalSec / 10
	if start < 0 {
		start = 0
	}
	if duration <= 0 {
		// choose a sane default if caller passes 0/negative
		duration = 10
	}
	if fps <= 0 {
		fps = 10
	}
	if scale <= 0 {
		scale = 320
	}

	// --- GIF ---
	if !Utility.Exists(gifOut) || force {
		if force && Utility.Exists(gifOut) {
			_ = os.Remove(gifOut)
		}

		gifArgs := []string{
			"-ss", strconv.Itoa(start),
			"-t", strconv.Itoa(duration),
			"-i", path,
			"-vf",
			fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=32[p];[s1][p]paletteuse=dither=bayer", fps, scale),
			"-loop", "0",
			"preview.gif",
		}
		logger.Info("ffmpeg: generate GIF preview", "src", path, "out", gifOut, "fps", fps, "scale", scale, "t", duration)
		wait := make(chan error)
		go Utility.RunCmd("ffmpeg", outDir, gifArgs, wait)
		if err := <-wait; err != nil {
			_ = os.Remove(gifOut)
			logger.Error("ffmpeg: GIF preview failed", "src", path, "out", gifOut, "err", err)
			return fmt.Errorf("generateVideoPreview: GIF generation failed for %q: %w", path, err)
		}
	}

	// --- MP4 ---
	if !Utility.Exists(mp4Out) || force {
		if force && Utility.Exists(mp4Out) {
			_ = os.Remove(mp4Out)
		}

		venc := "libx264"
		if s.hasEnableCudaNvcc() {
			venc = "h264_nvenc"
		}

		// Sample a sparse set of frames (~10 fps logical cadence via select)
		mp4Args := []string{
			"-y",
			"-i", path,
			"-ss", strconv.Itoa(start),
			"-t", strconv.Itoa(duration),
			"-filter_complex", fmt.Sprintf("[0:v]select='lt(mod(t,1/10),1)',setpts=N/(FRAME_RATE*TB),scale=%d:-2", scale),
			"-an",
			"-vcodec", venc,
			"preview.mp4",
		}

		logger.Info("ffmpeg: generate MP4 preview", "src", path, "out", mp4Out, "venc", venc, "scale", scale, "t", duration)
		wait := make(chan error)
		go Utility.RunCmd("ffmpeg", outDir, mp4Args, wait)
		if err := <-wait; err != nil {
			_ = os.Remove(mp4Out)
			logger.Warn("ffmpeg: MP4 preview failed; retrying with libx264 if applicable", "src", path, "err", err)

			// Retry with libx264 if NVENC failed
			if s.hasEnableCudaNvcc() {
				mp4ArgsRetry := append([]string(nil), mp4Args...)
				for i := range mp4ArgsRetry {
					if i > 0 && mp4ArgsRetry[i-1] == "-vcodec" {
						mp4ArgsRetry[i] = "libx264"
						break
					}
				}
				wait2 := make(chan error)
				go Utility.RunCmd("ffmpeg", outDir, mp4ArgsRetry, wait2)
				if err2 := <-wait2; err2 != nil {
					logger.Error("ffmpeg: MP4 preview retry failed", "src", path, "err", err2)
					return fmt.Errorf("generateVideoPreview: MP4 generation failed for %q: %w", path, err2)
				}
			} else {
				return fmt.Errorf("generateVideoPreview: MP4 generation failed for %q: %w", path, err)
			}
		}
	}

	return nil
}

// createVideoTimeLine extracts periodic thumbnails to build a timeline strip and
// then generates a WEBVTT (thumbnails.vtt) that indexes those images.
//
// Parameters:
//   - path: file path to a video file or an HLS directory (ending with playlist.m3u8 or its parent dir)
//   - width: output thumbnail height in pixels (video is scaled preserving AR), default 180 if 0
//   - fps: frames per second for timeline sampling (0 -> default 0.2, i.e., 1 frame per 5s)
//   - force: if true, regenerates timeline even if it already exists
//
// Returns an error if the input is invalid or the ffmpeg step fails.
func (s *server) createVideoTimeLine(path string, width int, fps float32, force bool) error {
	orig := path
	path = s.formatPath(path)
	if !Utility.Exists(path) {
		return fmt.Errorf("createVideoTimeLine: file not found: %q", path)
	}

	// Limit concurrent ffmpeg instances.
	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return errors.New("createVideoTimeLine: maximum concurrent ffmpeg instances reached; try again later")
	}

	// Defaults.
	if fps <= 0 {
		fps = 0.2 // 1 frame every 5 seconds
	}
	if width <= 0 {
		width = 180
	}

	// Support HLS dir paths (…/video/playlist.m3u8 or its parent directory).
	if Utility.Exists(filepath.ToSlash(path)+"/playlist.m3u8") && !strings.HasSuffix(path, "playlist.m3u8") {
		path = filepath.ToSlash(path) + "/playlist.m3u8"
	}

	if !strings.Contains(path, ".") {
		return fmt.Errorf("createVideoTimeLine: missing file extension for %q", path)
	}

	baseDir := path[:strings.LastIndex(path, "/")]
	name := ""
	if strings.HasSuffix(path, "playlist.m3u8") {
		name = baseDir[strings.LastIndex(baseDir, "/")+1:]
		baseDir = baseDir[:strings.LastIndex(baseDir, "/")]
	} else {
		name = path[strings.LastIndex(path, "/")+1 : strings.LastIndex(path, ".")]
	}

	output := filepath.ToSlash(filepath.Join(baseDir, ".hidden", name, "__timeline__"))

	
	// If it already exists, either reuse (create VTT only) or rebuild.
	if Utility.Exists(output) {
		if !force {
			logger.Info("timeline already exists; generating VTT only", "video", orig, "dir", output, "fps", fps)
			return createVttFile(output, fps)
		}
		if err := os.RemoveAll(output); err != nil {
			return fmt.Errorf("createVideoTimeLine: remove existing timeline %q: %w", output, err)
		}
	}

	if err := Utility.CreateDirIfNotExist(output); err != nil {
		return fmt.Errorf("createVideoTimeLine: create dir %q: %w", output, err)
	}

	// Ensure video is readable and non-zero length.
	durationSec := getVideoDuration(path)
	if durationSec <= 0 {
		return fmt.Errorf("createVideoTimeLine: zero-length or unreadable video: %q", path)
	}

	// Extract thumbnails with ffmpeg:
	//   - entire duration
	//   - scaled to -1:height (keep AR)
	//   - fps as requested
	wait := make(chan error)
	args := []string{
		"-y",
		"-i", path,
		"-ss", "0",
		"-t", Utility.ToString(durationSec),
		"-vf", "scale=-1:" + Utility.ToString(width) + ",fps=" + Utility.ToString(fps),
		"thumbnail_%05d.jpg",
	}
	logger.Info("ffmpeg: timeline extraction",
		"video", path,
		"out", output,
		"height", width,
		"fps", fps,
		"duration_sec", durationSec)

	go Utility.RunCmd("ffmpeg", output, args, wait)
	if err := <-wait; err != nil {
		logger.Error("ffmpeg timeline extraction failed", "video", path, "out", output, "err", err)
		return fmt.Errorf("createVideoTimeLine: ffmpeg extraction failed for %q: %w", path, err)
	}

	// Build WEBVTT index for the generated thumbnails.
	if err := createVttFile(output, fps); err != nil {
		return fmt.Errorf("createVideoTimeLine: VTT generation failed for %q: %w", output, err)
	}

	logger.Info("timeline created",
		"video", orig,
		"dir", output,
		"fps", fps,
		"height", width)
	return nil
}

// createVideoPreview generates still-image previews for a video into
// "<parent>/.hidden/<basename>/__preview__".
//
// Notes:
//   - If `path` points to an HLS folder, we target "<path>/playlist.m3u8".
//   - Skips work for items inside ".hidden" or ".temp".
//   - Waits (up to ~5 minutes) for a readable duration before extracting frames.
//   - `height` is used as the width in the ffmpeg scale filter, preserving
//     the previous behavior: scale=<height>:-1.
//
// The `nb` parameter is currently unused (kept for API compatibility).
func (s *server) createVideoPreview(path string, nb, height int, force bool) error {
	p := s.formatPath(path)
	if !Utility.Exists(p) {
		return fmt.Errorf("no file found at path %s", path)
	}

	// Skip hidden and temp content.
	if strings.Contains(p, ".hidden") || strings.Contains(p, ".temp") {
		return nil
	}

	// Limit concurrent ffmpeg invocations.
	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return errors.New("number of ffmpeg instances has been reached; try later")
	}

	// If path is a directory containing an HLS playlist, point to playlist.m3u8.
	if Utility.Exists(p+"/playlist.m3u8") && !strings.HasSuffix(p, "playlist.m3u8") {
		p += "/playlist.m3u8"
	}

	// Basic extension sanity check.
	if !strings.Contains(p, ".") {
		return fmt.Errorf("%s does not have an extension", p)
	}

	// Derive parent dir and "base name" used for the preview folder.
	parent := p[:strings.LastIndex(p, "/")]
	base := ""
	if strings.HasSuffix(p, "playlist.m3u8") {
		// For HLS: base is the folder name that contains playlist.m3u8, parent is its parent.
		base = parent[strings.LastIndex(parent, "/")+1:]
		parent = parent[:strings.LastIndex(parent, "/")]
	} else {
		// For regular files.
		base = p[strings.LastIndex(p, "/")+1 : strings.LastIndex(p, ".")]
	}

	outDir := parent + "/.hidden/" + base + "/__preview__"

	// Handle existing output directory.
	if Utility.Exists(outDir) {
		if !force {
			return nil
		}
		_ = os.RemoveAll(outDir) // ensure a clean slate
	}

	// Remove related cache entries.
	cache.RemoveItem(p)
	cache.RemoveItem(outDir)

	// Probe duration; wait up to 5 minutes if needed.
	const maxWaitSec = 300
	dur := getVideoDuration(p)
	for tries := 0; dur == 0 && tries < maxWaitSec; tries++ {
		time.Sleep(1 * time.Second)
		dur = getVideoDuration(p)
	}
	if dur == 0 {
		slog.Warn("createVideoPreview: video duration is zero", "path", p)
		return errors.New("the video length is 0 sec")
	}

	// Extract a short window of frames starting at 10% of the video.
	start := dur / 10
	span := 120 // seconds

	// Windows sometimes fails on first mkdir; keep the resilient loop.
	var runErr error
	for tries := 0; tries < maxWaitSec; tries++ {
		Utility.CreateDirIfNotExist(outDir)

		wait := make(chan error, 1)
		args := []string{
			"-i", p,
			"-ss", Utility.ToString(start),
			"-t", Utility.ToString(span),
			"-vf", "scale=" + Utility.ToString(height) + ":-1,fps=.250",
			"preview_%05d.jpg",
		}
		go Utility.RunCmd("ffmpeg", outDir, args, wait)

		if err := <-wait; err == nil {
			runErr = nil
			break
		} else {
			runErr = err
			time.Sleep(1 * time.Second)
		}
	}
	if runErr != nil {
		return runErr
	}

	// Notify clients to refresh the directory view.
	if client, err := getEventClient(); err == nil {
		dir := filepath.Dir(p)
		dir = strings.ReplaceAll(dir, "\\", "/")
		client.Publish("reload_dir_event", []byte(dir))
	}

	return nil
}

func ensureAACDefault(video string) error {
	streamInfos, err := getStreamInfos(video)
	if err != nil {
		return err
	}
	var audioEncoding string
	aacIndex := -1
	audioCount := 0
	for _, s := range streamInfos["streams"].([]any) {
		sm := s.(map[string]any)
		if sm["codec_type"].(string) == "audio" {
			audioCount++
			codec := sm["codec_name"].(string)
			if codec == "aac" && aacIndex == -1 {
				aacIndex = audioCount - 1
			}
			audioEncoding = codec
		}
	}
	if audioEncoding == "aac" && (audioCount <= 1 || aacIndex == -1) {
		return nil
	}
	output := strings.ReplaceAll(video, ".mp4", ".temp.mp4")
	defer os.Remove(output)
	args := []string{"-i", video, "-map", "0", "-c:v", "copy", "-c:s", "mov_text"}
	if audioEncoding != "aac" {
		args = append(args, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
	} else {
		args = append(args, "-c:a", "copy")
		for i := 0; i < audioCount; i++ {
			if i == aacIndex {
				args = append(args, fmt.Sprintf("-disposition:a:%d", i), "default")
			} else {
				args = append(args, fmt.Sprintf("-disposition:a:%d", i), "none")
			}
		}
	}
	args = append(args, output)
	wait := make(chan error)
	go Utility.RunCmd("ffmpeg", filepath.Dir(video), args, wait)
	if err := <-wait; err != nil {
		return err
	}
	if err := os.Remove(video); err != nil {
		return err
	}
	return os.Rename(output, video)
}

// createVideoMpeg4H264 converts any input to MP4/H.264, mapping audio/subtitle tracks.
// (Public method signature preserved.)
func (srv *server) createVideoMpeg4H264(path string) (string, error) {
	cache.RemoveItem(path)
	_ = extractSubtitleTracks(path)

	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return "", errors.New("maximum concurrent ffmpeg processes reached; try again later")
	}
	if !strings.Contains(path, ".") {
		return "", fmt.Errorf("%s: missing file extension", path)
	}

	path = filepath.ToSlash(path)
	dir := path[:strings.LastIndex(path, "/")]
	name := path[strings.LastIndex(path, "/"):strings.LastIndex(path, ".")]
	out := dir + "/" + name + ".mp4"

	if !strings.HasSuffix(strings.ToLower(path), ".mp4") {
		if Utility.Exists(out) {
			_ = os.Remove(out)
		}
	} else {
		hevc := dir + "/" + name + ".hevc"
		if Utility.Exists(hevc) {
			return "", fmt.Errorf("conversion already in progress: %s", out)
		}
		_ = Utility.MoveFile(out, hevc)
		path = hevc
	}

	streams, err := getStreamInfos(path)
	if err != nil {
		return "", err
	}

	videoCodec := ""
	for _, s := range streams["streams"].([]any) {
		sm := s.(map[string]any)
		if sm["codec_type"] == "video" {
			videoCodec = sm["codec_long_name"].(string)
			break
		}
	}

	args := []string{"-i", path, "-c:v"}
	if srv.hasEnableCudaNvcc() {
		if strings.HasPrefix(videoCodec, "H.264") || strings.HasPrefix(videoCodec, "MPEG-4 part 2") {
			args = append(args, "h264_nvenc")
		} else if strings.HasPrefix(videoCodec, "H.265") || strings.HasPrefix(videoCodec, "Motion JPEG") {
			args = append(args, "h264_nvenc", "-pix_fmt", "yuv420p")
		} else {
			return "", fmt.Errorf("no NVENC profile for codec: %s", videoCodec)
		}
	} else {
		if strings.HasPrefix(videoCodec, "H.264") || strings.HasPrefix(videoCodec, "MPEG-4 part 2") {
			args = append(args, "libx264")
		} else if strings.HasPrefix(videoCodec, "H.265") || strings.HasPrefix(videoCodec, "Motion JPEG") {
			args = append(args, "libx264", "-pix_fmt", "yuv420p")
		} else {
			return "", fmt.Errorf("no software encoder for codec: %s", videoCodec)
		}
	}

	// map primary video stream only (ignore cover art or extra video matter)
	args = append(args, "-map", "0:v:0")
	audioCount := countStreamsByType(streams, "audio")
	if audioCount == 0 {
		audioCount = 1
	}
	for i := 0; i < audioCount; i++ {
		args = append(args, "-map", fmt.Sprintf("0:a:%d?", i), "-c:a:"+fmt.Sprint(i), "aac")
	}

	// map compatible subtitles
	subIdx := 0
	streamIdx := 0
	for _, s := range streams["streams"].([]any) {
		sm := s.(map[string]any)
		if sm["codec_type"].(string) == "subtitle" {
			codec := sm["codec_name"].(string)
			if codec == "subrip" || codec == "ass" || codec == "ssa" {
				args = append(args,
					"-map", fmt.Sprintf("0:s:%d?", streamIdx),
					"-c:s:"+fmt.Sprint(subIdx), "mov_text",
				)
				subIdx++
			}
		}
		streamIdx++
	}
	args = append(args, out)

	wait := make(chan error)
	go Utility.RunCmd("ffmpeg", filepath.Dir(path), args, wait)
	if err := <-wait; err != nil {
		return "", err
	}
	_ = os.Remove(path)
	return out, nil
}

func countStreamsByType(streams map[string]any, kind string) int {
	if streams == nil {
		return 0
	}
	cnt := 0
	if sList, ok := streams["streams"].([]any); ok {
		for _, entry := range sList {
			if sm, _ := entry.(map[string]any); ok {
				if codecType, _ := sm["codec_type"].(string); codecType == kind {
					cnt++
				}
			}
		}
	}
	return cnt
}

func (srv *server) hasEnableCudaNvcc() bool {

	// Here I will check if the server has enable cuda...
	if !srv.HasEnableGPU {
		return false
	}

	getVersion := exec.Command("ffmpeg", "-encoders")
	getVersion.Dir = os.TempDir()
	encoders, _ := getVersion.CombinedOutput()

	return strings.Contains(string(encoders), "hevc_nvenc")
}

// createHlsStream builds VOD HLS renditions and a master playlist from an input video.
//
// segment_target_duration: target segment length in seconds (EXT-X-TARGETDURATION)
// max_bitrate_ratio:       peak bitrate multiplier for -maxrate (e.g., 1.07)
// rate_monitor_buffer_ratio: buffer size multiplier for -bufsize (e.g., 1.5)
func (srv *server) createHlsStream(src, dest string, segment_target_duration int, max_bitrate_ratio, rate_monitor_buffer_ratio float32) error {
	// Throttle concurrent ffmpeg.
	if pids, _ := Utility.GetProcessIdsByName("ffmpeg"); len(pids) > MAX_FFMPEG_INSTANCE {
		return errors.New("too many ffmpeg instances; please try again later")
	}

	src = filepath.ToSlash(src)
	dest = filepath.ToSlash(dest)

	// Ensure destination exists.
	if err := Utility.CreateDirIfNotExist(dest); err != nil {
		logger.Error("createHlsStream: ensure dest dir failed", "dest", dest, "err", err)
		return err
	}

	// Probe stream info.
	streamInfos, err := getStreamInfos(src)
	if err != nil {
		logger.Error("createHlsStream: getStreamInfos failed", "src", src, "err", err)
		return err
	}

	keyint, err := getStreamFrameRateInterval(src)
	if err != nil || keyint <= 0 {
		if err != nil {
			logger.Warn("createHlsStream: FPS probe failed, falling back", "src", src, "err", err)
		}
		keyint = 25
	}

	// Resolve source video "encoding" long name for encoder choice.
	encoding := ""
	if streams, ok := streamInfos["streams"].([]interface{}); ok {
		for _, s := range streams {
			sm, _ := s.(map[string]interface{})
			if sm == nil {
				continue
			}
			if ct, _ := sm["codec_type"].(string); ct != "video" {
				continue
			}
			if afr, _ := sm["avg_frame_rate"].(string); afr == "0/0" {
				continue
			}
			if cn, _ := sm["codec_name"].(string); cn == "png" {
				continue
			}
			if cl, _ := sm["codec_long_name"].(string); cl != "" {
				encoding = cl
				break
			}
		}
	}
	if encoding == "" {
		return errors.New("no usable video stream found")
	}

	// Choose encoder (GPU if available).
	args := []string{"-hide_banner", "-y", "-i", src, "-c:v"}
	switch {
	case srv.hasEnableCudaNvcc():
		// NVENC path
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			args = append(args, "h264_nvenc")
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			args = append(args, "h264_nvenc", "-pix_fmt", "yuv420p")
		} else {
			return fmt.Errorf("no GPU encoder mapping for source codec %q", encoding)
		}
	default:
		// CPU path
		if strings.HasPrefix(encoding, "H.264") || strings.HasPrefix(encoding, "MPEG-4 part 2") {
			args = append(args, "libx264")
		} else if strings.HasPrefix(encoding, "H.265") || strings.HasPrefix(encoding, "Motion JPEG") {
			args = append(args, "libx264", "-pix_fmt", "yuv420p")
		} else {
			return fmt.Errorf("no CPU encoder mapping for source codec %q", encoding)
		}
	}

	// Decide rendition ladder from input width.
	w, _ := getVideoResolution(src) // if probe fails, w == -1; ladder will be minimal
	renditions := make([]map[string]string, 0, 6)
	if w >= 426 {
		renditions = append(renditions, map[string]string{"res": "426x240", "vbit": "1400k", "abit": "128k"})
	}
	if w >= 640 {
		renditions = append(renditions, map[string]string{"res": "640x360", "vbit": "1400k", "abit": "128k"})
	}
	if w >= 842 {
		renditions = append(renditions, map[string]string{"res": "842x480", "vbit": "1400k", "abit": "128k"})
	}
	if w >= 1280 {
		renditions = append(renditions, map[string]string{"res": "1280x720", "vbit": "2800k", "abit": "128k"})
	}
	if w >= 1920 {
		renditions = append(renditions, map[string]string{"res": "1920x1080", "vbit": "5000k", "abit": "192k"})
	}
	if w >= 3840 {
		renditions = append(renditions, map[string]string{"res": "3840x2160", "vbit": "5000k", "abit": "192k"})
	}
	if len(renditions) == 0 {
		// Fall back to a single low rung to avoid empty output.
		renditions = append(renditions, map[string]string{"res": "426x240", "vbit": "800k", "abit": "96k"})
	}

	// Common params applied per rendition/output.
	staticParams := []string{
		"-profile:v", "main",
		"-sc_threshold", "0",
		"-g", Utility.ToString(keyint),
		"-keyint_min", Utility.ToString(keyint),
		"-hls_time", Utility.ToString(segment_target_duration),
		"-hls_playlist_type", "vod",
	}

	// Build master playlist and output args (one output per rung).
	var master strings.Builder
	master.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")

	for _, r := range renditions {
		res := r["res"]
		vbit := r["vbit"]
		abit := r["abit"]

		parts := strings.Split(res, "x")
		if len(parts) != 2 {
			logger.Warn("createHlsStream: invalid resolution rung, skipping", "res", res)
			continue
		}
		width := parts[0]
		height := parts[1]
		name := height + "p"

		// Compute maxrate/bufsize from vbit (strip trailing 'k').
		numStr := strings.TrimSuffix(vbit, "k")
		vk := Utility.ToInt(numStr) // kbps
		maxrate := int(float32(vk) * max_bitrate_ratio)
		bufsize := int(float32(vk) * rate_monitor_buffer_ratio)

		// Per-output video filter: even dimensions, maintain AR.
		scale := fmt.Sprintf("scale=-2:min(%s\\,if(mod(ih\\,2)\\,ih-1\\,ih))", width)

		// Append params for this output.
		args = append(args,
			staticParams...,
		)
		args = append(args,
			"-vf", scale,
			// Map video once and up to 8 audio streams, encode audio to AAC.
			"-map", "0:v",
			"-map", "0:a:0?", "-c:a:0", "aac",
			"-map", "0:a:1?", "-c:a:1", "aac",
			"-map", "0:a:2?", "-c:a:2", "aac",
			"-map", "0:a:3?", "-c:a:3", "aac",
			"-map", "0:a:4?", "-c:a:4", "aac",
			"-map", "0:a:5?", "-c:a:5", "aac",
			"-map", "0:a:6?", "-c:a:6", "aac",
			"-map", "0:a:7?", "-c:a:7", "aac",
			// Text subtitles to mov_text if present (up to 8).
			"-map", "0:s:0?", "-c:s:0", "mov_text",
			"-map", "0:s:1?", "-c:s:1", "mov_text",
			"-map", "0:s:2?", "-c:s:2", "mov_text",
			"-map", "0:s:3?", "-c:s:3", "mov_text",
			"-map", "0:s:4?", "-c:s:4", "mov_text",
			"-map", "0:s:5?", "-c:s:5", "mov_text",
			"-map", "0:s:6?", "-c:s:6", "mov_text",
			"-map", "0:s:7?", "-c:s:7", "mov_text",
			// Bitrate control and output.
			"-b:v", vbit,
			"-maxrate", fmt.Sprintf("%dk", maxrate),
			"-bufsize", fmt.Sprintf("%dk", bufsize),
			"-b:a", abit,
			"-hls_segment_filename", filepath.ToSlash(filepath.Join(dest, name+"_%%04d.ts")),
			filepath.ToSlash(filepath.Join(dest, name+".m3u8")),
		)

		// Master manifest entry (BANDWIDTH expects bits per second).
		master.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\n%s\n", vk*1000, res, name+".m3u8"))
	}

	logger.Info("ffmpeg: create HLS",
		"src", src, "dest", dest,
		"keyint", keyint,
		"segment", segment_target_duration,
		"rungs", len(renditions),
	)

	// Run ffmpeg once to produce all variant playlists and segments.
	wait := make(chan error)
	go Utility.RunCmd("ffmpeg", filepath.Dir(src), args, wait)
	if runErr := <-wait; runErr != nil {
		logger.Error("createHlsStream: ffmpeg failed", "src", src, "dest", dest, "err", runErr)
		return runErr
	}

	// Write the master playlist.
	if err := os.WriteFile(filepath.Join(dest, "playlist.m3u8"), []byte(master.String()), 0644); err != nil {
		logger.Error("createHlsStream: write master playlist failed", "dest", dest, "err", err)
		return err
	}
	return nil
}

// createHlsStreamFromMpeg4H264 converts an MP4/H.264 file into a VOD HLS folder beside it.
// On success it writes <basename>/playlist.m3u8 and removes the original file.
//
// Notes:
//   - Uses a temp workdir and moves the finished folder into place atomically.
//   - Keeps the public signature unchanged.
//   - Structured logging replaces fmt prints.
func (srv *server) createHlsStreamFromMpeg4H264(path string) error {
	// Evict any cached entry for the input file.
	cache.RemoveItem(path)

	// Throttle concurrent ffmpeg.
	if pids, _ := Utility.GetProcessIdsByName("ffmpeg"); len(pids) > MAX_FFMPEG_INSTANCE {
		return errors.New("too many ffmpeg instances; please try again later")
	}

	p := filepath.ToSlash(path)
	if !strings.Contains(p, ".") {
		return fmt.Errorf("input %q has no extension", p)
	}

	ext := p[strings.LastIndex(p, ".")+1:]
	outputBase := p[:strings.LastIndex(p, ".")] // destination folder path (no extension)

	// Remove any stale output directory from previous runs.
	if err := os.RemoveAll(outputBase); err != nil {
		logger.Warn("createHlsStreamFromMpeg4H264: cleanup stale output failed", "output", outputBase, "err", err)
	}

	// Prepare temp workspace.
	tmpID := Utility.GenerateUUID(p[strings.LastIndex(p, "/")+1:])
	tmpFile := filepath.ToSlash(filepath.Join(os.TempDir(), tmpID+"."+ext))
	tmpOut := filepath.ToSlash(filepath.Join(os.TempDir(), tmpID))

	// Ensure clean temp targets.
	_ = os.Remove(tmpOut)
	if err := Utility.CreateDirIfNotExist(tmpOut); err != nil {
		logger.Error("createHlsStreamFromMpeg4H264: create temp output dir failed", "dir", tmpOut, "err", err)
		return err
	}

	// Best-effort cleanup for temp artifacts.
	defer func() {
		if rmErr := os.Remove(tmpFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.Warn("createHlsStreamFromMpeg4H264: temp file cleanup failed", "file", tmpFile, "err", rmErr)
		}
		if rmErr := os.Remove(tmpOut); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.Warn("createHlsStreamFromMpeg4H264: temp dir cleanup failed", "dir", tmpOut, "err", rmErr)
		}
	}()

	// Copy the source into temp.
	if err := Utility.CopyFile(p, tmpFile); err != nil {
		logger.Error("createHlsStreamFromMpeg4H264: copy input to temp failed", "src", p, "dst", tmpFile, "err", err)
		return err
	}

	// Build the HLS ladder in temp dir.
	if err := srv.createHlsStream(tmpFile, tmpOut, 4, 1.07, 1.5); err != nil {
		logger.Error("createHlsStreamFromMpeg4H264: HLS creation failed", "src", p, "tmpOut", tmpOut, "err", err)
		return err
	}

	// Move temp output into final location:
	//   1) rename tmpOut -> temp sibling named like final folder
	//   2) move that folder to the final parent dir
	tmpSibling := filepath.ToSlash(filepath.Join(os.TempDir(), outputBase[strings.LastIndex(outputBase, "/")+1:]))
	if err := os.Rename(tmpOut, tmpSibling); err != nil {
		logger.Error("createHlsStreamFromMpeg4H264: rename temp output failed", "from", tmpOut, "to", tmpSibling, "err", err)
		return err
	}
	if err := Utility.Move(tmpSibling, filepath.ToSlash(filepath.Dir(outputBase))); err != nil {
		logger.Error("createHlsStreamFromMpeg4H264: move output folder into place failed", "from", tmpSibling, "toDir", filepath.Dir(outputBase), "err", err)
		return err
	}

	// Success condition: master playlist exists.
	master := filepath.ToSlash(filepath.Join(outputBase, "playlist.m3u8"))
	if Utility.Exists(master) {
		// Reassociate index entries from file.mp4 -> file (folder).
		rel := strings.ReplaceAll(p, config.GetDataDir()+"/files", "")
		newRel := rel[:strings.LastIndex(rel, ".")] // drop extension
		reassociatePath(rel, newRel, srv.Domain)

		// Remove original file to keep only the stream folder.
		if err := os.Remove(p); err != nil {
			logger.Warn("createHlsStreamFromMpeg4H264: remove source failed (continuing)", "src", p, "err", err)
		}

		logger.Info("createHlsStreamFromMpeg4H264: HLS created",
			"src", p, "dest", outputBase, "master", master)
		return nil
	}

	err := fmt.Errorf("expected master playlist not found at %q", master)
	logger.Error("createHlsStreamFromMpeg4H264: missing master playlist", "dest", outputBase, "err", err)
	return err
}

// extractSubtitleTracks dumps all text-based subtitle streams to individual .vtt files
// beside the input, under: <dir>/.hidden/<basename>/__subtitles__/.
// If there are 0 subtitle tracks it returns an error, if there is exactly 1 it
// returns nil (nothing to split), matching the original behavior.
func extractSubtitleTracks(videoPath string) error {
	videoPath = filepath.ToSlash(videoPath)

	// Probe subtitle streams ("s").
	tracks := getTrackInfos(videoPath, "s")
	if len(tracks) == 0 {
		return fmt.Errorf("no subtitle track found for %q", videoPath)
	}
	if len(tracks) == 1 {
		// Only one language/track -> nothing to split.
		return nil
	}

	// Derive destination: <dir>/.hidden/<basename>/__subtitles__
	dir := videoPath[:strings.LastIndex(videoPath, "/")]
	base := filepath.Base(videoPath)
	name := base
	if dot := strings.LastIndex(base, "."); dot > 0 {
		name = base[:dot]
	}
	dest := filepath.ToSlash(filepath.Join(dir, ".hidden", name, "__subtitles__"))

	// If already extracted, don't redo work.
	if Utility.Exists(dest) {
		return fmt.Errorf("subtitle tracks for %q already exist at %s", base, dest)
	}
	if err := Utility.CreateDirIfNotExist(dest); err != nil {
		logger.Error("extractSubtitleTracks: mkdir failed", "dest", dest, "err", err)
		return err
	}

	// Supported text-based subtitle codecs we can convert to WebVTT.
	supported := map[string]struct{}{
		"ass": {}, "ssa": {}, "dvbsub": {}, "dvdsub": {}, "jacosub": {}, "microdvd": {},
		"mpl2": {}, "pjs": {}, "realtext": {}, "sami": {}, "webvtt": {}, "vplayer": {},
		"subviewer1": {}, "text": {}, "subrip": {}, "srt": {}, "stl": {}, "mov_text": {},
	}

	// Build ffmpeg extraction args.
	args := []string{"-y", "-i", videoPath}
	mapped := 0

	for _, t := range tracks {
		m, ok := t.(map[string]interface{})
		if !ok {
			continue
		}

		// codec_name (string)
		codec, _ := m["codec_name"].(string)
		if _, ok := supported[strings.ToLower(codec)]; !ok {
			continue
		}

		// index (number)
		var idx int
		switch v := m["index"].(type) {
		case float64:
			idx = int(v)
		case int:
			idx = v
		default:
			idx = Utility.ToInt(m["index"])
		}

		// language tag (optional)
		lang := "und"
		if tagsAny, ok := m["tags"].(map[string]interface{}); ok {
			if l, ok := tagsAny["language"].(string); ok && strings.TrimSpace(l) != "" {
				lang = l
			}
		}

		outName := name + "." + lang + ".vtt"
		args = append(args, "-map", "0:"+Utility.ToString(idx), outName)
		mapped++
	}

	if mapped == 0 {
		// Nothing supported to extract.
		logger.Warn("extractSubtitleTracks: no supported text-based subtitles", "path", videoPath)
		return nil
	}

	logger.Info("ffmpeg: extract subtitles", "src", videoPath, "dest", dest, "streams", mapped)

	// Run ffmpeg in destination directory so output files land there.
	wait := make(chan error)
	go Utility.RunCmd("ffmpeg", dest, args, wait)
	if err := <-wait; err != nil {
		logger.Error("ffmpeg: subtitle extraction failed", "src", videoPath, "dest", dest, "err", err)
		return fmt.Errorf("subtitle extraction failed for %q: %w", videoPath, err)
	}

	return nil
}
