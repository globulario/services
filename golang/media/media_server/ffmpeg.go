package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

func probeVideoDuration(local string) (int, error) {
	local = strings.ReplaceAll(local, "\\", "/")
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "compact=print_section=0:nokey=1:escape=csv",
		"-show_entries", "format=duration",
		local,
	)
	cmd.Dir = filepath.Dir(local)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logger.Error("ffprobe duration failed", "path", local, "stderr", strings.TrimSpace(stderr.String()), "err", err)
		return 0, err
	}

	dur, err := strconv.ParseFloat(strings.TrimSpace(out.String()), 64)
	if err != nil {
		logger.Error("ffprobe duration parse failed", "path", local, "raw", strings.TrimSpace(out.String()), "err", err)
		return 0, err
	}
	return Utility.ToInt(dur + 0.5), nil
}

// getVideoDuration returns the duration of the given media file in seconds, rounded.
func (srv *server) getVideoDuration(path string) int {
	duration := 0
	_ = srv.withWorkFile(path, func(wf mediaWorkFile) error {
		d, err := probeVideoDuration(wf.LocalPath)
		if err != nil {
			return err
		}
		duration = d
		return nil
	})
	return duration
}

// ensureFastStartMP4 rewrites an MP4 so that its 'moov' atom is placed
// at the beginning of the file, which allows browsers to start playback
// immediately when streaming over HTTP (progressive download).
// It does a fast remux with `-c copy`, so it is cheap compared to a full re-encode.
// You can also call this on MP4s downloaded via yt-dlp (e.g. in uploadedVideo).
func (srv *server) ensureFastStartMP4(path string) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".mp4" {
		return nil
	}
	if strings.Contains(path, "/.hidden/") || strings.Contains(path, "__preview__") {
		return nil
	}

	checkedLocalFastStart := false
	if !srv.isMinioPath(path) {
		localPath := filepath.ToSlash(srv.formatPath(path))
		if srv.localPathExists(localPath) {
			if hasFastStart, err := hasFastStartMoov(localPath); err == nil {
				if hasFastStart {
					return nil
				}
				checkedLocalFastStart = true
			}
		}
	}

	return srv.withWorkFile(path, func(wf mediaWorkFile) error {
		localPath := strings.ReplaceAll(wf.LocalPath, "\\", "/")

		if !checkedLocalFastStart || wf.IsMinio {
			hasFastStart, err := hasFastStartMoov(localPath)
			if err != nil {
				logger.Debug("faststart check skipped (probe failed)", "path", localPath, "err", err)
				return nil
			}
			if hasFastStart {
				return nil
			}
		}

		dir := filepath.Dir(localPath)
		base := strings.TrimSuffix(filepath.Base(localPath), filepath.Ext(localPath))
		tmpName := base + ".faststart.mp4"
		tmpPath := filepath.Join(dir, tmpName)

		args := []string{
			"-y",
			"-i", filepath.Base(localPath),
			"-c", "copy",
			"-movflags", "+faststart",
			filepath.Base(tmpPath),
		}

		wait := make(chan error, 1)
		go Utility.RunCmd("ffmpeg", dir, args, wait)
		if err := <-wait; err != nil {
			return err
		}

		if err := rewriteInPlace(localPath, tmpPath); err == nil {
			_ = os.Remove(tmpPath)
		} else {
			logger.Warn("faststart rewrite fallback to rename", "path", localPath, "err", err)
			if err := os.Remove(localPath); err != nil {
				return err
			}
			if err := os.Rename(tmpPath, localPath); err != nil {
				return err
			}
		}

		if wf.IsMinio {
			ctx := context.Background()
			if err := srv.minioUploadFile(ctx, wf.LogicalPath, localPath, "video/mp4"); err != nil {
				return err
			}
		}
		return nil
	})
}

