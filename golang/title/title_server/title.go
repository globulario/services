package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/title/titlepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (srv *server) getTitleById(indexPath, titleId string) (*titlepb.Title, error) {
	// Resolve index path and open index
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, err
	}
	if !Utility.Exists(resolved) {
		return nil, errors.New("no database found at path " + resolved)
	}
	index, err := srv.getIndex(resolved)
	if err != nil {
		return nil, err
	}

	// Try to fetch existing raw title from internal store
	uuid := Utility.GenerateUUID(titleId)
	raw, err := index.GetInternal([]byte(uuid))
	if err == nil && len(raw) > 0 {
		t := new(titlepb.Title)
		if err := protojson.Unmarshal(raw, t); err != nil {
			return nil, err
		}

		if needsFix := srv.titleNeedsFix(t); needsFix && imdbIDRE.MatchString(t.ID) {
			titleCopy := proto.Clone(t).(*titlepb.Title)
			go srv.asyncEnrichTitle(resolved, titleCopy)
		}

		return t, nil
	}

	// If we reach here, no stored title was found. If the requested ID looks like
	// an IMDb ID, attempt to build it on-demand and persist it for future calls.
	if imdbIDRE.MatchString(titleId) {
		enriched, err := srv.buildTitleFromIMDB(titleId)
		if err != nil {
			return nil, err
		}
		if enriched == nil {
			return nil, errors.New("no title found with id " + titleId)
		}

		// Ensure UUID
		if enriched.UUID == "" {
			enriched.UUID = Utility.GenerateUUID(enriched.ID)
		}

		// Update casting index
		enriched.Actors = srv.saveTitleCasting(resolved, enriched.ID, "Acting", enriched.Actors)
		enriched.Writers = srv.saveTitleCasting(resolved, enriched.ID, "Writing", enriched.Writers)
		enriched.Directors = srv.saveTitleCasting(resolved, enriched.ID, "Directing", enriched.Directors)

		// Index document
		if err := index.Index(enriched.UUID, enriched); err != nil {
			logger.Warn("index imdb-built title", "titleID", enriched.ID, "err", err)
		}
		// Persist raw
		if rawOut, err := protojson.Marshal(enriched); err == nil {
			if err := index.SetInternal([]byte(enriched.UUID), rawOut); err != nil {
				logger.Warn("store imdb-built title raw", "titleID", enriched.ID, "err", err)
			}
		} else {
			logger.Warn("marshal imdb-built title", "titleID", enriched.ID, "err", err)
		}

		return enriched, nil
	}

	return nil, errors.New("no title found with id " + titleId)
}

// GetTitleById returns Title with associated file paths, if any.
func (srv *server) GetTitleById(ctx context.Context, rqst *titlepb.GetTitleByIdRequest) (*titlepb.GetTitleByIdResponse, error) {
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	title, err := srv.getTitleById(resolved, rqst.TitleId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	paths := []string{}
	if assoc := srv.getAssociations(resolved); assoc != nil {
		if data, err := assoc.GetItem(rqst.TitleId); err == nil {
			a := new(fileTileAssociation)
			if err := json.Unmarshal(data, a); err == nil {
				paths = a.Paths
			}
		}
	}
	return &titlepb.GetTitleByIdResponse{Title: title, FilesPaths: paths}, nil
}

func (srv *server) titleNeedsFix(t *titlepb.Title) bool {
	if t == nil {
		return false
	}
	if t.Year == 0 || t.Rating == 0 || t.RatingCount == 0 || t.Duration == "" {
		return true
	}
	if len(t.Actors) == 0 || len(t.Writers) == 0 || len(t.Directors) == 0 {
		return true
	}
	if t.Type == "TVEpisode" && (t.Season == 0 || t.Episode == 0 || t.Serie == "") {
		return true
	}
	return false
}

func (srv *server) asyncEnrichTitle(indexPath string, title *titlepb.Title) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("asyncEnrichTitle: panic recovered", "titleID", title.GetID(), "err", r)
		}
	}()
	index, err := srv.getIndex(indexPath)
	if err != nil {
		logger.Warn("asyncEnrichTitle: open index failed", "indexPath", indexPath, "err", err)
		return
	}
	enriched, err := srv.buildTitleFromIMDB(title.ID)
	if err != nil || enriched == nil {
		if err != nil {
			logger.Warn("asyncEnrichTitle: buildTitleFromIMDB failed", "titleID", title.ID, "err", err)
		}
		return
	}

	merged := proto.Clone(title).(*titlepb.Title)
	srv.mergeTitleWithEnriched(merged, enriched)
	if merged.UUID == "" {
		merged.UUID = Utility.GenerateUUID(merged.ID)
	}
	merged.Actors = srv.saveTitleCasting(indexPath, merged.ID, "Acting", merged.Actors)
	merged.Writers = srv.saveTitleCasting(indexPath, merged.ID, "Writing", merged.Writers)
	merged.Directors = srv.saveTitleCasting(indexPath, merged.ID, "Directing", merged.Directors)

	if err := index.Index(merged.UUID, merged); err != nil {
		logger.Warn("asyncEnrichTitle: reindex enriched title failed", "titleID", merged.ID, "err", err)
	}
	if rawOut, err := protojson.Marshal(merged); err == nil {
		if err := index.SetInternal([]byte(merged.UUID), rawOut); err != nil {
			logger.Warn("asyncEnrichTitle: store enriched title raw failed", "titleID", merged.ID, "err", err)
		}
	} else {
		logger.Warn("asyncEnrichTitle: marshal enriched title failed", "titleID", merged.ID, "err", err)
	}
}

