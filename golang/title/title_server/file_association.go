package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fileTileAssociation describes the relationship between a file/dir and titles.
type fileTileAssociation struct {
	ID     string
	Titles []string
	Paths  []string
}

// getTitleFiles returns all file paths associated with a given title ID.
// It looks up the reverse entry keyed by the title ID in the associations store,
// trims any stale paths that no longer exist on disk, then persists the cleaned
// list back to the store when needed.
func (srv *server) getTitleFiles(indexPath, titleId string) ([]string, error) {
	if !Utility.Exists(indexPath) {
		return nil, fmt.Errorf("open associations: no database at %s", indexPath)
	}

	store, err := srv.getStore(filepath.Base(indexPath), indexPath)
	if err != nil {
		return nil, fmt.Errorf("open associations store: %w", err)
	}

	data, err := store.GetItem(titleId)
	if err != nil {
		return nil, fmt.Errorf("read associations for title %s: %w", titleId, err)
	}

	assoc := &fileTileAssociation{ID: "", Titles: []string{}, Paths: []string{}}
	if err := json.Unmarshal(data, assoc); err != nil {
		return nil, fmt.Errorf("decode associations for title %s: %w", titleId, err)
	}

	// Prune non-existent paths; keep relative paths that resolve under dataDir or root.
	pruned := make([]string, 0, len(assoc.Paths))
	for _, p := range assoc.Paths {
		switch {
		case Utility.Exists(p):
			pruned = append(pruned, p)
		case Utility.Exists(config.GetDataDir() + "/files" + p):
			pruned = append(pruned, p)
		case Utility.Exists("/" + p):
			pruned = append(pruned, p)
		default:
			logger.Debug("dropping stale association path", "titleID", titleId, "path", p)
		}
	}

	// Persist cleanup or delete empty associations.
	if len(pruned) != len(assoc.Paths) {
		assoc.Paths = pruned
		if len(assoc.Paths) == 0 {
			_ = store.RemoveItem(titleId)
			if assoc.ID != "" {
				_ = store.RemoveItem(assoc.ID)
			}
		} else {
			if raw, e := json.Marshal(assoc); e == nil {
				_ = store.SetItem(assoc.ID, raw)
				_ = store.SetItem(titleId, raw)
			}
		}
	}

	logger.Debug("getTitleFiles", "titleID", titleId, "paths", pruned)
	return pruned, nil
}

// GetTitleFiles is the public RPC that returns all file paths associated with a title.
func (srv *server) GetTitleFiles(ctx context.Context, rqst *titlepb.GetTitleFilesRequest) (*titlepb.GetTitleFilesResponse, error) {
	if rqst == nil || rqst.IndexPath == "" || rqst.TitleId == "" {
		return nil, status.Error(codes.InvalidArgument, "index path and title id are required")
	}

	paths, err := srv.getTitleFiles(rqst.IndexPath, rqst.TitleId)
	if err != nil {
		logger.Error("GetTitleFiles failed", "indexPath", rqst.IndexPath, "titleID", rqst.TitleId, "err", err)
		return nil, status.Errorf(codes.Internal, "get title files: %v", err)
	}

	return &titlepb.GetTitleFilesResponse{FilePaths: paths}, nil
}

// DissociateFileWithTitle removes the association between a file and a title.
func (srv *server) DissociateFileWithTitle(ctx context.Context, rqst *titlepb.DissociateFileWithTitleRequest) (*titlepb.DissociateFileWithTitleResponse, error) {
	abs := strings.ReplaceAll(rqst.FilePath, "\\", "/")
	if !Utility.Exists(abs) {
		if strings.HasPrefix(abs, "/users/") || strings.HasPrefix(abs, "/applications/") {
			abs = config.GetDataDir() + "/files" + abs
		} else if Utility.Exists("/" + abs) {
			abs = "/" + abs
		}
	}
	if !Utility.Exists(abs) {
		return nil, status.Errorf(codes.InvalidArgument, "no file found with path %s", abs)
	}
	if err := srv.dissociateFileWithTitle(rqst.IndexPath, rqst.TitleId, abs); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.DissociateFileWithTitleResponse{}, nil
}

