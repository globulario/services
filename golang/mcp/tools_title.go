package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	titlepb "github.com/globulario/services/golang/title/titlepb"
)

// ── Title service endpoint ──────────────────────────────────────────────────

func titleEndpoint() string {
	if ep := os.Getenv("GLOBULAR_TITLE_ENDPOINT"); ep != "" {
		return ep
	}
	return gatewayEndpoint()
}

func registerTitleTools(s *server) {

	// ── title_search ────────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "title_search",
		Description: `Search the title/video/audio bleve index. Returns a summary (total hits) followed by individual hits with snippets.

Use index_path to choose the collection:
  /search/titles  — movies, series, episodes
  /search/videos  — YouTube, individual video files
  /search/audios  — music, podcasts

Examples:
- title_search(query="Coluche", index_path="/search/videos")
- title_search(query="comedy", index_path="/search/titles", size=10)`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query":      {Type: "string", Description: "Search query (bleve query string syntax)"},
				"index_path": {Type: "string", Description: "Index to search (default: /search/titles)", Enum: []string{"/search/titles", "/search/videos", "/search/audios"}},
				"size":       {Type: "number", Description: "Max results to return (default: 20)"},
				"offset":     {Type: "number", Description: "Pagination offset (default: 0)"},
			},
			Required: []string{"query"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		query := getStr(args, "query")
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		indexPath := getStr(args, "index_path")
		if indexPath == "" {
			indexPath = "/search/titles"
		}
		size := getInt(args, "size", 20)
		offset := getInt(args, "offset", 0)

		conn, err := s.clients.get(ctx, titleEndpoint())
		if err != nil {
			return nil, fmt.Errorf("title service connection: %w", err)
		}
		client := titlepb.NewTitleServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		rq := &titlepb.SearchTitlesRequest{
			Query:     query,
			IndexPath: indexPath,
			Size:      int32(size),
			Offset:    int32(offset),
		}

		stream, err := client.SearchTitles(callCtx, rq)
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(titleEndpoint())
			}
			return nil, fmt.Errorf("SearchTitles: %w", err)
		}

		var summary map[string]interface{}
		facets := make([]map[string]interface{}, 0)
		hits := make([]map[string]interface{}, 0)

		for {
			rsp, err := stream.Recv()
			if err != nil {
				break
			}
			if rsp.GetSummary() != nil {
				s := rsp.GetSummary()
				summary = map[string]interface{}{
					"query": s.GetQuery(),
					"total": s.GetTotal(),
					"took":  s.GetTook(),
				}
			}
			if rsp.GetFacets() != nil {
				for _, f := range rsp.GetFacets().GetFacets() {
					terms := make([]map[string]interface{}, 0, len(f.GetTerms()))
					for _, t := range f.GetTerms() {
						terms = append(terms, map[string]interface{}{
							"term":  t.GetTerm(),
							"count": t.GetCount(),
						})
					}
					facets = append(facets, map[string]interface{}{
						"field": f.GetField(),
						"total": f.GetTotal(),
						"terms": terms,
					})
				}
			}
			if rsp.GetHit() != nil {
				h := rsp.GetHit()
				hit := map[string]interface{}{
					"score": h.GetScore(),
					"index": h.GetIndex(),
				}
				snippets := make([]map[string]interface{}, 0, len(h.GetSnippets()))
				for _, sn := range h.GetSnippets() {
					snippets = append(snippets, map[string]interface{}{
						"field":     sn.GetField(),
						"fragments": sn.GetFragments(),
					})
				}
				hit["snippets"] = snippets

				if h.GetVideo() != nil {
					v := h.GetVideo()
					hit["video"] = map[string]interface{}{
						"id":          v.GetID(),
						"title":       v.GetTitle(),
						"description": v.GetDescription(),
						"url":         v.GetURL(),
						"duration":    v.GetDuration(),
						"genres":      v.GetGenres(),
						"tags":        v.GetTags(),
					}
				}
				if h.GetTitle() != nil {
					t := h.GetTitle()
					hit["title_doc"] = map[string]interface{}{
						"id":          t.GetID(),
						"name":        t.GetName(),
						"description": t.GetDescription(),
						"type":        t.GetType(),
						"genres":      t.GetGenres(),
						"rating":      t.GetRating(),
					}
				}
				if h.GetAudio() != nil {
					a := h.GetAudio()
					hit["audio"] = map[string]interface{}{
						"id":     a.GetID(),
						"title":  a.GetTitle(),
						"artist": a.GetArtist(),
						"album":  a.GetAlbum(),
					}
				}
				hits = append(hits, hit)
			}
		}

		result := map[string]interface{}{
			"hits": hits,
		}
		if summary != nil {
			result["summary"] = summary
		}
		if len(facets) > 0 {
			result["facets"] = facets
		}
		return result, nil
	})

	// ── title_index_stats ───────────────────────────────────────────────────
	s.register(toolDef{
		Name: "title_index_stats",
		Description: `Get document counts for each search index (titles, videos, audios).
Useful to check if the bleve index is populated or needs rebuilding.`,
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, titleEndpoint())
		if err != nil {
			return nil, fmt.Errorf("title service connection: %w", err)
		}
		client := titlepb.NewTitleServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		indices := []struct {
			name string
			path string
		}{
			{"titles", "/search/titles"},
			{"videos", "/search/videos"},
			{"audios", "/search/audios"},
		}

		stats := make(map[string]interface{})
		for _, idx := range indices {
			rq := &titlepb.SearchTitlesRequest{
				Query:     "*",
				IndexPath: idx.path,
				Size:      0,
			}
			stream, err := client.SearchTitles(callCtx, rq)
			if err != nil {
				stats[idx.name] = map[string]interface{}{"error": err.Error()}
				continue
			}
			count := uint64(0)
			for {
				rsp, err := stream.Recv()
				if err != nil {
					break
				}
				if rsp.GetSummary() != nil {
					count = rsp.GetSummary().GetTotal()
					break
				}
			}
			stats[idx.name] = map[string]interface{}{
				"index_path": idx.path,
				"doc_count":  count,
			}
		}

		return stats, nil
	})

	// ── title_rebuild_index ─────────────────────────────────────────────────
	s.register(toolDef{
		Name: "title_rebuild_index",
		Description: `Rebuild the bleve search index from ScyllaDB metadata store. Use this when search returns no results but file associations exist (e.g. after a reinstall or data restore).

By default rebuilds all collections. Set incremental=true to add missing docs without wiping.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"collections": {Type: "string", Description: "Comma-separated collections to rebuild (default: all). Options: titles, videos, audios"},
				"incremental": {Type: "boolean", Description: "If true, add missing docs without wiping the index (default: false)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if s.cfg.ReadOnly {
			return map[string]interface{}{
				"ok":    false,
				"error": "title_rebuild_index blocked: MCP server is in read-only mode",
			}, nil
		}

		conn, err := s.clients.get(ctx, titleEndpoint())
		if err != nil {
			return nil, fmt.Errorf("title service connection: %w", err)
		}
		client := titlepb.NewTitleServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 120*time.Second)
		defer cancel()

		rq := &titlepb.RebuildIndexRequest{
			Incremental: getBool(args, "incremental", false),
		}
		collStr := getStr(args, "collections")
		if collStr != "" {
			for _, c := range strings.Split(collStr, ",") {
				c = strings.TrimSpace(c)
				if c != "" {
					rq.Collections = append(rq.Collections, c)
				}
			}
		}

		start := time.Now()
		_, err = client.RebuildIndexFromStore(callCtx, rq)
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(titleEndpoint())
			}
			return nil, fmt.Errorf("RebuildIndexFromStore: %w", err)
		}

		collections := "titles, videos, audios"
		if len(rq.Collections) > 0 {
			collections = collStr
		}

		return map[string]interface{}{
			"ok":          true,
			"collections": collections,
			"incremental": rq.Incremental,
			"duration_ms": time.Since(start).Milliseconds(),
		}, nil
	})
}