func (srv *server) mergeTitleWithEnriched(target, enriched *titlepb.Title) {
	if target == nil || enriched == nil {
		return
	}
	if target.Year == 0 && enriched.Year != 0 {
		target.Year = enriched.Year
	}
	if (target.Rating == 0 && enriched.Rating != 0) || (target.RatingCount == 0 && enriched.RatingCount != 0) {
		if target.Rating == 0 {
			target.Rating = enriched.Rating
		}
		if target.RatingCount == 0 {
			target.RatingCount = enriched.RatingCount
		}
	}
	if target.Duration == "" && enriched.Duration != "" {
		target.Duration = enriched.Duration
	}
	if len(target.Genres) == 0 && len(enriched.Genres) > 0 {
		target.Genres = enriched.Genres
	}
	if len(target.Language) == 0 && len(enriched.Language) > 0 {
		target.Language = enriched.Language
	}
	if len(target.Nationalities) == 0 && len(enriched.Nationalities) > 0 {
		target.Nationalities = enriched.Nationalities
	}
	if target.Description == "" && enriched.Description != "" {
		target.Description = enriched.Description
	}
	if target.Poster == nil && enriched.Poster != nil {
		target.Poster = enriched.Poster
	}

	mergePersons := func(dst []*titlepb.Person, src []*titlepb.Person) []*titlepb.Person {
		if len(src) == 0 {
			return dst
		}
		seen := make(map[string]struct{}, len(dst))
		for _, p := range dst {
			if p != nil && p.ID != "" {
				seen[p.ID] = struct{}{}
			}
		}
		for _, p := range src {
			if p == nil || p.ID == "" {
				continue
			}
			if _, ok := seen[p.ID]; ok {
				continue
			}
			dst = append(dst, p)
			seen[p.ID] = struct{}{}
		}
		return dst
	}

	target.Actors = mergePersons(target.Actors, enriched.Actors)
	target.Writers = mergePersons(target.Writers, enriched.Writers)
	target.Directors = mergePersons(target.Directors, enriched.Directors)
	if target.Type == "TVEpisode" {
		if target.Season == 0 && enriched.Season != 0 {
			target.Season = enriched.Season
		}
		if target.Episode == 0 && enriched.Episode != 0 {
			target.Episode = enriched.Episode
		}
		if target.Serie == "" && enriched.Serie != "" {
			target.Serie = enriched.Serie
		}
	}
}

