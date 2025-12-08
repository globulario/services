package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/title/titlepb"
		"google.golang.org/protobuf/encoding/protojson"
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

	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, fmt.Errorf("resolve index path: %w", err)
	}
	if !Utility.Exists(resolved) {
		return nil, fmt.Errorf("open associations: no database at %s", resolved)
	}

	store, err := srv.getStore(filepath.Base(resolved), resolved)
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

	return assoc.Paths, nil
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
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}

	abs := strings.ReplaceAll(rqst.FilePath, "\\", "/")

	if err := srv.dissociateFileWithTitle(token, rqst.IndexPath, rqst.TitleId, abs); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.DissociateFileWithTitleResponse{}, nil
}

// dissociateFileWithTitle performs the actual store/index updates for a dissociation.
func (srv *server) dissociateFileWithTitle(token, indexPath, titleId, absoluteFilePath string) error {

	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return err
	}
	var uuid string

	filePath := strings.ReplaceAll(absoluteFilePath, config.GetDataDir()+"/files", "")
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	uuid = Utility.CreateFileChecksum(absoluteFilePath)
	associations, err := srv.getStore(filepath.Base(indexPath), resolved)
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
			_ = srv.deleteVideo(token, indexPath, titleId)
		} else if strings.HasSuffix(indexPath, "/search/audios") {
			_ = srv.deleteAudio(token, indexPath, titleId)
		} else if strings.HasSuffix(indexPath, "/search/titles") {
			_ = srv.deleteTitle(token, indexPath, titleId)
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

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	titles, err := srv.getFileTitles(resolved, filePath, abs)
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
	// Check if the index path exists.
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if !Utility.Exists(resolved) {
		return nil, errors.New("no database found at path " + resolved)
	}

	uuid := Utility.GenerateUUID(filePath)

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

	playlist := filepath.ToSlash(filepath.Join(absolutePath, "playlist.m3u8"))
	if Utility.Exists(playlist) {
		filePlaylist := filepath.ToSlash(filepath.Join(filePath, "playlist.m3u8"))
		if plTitles, err := srv.getFileTitles(indexPath, filePlaylist, playlist); err == nil {
			titles = append(titles, plTitles...)
		}
	}
	files, err := os.ReadDir(absolutePath)
	if err == nil {
		for _, f := range files {
			if strings.EqualFold(f.Name(), "playlist.m3u8") {
				continue
			}
			sub, err := srv.getFileTitles(indexPath, filePath+"/"+f.Name(), absolutePath+"/"+f.Name())
			if err == nil {
				titles = append(titles, sub...)
			}
		}
	}

	return titles, nil
}

