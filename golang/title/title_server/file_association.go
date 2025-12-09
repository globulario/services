package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/globulario/services/golang/title/titlepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fileTileAssociation describes the relationship between a file/dir and titles.
type fileTileAssociation struct {
	ID     string   `json:"id"`
	Titles []string `json:"titles"`
	Paths  []string `json:"paths"`
}

// --- small helpers -----------------------------------------------------------

func removeString(list []string, value string) []string {
	if len(list) == 0 {
		return list
	}
	out := list[:0]
	for _, v := range list {
		if v != value {
			out = append(out, v)
		}
	}
	return out
}

func containsString(list []string, value string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// --- Title -> Files ----------------------------------------------------------

// getTitleFiles returns all file paths associated with a given title ID.
func (srv *server) getTitleFiles(indexPath, titleId string) ([]string, error) {
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, fmt.Errorf("resolve index path: %w", err)
	}

	store, err := srv.getStore(filepath.Base(resolved), resolved)
	if err != nil {
		return nil, fmt.Errorf("open associations store: %w", err)
	}

	data, err := store.GetItem(titleId)
	if err != nil {
		return nil, fmt.Errorf("read associations for title %s: %w", titleId, err)
	}

	assoc := &fileTileAssociation{ID: titleId}
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

// --- File <-> Title association ----------------------------------------------

// internal helper used by DeleteTitle / DeleteAudio and by the RPC below.
func (srv *server) dissociateFileWithTitle(_token, indexPath, titleId, filePath string) error {
	if indexPath == "" || titleId == "" || filePath == "" {
		return fmt.Errorf("indexPath, titleId, and filePath are required")
	}

	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return fmt.Errorf("resolve index path: %w", err)
	}

	store, err := srv.getStore(filepath.Base(resolved), resolved)
	if err != nil {
		return fmt.Errorf("open associations store: %w", err)
	}

	fileKey := filePath

	// Forward: fileKey -> titles
	fileAssoc := &fileTileAssociation{ID: fileKey}
	if data, err := store.GetItem(fileKey); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, fileAssoc)
	}

	fileAssoc.Titles = removeString(fileAssoc.Titles, titleId)
	fileAssoc.Paths = removeString(fileAssoc.Paths, fileKey)

	if len(fileAssoc.Titles) == 0 && len(fileAssoc.Paths) == 0 {
		_ = store.RemoveItem(fileKey)
	} else if raw, err := json.Marshal(fileAssoc); err == nil {
		_ = store.SetItem(fileKey, raw)
	}

	// Reverse: titleId -> file paths
	titleAssoc := &fileTileAssociation{ID: titleId}
	if data, err := store.GetItem(titleId); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, titleAssoc)
	}

	titleAssoc.Paths = removeString(titleAssoc.Paths, fileKey)
	if len(titleAssoc.Paths) == 0 {
		_ = store.RemoveItem(titleId)
	} else if raw, err := json.Marshal(titleAssoc); err == nil {
		_ = store.SetItem(titleId, raw)
	}

	dir := filepath.Dir(fileKey)
	_ = srv.publish("reload_dir_event", []byte(dir))

	return nil
}

// DissociateFileWithTitle RPC – now just delegates to the helper above.
func (srv *server) DissociateFileWithTitle(ctx context.Context, rqst *titlepb.DissociateFileWithTitleRequest) (*titlepb.DissociateFileWithTitleResponse, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if rqst.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path is required")
	}
	if rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "index path is required")
	}
	if rqst.TitleId == "" {
		return nil, status.Error(codes.InvalidArgument, "title id is required")
	}

	if err := srv.dissociateFileWithTitle("", rqst.IndexPath, rqst.TitleId, rqst.FilePath); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.DissociateFileWithTitleResponse{}, nil
}

