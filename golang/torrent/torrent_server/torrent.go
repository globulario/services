// Package main contains torrent management helpers for the Globular service.
// It exposes RPC handlers and utility functions to download torrents, stream
// live progress to clients, and persist "recent links" on disk.
package main

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/torrent/torrentpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsUrl reports whether str parses as an absolute URL with scheme and host.
// Kept for backward compatibility with existing callers (name intentionally not IsURL).
func IsUrl(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// TorrentTransfer tracks a torrent and where its files should be copied once
// download completes.
type TorrentTransfer struct {
	dst   string
	lnk   string
	seed  bool
	owner string
	tor   *torrent.Torrent
}

// TorrentLnk is the persisted record of a torrent "link" (magnet or .torrent file).
type TorrentLnk struct {
	Name  string // Human-readable torrent name.
	Dir   string // Destination directory where files are copied once complete.
	Lnk   string // Magnet URL or local/remote path to a .torrent file.
	Seed  bool   // If true, continue to seed after download completes.
	Owner string // Account identifier that requested the torrent.
}

func (srv *server) resolveDestPath(dest string) string {
	if dest == "" {
		return dest
	}
	cleaned := filepath.Clean(dest)
	dataRoot := filepath.Clean(filepath.Join(config.GetDataDir(), "files"))
	slash := filepath.ToSlash(cleaned)
	dataRootSlash := filepath.ToSlash(dataRoot)
	if strings.HasPrefix(slash, dataRootSlash) {
		return cleaned
	}
	if strings.HasPrefix(slash, "/users/") || strings.HasPrefix(slash, "/applications/") || strings.HasPrefix(slash, "/templates/") ||
		slash == "/users" || slash == "/applications" || slash == "/templates" {
		tail := strings.TrimPrefix(slash, "/")
		return filepath.Join(dataRoot, tail)
	}
	return cleaned
}

// saveTorrentLnks persists the provided slice into srv.DownloadDir/lnks.gob.
// The file is overwritten atomically.
func (srv *server) saveTorrentLnks(lnks []TorrentLnk) error {
	if srv.DownloadDir == "" {
		return errors.New("saveTorrentLnks: empty DownloadDir on server")
	}
	if err := Utility.CreateDirIfNotExist(srv.DownloadDir); err != nil {
		return fmt.Errorf("saveTorrentLnks: ensure download dir: %w", err)
	}

	tmp := filepath.Join(srv.DownloadDir, "lnks.gob.tmp")
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("saveTorrentLnks: create temp file: %w", err)
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(lnks); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("saveTorrentLnks: encode gob: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("saveTorrentLnks: close temp file: %w", err)
	}
	final := filepath.Join(srv.DownloadDir, "lnks.gob")
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("saveTorrentLnks: atomic rename: %w", err)
	}
	slog.Info("saved torrent links", "path", final, "count", len(lnks))
	return nil
}

// readTorrentLnks loads previously saved links from srv.DownloadDir/lnks.gob.
// If the file does not exist, it returns an empty slice and a nil error.
func (srv *server) readTorrentLnks() ([]TorrentLnk, error) {
	lnks := make([]TorrentLnk, 0)
	if srv.DownloadDir == "" {
		return lnks, errors.New("readTorrentLnks: empty DownloadDir on server")
	}

	path := filepath.Join(srv.DownloadDir, "lnks.gob")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lnks, nil
		}
		return lnks, fmt.Errorf("readTorrentLnks: open %s: %w", path, err)
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	if err := dec.Decode(&lnks); err != nil {
		return lnks, fmt.Errorf("readTorrentLnks: decode gob: %w", err)
	}
	return lnks, nil
}

