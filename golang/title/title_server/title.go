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

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// saveVideoMetadata persists minimal, recoverable metadata next to the media.
// - If absolutefilePath is a directory: writes "<dir>/infos.json" with the JSON-serialized Video.
// - If it's a file: writes base64(protojson(video)) into the media container's "comment" tag.
//   If that write changes the file checksum, the associations KV key is migrated from the
//   old checksum to the new one so lookups remain valid. Finally, it publishes a reload event.
//
// absolutefilePath: absolute OS path to a media file OR directory.
// indexPath:        bleve index path (used to find the right associations store).
// video:            the Video proto to persist (Poster.ContentUrl may be pre-filled by caller).
func (srv *server) saveVideoMetadata(absolutefilePath, indexPath string, video *titlepb.Video) error {
	if video == nil {
		return fmt.Errorf("missing video")
	}

	info, err := os.Stat(absolutefilePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", absolutefilePath, err)
	}

	raw, err := protojson.Marshal(video)
	if err != nil {
		return fmt.Errorf("marshal video %q: %w", video.GetID(), err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	if info.IsDir() {
		dst := filepath.Join(absolutefilePath, "infos.json")
		if err := os.WriteFile(dst, raw, 0o664); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
		_ = srv.publish("reload_dir_event", []byte(dir))
		logger.Info("saved video metadata (dir)", "path", dst, "videoID", video.GetID())
		return nil
	}

	// Only write tag if it actually changed to avoid unnecessary rewrites.
	needSave := true
	if meta, err := Utility.ReadMetadata(absolutefilePath); err == nil {
		if f, ok := meta["format"].(map[string]any); ok {
			if tags, ok := f["tags"].(map[string]any); ok {
				if c, ok := tags["comment"].(string); ok && len(c) > 0 {
					needSave = c != encoded
				}
			}
		}
	}

	if !needSave {
		logger.Debug("video metadata unchanged; skipping write", "file", absolutefilePath, "videoID", video.GetID())
		return nil
	}

	oldChecksum := Utility.CreateFileChecksum(absolutefilePath)

	if err := Utility.SetMetadata(absolutefilePath, "comment", encoded); err != nil {
		return fmt.Errorf("set metadata on %s: %w", absolutefilePath, err)
	}

	// If the file checksum changed, migrate the association key to the new checksum.
	newChecksum := Utility.CreateFileChecksum(absolutefilePath)
	if oldChecksum != newChecksum {
		if store := srv.getAssociations(indexPath); store != nil {
			if data, err := store.GetItem(oldChecksum); err == nil {
				_ = store.RemoveItem(oldChecksum)
				_ = store.SetItem(newChecksum, data)
				logger.Info("associations key migrated after metadata write",
					"old", oldChecksum, "new", newChecksum, "file", absolutefilePath)
			}
		}
	}

	// Ask clients to refresh that folder view.
	dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
	_ = srv.publish("reload_dir_event", []byte(dir))
	logger.Info("saved video metadata (file)", "file", absolutefilePath, "videoID", video.GetID())
	return nil
}

// saveTitleMetadata persists minimal, recoverable metadata next to the media.
// - For directories: writes "<dir>/infos.json" (pretty small JSON bundle).
// - For files: writes base64(protojson(title)) into the media container's
//   "comment" tag (via Utility.SetMetadata). If that write changes the file
//   checksum, the associations KV is updated to the new checksum key.
//
// absolutefilePath: absolute OS path to a media file OR a directory containing it.
// indexPath:        path to the bleve index (used to find the right associations store).
// title:            the Title proto to persist (Poster.ContentUrl may be filled by caller).
func (srv *server) saveTitleMetadata(absolutefilePath, indexPath string, title *titlepb.Title) error {
	if title == nil || len(title.Name) == 0 {
		return errors.New("missing title or empty title name")
	}

	info, err := os.Stat(absolutefilePath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", absolutefilePath, err)
	}

	// Serialize title as JSON; the file-tagged variant is base64-encoded.
	raw, err := protojson.Marshal(title)
	if err != nil {
		return fmt.Errorf("marshal title %q: %w", title.GetID(), err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)

	if info.IsDir() {
		// Folder case → write infos.json then tell clients to reload that dir.
		dst := filepath.Join(absolutefilePath, "infos.json")
		if err := os.WriteFile(dst, raw, 0o664); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
		dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
		_ = srv.publish("reload_dir_event", []byte(dir))
		logger.Info("saved title metadata (dir)", "path", dst, "titleID", title.GetID())
		return nil
	}

	// File case → only write tag when it has actually changed, to avoid unnecessary rewrites.
	needSave := true
	if meta, err := Utility.ReadMetadata(absolutefilePath); err == nil {
		if f, ok := meta["format"].(map[string]any); ok {
			if tags, ok := f["tags"].(map[string]any); ok {
				if c, ok := tags["comment"].(string); ok && len(c) > 0 {
					needSave = c != encoded
				}
			}
		}
	}

	if !needSave {
		logger.Debug("title metadata unchanged; skipping write", "file", absolutefilePath, "titleID", title.GetID())
		return nil
	}

	oldChecksum := Utility.CreateFileChecksum(absolutefilePath)

	// Write the new metadata into the media file's "comment" tag.
	if err := Utility.SetMetadata(absolutefilePath, "comment", encoded); err != nil {
		return fmt.Errorf("set metadata on %s: %w", absolutefilePath, err)
	}

	// If the write changed the file checksum, update the associations KV key.
	newChecksum := Utility.CreateFileChecksum(absolutefilePath)
	if oldChecksum != newChecksum {
		if store := srv.getAssociations(indexPath); store != nil {
			if data, err := store.GetItem(oldChecksum); err == nil {
				_ = store.RemoveItem(oldChecksum)
				_ = store.SetItem(newChecksum, data)
				logger.Info("associations key migrated after metadata write",
					"old", oldChecksum, "new", newChecksum, "file", absolutefilePath)
			}
		}
	}

	// Ask clients to refresh that folder view.
	dir := filepath.Dir(strings.ReplaceAll(absolutefilePath, config.GetDataDir()+"/files", ""))
	_ = srv.publish("reload_dir_event", []byte(dir))
	logger.Info("saved title metadata (file)", "file", absolutefilePath, "titleID", title.GetID())
	return nil
}

// getTitleById returns a Title stored in the internal store.
func (srv *server) getTitleById(indexPath, titleId string) (*titlepb.Title, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}
	uuid := Utility.GenerateUUID(titleId)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("no title found with id " + titleId)
	}
	t := new(titlepb.Title)
	if err := protojson.Unmarshal(raw, t); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTitleById returns Title with associated file paths, if any.
func (srv *server) GetTitleById(ctx context.Context, rqst *titlepb.GetTitleByIdRequest) (*titlepb.GetTitleByIdResponse, error) {
	title, err := srv.getTitleById(rqst.IndexPath, rqst.TitleId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	paths := []string{}
	if assoc := srv.getAssociations(rqst.IndexPath); assoc != nil {
		if data, err := assoc.GetItem(rqst.TitleId); err == nil {
			a := new(fileTileAssociation)
			if err := json.Unmarshal(data, a); err == nil {
				paths = a.Paths
			}
		}
	}
	return &titlepb.GetTitleByIdResponse{Title: title, FilesPaths: paths}, nil
}

// CreateTitle indexes or updates a Title, enriches poster with a thumbnail, sets RBAC ownership, and publishes update event.
func (srv *server) CreateTitle(ctx context.Context, rqst *titlepb.CreateTitleRequest) (*titlepb.CreateTitleResponse, error) {
	if err := checkNotNil("title", rqst.Title); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := checkArg("title id", rqst.Title.GetID()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}

	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}

	rqst.Title.UUID = Utility.GenerateUUID(rqst.Title.ID)
	rqst.Title.Actors = srv.saveTitleCasting(rqst.IndexPath, rqst.Title.ID, "Acting", rqst.Title.Actors)
	rqst.Title.Writers = srv.saveTitleCasting(rqst.IndexPath, rqst.Title.ID, "Writing", rqst.Title.Writers)
	rqst.Title.Directors = srv.saveTitleCasting(rqst.IndexPath, rqst.Title.ID, "Directing", rqst.Title.Directors)

	if err := index.Index(rqst.Title.UUID, rqst.Title); err != nil {
		return nil, status.Errorf(codes.Internal, "index title: %v", err)
	}

	// Poster thumbnail enrichment
	if rqst.Title.Poster != nil {
		tmp := os.TempDir() + "/" + rqst.Title.Poster.URL[strings.LastIndex(rqst.Title.Poster.URL, "/")+1:]
		defer os.Remove(tmp)
		if err := Utility.DownloadFile(rqst.Title.Poster.URL, tmp); err == nil {
			if thumb, err := Utility.CreateThumbnail(tmp, 300, 180); err == nil {
				rqst.Title.Poster.ContentUrl = thumb
			}
		}
	} else {
		return nil, status.Errorf(codes.InvalidArgument, "no poster was given")
	}

	// RBAC
	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rbac client: %v", err)
	}
	if perms, _ := rbacClient.GetResourcePermissions(rqst.Title.ID); perms == nil {
		if err := rbacClient.AddResourceOwner(rqst.Title.ID, "title_infos", clientId, rbacpb.SubjectType_ACCOUNT); err != nil {
			return nil, status.Errorf(codes.Internal, "set title owner: %v", err)
		}
	}

	// Persist raw JSON
	if raw, err := protojson.Marshal(rqst.Title); err == nil {
		if err := index.SetInternal([]byte(rqst.Title.UUID), raw); err != nil {
			return nil, status.Errorf(codes.Internal, "store raw title: %v", err)
		}
	} else {
		logger.Error("marshal title", "titleID", rqst.Title.ID, "err", err)
	}

	evt, err := srv.getEventClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "event client: %v", err)
	}
	evt.Publish("update_title_infos_evt", []byte(fmt.Sprintf(`{"id":%q}`, rqst.Title.ID)))
	logger.Info("title created", "titleID", rqst.Title.ID)
	return &titlepb.CreateTitleResponse{}, nil
}