func hasFastStartMoov(path string) (bool, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "format_tags=major_brand",
		"-show_entries", "format_flags=faststart",
		"-of", "json",
		path,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("%w: %s", err, strings.TrimSpace(out.String()))
	}
	type probeResp struct {
		Format struct {
			Tags struct {
				MajorBrand string `json:"major_brand"`
			} `json:"tags"`
			FormatFlags string `json:"format_flags"`
		} `json:"format"`
	}
	var resp probeResp
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return false, err
	}
	flags := strings.ToLower(resp.Format.FormatFlags)
	if strings.Contains(flags, "faststart") {
		return true, nil
	}
	brand := strings.ToLower(resp.Format.Tags.MajorBrand)
	if strings.Contains(brand, "isom") || strings.Contains(brand, "mp42") {
		if strings.Contains(flags, "movflags=+faststart") {
			return true, nil
		}
	}
	return false, nil
}

// rewriteInPlace copies the content of src over dst without changing dst's inode, so
// filesystem watchers only see a modification instead of a delete + recreate.
func rewriteInPlace(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
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
func (srv *server) getStreamFrameRateInterval(path string) (int, error) {
	fps := -1
	err := srv.withWorkFile(path, func(wf mediaWorkFile) error {
		local := strings.ReplaceAll(wf.LocalPath, "\\", "/")
		cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v", "-of", "default=noprint_wrappers=1:nokey=1", "-show_entries", "stream=r_frame_rate", local)
		cmd.Dir = filepath.Dir(local)
		data, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ffprobe r_frame_rate failed: %w", err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), "/")
		if len(parts) != 2 {
			return fmt.Errorf("unexpected r_frame_rate: %q", strings.TrimSpace(string(data)))
		}
		fpsVal := Utility.ToNumeric(parts[0]) / Utility.ToNumeric(parts[1])
		fps = int(fpsVal + .5)
		return nil
	})
	return fps, err
}

// getTrackInfos runs ffprobe to extract stream info of a given type (e.g. "a" for audio, "s" for subtitles).
//
// It returns a slice of stream metadata (as generic maps) or nil if none were found.
// Errors are logged but not returned, to keep the original function signature.
func (srv *server) getTrackInfos(path, streamType string) []interface{} {
	var streams []interface{}
	_ = srv.withWorkFile(path, func(wf mediaWorkFile) error {
		local := filepath.ToSlash(wf.LocalPath)

		args := []string{
			"-v", "error",
			"-show_entries", "stream=index,codec_name,codec_type:stream_tags=language",
			"-select_streams", streamType,
			"-of", "json",
			local,
		}

		cmd := exec.Command("ffprobe", args...)
		cmd.Dir = filepath.Dir(local)

		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("ffprobe getTrackInfos failed", "path", local, "streamType", streamType, "err", err, "stderr", string(output))
			return nil
		}

		var infos map[string]interface{}
		if err := json.Unmarshal(output, &infos); err != nil {
			logger.Error("ffprobe getTrackInfos: invalid JSON", "path", local, "err", err, "raw", string(output))
			return nil
		}

		sVal, ok := infos["streams"].([]interface{})
		if !ok {
			logger.Warn("ffprobe getTrackInfos: no streams found", "path", local, "streamType", streamType)
			return nil
		}
		streams = sVal
		return nil
	})
	return streams
}

