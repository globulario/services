package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		normalizeTitleType(t)

		if needsFix := srv.titleNeedsFix(t); needsFix && imdbIDRE.MatchString(t.ID) {
			titleCopy := proto.Clone(t).(*titlepb.Title)
			go srv.asyncEnrichTitle(resolved, titleCopy)
		}

		// If this is a TVEpisode, ensure the parent series title exists.
		srv.triggerSeriesEnsure(resolved, t)

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

		// If this is a TVEpisode, ensure the parent series title exists.
		srv.triggerSeriesEnsure(resolved, enriched)

		return enriched, nil
	}

	return nil, errors.New("no title found with id " + titleId)
}

// triggerSeriesEnsure fires a background goroutine to ensure the parent series
// title exists when the given title is a TVEpisode with a Serie field.
// Used on read paths (getTitleById) where eventual consistency is fine.
func (srv *server) triggerSeriesEnsure(indexPath string, t *titlepb.Title) {
	if t == nil || t.Type != "TVEpisode" || t.Serie == "" || !imdbIDRE.MatchString(t.Serie) {
		return
	}

	// Fast check: if the series is already indexed, skip the goroutine entirely.
	uuid := Utility.GenerateUUID(t.Serie)
	if idx, err := srv.getIndex(indexPath); err == nil {
		if raw, err := idx.GetInternal([]byte(uuid)); err == nil && len(raw) > 0 {
			return
		}
	}

	go srv.ensureSeriesTitle(indexPath, t.Serie)
}

// ensureSeriesSync is the synchronous variant of triggerSeriesEnsure.
// Used on write paths (CreateTitle, UpdateTitleMetadata, reindexTitles) so
// the parent series exists before the call returns.
func (srv *server) ensureSeriesSync(indexPath string, t *titlepb.Title) {
	if t == nil || t.Type != "TVEpisode" || t.Serie == "" || !imdbIDRE.MatchString(t.Serie) {
		return
	}
	srv.ensureSeriesTitle(indexPath, t.Serie)
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

// castNeedsEnrich returns true if any person across actors/directors/writers
// is missing an ID or biography, indicating TMDb enrichment is needed.
func (srv *server) castNeedsEnrich(t *titlepb.Title) bool {
	if t == nil {
		return false
	}
	for _, group := range [][]*titlepb.Person{t.Actors, t.Directors, t.Writers} {
		for _, p := range group {
			if p != nil && (p.ID == "" || p.Biography == "") {
				return true
			}
		}
	}
	return false
}

// ensureSeriesTitle checks if the parent series title exists in the index.
// If not, it builds it from IMDb/TMDb and persists it. This runs in the
// background so it doesn't block episode creation.
func (srv *server) ensureSeriesTitle(indexPath, seriesIMDbID string) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ensureSeriesTitle: panic recovered", "seriesID", seriesIMDbID, "err", r)
		}
	}()

	index, err := srv.getIndex(indexPath)
	if err != nil {
		logger.Warn("ensureSeriesTitle: open index failed", "indexPath", indexPath, "err", err)
		return
	}

	// Check if the series title already exists.
	uuid := Utility.GenerateUUID(seriesIMDbID)
	if raw, err := index.GetInternal([]byte(uuid)); err == nil && len(raw) > 0 {
		logger.Debug("ensureSeriesTitle: series already exists", "seriesID", seriesIMDbID)
		return
	}

	logger.Info("ensureSeriesTitle: auto-creating parent series", "seriesID", seriesIMDbID)
	enriched, err := srv.buildTitleFromIMDB(seriesIMDbID)
	if err != nil || enriched == nil {
		if err != nil {
			logger.Warn("ensureSeriesTitle: buildTitleFromIMDB failed", "seriesID", seriesIMDbID, "err", err)
		}
		return
	}

	if enriched.UUID == "" {
		enriched.UUID = Utility.GenerateUUID(enriched.ID)
	}
	enriched.Actors = srv.saveTitleCasting(indexPath, enriched.ID, "Acting", enriched.Actors)
	enriched.Writers = srv.saveTitleCasting(indexPath, enriched.ID, "Writing", enriched.Writers)
	enriched.Directors = srv.saveTitleCasting(indexPath, enriched.ID, "Directing", enriched.Directors)

	if err := index.Index(enriched.UUID, enriched); err != nil {
		logger.Warn("ensureSeriesTitle: index failed", "seriesID", enriched.ID, "err", err)
	}
	if rawOut, err := protojson.Marshal(enriched); err == nil {
		if err := index.SetInternal([]byte(enriched.UUID), rawOut); err != nil {
			logger.Warn("ensureSeriesTitle: store raw failed", "seriesID", enriched.ID, "err", err)
		}
	}

	// Also persist to metadata store.
	if err := srv.persistMetadata(indexPath, "titles", enriched.ID, enriched); err != nil {
		logger.Warn("ensureSeriesTitle: persistMetadata failed", "seriesID", enriched.ID, "err", err)
	}

	logger.Info("ensureSeriesTitle: parent series created", "seriesID", enriched.ID, "name", enriched.Name, "type", enriched.Type)
}