// UpdateTitleMetadata updates persisted metadata for files associated with the title.
func (srv *server) UpdateTitleMetadata(ctx context.Context, rqst *titlepb.UpdateTitleMetadataRequest) (*titlepb.UpdateTitleMetadataResponse, error) {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}
	if _, err := index.GetInternal([]byte(Utility.GenerateUUID(rqst.Title.ID))); err != nil {
		return nil, status.Errorf(codes.NotFound, "title %q not found in index", rqst.Title.ID)
	}
	if paths, err := srv.getTitleFiles(rqst.IndexPath, rqst.Title.ID); err == nil {
		for _, p := range paths {
			abs := strings.ReplaceAll(p, "\\", "/")
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
			_ = srv.saveTitleMetadata(abs, rqst.IndexPath, rqst.Title)
		}
	}
	return &titlepb.UpdateTitleMetadataResponse{}, nil
}

// deleteTitle removes a Title and its permissions, updates casting and associations, and publishes events.
func (srv *server) deleteTitle(indexPath, titleId string) error {
	title, err := srv.getTitleById(indexPath, titleId)
	if err != nil {
		return err
	}
	for _, p := range title.Actors {
		if x, err := srv.getPersonById(indexPath, p.ID); err == nil {
			x.Acting = Utility.RemoveString(x.Acting, titleId)
			_ = srv.createPerson(indexPath, x)
		}
	}
	for _, p := range title.Writers {
		if x, err := srv.getPersonById(indexPath, p.ID); err == nil {
			x.Writing = Utility.RemoveString(x.Writing, titleId)
			_ = srv.createPerson(indexPath, x)
		}
	}
	for _, p := range title.Directors {
		if x, err := srv.getPersonById(indexPath, p.ID); err == nil {
			x.Directing = Utility.RemoveString(x.Directing, titleId)
			_ = srv.createPerson(indexPath, x)
		}
	}

	dirs := make([]string, 0)
	if paths, err := srv.getTitleFiles(indexPath, titleId); err == nil {
		for _, p := range paths {
			_ = srv.dissociateFileWithTitle(indexPath, titleId, p)
			dirs = append(dirs, filepath.Dir(strings.ReplaceAll(p, config.GetDataDir()+"/files", "")))
		}
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}
	uuid := Utility.GenerateUUID(titleId)
	if err := index.Delete(uuid); err != nil {
		return err
	}
	if err := index.DeleteInternal([]byte(uuid)); err != nil {
		return err
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if err := rbacClient.DeleteResourcePermissions(titleId); err != nil {
		return err
	}

	if err := srv.publish("delete_title_event", []byte(titleId)); err != nil {
		return err
	}
	for _, d := range dirs {
		_ = srv.publish("reload_dir_event", []byte(d))
	}
	return nil
}

// DeleteTitle removes a Title by ID.
func (srv *server) DeleteTitle(ctx context.Context, rqst *titlepb.DeleteTitleRequest) (*titlepb.DeleteTitleResponse, error) {
	if err := srv.deleteTitle(rqst.IndexPath, rqst.TitleId); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	logger.Info("title deleted", "titleID", rqst.TitleId)
	return &titlepb.DeleteTitleResponse{}, nil
}

// createVideo indexes a Video, sets ownership and persists raw JSON, then publishes update event.
func (srv *server) createVideo(indexPath, clientId string, video *titlepb.Video) error {
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}
	if len(video.ID) == 0 {
		return errors.New("no video id was given")
	}
	video.UUID = Utility.GenerateUUID(video.ID)
	if err := index.Index(video.UUID, video); err != nil {
		return err
	}
	video.Casting = srv.saveTitleCasting(indexPath, video.ID, "Casting", video.Casting)

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if perms, _ := rbacClient.GetResourcePermissions(video.ID); perms == nil {
		if err := rbacClient.AddResourceOwner(video.ID, "video_infos", clientId, rbacpb.SubjectType_ACCOUNT); err != nil {
			return err
		}
	}
	if raw, err := protojson.Marshal(video); err == nil {
		if err := index.SetInternal([]byte(video.UUID), raw); err != nil {
			return err
		}
	} else {
		return err
	}

	evt, err := srv.getEventClient()
	if err != nil {
		return err
	}
	payload, _ := protojson.Marshal(video)
	return evt.Publish("update_video_infos_evt", payload)
}