// CreateTitle indexes or updates a Title, enriches poster with a thumbnail, sets RBAC ownership, and publishes update event.
func (srv *server) CreateTitle(ctx context.Context, rqst *titlepb.CreateTitleRequest) (*titlepb.CreateTitleResponse, error) {
	if err := checkNotNil("title", rqst.Title); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := checkArg("title id", rqst.Title.GetID()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}

	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	index, err := srv.getIndex(resolved)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}

	rqst.Title.UUID = Utility.GenerateUUID(rqst.Title.ID)
	if existing, err := srv.getTitleById(resolved, rqst.Title.ID); err == nil && existing != nil {
		srv.mergeTitleWithEnriched(rqst.Title, existing)
	}
	rqst.Title.Actors = srv.saveTitleCasting(resolved, rqst.Title.ID, "Acting", rqst.Title.Actors)
	rqst.Title.Writers = srv.saveTitleCasting(resolved, rqst.Title.ID, "Writing", rqst.Title.Writers)
	rqst.Title.Directors = srv.saveTitleCasting(resolved, rqst.Title.ID, "Directing", rqst.Title.Directors)

	if err := srv.indexTitleDoc(index, rqst.Title); err != nil {
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
		if err := rbacClient.AddResourceOwner(token, rqst.Title.ID, clientId, "title_infos", rbacpb.SubjectType_ACCOUNT); err != nil {
			return nil, status.Errorf(codes.Internal, "set title owner: %v", err)
		}
	}

	if err := srv.persistMetadata(rqst.IndexPath, "titles", rqst.Title.ID, rqst.Title); err != nil {
		logger.Warn("persistMetadata title failed", "titleID", rqst.Title.ID, "err", err)
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
	if err := checkNotNil("title", rqst.Title); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	if err := checkArg("title id", rqst.Title.GetID()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	index, err := srv.getIndex(resolved)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}
	if _, err := index.GetInternal([]byte(Utility.GenerateUUID(rqst.Title.ID))); err != nil {
		return nil, status.Errorf(codes.NotFound, "title %q not found in index", rqst.Title.ID)
	}

	if existing, err := srv.getTitleById(resolved, rqst.Title.ID); err == nil && existing != nil {
		srv.mergeTitleWithEnriched(rqst.Title, existing)
	}

	if enriched, err := srv.buildTitleFromIMDB(rqst.Title.ID); err != nil {
		logger.Warn("UpdateTitleMetadata buildTitleFromIMDB failed", "titleID", rqst.Title.ID, "err", err)
	} else if enriched != nil {
		srv.mergeTitleWithEnriched(rqst.Title, enriched)
	}

	rqst.Title.Actors = srv.saveTitleCasting(resolved, rqst.Title.ID, "Acting", rqst.Title.Actors)
	rqst.Title.Writers = srv.saveTitleCasting(resolved, rqst.Title.ID, "Writing", rqst.Title.Writers)
	rqst.Title.Directors = srv.saveTitleCasting(resolved, rqst.Title.ID, "Directing", rqst.Title.Directors)

	uuid := Utility.GenerateUUID(rqst.Title.ID)
	rqst.Title.UUID = uuid
	if err := srv.indexTitleDoc(index, rqst.Title); err != nil {
		return nil, status.Errorf(codes.Internal, "reindex title: %v", err)
	}
	if err := srv.persistMetadata(rqst.IndexPath, "titles", rqst.Title.ID, rqst.Title); err != nil {
		logger.Warn("persistMetadata title failed", "titleID", rqst.Title.ID, "err", err)
	}

	return &titlepb.UpdateTitleMetadataResponse{}, nil
}

// deleteTitle removes a Title and its permissions, updates casting and associations, and publishes events.
func (srv *server) deleteTitle(token, indexPath, titleId string) error {
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
			_ = srv.dissociateFileWithTitle(token, indexPath, titleId, p)
			dirs = append(dirs, filepath.Dir(p))
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
	srv.removeMetadata(indexPath, "titles", titleId)

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if err := rbacClient.DeleteResourcePermissions(token, titleId); err != nil {
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
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}
	if err := srv.deleteTitle(token, rqst.IndexPath, rqst.TitleId); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	logger.Info("title deleted", "titleID", rqst.TitleId)
	return &titlepb.DeleteTitleResponse{}, nil
}

// createVideo indexes a Video, sets ownership and persists raw JSON, then publishes update event.
func (srv *server) createVideo(token, indexPath, clientId string, video *titlepb.Video) error {
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return err
	}
	if len(video.ID) == 0 {
		return errors.New("no video id was given")
	}
	video.UUID = Utility.GenerateUUID(video.ID)
	if err := srv.indexVideoDoc(index, video); err != nil {
		return err
	}
	video.Casting = srv.saveTitleCasting(indexPath, video.ID, "Casting", video.Casting)

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if perms, _ := rbacClient.GetResourcePermissions(video.ID); perms == nil {
		if err := rbacClient.AddResourceOwner(token, video.ID, clientId, "video_infos", rbacpb.SubjectType_ACCOUNT); err != nil {
			return err
		}
	}

	if err := srv.persistMetadata(indexPath, "videos", video.ID, video); err != nil {
		logger.Warn("persistMetadata video failed", "videoID", video.ID, "err", err)
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
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve index path: %v", err)
	}
	index, err := srv.getIndex(resolved)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "open index: %v", err)
	}
	if _, err := index.GetInternal([]byte(Utility.GenerateUUID(video.ID))); err != nil {
		return nil, status.Errorf(codes.NotFound, "video %q not found in index", video.ID)
	}

	return &titlepb.UpdateVideoMetadataResponse{}, nil
}

// CreateVideo inserts or updates a Video and sets RBAC ownership.
func (srv *server) CreateVideo(ctx context.Context, rqst *titlepb.CreateVideoRequest) (*titlepb.CreateVideoResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}
	if err := srv.createVideo(token, rqst.IndexPath, clientId, rqst.Video); err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	return &titlepb.CreateVideoResponse{}, nil
}