// listEpisodesBySeries scans the Bleve internal store for all TVEpisode titles
// whose Serie field matches seriesId. Results are sorted by season then episode.
// If season > 0, only episodes of that season are returned.
func (srv *server) listEpisodesBySeries(indexPath, seriesId string, season int32) ([]*titlepb.Title, error) {
	index, err := srv.getIndex(indexPath)
	if err != nil {
		return nil, err
	}

	// Use a Bleve field-scoped query to find episodes with matching Serie.
	q := bleve.NewQueryStringQuery(fmt.Sprintf(`+Type:TVEpisode +Serie:"%s"`, seriesId))
	req := bleve.NewSearchRequest(q)
	req.Size = 10000 // upper bound — a series rarely exceeds a few hundred episodes
	req.From = 0

	result, err := index.Search(req)
	if err != nil {
		return nil, fmt.Errorf("search episodes for series %s: %w", seriesId, err)
	}

	episodes := make([]*titlepb.Title, 0, len(result.Hits))
	for _, hit := range result.Hits {
		raw, err := index.GetInternal([]byte(hit.ID))
		if err != nil || len(raw) == 0 {
			continue
		}
		t := new(titlepb.Title)
		if err := protojson.Unmarshal(raw, t); err != nil {
			continue
		}
		// Double-check: Serie must match exactly (Bleve may tokenize).
		if t.Type != "TVEpisode" || t.Serie != seriesId {
			continue
		}
		if season > 0 && t.Season != season {
			continue
		}
		episodes = append(episodes, t)
	}

	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].Season != episodes[j].Season {
			return episodes[i].Season < episodes[j].Season
		}
		return episodes[i].Episode < episodes[j].Episode
	})

	return episodes, nil
}