// UpdateVideoMetadata updates persisted metadata of files associated with a given video.
func (srv *server) UpdateVideoMetadata(ctx context.Context, rqst *titlepb.UpdateVideoMetadataRequest) (*titlepb.UpdateVideoMetadataResponse, error) {
	video := rqst.Video
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}
	if _, err := index.GetInternal([]byte(Utility.GenerateUUID(video.ID))); err != nil {
		return nil, status.Errorf(codes.NotFound, "video %q not found in index", video.ID)
	}
	if paths, err := srv.getTitleFiles(rqst.IndexPath, video.ID); err == nil {
		for _, p := range paths {
			abs := strings.ReplaceAll(p, "\\", "/")
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
			_ = srv.saveVideoMetadata(abs, rqst.IndexPath, video)
		}
	}
	return &titlepb.UpdateVideoMetadataResponse{}, nil
}

// CreateVideo inserts or updates a Video and sets RBAC ownership.
func (srv *server) CreateVideo(ctx context.Context, rqst *titlepb.CreateVideoRequest) (*titlepb.CreateVideoResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.createVideo(rqst.IndexPath, clientId, rqst.Video); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	logger.Info("video created", "videoID", rqst.Video.GetID())
	return &titlepb.CreateVideoResponse{}, nil
}

// getVideoById returns a Video by ID.
func (srv *server) getVideoById(indexPath, id string) (*titlepb.Video, error) {
	if !Utility.Exists(indexPath) {
		return nil, errors.New("no database found at path " + indexPath)
	}
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}
	uuid := Utility.GenerateUUID(id)
	raw, err := index.GetInternal([]byte(uuid))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, errors.New("no video found with id " + id)
	}
	video := new(titlepb.Video)
	if err := protojson.Unmarshal(raw, video); err != nil {
		return nil, err
	}
	// Clean casting references
	clean := make([]*titlepb.Person, 0, len(video.Casting))
	for _, c := range video.Casting {
		if p, err := srv.getPersonById(indexPath, c.ID); err == nil {
			clean = append(clean, p)
		}
	}
	video.Casting = clean
	return video, nil
}