// AssociateFileWithTitle associates a file/folder to a title, and persists minimal metadata.
func (srv *server) AssociateFileWithTitle(ctx context.Context, rqst *titlepb.AssociateFileWithTitleRequest) (*titlepb.AssociateFileWithTitleResponse, error) {
	rawPath := filepath.ToSlash(rqst.FilePath)
	logicalPath := srv.logicalAssociationPath(rawPath)
	abs, hasLocal := srv.resolveLocalAssociationPath(rawPath, logicalPath)
	if !hasLocal {
		abs = ""
		// Reject obviously invalid requests, but allow well-known logical prefixes
		if !strings.HasPrefix(logicalPath, "/users/") && !strings.HasPrefix(logicalPath, "/applications/") {
			return nil, status.Errorf(codes.InvalidArgument, "no file found with path %s", rqst.FilePath)
		}
	}

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if !Utility.Exists(resolved) {
		return nil, status.Errorf(codes.InvalidArgument, "no database found at path %s", resolved)
	}

	// Save lightweight metadata for recovery.
	if hasLocal && strings.HasSuffix(resolved, "/search/titles") {
		title, err := srv.getTitleById(resolved, rqst.TitleId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		if title.Poster != nil && len(title.Poster.ContentUrl) == 0 {
			title.Poster.ContentUrl = title.Poster.URL
		}
	} else if hasLocal && strings.HasSuffix(resolved, "/search/videos") {
		video, err := srv.getVideoById(resolved, rqst.TitleId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
		if video == nil {
			return nil, status.Errorf(codes.NotFound, "video %q not found", rqst.TitleId)
		}
		if video.Poster != nil && len(video.Poster.ContentUrl) == 0 {
			video.Poster.ContentUrl = video.Poster.URL
		}
	}

	filePath := logicalPath
	uuid := Utility.GenerateUUID(filePath)
	store, err := srv.getStore(filepath.Base(rqst.IndexPath), resolved)
	if err != nil {
		return nil, err
	}

	assoc, err := srv.loadAssociation(store, uuid, filePath, abs)
	if err != nil {
		logger.Warn("AssociateFileWithTitle: loadAssociation failed", "err", err, "uuid", uuid, "path", filePath)
	}
	if assoc == nil {
		assoc = &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	} else if assoc.ID == "" {
		assoc.ID = uuid
	}
	if !Utility.Contains(assoc.Paths, filePath) {
		assoc.Paths = append(assoc.Paths, filePath)
	}
	if !Utility.Contains(assoc.Titles, rqst.TitleId) {
		assoc.Titles = append(assoc.Titles, rqst.TitleId)
	}
	if err := srv.persistAssociation(store, assoc, uuid, filePath, abs); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	// Reverse index for the title.
	var data []byte
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
	raw, _ := json.Marshal(assoc)
	if err := store.SetItem(rqst.TitleId, raw); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	dir := filepath.Dir(filePath)
	_ = srv.publish("reload_dir_event", []byte(dir))

	return &titlepb.AssociateFileWithTitleResponse{}, nil
}

// GetFileVideos returns the list of Videos associated with a file/folder.
func (srv *server) GetFileVideos(ctx context.Context, rqst *titlepb.GetFileVideosRequest) (*titlepb.GetFileVideosResponse, error) {
	filePath := srv.logicalAssociationPath(filepath.ToSlash(rqst.FilePath))
	uuid := normalizeAssociationPath(filePath)
	if uuid == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid file path %q", rqst.FilePath)
	}
	abs, hasLocal := srv.resolveLocalAssociationPath(filepath.ToSlash(rqst.FilePath), filePath)

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, err
	}
	store, err := srv.getStore(filepath.Base(resolved), resolved)
	if err != nil {
		return nil, err
	}

	data, err := store.GetItem(uuid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	assoc := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}

	if err := json.Unmarshal(data, assoc); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	videos := make([]*titlepb.Video, 0, len(assoc.Titles))
	for _, t := range assoc.Titles {
		if hasLocal {
			if recovery, err := srv.buildVideoFromSavedMetadata(abs, t); err == nil && recovery != nil && recovery.ID != "" {
				logger.Debug("GetFileVideos: recovered metadata for video", "path", abs, "videoID", recovery.ID)
				videos = append(videos, recovery)
				continue
			}
		}
		if v, err := srv.getVideoById(rqst.IndexPath, t); err == nil && v != nil && v.ID != "" {
			videos = append(videos, v)
			continue
		}
		videos = append(videos, &titlepb.Video{ID: t})
	}

	if len(videos) == 0 {
		return nil, errors.New("no videos associations found for file " + rqst.FilePath)
	}
	return &titlepb.GetFileVideosResponse{Videos: &titlepb.Videos{Videos: videos}}, nil
}

func (srv *server) buildVideoFromSavedMetadata(abs, videoID string) (*titlepb.Video, error) {
	if abs == "" {
		return nil, fmt.Errorf("no local path")
	}
	if video, err := loadVideoMetadataCache(abs); err == nil {
		if video.ID == "" {
			video.ID = videoID
		}
		return video, nil
	}
	if video, err := readVideoFromMetadataComment(abs); err == nil {
		if video.ID == "" {
			video.ID = videoID
		}
		return video, nil
	}
	return nil, fmt.Errorf("no saved metadata for %s", abs)
}