// dissociateFileWithTitle performs the actual store/index updates for a dissociation.
func (srv *server) dissociateFileWithTitle(indexPath, titleId, absoluteFilePath string) error {
	if !Utility.Exists(indexPath) {
		return errors.New("no database found at path " + indexPath)
	}

	var uuid string
	info, err := os.Stat(absoluteFilePath)
	if err != nil {
		return err
	}
	filePath := strings.ReplaceAll(absoluteFilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if info.IsDir() {
		uuid = Utility.GenerateUUID(filePath)
	} else {
		uuid = Utility.CreateFileChecksum(absoluteFilePath)
	}

	associations, err := srv.getStore(filepath.Base(indexPath), indexPath)
	if err != nil {
		return err
	}

	fileData, err := associations.GetItem(uuid)
	fileAssoc := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		if err := json.Unmarshal(fileData, fileAssoc); err != nil {
			return err
		}
	}
	fileAssoc.Paths = Utility.RemoveString(fileAssoc.Paths, filePath)
	fileAssoc.Titles = Utility.RemoveString(fileAssoc.Titles, titleId)

	if len(fileAssoc.Paths) == 0 || len(fileAssoc.Titles) == 0 {
		associations.RemoveItem(uuid)
	} else {
		raw, _ := json.Marshal(fileAssoc)
		if err := associations.SetItem(uuid, raw); err != nil {
			return err
		}
	}

	titleData, err := associations.GetItem(titleId)
	titleAssoc := &fileTileAssociation{ID: titleId, Titles: []string{}, Paths: []string{}}
	if err == nil {
		if err := json.Unmarshal(titleData, titleAssoc); err != nil {
			return err
		}
	}
	titleAssoc.Paths = Utility.RemoveString(titleAssoc.Paths, filePath)

	if len(titleAssoc.Paths) == 0 {
		associations.RemoveItem(titleId)
		if strings.HasSuffix(indexPath, "/search/videos") {
			_ = srv.deleteVideo(indexPath, titleId)
		} else if strings.HasSuffix(indexPath, "/search/audios") {
			_ = srv.deleteAudio(indexPath, titleId)
		} else if strings.HasSuffix(indexPath, "/search/titles") {
			_ = srv.deleteTitle(indexPath, titleId)
		}
	} else {
		raw, _ := json.Marshal(titleAssoc)
		if err := associations.SetItem(titleId, raw); err != nil {
			return err
		}
	}

	dir := filepath.Dir(strings.ReplaceAll(filePath, config.GetDataDir()+"/files", ""))
	_ = srv.publish("reload_dir_event", []byte(dir))
	return nil
}

// GetFileTitles returns the list of Titles associated with a given file or folder.
func (srv *server) GetFileTitles(ctx context.Context, rqst *titlepb.GetFileTitlesRequest) (*titlepb.GetFileTitlesResponse, error) {
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	abs := strings.ReplaceAll(rqst.FilePath, "\\", "/")

	if !Utility.Exists(abs) {
		if strings.HasPrefix(abs, "/users/") || strings.HasPrefix(abs, "/applications/") {
			abs = config.GetDataDir() + "/files" + abs
		} else if Utility.Exists("/" + abs) {
			abs = "/" + abs
		}
	}
	if !Utility.Exists(abs) {
		return nil, status.Errorf(codes.InvalidArgument, "no file found with path %s", abs)
	}

	titles, err := srv.getFileTitles(rqst.IndexPath, filePath, abs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if len(titles) == 0 {
		return nil, errors.New("no titles associations found for file " + rqst.FilePath)
	}
	return &titlepb.GetFileTitlesResponse{Titles: &titlepb.Titles{Titles: titles}}, nil
}

// getFileTitles recursively collects Titles associated with a file/folder.
func (srv *server) getFileTitles(indexPath, filePath, absolutePath string) ([]*titlepb.Title, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}
	var uuid string
	info, err := os.Stat(absolutePath)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		uuid = Utility.GenerateUUID(filePath)
	} else {
		uuid = Utility.CreateFileChecksum(absolutePath)
	}
	associations, err := srv.getStore(filepath.Base(indexPath), indexPath)
	if err != nil {
		return nil, err
	}
	data, err := associations.GetItem(uuid)
	assoc := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		if err := json.Unmarshal(data, assoc); err != nil {
			return nil, err
		}
	}

	titles := make([]*titlepb.Title, 0, len(assoc.Titles))
	for _, t := range assoc.Titles {
		if title, err := srv.getTitleById(indexPath, t); err == nil {
			titles = append(titles, title)
		}
	}

	if info.IsDir() && !Utility.Exists(absolutePath+"/playlist.m3u8") {
		files, err := os.ReadDir(absolutePath)
		if err == nil {
			for _, f := range files {
				sub, err := srv.getFileTitles(indexPath, filePath+"/"+f.Name(), absolutePath+"/"+f.Name())
				if err == nil {
					titles = append(titles, sub...)
				}
			}
		}
	}
	return titles, nil
}