// AssociateFileWithTitle associates a **file path** (as-is) with a title ID.
func (srv *server) AssociateFileWithTitle(ctx context.Context, rqst *titlepb.AssociateFileWithTitleRequest) (*titlepb.AssociateFileWithTitleResponse, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if rqst.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path is required")
	}
	if rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "index path is required")
	}
	if rqst.TitleId == "" {
		return nil, status.Error(codes.InvalidArgument, "title id is required")
	}

	fileKey := rqst.FilePath

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	store, err := srv.getStore(filepath.Base(rqst.IndexPath), resolved)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open associations: %v", err)
	}

	// Forward: file -> titles
	fileAssoc := &fileTileAssociation{ID: fileKey}
	if data, err := store.GetItem(fileKey); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, fileAssoc)
	}
	if !containsString(fileAssoc.Paths, fileKey) {
		fileAssoc.Paths = append(fileAssoc.Paths, fileKey)
	}
	if !containsString(fileAssoc.Titles, rqst.TitleId) {
		fileAssoc.Titles = append(fileAssoc.Titles, rqst.TitleId)
	}
	if raw, err := json.Marshal(fileAssoc); err == nil {
		if err := store.SetItem(fileKey, raw); err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	} else {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	// Reverse: title -> files
	titleAssoc := &fileTileAssociation{ID: rqst.TitleId}
	if data, err := store.GetItem(rqst.TitleId); err == nil && len(data) > 0 {
		_ = json.Unmarshal(data, titleAssoc)
	}
	if !containsString(titleAssoc.Paths, fileKey) {
		titleAssoc.Paths = append(titleAssoc.Paths, fileKey)
	}
	if raw, err := json.Marshal(titleAssoc); err == nil {
		if err := store.SetItem(rqst.TitleId, raw); err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	} else {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	dir := filepath.Dir(fileKey)
	_ = srv.publish("reload_dir_event", []byte(dir))

	return &titlepb.AssociateFileWithTitleResponse{}, nil
}

// --- File -> Titles / Videos / Audios ----------------------------------------

// GetFileTitles returns the list of Titles associated with a given file path.
func (srv *server) GetFileTitles(ctx context.Context, rqst *titlepb.GetFileTitlesRequest) (*titlepb.GetFileTitlesResponse, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if rqst.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path is required")
	}
	if rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "index path is required")
	}

	fileKey := rqst.FilePath

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	store, err := srv.getStore(filepath.Base(rqst.IndexPath), resolved)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open associations: %v", err)
	}

	data, err := store.GetItem(fileKey)
	if err != nil {
		return nil, errors.New("no titles associations found for file " + rqst.FilePath)
	}

	assoc := &fileTileAssociation{ID: fileKey}
	if err := json.Unmarshal(data, assoc); err != nil {
		return nil, status.Errorf(codes.Internal, "decode association: %v", err)
	}

	if len(assoc.Titles) == 0 {
		return nil, errors.New("no titles associations found for file " + rqst.FilePath)
	}

	titles := make([]*titlepb.Title, 0, len(assoc.Titles))
	for _, id := range assoc.Titles {
		if title, err := srv.getTitleById(rqst.IndexPath, id); err == nil && title != nil {
			titles = append(titles, title)
		} else {
			// Fallback stub if index lookup fails.
			titles = append(titles, &titlepb.Title{ID: id})
		}
	}

	return &titlepb.GetFileTitlesResponse{Titles: &titlepb.Titles{Titles: titles}}, nil
}

// GetFileVideos returns the list of Videos associated with a file path.
func (srv *server) GetFileVideos(ctx context.Context, rqst *titlepb.GetFileVideosRequest) (*titlepb.GetFileVideosResponse, error) {
	if rqst == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if rqst.FilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path is required")
	}
	if rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "index path is required")
	}

	fileKey := rqst.FilePath

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	store, err := srv.getStore(filepath.Base(resolved), resolved)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open associations: %v", err)
	}

	data, err := store.GetItem(fileKey)
	if err != nil {
		return nil, errors.New("no videos associations found for file " + rqst.FilePath)
	}

	assoc := &fileTileAssociation{ID: fileKey}
	if err := json.Unmarshal(data, assoc); err != nil {
		return nil, status.Errorf(codes.Internal, "decode association: %v", err)
	}

	videos := make([]*titlepb.Video, 0, len(assoc.Titles))
	for _, id := range assoc.Titles {
		if v, err := srv.getVideoById(rqst.IndexPath, id); err == nil && v != nil && v.ID != "" {
			videos = append(videos, v)
		} else {
			// Fallback stub if index lookup fails.
			videos = append(videos, &titlepb.Video{ID: id})
		}
	}

	if len(videos) == 0 {
		return nil, errors.New("no videos associations found for file " + rqst.FilePath)
	}
	return &titlepb.GetFileVideosResponse{Videos: &titlepb.Videos{Videos: videos}}, nil
}

// GetFileAudios returns the list of Audio documents associated with a file path.
func (srv *server) GetFileAudios(ctx context.Context, rqst *titlepb.GetFileAudiosRequest) (*titlepb.GetFileAudiosResponse, error) {
	if rqst == nil || rqst.FilePath == "" || rqst.IndexPath == "" {
		return nil, status.Error(codes.InvalidArgument, "file path and index path are required")
	}

	fileKey := rqst.FilePath

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

	data, err := store.GetItem(fileKey)
	assoc := &fileTileAssociation{ID: fileKey}
	if err == nil && len(data) > 0 {
		if uerr := json.Unmarshal(data, assoc); uerr != nil {
			logger.Error("decode association failed", "key", fileKey, "err", uerr)
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