// processTorrent runs a goroutine that:
//   - tracks active torrent transfers
//   - periodically updates progress for connected clients
//   - copies completed files to their destination and assigns ownership metadata
//   - handles actions sent into srv.actions and shutdown via srv.done
//
// It must be called once during service initialization.
func (srv *server) processTorrent() {
	pending := make([]*TorrentTransfer, 0)
	infos := make(map[string]*torrentpb.TorrentInfo)
	ticker := time.NewTicker(1 * time.Second)
	getTorrentsInfoActions := make([]map[string]interface{}, 0)
	token, _ := security.GetLocalToken(srv.Mac)

	go func() {
		for {
			select {
			case a := <-srv.actions:
				switch a["action"] {
				case "setTorrentTransfer":
					t := a["torrentTransfer"].(*TorrentTransfer)
					pending = append(pending, t)

					lnks, err := srv.readTorrentLnks()
					if err != nil {
						slog.Warn("read torrent links failed; starting fresh list", "err", err)
						lnks = make([]TorrentLnk, 0)
					}
					exists := false
					for _, l := range lnks {
						if l.Lnk == t.lnk {
							exists = true
							break
						}
					}
					if !exists {
						lnks = append(lnks, TorrentLnk{
							Dir:   t.dst,
							Lnk:   t.lnk,
							Seed:  t.seed,
							Name:  t.tor.Name(),
							Owner: t.owner,
						})
						if err := srv.saveTorrentLnks(lnks); err != nil {
							slog.Error("failed to save torrent links", "err", err)
						}
					}

				case "getTorrentsInfo":
					getTorrentsInfoActions = append(getTorrentsInfoActions, a)

				case "dropTorrent":
					name := a["name"].(string)
					delete(infos, name)

					kept := make([]*TorrentTransfer, 0, len(pending))
					for _, p := range pending {
						if p.tor.Name() == name {
							p.tor.Drop()
							_ = os.RemoveAll(filepath.Join(srv.DownloadDir, p.tor.Name()))
							slog.Info("dropped torrent", "name", name)
						} else {
							kept = append(kept, p)
						}
					}
					pending = kept

					// remove from persisted links
					if lnks, err := srv.readTorrentLnks(); err == nil {
						filtered := make([]TorrentLnk, 0, len(lnks))
						for _, l := range lnks {
							if l.Name != name {
								filtered = append(filtered, l)
							}
						}
						if err := srv.saveTorrentLnks(filtered); err != nil {
							slog.Error("failed to update saved torrent links on drop", "err", err)
						}
					}

				case "getTorrentLnks":
					lnks, err := srv.readTorrentLnks()
					out := make([]*torrentpb.TorrentLnk, 0, len(lnks))
					if err == nil {
						for _, l := range lnks {
							out = append(out, &torrentpb.TorrentLnk{
								Name:  l.Name,
								Lnk:   l.Lnk,
								Dest:  l.Dir,
								Seed:  l.Seed,
								Owner: l.Owner,
							})
						}
					} else {
						slog.Error("read torrent links failed", "err", err)
					}
					a["lnks"].(chan []*torrentpb.TorrentLnk) <- out
				default:
					slog.Warn("unknown torrent action", "action", a["action"])
				}

			case <-ticker.C:
				// Build the latest infos snapshot.
				snapshot := make([]*torrentpb.TorrentInfo, 0, len(pending))
				for _, p := range pending {
					name := p.tor.Name()
					infos[name] = getTorrentInfo(p.tor, infos[name])
					infos[name].Destination = p.dst
					snapshot = append(snapshot, infos[name])

					// Copy fully downloaded files.
					for _, fi := range infos[name].Files {
						if fi.Percent == 100 {
							src := filepath.Join(srv.DownloadDir, fi.Path)
							dst := filepath.Join(p.dst, fi.Path)
							if Utility.Exists(src) && !Utility.Exists(dst) {
								dir := filepath.Dir(dst)
								if err := Utility.CreateDirIfNotExist(dir); err == nil {
									rel := dir
									if strings.Contains(dir, "/files/users/") {
										rel = dir[strings.Index(dir, "/users/"):]
									}
									// Mark ownership on the directory and its parent (used for reload notification).
									srv.addResourceOwner(token, rel, p.owner, "file", rbacpb.SubjectType_ACCOUNT)
									rel = filepath.Dir(rel)
									if err := Utility.CopyFile(src, dst); err != nil {
										slog.Error("copy torrent file failed", "src", src, "dst", dst, "err", err)
									} else {
										srv.addResourceOwner(token, dst, p.owner, "file", rbacpb.SubjectType_ACCOUNT)
										if ev, err := srv.getEventClient(); err == nil {
											_ = ev.Publish("reload_dir_event", []byte(rel))
										}
										slog.Info("copied torrent file", "src", src, "dst", dst)
									}
								} else if err != nil {
									slog.Error("ensure destination dir failed", "dir", dir, "err", err)
								}
							}
						}
					}
				}

				// Fan out snapshot to streaming clients.
				for i := 0; i < len(getTorrentsInfoActions); {
					a := getTorrentsInfoActions[i]
					stream := a["stream"].(torrentpb.TorrentService_GetTorrentInfosServer)
					if err := stream.Send(&torrentpb.GetTorrentInfosResponse{Infos: snapshot}); err != nil {
						slog.Warn("client stream closed; removing", "err", err)
						a["exit"].(chan bool) <- true
						getTorrentsInfoActions = append(getTorrentsInfoActions[:i], getTorrentsInfoActions[i+1:]...)
						continue
					}
					i++
				}

			case <-srv.done:
				srv.torrent_client_.Close()
				for _, a := range getTorrentsInfoActions {
					a["exit"].(chan bool) <- true
				}
				slog.Info("torrent processor stopped")
				return
			}
		}
	}()
}

