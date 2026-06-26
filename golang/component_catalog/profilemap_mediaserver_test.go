package component_catalog

import (
	"reflect"
	"testing"
)

func msContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestMediaServerProfile_TorrentAuthorizedByMediaServerNotCompute is the
// placement regression for the media-server role. torrent — and the rest of the
// media stack — must be authorized ONLY by media-server, never by accidental
// compute/core membership. That accidental membership is exactly what stranded
// torrent as a compute-only orphan on a [control-plane core storage] node and
// bloated core with the media/transcoding stack.
func TestMediaServerProfile_TorrentAuthorizedByMediaServerNotCompute(t *testing.T) {
	// The whole media stack is owned exclusively by media-server.
	for _, pkg := range []string{"media", "title", "ffmpeg", "yt-dlp", "torrent"} {
		got := ProfilesForPackage(pkg)
		if !reflect.DeepEqual(got, []string{"media-server"}) {
			t.Errorf("%s must be authorized only by media-server, got %v", pkg, got)
		}
	}

	// torrent must NOT be reachable from compute or core (the old accidental homes).
	if msContains(PackagesForProfiles([]string{"compute"}), "torrent") {
		t.Error("torrent must NOT be authorized by compute")
	}
	if msContains(PackagesForProfiles([]string{"core"}), "torrent") {
		t.Error("torrent must NOT be authorized by core")
	}

	// search stays in core (general indexing, not media-specific).
	if got := ProfilesForPackage("search"); !msContains(got, "core") {
		t.Errorf("search must remain in core, got %v", got)
	}

	// media-server inherits core: a media-server node expands to include core.
	norm := NormalizeProfiles([]string{"media-server"})
	if !msContains(norm, "media-server") || !msContains(norm, "core") {
		t.Errorf("media-server must expand to include core via inheritance, got %v", norm)
	}

	// A media-server node installs BOTH the media stack AND the inherited core
	// platform floor (including search).
	pkgs := PackagesForProfiles([]string{"media-server"})
	for _, want := range []string{
		"media", "title", "ffmpeg", "yt-dlp", "torrent", // media stack
		"dns", "rbac", "authentication", "file", "search", // core floor
	} {
		if !msContains(pkgs, want) {
			t.Errorf("media-server node must install %q (media stack + inherited core); missing from %v", want, pkgs)
		}
	}
}