// generateVideoPreview creates preview.gif and preview.mp4 next to the source,
// under "<dir>/.hidden/<name>/".
//   - GIF: sampled window starting at ~10% into the video, duration = `duration` seconds,
//     palettegen/paletteuse pipeline for quality.
//   - MP4: short, silent H.264 clip using either NVENC or libx264.
//
// It will skip work if outputs already exist unless `force` is true.
func (srv *server) generateVideoPreview(path string, fps, scale, duration int, force bool) error {

	logicalPath := filepath.ToSlash(path)

	if strings.Contains(logicalPath, ".hidden") || strings.Contains(logicalPath, ".temp") {
		logger.Info("generateVideoPreview: skipping hidden/temp path", "path", logicalPath)
		return nil
	}

	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return errors.New("generateVideoPreview: maximum ffmpeg instances reached; try again later")
	}

	return srv.withWorkFile(logicalPath, func(wf mediaWorkFile) error {
		localInput := filepath.ToSlash(wf.LocalPath)
		if !srv.pathExists(localInput) {
			return fmt.Errorf("generateVideoPreview: no file found at path %q", logicalPath)
		}

		inputLogical := filepath.ToSlash(wf.LogicalPath)
		if !strings.HasSuffix(localInput, "playlist.m3u8") && !wf.IsMinio {
			playlistLocal := filepath.ToSlash(filepath.Join(localInput, "playlist.m3u8"))
			if srv.pathExists(playlistLocal) && !strings.HasSuffix(inputLogical, "playlist.m3u8") {
				localInput = playlistLocal
				inputLogical = filepath.ToSlash(filepath.Join(inputLogical, "playlist.m3u8"))
			}
		}

		if !strings.Contains(localInput, ".") {
			return fmt.Errorf("generateVideoPreview: %q has no file extension", inputLogical)
		}

		totalSec, err := probeVideoDuration(localInput)
		if err != nil || totalSec == 0 {
			return fmt.Errorf("generateVideoPreview: video length is 0 sec for %q", inputLogical)
		}

		dir := inputLogical[:strings.LastIndex(inputLogical, "/")]
		name := ""
		if strings.HasSuffix(inputLogical, "playlist.m3u8") {
			name = dir[strings.LastIndex(dir, "/")+1:]
			dir = dir[:strings.LastIndex(dir, "/")]
		} else {
			name = inputLogical[strings.LastIndex(inputLogical, "/")+1 : strings.LastIndex(inputLogical, ".")]
		}
		logicalOutDir := filepath.ToSlash(filepath.Join(dir, ".hidden", name))

		localOutDir, cleanup, err := srv.prepareOutputDir(logicalOutDir, wf)
		if err != nil {
			return fmt.Errorf("generateVideoPreview: cannot create output dir %q: %w", logicalOutDir, err)
		}
		defer cleanup()

		gifLogical := filepath.ToSlash(filepath.Join(logicalOutDir, "preview.gif"))
		mp4Logical := filepath.ToSlash(filepath.Join(logicalOutDir, "preview.mp4"))
		gifOut := filepath.ToSlash(filepath.Join(localOutDir, "preview.gif"))
		mp4Out := filepath.ToSlash(filepath.Join(localOutDir, "preview.mp4"))

		exists := func(logical, local string) bool {
			if wf.IsMinio {
				return srv.minioObjectExists(context.Background(), logical)
			}
			return srv.pathExists(local)
		}

		gifExists := exists(gifLogical, gifOut)
		mp4Exists := exists(mp4Logical, mp4Out)
		if gifExists && mp4Exists && !force {
			logger.Info("generateVideoPreview: previews already exist; skipping", "path", inputLogical, "out", logicalOutDir)
			return nil
		}

		start := totalSec / 10
		if start < 0 {
			start = 0
		}
		if duration <= 0 {
			duration = 10
		}
		if fps <= 0 {
			fps = 10
		}
		if scale <= 0 {
			scale = 320
		}

		runCmd := func(args []string, workdir string) error {
			wait := make(chan error, 1)
			go Utility.RunCmd("ffmpeg", workdir, args, wait)
			return <-wait
		}

		if !gifExists || force {
			if !wf.IsMinio && force && srv.pathExists(gifOut) {
				_ = os.Remove(gifOut)
			}
			gifArgs := []string{
				"-ss", strconv.Itoa(start),
				"-t", strconv.Itoa(duration),
				"-i", localInput,
				"-vf",
				fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=32[p];[s1][p]paletteuse=dither=bayer", fps, scale),
				"-loop", "0",
				"preview.gif",
			}
			logger.Info("ffmpeg: generate GIF preview", "src", inputLogical, "out", gifLogical, "fps", fps, "scale", scale, "t", duration)
			if err := runCmd(gifArgs, localOutDir); err != nil {
				_ = os.Remove(gifOut)
				return fmt.Errorf("generateVideoPreview: GIF generation failed for %q: %w", inputLogical, err)
			}
		}

		if !mp4Exists || force {
			if !wf.IsMinio && force && srv.pathExists(mp4Out) {
				_ = os.Remove(mp4Out)
			}

			venc := "libx264"
			if srv.hasEnableCudaNvcc() {
				venc = "h264_nvenc"
			}

			mp4Args := []string{
				"-y",
				"-i", localInput,
				"-ss", strconv.Itoa(start),
				"-t", strconv.Itoa(duration),
				"-filter_complex", fmt.Sprintf("[0:v]select='lt(mod(t,1/10),1)',setpts=N/(FRAME_RATE*TB),scale=%d:-2", scale),
				"-an",
				"-vcodec", venc,
				"-movflags", "+faststart", "preview.mp4",
			}

			logger.Info("ffmpeg: generate MP4 preview", "src", inputLogical, "out", mp4Logical, "venc", venc, "scale", scale, "t", duration)
			if err := runCmd(mp4Args, localOutDir); err != nil {
				logger.Warn("ffmpeg: MP4 preview failed; retrying with libx264 if applicable", "src", inputLogical, "err", err)
				if srv.hasEnableCudaNvcc() {
					mp4ArgsRetry := append([]string(nil), mp4Args...)
					for i := range mp4ArgsRetry {
						if i > 0 && mp4ArgsRetry[i-1] == "-vcodec" {
							mp4ArgsRetry[i] = "libx264"
							break
						}
					}
					if err2 := runCmd(mp4ArgsRetry, localOutDir); err2 != nil {
						return fmt.Errorf("generateVideoPreview: MP4 generation failed for %q: %w", inputLogical, err2)
					}
				} else {
					return fmt.Errorf("generateVideoPreview: MP4 generation failed for %q: %w", inputLogical, err)
				}
			}
		}

		if wf.IsMinio {
			ctx := context.Background()
			if err := srv.minioUploadDir(ctx, logicalOutDir, localOutDir); err != nil {
				return fmt.Errorf("generateVideoPreview: upload to minio failed: %w", err)
			}
		}
		return nil
	})
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
func (srv *server) createVideoTimeLine(path string, width int, fps float32, force bool) error {
	logicalOrig := filepath.ToSlash(path)

	fmt.Println("------------------------> create time line ", logicalOrig)
	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return errors.New("createVideoTimeLine: maximum concurrent ffmpeg instances reached; try again later")
	}

	if fps <= 0 {
		fps = 0.2
	}
	if width <= 0 {
		width = 180
	}

	return srv.withWorkFile(logicalOrig, func(wf mediaWorkFile) error {
		inputLogical := filepath.ToSlash(wf.LogicalPath)
		localPath := filepath.ToSlash(wf.LocalPath)

		if !strings.HasSuffix(localPath, "playlist.m3u8") && !wf.IsMinio {
			plLocal := filepath.ToSlash(localPath + "/playlist.m3u8")
			if srv.pathExists(plLocal) && !strings.HasSuffix(inputLogical, "playlist.m3u8") {
				localPath = plLocal
				inputLogical = filepath.ToSlash(filepath.Join(inputLogical, "playlist.m3u8"))
			}
		}

		if !strings.Contains(localPath, ".") {
			return fmt.Errorf("createVideoTimeLine: missing file extension for %q", inputLogical)
		}

		baseDir := inputLogical[:strings.LastIndex(inputLogical, "/")]
		name := ""
		if strings.HasSuffix(inputLogical, "playlist.m3u8") {
			name = baseDir[strings.LastIndex(baseDir, "/")+1:]
			baseDir = baseDir[:strings.LastIndex(baseDir, "/")]
		} else {
			name = inputLogical[strings.LastIndex(inputLogical, "/")+1 : strings.LastIndex(inputLogical, ".")]
		}
		logicalOutput := filepath.ToSlash(filepath.Join(baseDir, ".hidden", name, "__timeline__"))

		if !force {
			if hasImages, err := srv.timelineHasImages(logicalOutput); err == nil && hasImages {
				if err := srv.updateVttFile(logicalOutput); err != nil {
					logger.Warn("timeline VTT update failed", "video", logicalOrig, "dir", logicalOutput, "err", err)
				}
				logger.Info("timeline already exists; skipping", "video", logicalOrig, "dir", logicalOutput)
				return nil
			} else if err != nil {
				logger.Warn("timeline check failed", "video", logicalOrig, "dir", logicalOutput, "err", err)
			}
		}

		if !wf.IsMinio {
			if err := Utility.CreateDirIfNotExist(filepath.Dir(srv.formatPath(logicalOutput))); err != nil {
				return fmt.Errorf("createVideoTimeLine: create dir %q: %w", filepath.Dir(logicalOutput), err)
			}
		}

		durationSec, err := probeVideoDuration(localPath)
		if err != nil || durationSec <= 0 {
			return fmt.Errorf("createVideoTimeLine: zero-length or unreadable video: %q", inputLogical)
		}
		expectedThumbs := int(math.Ceil(float64(durationSec) * float64(fps)))
		if expectedThumbs < 1 {
			expectedThumbs = 1
		}

		if !wf.IsMinio {
			localOutput := srv.formatPath(logicalOutput)
			if srv.pathExists(localOutput) {
				if !force {
					thumbCount := 0
					if entries, err := Utility.ReadDir(localOutput); err == nil {
						for _, e := range entries {
							if strings.HasSuffix(strings.ToLower(e.Name()), ".jpg") {
								thumbCount++
							}
						}
					} else {
						logger.Warn("timeline check: failed to read directory", "video", inputLogical, "dir", localOutput, "err", err)
					}

					diff := math.Abs(float64(thumbCount - expectedThumbs))
					if thumbCount > 0 && diff <= 1 {
						logger.Info("timeline already exists; regenerating VTT only", "video", logicalOrig, "dir", logicalOutput, "fps", fps, "thumbnails", thumbCount)
						return srv.createVttFile(localOutput, logicalOutput, fps)
					}

					logger.Info("timeline mismatch detected; regenerating thumbnails", "video", logicalOrig, "dir", logicalOutput, "expected", expectedThumbs, "found", thumbCount)
				}
				if err := os.RemoveAll(localOutput); err != nil {
					return fmt.Errorf("createVideoTimeLine: remove existing timeline %q: %w", localOutput, err)
				}
			}
		}

		outDir, cleanup, err := srv.prepareOutputDir(logicalOutput, wf)
		if err != nil {
			return fmt.Errorf("createVideoTimeLine: create dir %q: %w", logicalOutput, err)
		}
		defer cleanup()

		thumbPattern := "thumbnail_%05d.jpg"
		args := []string{
			"-y",
			"-i", localPath,
			"-ss", "0",
			"-t", Utility.ToString(durationSec),
			"-vf", "scale=-1:" + Utility.ToString(width) + ",fps=" + Utility.ToString(fps),
			thumbPattern,
		}
		logger.Info("ffmpeg: timeline extraction",
			"video", inputLogical,
			"out", filepath.Join(logicalOutput, thumbPattern),
			"height", width,
			"fps", fps,
			"duration_sec", durationSec)

		runFFmpeg := func() error {
			wait := make(chan error, 1)
			go Utility.RunCmd("ffmpeg", outDir, args, wait)
			return <-wait
		}

		if err := runFFmpeg(); err != nil {
			if !wf.IsMinio {
				time.Sleep(500 * time.Millisecond)
				if mkErr := Utility.CreateDirIfNotExist(outDir); mkErr == nil {
					if retryErr := runFFmpeg(); retryErr == nil {
						goto afterFF
					}
				}
			}
			logger.Error("ffmpeg timeline extraction failed", "video", inputLogical, "out", logicalOutput, "err", err)
			return fmt.Errorf("createVideoTimeLine: ffmpeg extraction failed for %q: %w", inputLogical, err)
		}
	afterFF:

		if err := srv.createVttFile(outDir, logicalOutput, fps); err != nil {
			return fmt.Errorf("createVideoTimeLine: VTT generation failed for %q: %w", logicalOutput, err)
		}

		if wf.IsMinio {
			ctx := context.Background()
			if err := srv.minioUploadDir(ctx, logicalOutput, outDir); err != nil {
				return fmt.Errorf("createVideoTimeLine: upload failed for %q: %w", logicalOutput, err)
			}
		}

		logger.Info("timeline created",
			"video", logicalOrig,
			"dir", logicalOutput,
			"fps", fps,
			"height", width)
		return nil
	})
}

