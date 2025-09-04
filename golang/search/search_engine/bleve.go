// Package search_engine provides a Bleve-based implementation of the
// SearchEngine interface. This refactor improves error messages, removes
// printlns, adds structured logging via slog, and keeps public prototypes
// unchanged. It targets bleve/v2.
package search_engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/search/searchpb"
	Utility "github.com/globulario/utility"
)

// logger is a lightweight, local slog logger for this engine.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// BleveSearchEngine implements SearchEngine on top of Bleve.
type BleveSearchEngine struct {
	mu     sync.RWMutex           // protects indexs
	indexs map[string]bleve.Index // path -> index
}

// NewBleveSearchEngine creates a new Bleve-powered search engine.
func NewBleveSearchEngine() *BleveSearchEngine {
	return &BleveSearchEngine{indexs: make(map[string]bleve.Index)}
}

// getIndex returns or opens/creates an index at the given path.
// It validates the path and ensures the parent directory exists.
func (engine *BleveSearchEngine) getIndex(path string) (bleve.Index, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("index path is empty")
	}

	// Normalize path for the current OS.
	path = filepath.Clean(path)

	engine.mu.RLock()
	idx := engine.indexs[path]
	engine.mu.RUnlock()

	// If we already have an index cached, ensure the on-disk path still exists.
	if idx != nil {
		if !Utility.Exists(path) {
			engine.mu.Lock()
			_ = idx.Close()
			delete(engine.indexs, path)
			engine.mu.Unlock()
			return nil, fmt.Errorf("index path does not exist: %s", path)
		}
		return idx, nil
	}

	// Ensure directory exists before creating/opening the index.
	if !Utility.Exists(path) {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return nil, fmt.Errorf("create index directory failed: %w", err)
		}
	}

	// Try to open; if it fails because it doesn't exist, create it.
	index, err := bleve.Open(path)
	if err != nil {
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(path, mapping)
		if err != nil {
			return nil, fmt.Errorf("open/create bleve index failed: %w", err)
		}
		logger.Info("created bleve index", "path", path)
	} else {
		logger.Debug("opened bleve index", "path", path)
	}

	engine.mu.Lock()
	engine.indexs[path] = index
	engine.mu.Unlock()
	return index, nil
}

// GetVersion returns the underlying query engine version label.
func (engine *BleveSearchEngine) GetVersion() string { return "2.x" }

// SearchDocuments executes a query against one or more index paths.
// Public prototype preserved.
func (engine *BleveSearchEngine) SearchDocuments(paths []string, language string, fields []string, q string, offset, pageSize, snippetLength int32) (*searchpb.SearchResults, error) { //nolint
	results := &searchpb.SearchResults{Results: make([]*searchpb.SearchResult, 0)}

	if len(paths) == 0 {
		return nil, errors.New("no index paths supplied")
	}
	if strings.TrimSpace(q) == "" {
		return nil, errors.New("query string is empty")
	}

	for _, p := range paths {
		index, err := engine.getIndex(p)
		if err != nil {
			logger.Warn("skip index (unavailable)", "path", p, "err", err)
			continue
		}

		query := bleve.NewQueryStringQuery(q)
		sr := bleve.NewSearchRequest(query)
		sr.Fields = fields
		sr.From = int(offset)
		sr.Size = int(pageSize)
		// Always return HTML highlights; snippetLength can be honored by client when rendering.
		sr.Highlight = bleve.NewHighlightWithStyle("html")

		searchResult, err := index.Search(sr)
		if err != nil {
			logger.Error("search failed", "path", p, "err", err)
			continue
		}

		for _, hit := range searchResult.Hits {
			id := hit.ID
			// Retrieve the raw user payload stored via SetInternal
			raw, err := index.GetInternal([]byte(id))
			if err != nil {
				logger.Warn("get raw data failed", "path", p, "id", id, "err", err)
				continue
			}

			res := &searchpb.SearchResult{
				Data:   string(raw),
				DocId:  id,
				Rank:   int32(hit.Score * 100),
				Snippet: func() string {
					if len(hit.Fragments) == 0 {
						return ""
					}
					data, err := Utility.ToJson(hit.Fragments)
					if err != nil {
						logger.Warn("fragment marshal failed", "id", id, "err", err)
						return ""
					}
					return string(data)
				}(),
			}
			results.Results = append(results.Results, res)
		}
	}

	if len(results.Results) == 0 {
		return nil, errors.New("no results found")
	}
	return results, nil
}

// DeleteDocument removes a document from the specified index by id.
// Public prototype preserved.
func (engine *BleveSearchEngine) DeleteDocument(path string, id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("document id is empty")
	}
	index, err := engine.getIndex(path)
	if err != nil {
		return err
	}
	if err := index.Delete(id); err != nil {
		return fmt.Errorf("delete document %q failed: %w", id, err)
	}
	return nil
}

// indexJsonObject indexes a single JSON object and stores the raw payload.
// Internal helper; public prototype remains IndexJsonObject below.
func (engine *BleveSearchEngine) indexJsonObject(index bleve.Index, obj map[string]interface{}, language string, idField string, indexs []string, data string) error { //nolint
	if strings.TrimSpace(idField) == "" {
		return errors.New("id field name is empty")
	}

	rawID, ok := obj[idField]
	if !ok {
		return fmt.Errorf("missing id field %q", idField)
	}
	id, ok := rawID.(string)
	if !ok || strings.TrimSpace(id) == "" {
		return fmt.Errorf("id field %q must be a non-empty string", idField)
	}

	if err := index.Index(id, obj); err != nil {
		return fmt.Errorf("index document %q failed: %w", id, err)
	}

	// Persist original JSON alongside the index for retrieval.
	if data != "" {
		if err := index.SetInternal([]byte(id), []byte(data)); err != nil {
			return fmt.Errorf("store raw data for %q failed: %w", id, err)
		}
		return nil
	}

	dataJSON, err := Utility.ToJson(obj)
	if err != nil {
		return fmt.Errorf("serialize object for %q failed: %w", id, err)
	}
	if err := index.SetInternal([]byte(id), []byte(dataJSON)); err != nil {
		return fmt.Errorf("store raw data for %q failed: %w", id, err)
	}
	return nil
}

// IndexJsonObject indexes a JSON string which may represent an object
// or an array of objects. Public prototype preserved.
func (engine *BleveSearchEngine) IndexJsonObject(path string, jsonStr string, language string, id string, indexs []string, data string) error { //nolint
	index, err := engine.getIndex(path)
	if err != nil {
		return err
	}

	var obj interface{}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	switch v := obj.(type) {
	case map[string]interface{}:
		return engine.indexJsonObject(index, v, language, id, indexs, data)
	case []interface{}:
		for i := 0; i < len(v); i++ {
			m, ok := v[i].(map[string]interface{})
			if !ok {
				return fmt.Errorf("array element %d is not an object", i)
			}
			if err := engine.indexJsonObject(index, m, language, id, indexs, data); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.New("JSON must be an object or array of objects")
	}
}

// Count returns the number of documents in an index.
// Public prototype preserved.
func (engine *BleveSearchEngine) Count(path string) int32 {
	index, err := engine.getIndex(path)
	if err != nil {
		logger.Warn("count failed: index unavailable", "path", path, "err", err)
		return -1
	}
	n, err := index.DocCount()
	if err != nil {
		logger.Error("count failed", "path", path, "err", err)
		return -1
	}
	return int32(n)
}

