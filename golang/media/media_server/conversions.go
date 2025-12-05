package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"

	Utility "github.com/globulario/utility"
)

// getStreamInfos runs ffprobe and returns parsed stream info (format+streams).
func (srv *server) getStreamInfos(path string) (map[string]interface{}, error) {
	var infos map[string]interface{}
	err := srv.withWorkFile(path, func(wf mediaWorkFile) error {
		local := strings.ReplaceAll(wf.LocalPath, "\\", "/")
		cmd := exec.Command("ffprobe", "-v", "error", "-show_format", "-show_streams", "-print_format", "json", local)
		cmd.Dir = filepath.Dir(local)

		data, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		tmp := make(map[string]interface{})
		if err := json.Unmarshal(data, &tmp); err != nil {
			return err
		}
		infos = tmp
		return nil
	})
	return infos, err
}

// getCodec returns the codec name of the first video stream.
func (srv *server) getCodec(path string) string {
	codec := ""
	_ = srv.withWorkFile(path, func(wf mediaWorkFile) error {
		local := strings.ReplaceAll(wf.LocalPath, "\\", "/")
		cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=codec_name", "-of", "default=noprint_wrappers=1:nokey=1", local)
		cmd.Dir = filepath.Dir(local)
		data, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("ffprobe codec probe failed", "path", local, "err", err)
			return err
		}
		codec = strings.TrimSpace(string(data))
		return nil
	})
	return codec
}

// getVideoResolution returns width and height of the first video stream.
func (srv *server) getVideoResolution(path string) (int, int) {
	width, height := -1, -1
	_ = srv.withWorkFile(path, func(wf mediaWorkFile) error {
		local := strings.ReplaceAll(wf.LocalPath, "\\", "/")
		cmd := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "default=nw=1", local)
		cmd.Dir = filepath.Dir(local)

		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			logger.Error("ffprobe resolution failed", "path", local, "stderr", stderr.String(), "err", err)
			return err
		}

		outStr := out.String()
		wStr := outStr[strings.Index(outStr, "=")+1 : strings.Index(outStr, "\n")]
		hStr := outStr[strings.LastIndex(outStr, "=")+1:]
		width = Utility.ToInt(strings.TrimSpace(wStr))
		height = Utility.ToInt(strings.TrimSpace(hStr))
		return nil
	})
	return width, height
}

// Example of replacing fmt prints with slog in a helper.
func exampleLogUsage() {
	logger.Info("conversion started", "path", "/some/file.mp4")
	logger.Error("conversion failed", "err", errors.New("sample error"))
	slog.Debug("detailed debug") // if enabled via InitLogger("debug")
}