func readVideoFromMetadataComment(abs string) (*titlepb.Video, error) {
	meta, err := Utility.ReadMetadata(abs)
	if err != nil {
		return nil, err
	}
	format, ok := meta["format"].(map[string]any)
	if !ok {
		return nil, errors.New("metadata missing format section")
	}
	tags, ok := format["tags"].(map[string]any)
	if !ok {
		return nil, errors.New("metadata missing tags")
	}
	rawComment, _ := tags["comment"].(string)
	if rawComment == "" {
		return nil, errors.New("metadata comment empty")
	}
	clean := strings.TrimSpace(rawComment)
	var jsonBytes []byte
	if decoded, derr := base64.StdEncoding.DecodeString(clean); derr == nil {
		jsonBytes = decoded
	} else {
		jsonBytes = []byte(clean)
	}
	if !strings.Contains(string(jsonBytes), "{") {
		return nil, errors.New("metadata comment does not contain JSON")
	}
	jsonBytes = normalizeEmbeddedProtoJSON(jsonBytes)
	video := new(titlepb.Video)
	if err := protojson.Unmarshal(jsonBytes, video); err != nil {
		return nil, err
	}
	return video, nil
}

func normalizeEmbeddedProtoJSON(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	replacer := strings.NewReplacer(
		`"PublisherId":`, `"PublisherID":`,
	)
	return []byte(replacer.Replace(string(data)))
}

func (srv *server) loadAssociation(store storage_store.Store, uuid, filePath, abs string) (*fileTileAssociation, error) {
	if store == nil {
		return nil, errors.New("association store unavailable")
	}
	lookups := canonicalPaths(abs, filePath)
	if uuid != "" {
		lookups = append([]string{uuid}, lookups...)
	}
	seen := map[string]struct{}{}
	for _, key := range lookups {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if assoc, err := srv.readAssociation(store, key); err == nil {
			if assoc.ID == "" {
				assoc.ID = uuid
			}
			if uuid != "" {
				if raw, err := json.Marshal(assoc); err == nil {
					_ = store.SetItem(uuid, raw)
				}
			}
			return assoc, nil
		}
	}

	normalizedFilePath := normalizeAssociationPath(filePath)
	normalizedAbs := normalizeAssociationPath(abs)
	fallbacks := canonicalPaths(normalizedAbs, normalizedFilePath)
	if assoc, err := srv.searchAssociationByPath(store, normalizedFilePath); err == nil {
		return assoc, nil
	}
	if assoc, err := srv.searchAssociationByPath(store, normalizedAbs); err == nil {
		return assoc, nil
	}
	return nil, fmt.Errorf("no association for keys: %s", strings.Join(fallbacks, ","))
}

func (srv *server) readAssociation(store storage_store.Store, key string) (*fileTileAssociation, error) {
	data, err := store.GetItem(key)
	if err != nil {
		return nil, err
	}
	assoc := &fileTileAssociation{ID: key, Titles: []string{}, Paths: []string{}}
	if err := json.Unmarshal(data, assoc); err != nil {
		return nil, err
	}
	return assoc, nil
}

func normalizeAssociationPath(path string) string {
	if path == "" {
		return ""
	}
	cleaned := filepath.ToSlash(filepath.Clean(path))
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func (srv *server) logicalAssociationPath(path string) string {
	cleaned := normalizeAssociationPath(path)
	if cleaned == "" {
		return "/"
	}
	dataPrefix := filepath.ToSlash(config.GetDataDir() + "/files")
	configPrefix := filepath.ToSlash(config.GetConfigDir() + "/files")
	switch {
	case strings.HasPrefix(cleaned, dataPrefix):
		cleaned = strings.TrimPrefix(cleaned, dataPrefix)
	case strings.HasPrefix(cleaned, configPrefix):
		cleaned = strings.TrimPrefix(cleaned, configPrefix)
	}
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" {
		return "/"
	}
	return "/" + cleaned
}

func (srv *server) resolveLocalAssociationPath(raw, logical string) (string, bool) {
	candidates := []string{}
	appendCandidate := func(p string) {
		if p == "" {
			return
		}
		candidates = append(candidates, filepath.ToSlash(p))
	}
	appendCandidate(raw)
	if strings.HasPrefix(raw, "/") {
		appendCandidate(raw)
	} else {
		appendCandidate("/" + strings.TrimPrefix(raw, "/"))
	}
	if logical != "" {
		appendCandidate(logical)
	}
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalized := normalizeAssociationPath(candidate)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		if Utility.Exists(normalized) {
			return normalized, true
		}
	}
	return "", false
}

