package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	Utility "github.com/globulario/utility"
)

// getStreamInfos runs ffprobe and returns parsed stream info (format+streams).
func getStreamInfos(path string) (map[string]interface{}, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	cmd := exec.Command("ffprobe", "-v", "error", "-show_format", "-show_streams", "-print_format", "json", path)
	cmd.Dir = filepath.Dir(path)

	data, _ := cmd.CombinedOutput()
	infos := make(map[string]interface{})
	if err := json.Unmarshal(data, &infos); err != nil {
		return nil, err
	}
	return infos, nil
}

// getCodec returns the codec name of the first video stream.
func getCodec(path string) string {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", path)
	cmd.Dir = os.TempDir()
	codec, _ := cmd.CombinedOutput()
	return strings.TrimSpace(string(codec))
}

// getVideoResolution returns width and height of the first video stream.
func getVideoResolution(path string) (int, int) {
	path = strings.ReplaceAll(path, "\\", "/")
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "default=nw=1", path)
	cmd.Dir = filepath.Dir(path)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logger.Error("ffprobe resolution failed", "path", path, "stderr", stderr.String(), "err", err)
		return -1, -1
	}

	outStr := out.String()
	wStr := outStr[strings.Index(outStr, "=")+1 : strings.Index(outStr, "\n")]
	hStr := outStr[strings.LastIndex(outStr, "=")+1:]
	return Utility.ToInt(strings.TrimSpace(wStr)), Utility.ToInt(strings.TrimSpace(hStr))
}

// Example of replacing fmt prints with slog in a helper.
func exampleLogUsage() {
	logger.Info("conversion started", "path", "/some/file.mp4")
	logger.Error("conversion failed", "err", errors.New("sample error"))
	slog.Debug("detailed debug") // if enabled via InitLogger("debug")
}
