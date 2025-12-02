// --- pathutil.go ---
package main

import (
	"log/slog"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

func hasVirtualRoot(p string) bool {
	for _, prefix := range []string{"/users", "/applications", "/templates"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

// toSlash normalizes path separators to forward slashes, for consistent internal logic.
func toSlash(p string) string { return strings.ReplaceAll(p, "\\", "/") }

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

	// Unescape URL-encoded input, unify slashes for internal checks.
	p, _ := url.PathUnescape(in)
	p = toSlash(p)

	// Fast-path root
	if p == "/" {
		return cleanPathOS(srv.Root)
	}

	// Respect already-public absolute paths.
	if isAbsLike(p) {
		// If path lives in a public mount, keep it as-is.
		if srv.isPublic(p) {
			return cleanPathOS(p)
		}

		// Ensure virtual roots map under the data files directory.
		if hasVirtualRoot(p) {
			trimmed := strings.TrimPrefix(p, "/")
			mapped := filepath.Join(config.GetDataDir(), "files", trimmed)
			return cleanPathOS(mapped)
		}

		// If the absolute path is directly on disk, prefer it (network mounts etc.)
		if Utility.Exists(p) {
			return cleanPathOS(p)
		}

		// Try data/files roots and webroot mirroring semantics
		if strings.HasPrefix(p, "/users/") || strings.HasPrefix(p, "/applications/") {
			pp := toSlash(config.GetDataDir() + "/files" + p)
			if Utility.Exists(pp) {
				return cleanPathOS(pp)
			}
		}
		if pr := toSlash(config.GetWebRootDir() + p); Utility.Exists(pr) {
			return cleanPathOS(pr)
		}
		if pr := toSlash(srv.Root + p); Utility.Exists(pr) {
			return cleanPathOS(pr)
		}
		// Last resort, join under Root (even if it doesn't exist yet—creator funcs may follow)
		return cleanPathOS(filepath.Join(srv.Root, p))
	}

	// Relative input → anchor under Root
	return cleanPathOS(filepath.Join(srv.Root, p))
}

// getMimeTypesUrl returns a data-URL thumbnail for a mime icon path.
func (srv *server) getMimeTypesUrl(iconPath string) (string, error) {
	// Resolve relative icon paths against CWD just like original code did.
	icon := toSlash(iconPath)
	thumb, err := srv.getThumbnail(icon, 80, 80)
	if err != nil {
		slog.Error("mime icon thumbnail failed", "icon", icon, "err", err)
	}
	return thumb, err
}
