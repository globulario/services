// --- pathutil.go ---
package main

import (
	"context"
	"encoding/base64"
	"log/slog"
	"mime"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	//"github.com/globulario/services/golang/config"
	//Utility "github.com/globulario/utility"
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

// formatPath normalizes an incoming API path to an absolute filesystem path on the host.
// Behavior is kept compatible with the original logic, but simplified and documented.
func (srv *server) formatPath(in string) string {
	if in == "" {
		return srv.Root
	}

	p, _ := url.PathUnescape(in)
	p = filepath.ToSlash(p)

	if p == "/" {
		return srv.Root
	}

	return p
}

var mimeIconCache sync.Map

// getMimeTypesUrl returns a cached data URL for a static mime icon.
func (srv *server) getMimeTypesUrl(iconPath string) (string, error) {
	icon := filepath.ToSlash(srv.formatPath(iconPath))
	if val, ok := mimeIconCache.Load(icon); ok {
		return val.(string), nil
	}

	data, err := srv.storageReadFile(context.Background(), icon)
	if err != nil {
		slog.Error("mime icon read failed", "icon", icon, "err", err)
		return "", err
	}

	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(icon)))
	if mimeType == "" {
		mimeType = "image/png"
	}
	thumb := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	mimeIconCache.Store(icon, thumb)
	return thumb, nil
}
