// --- pathutil.go ---
package main

import (
	"context"
	"encoding/base64"
	"log/slog"
	"mime"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

func hasVirtualRoot(p string) bool {
	for _, prefix := range []string{"/users", "/applications", "/templates"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

// cleanPath cleans redundant elements and converts to OS-native separators for FS ops.
func cleanPathOS(p string) string { return filepath.Clean(p) }

// isAbsLike detects absolute or root-like inputs (both /unix and C:\win or \\share).
func isAbsLike(p string) bool {
	if p == "" {
		return false
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return true
	}
	if runtime.GOOS == "windows" {
		// e.g., C:\ or C:/
		if len(p) >= 2 && (p[1] == ':' && (p[2:] == "" || p[2] == '/' || p[2] == '\\')) {
			return true
		}
	}
	return false
}


var mimeIconCache sync.Map

// getMimeTypesUrl returns a cached data URL for a static mime icon.
func (srv *server) getMimeTypesUrl(iconPath string) (string, error) {
	
	if val, ok := mimeIconCache.Load(iconPath); ok {
		return val.(string), nil
	}

	data, err := srv.storageReadFile(context.Background(), iconPath)
	if err != nil {
		slog.Error("mime icon read failed", "icon", iconPath, "err", err)
		return "", err
	}

	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(iconPath)))
	if mimeType == "" {
		mimeType = "image/png"
	}
	thumb := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	mimeIconCache.Store(iconPath, thumb)
	return thumb, nil
}