// AssociateFileWithTitle associates a file/folder to a title, and persists minimal metadata.
func (srv *server) AssociateFileWithTitle(ctx context.Context, rqst *titlepb.AssociateFileWithTitleRequest) (*titlepb.AssociateFileWithTitleResponse, error) {
	abs := strings.ReplaceAll(rqst.FilePath, "\\", "/")
	if !Utility.Exists(abs) {
		if strings.HasPrefix(abs, "/users/") || strings.HasPrefix(abs, "/applications/") {
			abs = config.GetDataDir() + "/files" + abs
		} else if Utility.Exists("/" + abs) {
			abs = "/" + abs
		}
		if !Utility.Exists(abs) {
			return nil, status.Errorf(codes.InvalidArgument, "no file found with path %s", abs)
		}
	}

	// Save lightweight metadata for recovery.
	if strings.HasSuffix(rqst.IndexPath, "/search/titles") {
		title, err := srv.getTitleById(rqst.IndexPath, rqst.TitleId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		if title.Poster != nil && len(title.Poster.ContentUrl) == 0 {
			title.Poster.ContentUrl = title.Poster.URL
		}
		_ = srv.saveTitleMetadata(abs, rqst.IndexPath, title)
	} else if strings.HasSuffix(rqst.IndexPath, "/search/videos") {
		video, err := srv.getVideoById(rqst.IndexPath, rqst.TitleId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		if video == nil {
			return nil, status.Errorf(codes.NotFound, "video %q not found", rqst.TitleId)
		}
		if video.Poster != nil && len(video.Poster.ContentUrl) == 0 {
			video.Poster.ContentUrl = video.Poster.URL
		}
		_ = srv.saveVideoMetadata(abs, rqst.IndexPath, video)
	}

	var uuid string
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	info, _ := os.Stat(abs)
	if info.IsDir() {
		uuid = Utility.GenerateUUID(filePath)
	} else {
		uuid = Utility.CreateFileChecksum(abs)
	}

	store, err := srv.getStore(filepath.Base(rqst.IndexPath), rqst.IndexPath)
	if err != nil {
		return nil, err
	}

	data, err := store.GetItem(uuid)
	assoc := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		if err := json.Unmarshal(data, assoc); err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	if !Utility.Contains(assoc.Paths, filePath) {
		assoc.Paths = append(assoc.Paths, filePath)
	}
	if !Utility.Contains(assoc.Titles, rqst.TitleId) {
		assoc.Titles = append(assoc.Titles, rqst.TitleId)
	}
	raw, _ := json.Marshal(assoc)
	if err := store.SetItem(uuid, raw); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	// Reverse index for the title.
	data, err = store.GetItem(rqst.TitleId)
	assoc = &fileTileAssociation{ID: rqst.TitleId, Titles: []string{}, Paths: []string{}}
	if err == nil {
		if err := json.Unmarshal(data, assoc); err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	if !Utility.Contains(assoc.Paths, filePath) {
		assoc.Paths = append(assoc.Paths, filePath)
	}
	if !Utility.Contains(assoc.Titles, rqst.TitleId) {
		assoc.Titles = append(assoc.Titles, rqst.TitleId)
	}
	raw, _ = json.Marshal(assoc)
	if err := store.SetItem(rqst.TitleId, raw); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	dir := filepath.Dir(strings.ReplaceAll(filePath, config.GetDataDir()+"/files", ""))
	_ = srv.publish("reload_dir_event", []byte(dir))
	return &titlepb.AssociateFileWithTitleResponse{}, nil
}

// GetFileVideos returns the list of Videos associated with a file/folder.
func (srv *server) GetFileVideos(ctx context.Context, rqst *titlepb.GetFileVideosRequest) (*titlepb.GetFileVideosResponse, error) {
	filePath := strings.ReplaceAll(rqst.FilePath, config.GetConfigDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	abs := strings.ReplaceAll(rqst.FilePath, "\\", "/")

	if !Utility.Exists(abs) {
		if strings.HasPrefix(abs, "/users/") || strings.HasPrefix(abs, "/applications/") {
			abs = config.GetDataDir() + "/files" + abs
		} else if Utility.Exists("/" + abs) {
			abs = "/" + abs
		}
	}
	if !Utility.Exists(abs) {
		return nil, status.Errorf(codes.InvalidArgument, "no file found with path %s", abs)
	}

	var uuid string
	info, err := os.Stat(abs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	if info.IsDir() {
		uuid = Utility.GenerateUUID(filePath)
	} else {
		uuid = Utility.CreateFileChecksum(abs)
	}

	store, err := srv.getStore(filepath.Base(rqst.IndexPath), rqst.IndexPath)
	if err != nil {
		return nil, err
	}

	data, err := store.GetItem(uuid)
	assoc := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil {
		if err := json.Unmarshal(data, assoc); err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}

	videos := make([]*titlepb.Video, 0, len(assoc.Titles))
	for _, t := range assoc.Titles {
		if v, err := srv.getVideoById(rqst.IndexPath, t); err == nil && v != nil && v.ID != "" {
			videos = append(videos, v)
		}
	}

	if len(videos) == 0 {
		return nil, errors.New("no videos associations found for file " + rqst.FilePath)
	}
	return &titlepb.GetFileVideosResponse{Videos: &titlepb.Videos{Videos: videos}}, nil
}

// GetFileAudios returns the list of Audio documents associated with a file or folder.
// The file/folder key is computed as:
//   - directory: generateUUID(<relative path under dataDir>/files)
//   - file:      checksum of the absolute file path
// It then resolves those associated IDs to full Audio records.
func (srv *server) GetFileAudios(ctx context.Context, rqst *titlepb.GetFileAudiosRequest) (*titlepb.GetFileAudiosResponse, error) {
	if rqst == nil || rqst.FilePath == "" || rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path and index path are required")
	}

	// Keep relative path (if caller passed a /users or /applications path)
	relPath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	// Normalize absolute path (try resolving under dataDir if needed)
	abs := strings.ReplaceAll(rqst.FilePath, "\\", "/")
	if !Utility.Exists(abs) {
		if strings.HasPrefix(abs, "/users/") || strings.HasPrefix(abs, "/applications/") {
			abs = config.GetDataDir() + "/files" + abs
		} else if Utility.Exists("/" + abs) {
			abs = "/" + abs
		}
	}
	if !Utility.Exists(abs) {
		logger.Error("file not found", "path", rqst.FilePath, "resolved", abs)
		return nil, status.Errorf(codes.NotFound, "no file found at %s", rqst.FilePath)
	}

	// Compute the association key for the file/folder.
	var key string
	info, err := os.Stat(abs)
	if err != nil {
		logger.Error("stat failed", "path", abs, "err", err)
		return nil, status.Errorf(codes.Internal, "stat %s: %v", abs, err)
	}
	if info.IsDir() {
		key = Utility.GenerateUUID(relPath)
	} else {
		key = Utility.CreateFileChecksum(abs)
	}

	// Load associations
	store, err := srv.getStore(filepath.Base(rqst.IndexPath), rqst.IndexPath)
	if err != nil {
		logger.Error("open associations store failed", "indexPath", rqst.IndexPath, "err", err)
		return nil, status.Errorf(codes.Internal, "open associations: %v", err)
	}

	data, err := store.GetItem(key)
	assoc := &fileTileAssociation{ID: key, Titles: []string{}, Paths: []string{}}
	if err == nil && len(data) > 0 {
		if uerr := json.Unmarshal(data, assoc); uerr != nil {
			logger.Error("decode association failed", "key", key, "err", uerr)
			return nil, status.Errorf(codes.Internal, "decode association: %v", uerr)
		}
	}

	// Resolve audio IDs → audio objects.
	audios := make([]*titlepb.Audio, 0, len(assoc.Titles))
	for _, id := range assoc.Titles {
		audio, aerr := srv.getAudioById(rqst.IndexPath, id)
		if aerr == nil && audio != nil {
			audios = append(audios, audio)
		}
	}

	return &titlepb.GetFileAudiosResponse{
		Audios: &titlepb.Audios{Audios: audios},
	}, nil
}
