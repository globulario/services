package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// thumbnailDirFor returns the .hidden thumbnail directory for a video path.
// Example: /path/video.mkv -> /path/.hidden/video/__thumbnail__
func thumbnailDirFor(videoPath string) string {
	videoPath = filepath.Clean(videoPath)

	base := filepath.Dir(videoPath)
	name := filepath.Base(videoPath)

	// Remove extension for ANY file type
	if ext := filepath.Ext(name); ext != "" {
		name = strings.TrimSuffix(name, ext)
	}

	return filepath.Join(base, ".hidden", name, "__thumbnail__")
}

// mostRecentJPG returns the newest .jpg/.jpeg file in dir based on mod time.
func mostRecentJPG(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var newestPath string
	var newestTime time.Time

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := strings.ToLower(e.Name())
		if !strings.HasSuffix(name, ".jpg") && !strings.HasSuffix(name, ".jpeg") {
			continue
		}

		fullPath := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}

		if newestPath == "" || info.ModTime().After(newestTime) {
			newestPath = fullPath
			newestTime = info.ModTime()
		}
	}

	if newestPath == "" {
		return "", fmt.Errorf("no jpg found")
	}

	return newestPath, nil
}