// GetVideoById returns a Video and associated file paths.
func (srv *server) GetVideoById(ctx context.Context, rqst *titlepb.GetVideoByIdRequest) (*titlepb.GetVideoByIdResponse, error) {
	video, err := srv.getVideoById(rqst.IndexPath, rqst.VideoId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	paths := []string{}
	if assoc := srv.getAssociations(rqst.IndexPath); assoc != nil {
		if data, err := assoc.GetItem(rqst.VideoId); err == nil {
			a := new(fileTileAssociation)
			if err := json.Unmarshal(data, a); err == nil {
				paths = a.Paths
			}
		}
	}
	// Refresh casting to latest persons
	cast := make([]*titlepb.Person, len(video.Casting))
	for i := range video.Casting {
		if p, err := srv.getPersonById(rqst.IndexPath, video.Casting[i].ID); err == nil {
			cast[i] = p
		}
	}
	video.Casting = cast
	return &titlepb.GetVideoByIdResponse{Video: video, FilesPaths: paths}, nil
}

// deleteVideo removes a video and its associations and permissions, then publishes events.
func (srv *server) deleteVideo(indexPath, videoId string) error {
	video, err := srv.getVideoById(indexPath, videoId)
	if err != nil {
		return err
	}
	for _, c := range video.Casting {
		if p, err := srv.getPersonById(indexPath, c.ID); err == nil {
			p.Casting = Utility.RemoveString(p.Casting, video.ID)
			_ = srv.createPerson(indexPath, p)
		}
	}

	dirs := make([]string, 0)
	if paths, err := srv.getTitleFiles(indexPath, videoId); err == nil {
		for _, p := range paths {
			_ = srv.dissociateFileWithTitle(indexPath, videoId, p)
			dirs = append(dirs, filepath.Dir(strings.ReplaceAll(p, config.GetDataDir()+"/files", "")))
		}
	}

	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}
	uuid := Utility.GenerateUUID(videoId)
	if err := index.Delete(uuid); err != nil {
		return err
	}
	if err := index.DeleteInternal([]byte(uuid)); err != nil {
		return err
	}

	if val, err := index.GetInternal([]byte(uuid)); err != nil {
		return err
	} else if val != nil {
		return errors.New("expected nil, got " + string(val))
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if err := rbacClient.DeleteResourcePermissions(videoId); err != nil {
		return err
	}

	for _, d := range dirs {
		_ = srv.publish("reload_dir_event", []byte(d))
	}
	return srv.publish("delete_video_event", []byte(videoId))
}

// DeleteVideo removes a video by ID.
func (srv *server) DeleteVideo(ctx context.Context, rqst *titlepb.DeleteVideoRequest) (*titlepb.DeleteVideoResponse, error) {
	if err := srv.deleteVideo(rqst.IndexPath, rqst.VideoId); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	logger.Info("video deleted", "videoID", rqst.VideoId)
	return &titlepb.DeleteVideoResponse{}, nil
}

// SearchTitles searches the title index with facets and highlights.
func (srv *server) SearchTitles(rqst *titlepb.SearchTitlesRequest, stream titlepb.TitleService_SearchTitlesServer) error {
	index, err := srv.getIndex(rqst.IndexPath)
	if err != nil {
		return status.Errorf(codes.Internal, "%v", err)
	}

	query := bleve.NewQueryStringQuery(rqst.Query)
	req := bleve.NewSearchRequest(query)
	req.Size = int(rqst.Size)
	req.From = int(rqst.Offset)
	if req.Size == 0 {
		req.Size = 50
	}

	// Facets
	req.AddFacet("Genres", bleve.NewFacetRequest("Genres", req.Size))
	req.AddFacet("Types", bleve.NewFacetRequest("Type", req.Size))
	req.AddFacet("Tags", bleve.NewFacetRequest("Tags", req.Size))

	var (
		zero float64 = 0
		low  float64 = 3.5
		mid  float64 = 7.0
		ten  float64 = 10.0
	)
	rating := bleve.NewFacetRequest("Rating", req.Size)
	rating.AddNumericRange("low", &zero, &low)
	rating.AddNumericRange("medium", &low, &mid)
	rating.AddNumericRange("high", &mid, &ten)
	req.AddFacet("Rating", rating)

	req.Highlight = bleve.NewHighlightWithStyle("html")
	req.Fields = rqst.Fields

	result, err := index.Search(req)
	if err != nil {
		return err
	}

	// Summary first
	if err := stream.Send(&titlepb.SearchTitlesResponse{
		Result: &titlepb.SearchTitlesResponse_Summary{
			Summary: &titlepb.SearchSummary{
				Query: rqst.Query,
				Took:  result.Took.Milliseconds(),
				Total: result.Total,
			},
		},
	}); err != nil {
		return err
	}

	// Then hits
	for i, hit := range result.Hits {
		h := &titlepb.SearchHit{
			Score:    hit.Score,
			Index:    int32(i),
			Snippets: make([]*titlepb.Snippet, 0, len(hit.Fragments)),
		}

		for field, frags := range hit.Fragments {
			h.Snippets = append(h.Snippets, &titlepb.Snippet{Field: field, Fragments: frags})
		}

		// Load the full Title from the internal store and attach it via the oneof.
		if raw, err := index.GetInternal([]byte(hit.ID)); err == nil && len(raw) > 0 {
			t := new(titlepb.Title)
			if err := protojson.Unmarshal(raw, t); err == nil {
				// Optionally refresh Actor objects from index (kept from your version)
				actors := make([]*titlepb.Person, 0, len(t.Actors))
				for _, a := range t.Actors {
					if p, err := srv.getPersonById(rqst.IndexPath, a.GetID()); err == nil {
						actors = append(actors, p)
					}
				}
				t.Actors = actors

				// ✅ Use the oneof
				h.Result = &titlepb.SearchHit_Title{Title: t}
			}
		}

		if err := stream.Send(&titlepb.SearchTitlesResponse{
			Result: &titlepb.SearchTitlesResponse_Hit{Hit: h},
		}); err != nil {
			return err
		}
	}

	return nil
}