func canonicalPaths(abs, filePath string) []string {
	var (
		list        []string
		appendClean = func(p string) {
			if normalized := normalizeAssociationPath(p); normalized != "" {
				list = append(list, normalized)
			}
		}
	)

	appendClean(filePath)
	if abs != "" {
		appendClean(abs)
		if strings.HasPrefix(abs, "/") {
			appendClean(strings.TrimPrefix(abs, "/"))
		}
		if trimmed := strings.TrimPrefix(abs, config.GetDataDir()+"/files"); trimmed != abs {
			appendClean(trimmed)
			appendClean(strings.TrimPrefix(trimmed, "/"))
		}
	}
	return list
}

func (srv *server) persistAssociation(store storage_store.Store, assoc *fileTileAssociation, uuid, filePath, abs string) error {
	if store == nil || assoc == nil {
		return errors.New("invalid association state")
	}
	raw, err := json.Marshal(assoc)
	if err != nil {
		return err
	}
	seen := map[string]struct{}{}
	keys := append([]string{}, canonicalPaths(abs, filePath)...)
	if uuid != "" && !containsKey(keys, uuid) {
		keys = append([]string{uuid}, keys...)
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := store.SetItem(key, raw); err != nil {
			return err
		}
	}
	return nil
}

func containsKey(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

func (srv *server) searchAssociationByPath(store storage_store.Store, path string) (*fileTileAssociation, error) {
	if store == nil || path == "" {
		return nil, fmt.Errorf("invalid search path")
	}
	target := normalizeAssociationPath(path)
	if target == "" {
		return nil, fmt.Errorf("invalid search path")
	}
	keys, err := store.GetAllKeys()
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		assoc, err := srv.readAssociation(store, key)
		if err != nil {
			continue
		}
		for _, p := range assoc.Paths {
			if normalizeAssociationPath(p) == target {
				return assoc, nil
			}
		}
	}
	return nil, fmt.Errorf("no association with path %s", path)
}

// GetFileAudios returns the list of Audio documents associated with a file or folder.
// The file/folder key is computed as:
//   - directory: generateUUID(<relative path under dataDir>/files)
//   - file:      checksum of the absolute file path
//
// It then resolves those associated IDs to full Audio records.
func (srv *server) GetFileAudios(ctx context.Context, rqst *titlepb.GetFileAudiosRequest) (*titlepb.GetFileAudiosResponse, error) {
	if rqst == nil || rqst.FilePath == "" || rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path and index path are required")
	}

	// Keep relative path (if caller passed a /users or /applications path)
	relPath := strings.ReplaceAll(rqst.FilePath, config.GetDataDir()+"/files", "")
	relPath = strings.ReplaceAll(relPath, "\\", "/")

	uuid := Utility.GenerateUUID(relPath)

	// Load associations
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		logger.Error("resolve index path failed", "indexPath", rqst.IndexPath, "err", err)
		return nil, status.Errorf(codes.Internal, "resolve index path: %v", err)
	}

	store, err := srv.getStore(filepath.Base(rqst.IndexPath), resolved)
	if err != nil {
		logger.Error("open associations store failed", "indexPath", resolved, "err", err)
		return nil, status.Errorf(codes.Internal, "open associations: %v", err)
	}

	data, err := store.GetItem(uuid)
	assoc := &fileTileAssociation{ID: uuid, Titles: []string{}, Paths: []string{}}
	if err == nil && len(data) > 0 {
		if uerr := json.Unmarshal(data, assoc); uerr != nil {
			logger.Error("decode association failed", "key", uuid, "err", uerr)
			return nil, status.Errorf(codes.Internal, "decode association: %v", uerr)
		}
	}

	// Resolve audio IDs → audio objects.
	audios := make([]*titlepb.Audio, 0, len(assoc.Titles))
	for _, id := range assoc.Titles {
		audio, aerr := srv.getAudioById(resolved, id)
		if aerr == nil && audio != nil {
			audios = append(audios, audio)
		}
	}

	return &titlepb.GetFileAudiosResponse{
		Audios: &titlepb.Audios{Audios: audios},
	}, nil
}