// GetSeriesEpisodes returns all episodes for a given series, sorted by season/episode.
func (srv *server) GetSeriesEpisodes(ctx context.Context, rqst *titlepb.GetSeriesEpisodesRequest) (*titlepb.GetSeriesEpisodesResponse, error) {
	if rqst.SeriesId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "seriesId is required")
	}
	resolved, err := srv.resolveIndexPath(rqst.IndexPath)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	episodes, err := srv.listEpisodesBySeries(resolved, rqst.SeriesId, rqst.Season)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	logger.Info("GetSeriesEpisodes", "seriesId", rqst.SeriesId, "season", rqst.Season, "count", len(episodes))
	return &titlepb.GetSeriesEpisodesResponse{Episodes: episodes}, nil
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
			return deduplicatePersons(dst)
		}
		seenID := make(map[string]int, len(dst))   // ID → index in dst
		seenName := make(map[string]int, len(dst)) // lowercase FullName → index in dst
		for i, p := range dst {
			if p == nil {
				continue
			}
			if p.ID != "" {
				seenID[p.ID] = i
			}
			if n := strings.ToLower(strings.TrimSpace(p.FullName)); n != "" {
				seenName[n] = i
			}
		}
		for _, p := range src {
			if p == nil {
				continue
			}
			// Check by ID first.
			if p.ID != "" {
				if idx, ok := seenID[p.ID]; ok {
					// Existing entry — upgrade if the new one has more data.
					if dst[idx].Biography == "" && p.Biography != "" {
						dst[idx] = p
					}
					continue
				}
			}
			// Check by name (catches no-ID duplicates).
			nameKey := strings.ToLower(strings.TrimSpace(p.FullName))
			if nameKey != "" {
				if idx, ok := seenName[nameKey]; ok {
					// Keep the one with more data (prefer one with ID).
					if (dst[idx].ID == "" && p.ID != "") || (dst[idx].Biography == "" && p.Biography != "") {
						dst[idx] = p
					}
					continue
				}
			}
			if p.ID != "" {
				seenID[p.ID] = len(dst)
			}
			if nameKey != "" {
				seenName[nameKey] = len(dst)
			}
			dst = append(dst, p)
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

// normalizeTitleType fixes inconsistent type strings produced by different
// IMDb data sources (StalkR scraper vs suggestion API fallback).
// Canonical values follow Schema.org: TVSeries, TVEpisode, TVMiniSeries, etc.
// deduplicatePersons removes duplicate persons from a single list, matching
// by ID first, then by lowercase FullName. Prefers the entry with more data.
func deduplicatePersons(list []*titlepb.Person) []*titlepb.Person {
	if len(list) <= 1 {
		return list
	}
	out := make([]*titlepb.Person, 0, len(list))
	seenID := make(map[string]int)
	seenName := make(map[string]int)
	for _, p := range list {
		if p == nil {
			continue
		}
		if p.ID != "" {
			if idx, ok := seenID[p.ID]; ok {
				if out[idx].Biography == "" && p.Biography != "" {
					out[idx] = p
				}
				continue
			}
		}
		nameKey := strings.ToLower(strings.TrimSpace(p.FullName))
		if nameKey != "" {
			if idx, ok := seenName[nameKey]; ok {
				if (out[idx].ID == "" && p.ID != "") || (out[idx].Biography == "" && p.Biography != "") {
					out[idx] = p
				}
				continue
			}
		}
		if p.ID != "" {
			seenID[p.ID] = len(out)
		}
		if nameKey != "" {
			seenName[nameKey] = len(out)
		}
		out = append(out, p)
	}
	return out
}

func normalizeTitleType(t *titlepb.Title) {
	if t == nil {
		return
	}
	switch t.Type {
	case "TV Series":
		t.Type = "TVSeries"
	case "TV Mini Series":
		t.Type = "TVMiniSeries"
	case "TV Movie":
		t.Type = "TVMovie"
	case "TV Special":
		t.Type = "TVSpecial"
	case "Video Game":
		t.Type = "VideoGame"
	}
}

// enforceTitleInvariants normalises the type and Serie/Season/Episode fields:
//   - TVEpisode must have Serie, Season > 0, Episode > 0 — attempts IMDb enrichment if missing
//   - non-episode types have those fields cleared
//
// Returns an error only if the title is a TVEpisode and Serie is still empty
// after enrichment (Season/Episode=0 are tolerated as some episodes lack numbering).
func (srv *server) enforceTitleInvariants(t *titlepb.Title) error {
	if t == nil {
		return nil
	}
	normalizeTitleType(t)
	if t.Type == "TVEpisode" {
		// Attempt to fill missing fields from IMDb if the title ID looks valid.
		if (t.Serie == "" || t.Season == 0 || t.Episode == 0) && imdbIDRE.MatchString(t.ID) {
			if enriched, err := srv.buildTitleFromIMDB(t.ID); err == nil && enriched != nil {
				if t.Serie == "" && enriched.Serie != "" {
					t.Serie = enriched.Serie
				}
				if t.Season == 0 && enriched.Season != 0 {
					t.Season = enriched.Season
				}
				if t.Episode == 0 && enriched.Episode != 0 {
					t.Episode = enriched.Episode
				}
			}
		}
		if t.Serie == "" {
			return fmt.Errorf("TVEpisode %q requires a parent series ID (Serie field)", t.ID)
		}
	} else {
		// Non-episode: clear episode-only fields to prevent bad data.
		t.Serie = ""
		t.Season = 0
		t.Episode = 0
	}
	return nil
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
	if err := srv.enforceTitleInvariants(rqst.Title); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	// Enrich cast details (biography, picture, IDs) when persons are incomplete.
	if srv.castNeedsEnrich(rqst.Title) && imdbIDRE.MatchString(rqst.Title.ID) {
		if enriched, err := srv.buildTitleFromIMDB(rqst.Title.ID); err == nil && enriched != nil {
			srv.mergeTitleWithEnriched(rqst.Title, enriched)
		}
	}

	rqst.Title.Actors = srv.saveTitleCasting(resolved, rqst.Title.ID, "Acting", rqst.Title.Actors)
	rqst.Title.Writers = srv.saveTitleCasting(resolved, rqst.Title.ID, "Writing", rqst.Title.Writers)
	rqst.Title.Directors = srv.saveTitleCasting(resolved, rqst.Title.ID, "Directing", rqst.Title.Directors)

	if err := srv.indexTitleDoc(index, rqst.Title); err != nil {
		return nil, status.Errorf(codes.Internal, "index title: %v", err)
	}

	logger.Info("CreateTitle indexed", "titleID", rqst.Title.ID, "type", rqst.Title.Type, "serie", rqst.Title.Serie, "season", rqst.Title.Season, "episode", rqst.Title.Episode)

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

	// RBAC ownership is best-effort: if the RBAC backend (ScyllaDB) is unavailable
	// the title must still be indexed and returned successfully.
	if rbacClient, err := srv.getRbacClient(); err != nil {
		logger.Warn("CreateTitle: RBAC client unavailable; ownership not set", "titleID", rqst.Title.ID, "err", err)
	} else if perms, _ := rbacClient.GetResourcePermissions(rqst.Title.ID); perms == nil {
		if err := rbacClient.AddResourceOwner(token, rqst.Title.ID, clientId, "title_infos", rbacpb.SubjectType_ACCOUNT); err != nil {
			logger.Warn("CreateTitle: AddResourceOwner failed; ownership not set", "titleID", rqst.Title.ID, "err", err)
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

	// Synchronously ensure the parent series title exists if this is a TVEpisode.
	srv.ensureSeriesSync(resolved, rqst.Title)

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

	if err := srv.enforceTitleInvariants(rqst.Title); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
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

	// Synchronously ensure the parent series title exists if this is a TVEpisode.
	srv.ensureSeriesSync(resolved, rqst.Title)

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

	// RBAC ownership is best-effort: if the RBAC backend (ScyllaDB) is unavailable
	// the video must still be indexed and associated with the file.
	if rbacClient, err := srv.getRbacClient(); err != nil {
		logger.Warn("createVideo: RBAC client unavailable; ownership not set", "videoID", video.ID, "err", err)
	} else if perms, _ := rbacClient.GetResourcePermissions(video.ID); perms == nil {
		if err := rbacClient.AddResourceOwner(token, video.ID, clientId, "video_infos", rbacpb.SubjectType_ACCOUNT); err != nil {
			logger.Warn("createVideo: AddResourceOwner failed; ownership not set", "videoID", video.ID, "err", err)
		}
	}

	if err := srv.persistMetadata(indexPath, "videos", video.ID, video); err != nil {
		logger.Warn("persistMetadata video failed", "videoID", video.ID, "err", err)
	}

	evt, err := srv.getEventClient()
	if err != nil {
		logger.Warn("createVideo: event client unavailable; update event not published", "videoID", video.ID, "err", err)
		return nil
	}
	payload, _ := protojson.Marshal(video)
	if err := evt.Publish("update_video_infos_evt", payload); err != nil {
		logger.Warn("createVideo: publish update_video_infos_evt failed", "videoID", video.ID, "err", err)
	}
	return nil
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
// If the video record is missing from the index but file associations still exist
// (dangling reference caused by a failed earlier restore), the associations are
// cleaned up anyway so the path can be re-indexed via StartProcessVideo.
func (srv *server) deleteVideo(token, indexPath, videoId string) error {
	video, err := srv.getVideoById(indexPath, videoId)
	recordMissing := err != nil && strings.Contains(err.Error(), "no video found with id")
	if err != nil && !recordMissing {
		// Hard error (e.g. database directory not found) — nothing safe to do.
		return err
	}

	// Clean up casting back-references only when we have the record.
	if video != nil {
		for _, c := range video.Casting {
			if p, err := srv.getPersonById(indexPath, c.ID); err == nil {
				p.Casting = Utility.RemoveString(p.Casting, video.ID)
				_ = srv.createPerson(indexPath, p)
			}
		}
	}

	// Always clean up file associations, even when the video record is gone.
	// Without this a path stays permanently "already indexed" and restore is blocked.
	dirs := make([]string, 0)
	if paths, err := srv.getTitleFiles(indexPath, videoId); err == nil {
		for _, p := range paths {
			_ = srv.dissociateFileWithTitle(token, indexPath, videoId, p)
			dirs = append(dirs, filepath.Dir(p))
		}
	}

	if !recordMissing {
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

	logger.Debug("SearchTitles", "query", rqst.Query, "indexPath", rqst.IndexPath, "size", req.Size, "from", req.From)

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

	logger.Debug("SearchTitles results", "query", rqst.Query, "total", result.Total, "hits", len(result.Hits))

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
	normalizeTitleType(title)

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