// downloadFile fetches a remote file (following redirects) into dest.
// It returns the absolute path to the created file.
func downloadFile(fileURL, dest string) (string, error) {
	if fileURL == "" {
		return "", errors.New("downloadFile: empty fileURL")
	}
	if dest == "" {
		return "", errors.New("downloadFile: empty destination directory")
	}
	if err := Utility.CreateDirIfNotExist(dest); err != nil {
		return "", fmt.Errorf("downloadFile: ensure dest dir: %w", err)
	}

	u, err := url.Parse(fileURL)
	if err != nil {
		return "", fmt.Errorf("downloadFile: parse url: %w", err)
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) == 0 {
		return "", fmt.Errorf("downloadFile: cannot infer filename from URL %q", fileURL)
	}
	fileName := filepath.Join(dest, segments[len(segments)-1])

	req, err := http.NewRequest(http.MethodGet, fileURL, nil)
	if err != nil {
		return "", fmt.Errorf("downloadFile: new request: %w", err)
	}
	client := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			// Preserve opaque paths for some torrent hosts.
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloadFile: http get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("downloadFile: unexpected HTTP status %s", resp.Status)
	}

	f, err := os.Create(fileName)
	if err != nil {
		return "", fmt.Errorf("downloadFile: create %s: %w", fileName, err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("downloadFile: write %s: %w", fileName, err)
	}
	slog.Info("downloaded file", "path", fileName, "bytes", n)
	return fileName, nil
}

func percent(actual, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(actual) / float64(total) * 100
}

// getTorrentInfo converts an anacrolix/torrent.Torrent into the protobuf
// structure, updating fields incrementally to avoid reallocations.
func getTorrentInfo(t *torrent.Torrent, torrentInfo *torrentpb.TorrentInfo) *torrentpb.TorrentInfo {
	var updatedAt, downloaded int64
	if torrentInfo == nil {
		torrentInfo = new(torrentpb.TorrentInfo)
		torrentInfo.Name = t.Name()
		torrentInfo.Loaded = t.Info() != nil
		if torrentInfo.Loaded {
			torrentInfo.Size = t.Length()
		}
		go func() {
			<-t.GotInfo()
			torrentInfo.Files = make([]*torrentpb.TorrentFileInfo, len(t.Files()))
		}()
	} else {
		updatedAt = torrentInfo.UpdatedAt
		downloaded = torrentInfo.Downloaded
	}

	if torrentInfo.Files != nil {
		for i, f := range t.Files() {
			path := f.Path()
			file := torrentInfo.Files[i]
			if file == nil {
				file = &torrentpb.TorrentFileInfo{Path: path}
				torrentInfo.Files[i] = file
			}
			chunks := f.State()
			file.Size = f.Length()
			file.Chunks = int64(len(chunks))
			completed := 0
			for _, p := range chunks {
				if p.Complete {
					completed++
				}
			}
			file.Completed = int64(completed)
			file.Percent = percent(file.Completed, file.Chunks)
		}

		now := time.Now()
		if updatedAt != 0 {
			dt := float32(now.Sub(time.Unix(updatedAt, 0)))
			db := float32(t.BytesCompleted() - downloaded)
			rate := db * (float32(time.Second) / dt)
			if rate >= 0 {
				torrentInfo.DownloadRate = rate
			}
		}
		torrentInfo.Downloaded = t.BytesCompleted()
		torrentInfo.UpdatedAt = now.Unix()
		torrentInfo.Percent = percent(torrentInfo.Downloaded, torrentInfo.Size)
	}
	return torrentInfo
}

// setTorrentTransfer registers a torrent transfer to be tracked by the processor.
func (srv *server) setTorrentTransfer(t *torrent.Torrent, seed bool, lnk, dest, owner string) {
	a := map[string]interface{}{
		"action":          "setTorrentTransfer",
		"torrentTransfer": &TorrentTransfer{dst: dest, lnk: lnk, tor: t, seed: seed, owner: owner},
	}
	srv.actions <- a
}

