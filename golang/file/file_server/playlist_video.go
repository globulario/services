// --- playlist_video.go ---
package main

import (
	"log/slog"
	"os"
	"strings"

	Utility "github.com/globulario/utility"
)

// createVideoPreview delegates preview creation to Media service.
func (s *server) createVideoPreview(path string, nb int, height int) error {
	client, err := getMediaClient(); if err != nil { return err }
	return client.CreateVideoPreview(path, int32(height), int32(nb))
}

func (srv *server) generatePlaylist(path, token string) error {
	client, err := getMediaClient(); if err != nil { return err }
	return client.GeneratePlaylist(path, token)
}

func createVttFile(output string, fps float32) error {
	client, err := getMediaClient(); if err != nil { return err }
	return client.CreateVttFile(output, fps)
}

func (s *server) createVideoTimeLine(path string, width int, fps float32) error {
	client, err := getMediaClient(); if err != nil { return err }
	return client.CreateVideoTimeLine(path, int32(width), fps)
}

// processVideos regenerates media previews and playlists for given dirs.
func processVideos(srv *server, token string, dirs []string) {
	client, err := getMediaClient(); if err != nil { slog.Error("media client failed", "err", err); return }
	for _, d := range dirs {
		path := srv.formatPath(d)
		if !Utility.Exists(path) { continue }
		filePerms, _ := rbac_client_.GetResourcePermissionsByResourceType("file")
		perm, _ := rbac_client_.GetResourcePermissions(path)
		if st, err := os.Stat(path); err == nil {
			if st.IsDir() {
				for _, p := range filePerms {
					if strings.HasPrefix(p.Path, path) {
						_ = rbac_client_.DeleteResourcePermissions(p.Path)
						if err := rbac_client_.SetResourcePermissions(token, p.Path, p.ResourceType, p); err != nil { slog.Warn("perm update failed", "path", p.Path, "err", err) }
					}
				}
			} else if perm != nil {
				_ = rbac_client_.DeleteResourcePermissions(path)
				perm.Path = path
				if err := rbac_client_.SetResourcePermissions(token, path, perm.ResourceType, perm); err != nil { slog.Warn("perm update failed", "path", path, "err", err) }
			}
		}
		if err := client.CreateVideoPreview(path, 360, 5); err != nil { slog.Warn("preview failed", "path", path, "err", err) }
		if err := client.CreateVideoTimeLine(path, 360, 5); err != nil { slog.Warn("timeline failed", "path", path, "err", err) }
		if err := client.GeneratePlaylist(path, token); err != nil { slog.Warn("playlist failed", "path", path, "err", err) }
	}
}