func (srv *server) timelineHasImages(logicalDir string) (bool, error) {
	logicalDir = filepath.ToSlash(logicalDir)

	entries, err := srv.readDirEntries(logicalDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
			return true, nil
		}
	}
	return false, nil
}

func (srv *server) previewHasImages(logicalDir string) (bool, error) {
	logicalDir = filepath.ToSlash(logicalDir)

	entries, err := srv.readDirEntries(logicalDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".gif") || strings.HasSuffix(name, ".mp4") {
			return true, nil
		}
	}
	return false, nil
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
func (srv *server) createVideoPreview(path string, nb, height int, force bool) error {
	logicalPath := filepath.ToSlash(path)
	if strings.Contains(logicalPath, ".hidden") || strings.Contains(logicalPath, ".temp") {
		return nil
	}

	if procs, _ := Utility.GetProcessIdsByName("ffmpeg"); len(procs) > MAX_FFMPEG_INSTANCE {
		return errors.New("number of ffmpeg instances has been reached; try later")
	}

	return srv.withWorkFile(logicalPath, func(wf mediaWorkFile) error {
		inputLogical := filepath.ToSlash(wf.LogicalPath)
		localPath := filepath.ToSlash(wf.LocalPath)

		if !strings.HasSuffix(localPath, "playlist.m3u8") && !wf.IsMinio {
			plLocal := filepath.ToSlash(localPath + "/playlist.m3u8")
			if srv.pathExists(plLocal) && !strings.HasSuffix(inputLogical, "playlist.m3u8") {
				localPath = plLocal
				inputLogical = filepath.ToSlash(filepath.Join(inputLogical, "playlist.m3u8"))
			}
		}

		if !strings.Contains(localPath, ".") {
			return fmt.Errorf("%s does not have an extension", localPath)
		}

		parent := inputLogical[:strings.LastIndex(inputLogical, "/")]
		base := ""
		if strings.HasSuffix(inputLogical, "playlist.m3u8") {
			base = parent[strings.LastIndex(parent, "/")+1:]
			parent = parent[:strings.LastIndex(parent, "/")]
		} else {
			base = inputLogical[strings.LastIndex(inputLogical, "/")+1 : strings.LastIndex(inputLogical, ".")]
		}
		logicalOutDir := parent + "/.hidden/" + base + "/__preview__"

		cache.RemoveItem(inputLogical)
		cache.RemoveItem(logicalOutDir)

		if !force {
			if exists, err := srv.previewHasImages(logicalOutDir); err == nil && exists {
				logger.Info("preview already exists; skipping", "path", inputLogical, "dir", logicalOutDir)
				return nil
			} else if err != nil {
				logger.Warn("preview check failed", "path", inputLogical, "dir", logicalOutDir, "err", err)
			}
		}

		if !wf.IsMinio && srv.pathExists(srv.formatPath(logicalOutDir)) {
			_ = os.RemoveAll(srv.formatPath(logicalOutDir))
		}

		outDir, cleanup, err := srv.prepareOutputDir(logicalOutDir, wf)
		if err != nil {
			return err
		}
		defer cleanup()

		const maxWaitSec = 300
		dur := 0
		for tries := 0; tries < maxWaitSec; tries++ {
			if d, err := probeVideoDuration(localPath); err == nil && d > 0 {
				dur = d
				break
			}
			time.Sleep(1 * time.Second)
		}
		if dur == 0 {
			slog.Warn("createVideoPreview: video duration is zero", "path", inputLogical)
			return errors.New("the video length is 0 sec")
		}

		start := dur / 10
		span := 120

		var runErr error
		for tries := 0; tries < maxWaitSec; tries++ {
			Utility.CreateDirIfNotExist(outDir)

			wait := make(chan error, 1)
			args := []string{
				"-i", localPath,
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

		if wf.IsMinio {
			ctx := context.Background()
			if err := srv.minioUploadDir(ctx, logicalOutDir, outDir); err != nil {
				return err
			}
		}

		if client, err := getEventClient(); err == nil {
			dir := filepath.Dir(srv.formatPath(inputLogical))
			dir = strings.ReplaceAll(dir, "\\", "/")
			client.Publish("reload_dir_event", []byte(dir))
		}

		return nil
	})
}

// createVideoMpeg4H264 converts any input to MP4/H.264, mapping audio/subtitle tracks.
// (Public method signature preserved.)
func (srv *server) createVideoMpeg4H264(path string) (string, error) {
	cache.RemoveItem(path)
	_ = srv.extractSubtitleTracks(path)

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
		if srv.pathExists(out) {
			_ = os.Remove(out)
		}
	} else {
		hevc := dir + "/" + name + ".hevc"
		if srv.pathExists(hevc) {
			return "", fmt.Errorf("conversion already in progress: %s", out)
		}
		_ = Utility.MoveFile(out, hevc)
		path = hevc
	}

	streams, err := srv.getStreamInfos(path)
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
	args = append(args, "-movflags", "+faststart", out)

	wait := make(chan error, 1)
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
	sList, ok := streams["streams"].([]any)
	if !ok {
		return 0
	}
	cnt := 0
	for _, entry := range sList {
		sm, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if codecType, _ := sm["codec_type"].(string); codecType == kind {
			cnt++
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
func (srv *server) createHlsStream(src, dest string, segmentTarget int, maxBitrateRatio, rateMonitorBufferRatio float32) error {
	// Throttle concurrent ffmpeg.
	if pids, _ := Utility.GetProcessIdsByName("ffmpeg"); len(pids) > MAX_FFMPEG_INSTANCE {
		return errors.New("too many ffmpeg instances; please try again later")
	}

	src = filepath.ToSlash(src)
	dest = filepath.ToSlash(dest)

	if err := Utility.CreateDirIfNotExist(dest); err != nil {
		logger.Error("createHlsStream: ensure dest dir failed", "dest", dest, "err", err)
		return err
	}

	// --- Probe once --------------------------------------------------------
	streamInfos, err := srv.getStreamInfos(src)
	if err != nil {
		logger.Error("createHlsStream: getStreamInfos failed", "src", src, "err", err)
		return err
	}

	var vCodec string
	audioCodecs := make(map[string]struct{})

	if streams, ok := streamInfos["streams"].([]interface{}); ok {
		for _, s := range streams {
			sm, _ := s.(map[string]interface{})
			if sm == nil {
				continue
			}
			ct, _ := sm["codec_type"].(string)
			switch ct {
			case "video":
				if vCodec == "" {
					vCodec, _ = sm["codec_name"].(string)
				}
			case "audio":
				if cn, _ := sm["codec_name"].(string); cn != "" {
					audioCodecs[strings.ToLower(cn)] = struct{}{}
				}
			}
		}
	}

	// --- FAST PATH: already H.264 + AAC => segment only --------------------
	allAac := len(audioCodecs) == 0 // no audio or only AAC
	for c := range audioCodecs {
		if c != "aac" {
			allAac = false
			break
		}
	}

	if strings.ToLower(vCodec) == "h264" && allAac {
		logger.Info("createHlsStream: using fast copy HLS path",
			"src", src, "dest", dest, "vcodec", vCodec, "audioCodecs", audioCodecs)

		args := []string{
			"-hide_banner", "-y",
			"-i", src,
			"-codec:v", "copy",
			"-codec:a", "copy",
			"-start_number", "0",
			"-hls_time", Utility.ToString(segmentTarget),
			"-hls_playlist_type", "vod",
			"-hls_segment_filename",
			filepath.ToSlash(filepath.Join(dest, "segment_%04d.ts")),
			filepath.ToSlash(filepath.Join(dest, "playlist.m3u8")),
		}

		wait := make(chan error, 1)
		go Utility.RunCmd("ffmpeg", filepath.Dir(src), args, wait)
		if runErr := <-wait; runErr != nil {
			logger.Error("createHlsStream: fast copy HLS failed", "src", src, "dest", dest, "err", runErr)
			return runErr
		}
		return nil
	}

	// --- SLOW PATH: full re-encode with ladder (your current logic) -------
	keyint, err := srv.getStreamFrameRateInterval(src)
	if err != nil || keyint <= 0 {
		if err != nil {
			logger.Warn("createHlsStream: FPS probe failed, falling back", "src", src, "err", err)
		}
		keyint = 25
	}

	encodingLong := ""
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
				encodingLong = cl
				break
			}
		}
	}
	if encodingLong == "" {
		return errors.New("no usable video stream found")
	}

	args := []string{"-hide_banner", "-y", "-i", src, "-c:v"}
	switch {
	case srv.hasEnableCudaNvcc():
		if strings.HasPrefix(encodingLong, "H.264") || strings.HasPrefix(encodingLong, "MPEG-4 part 2") {
			args = append(args, "h264_nvenc")
		} else if strings.HasPrefix(encodingLong, "H.265") || strings.HasPrefix(encodingLong, "Motion JPEG") {
			args = append(args, "h264_nvenc", "-pix_fmt", "yuv420p")
		} else {
			return fmt.Errorf("no GPU encoder mapping for source codec %q", encodingLong)
		}
	default:
		if strings.HasPrefix(encodingLong, "H.264") || strings.HasPrefix(encodingLong, "MPEG-4 part 2") {
			args = append(args, "libx264")
		} else if strings.HasPrefix(encodingLong, "H.265") || strings.HasPrefix(encodingLong, "Motion JPEG") {
			args = append(args, "libx264", "-pix_fmt", "yuv420p")
		} else {
			return fmt.Errorf("no CPU encoder mapping for source codec %q", encodingLong)
		}
	}

	// Decide ladder from input width (same as you already do).
	w, _ := srv.getVideoResolution(src)
	renditions := make([]map[string]string, 0, 6)
	if w >= 426 {
		renditions = append(renditions, map[string]string{"res": "426x240", "vbit": "800k", "abit": "96k"})
	}
	if w >= 640 {
		renditions = append(renditions, map[string]string{"res": "640x360", "vbit": "1400k", "abit": "128k"})
	}
	if w >= 842 {
		renditions = append(renditions, map[string]string{"res": "842x480", "vbit": "1700k", "abit": "128k"})
	}
	if w >= 1280 {
		renditions = append(renditions, map[string]string{"res": "1280x720", "vbit": "2800k", "abit": "128k"})
	}
	if w >= 1920 {
		renditions = append(renditions, map[string]string{"res": "1920x1080", "vbit": "5000k", "abit": "192k"})
	}
	if len(renditions) == 0 {
		renditions = append(renditions, map[string]string{"res": "426x240", "vbit": "800k", "abit": "96k"})
	}

	staticParams := []string{
		"-profile:v", "main",
		"-sc_threshold", "0",
		"-g", Utility.ToString(keyint),
		"-keyint_min", Utility.ToString(keyint),
		"-hls_time", Utility.ToString(segmentTarget),
		"-hls_playlist_type", "vod",
	}

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

		numStr := strings.TrimSuffix(vbit, "k")
		vk := Utility.ToInt(numStr)
		maxrate := int(float32(vk) * maxBitrateRatio)
		bufsize := int(float32(vk) * rateMonitorBufferRatio)

		scale := fmt.Sprintf("scale=-2:min(%s\\,if(mod(ih\\,2)\\,ih-1\\,ih))", width)

		args = append(args, staticParams...)
		args = append(args,
			"-vf", scale,
			"-map", "0:v",
			"-map", "0:a:0?", "-c:a:0", "aac",
			"-b:v", vbit,
			"-maxrate", fmt.Sprintf("%dk", maxrate),
			"-bufsize", fmt.Sprintf("%dk", bufsize),
			"-b:a", abit,
			"-hls_segment_filename", filepath.ToSlash(filepath.Join(dest, name+"_%%04d.ts")),
			filepath.ToSlash(filepath.Join(dest, name+".m3u8")),
		)

		master.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s\n%s\n", vk*1000, res, name+".m3u8"))
	}

	logger.Info("ffmpeg: create HLS",
		"src", src, "dest", dest,
		"keyint", keyint,
		"segment", segmentTarget,
		"rungs", len(renditions),
	)

	wait := make(chan error, 1)
	go Utility.RunCmd("ffmpeg", filepath.Dir(src), args, wait)
	if runErr := <-wait; runErr != nil {
		logger.Error("createHlsStream: ffmpeg failed", "src", src, "dest", dest, "err", runErr)
		return runErr
	}

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
	if srv.pathExists(master) {
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
func (srv *server) extractSubtitleTracks(videoPath string) error {
	videoPath = filepath.ToSlash(videoPath)

	// Probe subtitle streams ("s").
	tracks := srv.getTrackInfos(videoPath, "s")
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
	if srv.pathExists(dest) {
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
	wait := make(chan error, 1)
	go Utility.RunCmd("ffmpeg", dest, args, wait)
	if err := <-wait; err != nil {
		logger.Error("ffmpeg: subtitle extraction failed", "src", videoPath, "dest", dest, "err", err)
		return fmt.Errorf("subtitle extraction failed for %q: %w", videoPath, err)
	}

	return nil
}