// dropTorrent removes a torrent and deletes its partial download folder.
func (srv *server) dropTorrent(name string) {
	a := map[string]interface{}{
		"action": "dropTorrent",
		"name":   name,
	}
	srv.actions <- a
}

// getTorrentsInfo registers a stream to receive periodic torrent info updates.
// It returns a channel that is closed when the stream should stop.
func (srv *server) getTorrentsInfo(stream torrentpb.TorrentService_GetTorrentInfosServer) chan bool {
	a := map[string]interface{}{
		"action": "getTorrentsInfo",
		"stream": stream,
		"exit":   make(chan bool),
	}
	srv.actions <- a
	return a["exit"].(chan bool)
}

// getTorrentLnks returns the saved torrent links as protobuf structures.
func (srv *server) getTorrentLnks() []*torrentpb.TorrentLnk {
	a := map[string]interface{}{
		"action": "getTorrentLnks",
		"lnks":   make(chan []*torrentpb.TorrentLnk),
	}
	srv.actions <- a
	return <-a["lnks"].(chan []*torrentpb.TorrentLnk)
}

// downloadTorrent creates a new torrent in the underlying anacrolix client and
// begins download (or seeding) based on the provided link. If link is a remote
// .torrent over HTTP(S), it is downloaded to dest first.
func (srv *server) downloadTorrent(link, dest string, seed bool, owner string) error {
	if link == "" {
		return errors.New("downloadTorrent: empty link")
	}
	if dest == "" {
		return errors.New("downloadTorrent: empty destination directory")
	}
	dest = srv.resolveDestPath(dest)

	var (
		t   *torrent.Torrent
		err error
	)

	if strings.HasPrefix(link, "magnet:") {
		t, err = srv.torrent_client_.AddMagnet(link)
		if err != nil {
			return fmt.Errorf("downloadTorrent: add magnet: %w", err)
		}
	} else {
		// Remote .torrent support.
		if IsUrl(link) {
			link, err = downloadFile(link, dest)
			if err != nil {
				return err
			}
		}
		if _, err = os.Stat(link); err != nil {
			return fmt.Errorf("downloadTorrent: stat %s: %w", link, err)
		}
		t, err = srv.torrent_client_.AddTorrentFromFile(link)
		if err != nil {
			return fmt.Errorf("downloadTorrent: add torrent from file: %w", err)
		}
	}

	go func() {
		<-t.GotInfo()
		t.DownloadAll()
		srv.setTorrentTransfer(t, seed, link, dest, owner)
	}()

	slog.Info("torrent started", "name", t.Name(), "dest", dest, "seed", seed, "owner", owner)
	return nil
}

// GetTorrentLnks returns all torrent links previously saved on the server.
// It is safe to call at any time and never blocks for long.
func (srv *server) GetTorrentLnks(ctx context.Context, rqst *torrentpb.GetTorrentLnksRequest) (*torrentpb.GetTorrentLnksResponse, error) {
	lnks := srv.getTorrentLnks()
	return &torrentpb.GetTorrentLnksResponse{Lnks: lnks}, nil
}

// GetTorrentInfos streams periodic snapshots of all active torrents to the client.
// The stream remains open until the client disconnects or the server shuts down.
func (srv *server) GetTorrentInfos(rqst *torrentpb.GetTorrentInfosRequest, stream torrentpb.TorrentService_GetTorrentInfosServer) error {
	<-srv.getTorrentsInfo(stream)
	return nil
}

// DownloadTorrent starts downloading (or seeding) a torrent specified by rqst.Link.
// If the link is an HTTP(S) URL pointing to a .torrent file, it is fetched first.
func (srv *server) DownloadTorrent(ctx context.Context, rqst *torrentpb.DownloadTorrentRequest) (*torrentpb.DownloadTorrentResponse, error) {
	clientID, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "DownloadTorrent: cannot resolve client identity: %v", err)
	}
	if err := srv.downloadTorrent(rqst.Link, rqst.Dest, rqst.Seed, clientID); err != nil {
		return nil, status.Errorf(codes.Internal, "DownloadTorrent: %v", err)
	}
	return &torrentpb.DownloadTorrentResponse{}, nil
}

// DropTorrent removes a torrent by name and clears partial data from disk.
func (srv *server) DropTorrent(ctx context.Context, rqst *torrentpb.DropTorrentRequest) (*torrentpb.DropTorrentResponse, error) {
	if rqst.GetName() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "DropTorrent: empty torrent name")
	}
	srv.dropTorrent(rqst.Name)
	return &torrentpb.DropTorrentResponse{}, nil
}