// getVideoById returns a Video by ID.
func (srv *server) getVideoById(indexPath, id string) (*titlepb.Video, error) {
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, err
	}
	if !Utility.Exists(resolved) {
		return nil, errors.New("no database found at path " + resolved)
	}
	index, err := srv.getIndex(resolved)
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
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	video, err := srv.getVideoById(resolved, rqst.VideoId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}
	paths := []string{}
	if assoc := srv.getAssociations(resolved); assoc != nil {
		if data, err := assoc.GetItem(rqst.VideoId); err == nil {
			a := new(fileTileAssociation)
			if err := json.Unmarshal(data, a); err == nil {
				paths = a.Paths
			}
		}
	}
	return &titlepb.GetVideoByIdResponse{Video: video, FilesPaths: paths}, nil
}

// deleteVideo removes a video and its associations and permissions, then publishes events.
func (srv *server) deleteVideo(token, indexPath, videoId string) error {
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
			_ = srv.dissociateFileWithTitle(token, indexPath, videoId, p)
			dirs = append(dirs, filepath.Dir(p))
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
	srv.removeMetadata(indexPath, "videos", videoId)

	if val, err := index.GetInternal([]byte(uuid)); err != nil {
		return err
	} else if val != nil {
		return errors.New("expected nil, got " + string(val))
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	if err := rbacClient.DeleteResourcePermissions(token, videoId); err != nil {
		return err
	}

	for _, d := range dirs {
		_ = srv.publish("reload_dir_event", []byte(d))
	}
	return srv.publish("delete_video_event", []byte(videoId))
}

// DeleteVideo removes a video by ID.
func (srv *server) DeleteVideo(ctx context.Context, rqst *titlepb.DeleteVideoRequest) (*titlepb.DeleteVideoResponse, error) {
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolve client id: %v", err)
	}

	if err := srv.deleteVideo(token, rqst.IndexPath, rqst.VideoId); err != nil {
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

		// Load the underlying document from the internal store and attach it via the oneof.
		if raw, err := index.GetInternal([]byte(hit.ID)); err == nil && len(raw) > 0 {
			switch {
			case tryUnmarshalTitleAndPopulate(raw, rqst.IndexPath, srv, h):
				// handled inside helper
			case tryUnmarshalVideo(raw, h):
			case tryUnmarshalAudio(raw, h):
			case tryUnmarshalPerson(raw, h):
			}
		}

		if err := stream.Send(&titlepb.SearchTitlesResponse{
			Result: &titlepb.SearchTitlesResponse_Hit{Hit: h},
		}); err != nil {
			return err
		}
	}

	// Finally stream facets so clients can render them.
	facets := &titlepb.SearchFacets{Facets: make([]*titlepb.SearchFacet, 0, len(result.Facets))}

	for _, f := range result.Facets {
		facet := &titlepb.SearchFacet{
			Field: f.Field,
			Total: int32(f.Total),
			Other: int32(f.Other),
			Terms: make([]*titlepb.SearchFacetTerm, 0, len(f.Terms.Terms())),
		}
		for _, t := range f.Terms.Terms() {
			facet.Terms = append(facet.Terms, &titlepb.SearchFacetTerm{Term: t.Term, Count: int32(t.Count)})
		}

		facets.Facets = append(facets.Facets, facet)
	}

	if err := stream.Send(&titlepb.SearchTitlesResponse{
		Result: &titlepb.SearchTitlesResponse_Facets{Facets: facets},
	}); err != nil {
		return err
	}

	return nil
}

// Helper to decode a Title document, refresh actor references, and attach to the hit.
func tryUnmarshalTitleAndPopulate(raw []byte, indexPath string, srv *server, h *titlepb.SearchHit) bool {
	title := new(titlepb.Title)
	if err := protojson.Unmarshal(raw, title); err != nil || title.GetID() == "" {
		return false
	}

	actors := make([]*titlepb.Person, 0, len(title.Actors))
	for _, actorRef := range title.Actors {
		if p, err := srv.getPersonById(indexPath, actorRef.GetID()); err == nil {
			actors = append(actors, p)
		}
	}
	title.Actors = actors

	h.Result = &titlepb.SearchHit_Title{Title: title}
	return true
}

func tryUnmarshalVideo(raw []byte, h *titlepb.SearchHit) bool {
	video := new(titlepb.Video)
	if err := protojson.Unmarshal(raw, video); err != nil || video.GetID() == "" {
		return false
	}
	h.Result = &titlepb.SearchHit_Video{Video: video}
	return true
}

func tryUnmarshalAudio(raw []byte, h *titlepb.SearchHit) bool {
	audio := new(titlepb.Audio)
	if err := protojson.Unmarshal(raw, audio); err != nil || audio.GetID() == "" {
		return false
	}
	h.Result = &titlepb.SearchHit_Audio{Audio: audio}
	return true
}

func tryUnmarshalPerson(raw []byte, h *titlepb.SearchHit) bool {
	person := new(titlepb.Person)
	if err := protojson.Unmarshal(raw, person); err != nil || person.GetID() == "" {
		return false
	}
	h.Result = &titlepb.SearchHit_Person{Person: person}
	return true
}
