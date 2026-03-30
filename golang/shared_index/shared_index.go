package shared_index

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/gocql/gocql"
)

// Config configures a SharedIndex instance.
type Config struct {
	Group         string        // "search", "title", "blog"
	ScyllaHosts   []string      // ScyllaDB hosts for the queue
	LocalIndexDir string        // base directory for local Bleve data
	PollInterval  time.Duration // queue poll interval for writer (default 500ms)
	SyncInterval  time.Duration // MinIO poll interval for readers (default 5s)
}

// SharedIndex provides cluster-aware Bleve search indexing.
//
// Any instance can enqueue index/delete operations. A single writer (elected
// via etcd) processes the queue using Bleve locally, then pushes snapshots to
// MinIO. All instances download snapshots and serve searches from local copies.
type SharedIndex struct {
	cfg      Config
	queue    *indexQueue
	snapshot *snapshotSync
	lease    *writerLease
	logger   *slog.Logger

	mu       sync.RWMutex
	indexes  map[string]bleve.Index  // indexName -> local Bleve handle
	versions map[string]int64        // indexName -> current snapshot version

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a new SharedIndex. Call Start() to begin operations.
func New(cfg Config, logger *slog.Logger) *SharedIndex {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = 5 * time.Second
	}
	if cfg.LocalIndexDir == "" {
		cfg.LocalIndexDir = filepath.Join(os.TempDir(), "shared_index", cfg.Group)
	}

	return &SharedIndex{
		cfg:      cfg,
		queue:    newIndexQueue(logger),
		snapshot: newSnapshotSync(cfg.Group, logger),
		lease:    newWriterLease(cfg.Group, logger),
		logger:   logger,
		indexes:  make(map[string]bleve.Index),
		versions: make(map[string]int64),
	}
}

// Start connects to ScyllaDB, starts the writer election, and begins the
// reader sync loop. Non-blocking.
func (si *SharedIndex) Start(ctx context.Context) error {
	if err := si.queue.connect(si.cfg.ScyllaHosts); err != nil {
		return fmt.Errorf("queue connect: %w", err)
	}

	if err := si.snapshot.EnsureBucket(); err != nil {
		si.logger.Warn("MinIO bucket init failed (snapshots disabled until available)", "err", err)
	}

	ctx, si.cancel = context.WithCancel(ctx)

	// Start writer election.
	si.lease.Campaign(ctx,
		func() { si.onElected() },
		func() { si.onLost() },
	)

	// Start reader sync loop (all instances, including the writer).
	si.wg.Add(1)
	go si.readerLoop(ctx)

	si.logger.Info("shared index started", "group", si.cfg.Group)
	return nil
}

// Stop gracefully shuts down the shared index.
func (si *SharedIndex) Stop() {
	if si.cancel != nil {
		si.cancel()
	}
	si.lease.Stop()
	si.wg.Wait()
	si.closeAllIndexes()
	si.queue.close()
	si.logger.Info("shared index stopped", "group", si.cfg.Group)
}

// Enqueue adds an index operation to the queue. Any instance can call this.
func (si *SharedIndex) Enqueue(indexName, docID, jsonStr, data, idField string, fields []string) error {
	return si.queue.Enqueue(indexName, docID, jsonStr, data, idField, fields, "index")
}

// EnqueueDelete adds a delete operation to the queue.
func (si *SharedIndex) EnqueueDelete(indexName, docID string) error {
	return si.queue.Enqueue(indexName, docID, "", "", "", nil, "delete")
}

// GetIndex returns a read-only Bleve index handle for searching.
// The index is kept in sync via the reader loop.
func (si *SharedIndex) GetIndex(indexName string) (bleve.Index, error) {
	si.mu.RLock()
	idx := si.indexes[indexName]
	si.mu.RUnlock()
	if idx != nil {
		return idx, nil
	}

	// Try opening from local disk (may have been downloaded by reader loop).
	return si.openIndex(indexName)
}

// Search executes a query against a local Bleve index.
func (si *SharedIndex) Search(indexName string, query string, fields []string, offset, limit int) ([]SearchResult, error) {
	idx, err := si.GetIndex(indexName)
	if err != nil {
		return nil, err
	}

	q := bleve.NewQueryStringQuery(query)
	sr := bleve.NewSearchRequest(q)
	sr.Fields = fields
	sr.From = offset
	sr.Size = limit
	sr.Highlight = bleve.NewHighlightWithStyle("html")

	result, err := idx.Search(sr)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	var results []SearchResult
	for _, hit := range result.Hits {
		raw, _ := idx.GetInternal([]byte(hit.ID))
		results = append(results, SearchResult{
			DocID: hit.ID,
			Score: hit.Score,
			Data:  string(raw),
		})
	}
	return results, nil
}

// SearchResult holds a single search hit.
type SearchResult struct {
	DocID string
	Score float64
	Data  string
}

// IsWriter returns true if this instance currently holds the writer lease.
func (si *SharedIndex) IsWriter() bool {
	return si.lease.IsWriter()
}

// --- Writer logic ---

func (si *SharedIndex) onElected() {
	si.logger.Info("writer role acquired, starting queue processor", "group", si.cfg.Group)
	si.wg.Add(1)
	go si.writerLoop()
}

func (si *SharedIndex) onLost() {
	si.logger.Info("writer role lost", "group", si.cfg.Group)
}

func (si *SharedIndex) writerLoop() {
	defer si.wg.Done()

	ticker := time.NewTicker(si.cfg.PollInterval)
	defer ticker.Stop()

	for {
		if !si.lease.IsWriter() {
			return
		}

		select {
		case <-ticker.C:
			si.processQueue()
		}
	}
}

func (si *SharedIndex) processQueue() {
	names, err := si.queue.DequeueAllIndexNames()
	if err != nil {
		si.logger.Warn("list queue index names failed", "err", err)
		return
	}

	for _, indexName := range names {
		items, err := si.queue.DequeueBatch(indexName, 100)
		if err != nil {
			si.logger.Warn("dequeue batch failed", "index", indexName, "err", err)
			continue
		}
		if len(items) == 0 {
			continue
		}

		idx, err := si.openIndex(indexName)
		if err != nil {
			si.logger.Error("open index for writing failed", "index", indexName, "err", err)
			continue
		}

		var processed []gocql.UUID
		for _, item := range items {
			if err := si.applyItem(idx, item); err != nil {
				si.logger.Warn("apply item failed", "index", indexName, "doc", item.DocID, "err", err)
				continue
			}
			processed = append(processed, item.ID)
		}

		// Delete processed items from queue.
		if len(processed) > 0 {
			if err := si.queue.DeleteProcessed(indexName, processed); err != nil {
				si.logger.Warn("delete processed failed", "index", indexName, "err", err)
			}
		}

		// Upload snapshot after batch.
		si.mu.RLock()
		ver := si.versions[indexName]
		si.mu.RUnlock()

		localDir := filepath.Join(si.cfg.LocalIndexDir, indexName)
		newVer, err := si.snapshot.UploadSnapshot(indexName, localDir, ver)
		if err != nil {
			si.logger.Warn("upload snapshot failed", "index", indexName, "err", err)
		} else {
			si.mu.Lock()
			si.versions[indexName] = newVer
			si.mu.Unlock()
		}

		si.logger.Info("processed queue batch", "index", indexName, "items", len(processed))
	}
}

func (si *SharedIndex) applyItem(idx bleve.Index, item QueueItem) error {
	switch item.Operation {
	case "delete":
		return idx.Delete(item.DocID)
	case "index":
		var obj interface{}
		if err := json.Unmarshal([]byte(item.JsonStr), &obj); err != nil {
			return fmt.Errorf("invalid JSON: %w", err)
		}

		switch v := obj.(type) {
		case map[string]interface{}:
			return si.indexObject(idx, v, item)
		case []interface{}:
			for _, elem := range v {
				m, ok := elem.(map[string]interface{})
				if !ok {
					continue
				}
				if err := si.indexObject(idx, m, item); err != nil {
					return err
				}
			}
			return nil
		default:
			return errors.New("JSON must be an object or array")
		}
	default:
		return fmt.Errorf("unknown operation: %s", item.Operation)
	}
}

func (si *SharedIndex) indexObject(idx bleve.Index, obj map[string]interface{}, item QueueItem) error {
	idField := item.IDField
	if idField == "" {
		idField = "id"
	}
	rawID, ok := obj[idField]
	if !ok {
		return fmt.Errorf("missing id field %q", idField)
	}
	id := fmt.Sprintf("%v", rawID)
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("empty id field %q", idField)
	}

	if err := idx.Index(id, obj); err != nil {
		return fmt.Errorf("index %q: %w", id, err)
	}

	// Store raw payload for retrieval.
	data := item.Data
	if data == "" {
		data = item.JsonStr
	}
	if data != "" {
		if err := idx.SetInternal([]byte(id), []byte(data)); err != nil {
			return fmt.Errorf("set internal %q: %w", id, err)
		}
	}
	return nil
}

// --- Reader logic ---

func (si *SharedIndex) readerLoop(ctx context.Context) {
	defer si.wg.Done()

	ticker := time.NewTicker(si.cfg.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			si.syncSnapshots()
		}
	}
}

func (si *SharedIndex) syncSnapshots() {
	// For each known index, check if a newer snapshot is available.
	si.mu.RLock()
	indexNames := make([]string, 0, len(si.indexes))
	for name := range si.indexes {
		indexNames = append(indexNames, name)
	}
	si.mu.RUnlock()

	for _, indexName := range indexNames {
		si.mu.RLock()
		ver := si.versions[indexName]
		si.mu.RUnlock()

		localDir := filepath.Join(si.cfg.LocalIndexDir, indexName)
		newVer, changed, err := si.snapshot.DownloadSnapshot(indexName, localDir, ver)
		if err != nil {
			si.logger.Warn("download snapshot failed", "index", indexName, "err", err)
			continue
		}
		if !changed {
			continue
		}

		// Close and reopen the index to pick up new segments.
		si.mu.Lock()
		if idx := si.indexes[indexName]; idx != nil {
			idx.Close()
			delete(si.indexes, indexName)
		}
		si.mu.Unlock()

		if _, err := si.openIndex(indexName); err != nil {
			si.logger.Error("reopen after sync failed", "index", indexName, "err", err)
		} else {
			si.mu.Lock()
			si.versions[indexName] = newVer
			si.mu.Unlock()
			si.logger.Info("index synced", "index", indexName, "version", newVer)
		}
	}
}

// --- Helpers ---

func (si *SharedIndex) openIndex(indexName string) (bleve.Index, error) {
	si.mu.RLock()
	idx := si.indexes[indexName]
	si.mu.RUnlock()
	if idx != nil {
		return idx, nil
	}

	localDir := filepath.Join(si.cfg.LocalIndexDir, indexName)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	idx, err := bleve.Open(localDir)
	if err != nil {
		mapping := bleve.NewIndexMapping()
		idx, err = bleve.New(localDir, mapping)
		if err != nil {
			return nil, fmt.Errorf("open/create bleve index: %w", err)
		}
		si.logger.Info("created local index", "index", indexName, "path", localDir)
	}

	si.mu.Lock()
	si.indexes[indexName] = idx
	si.mu.Unlock()
	return idx, nil
}

func (si *SharedIndex) closeAllIndexes() {
	si.mu.Lock()
	defer si.mu.Unlock()
	for name, idx := range si.indexes {
		if idx != nil {
			idx.Close()
		}
		delete(si.indexes, name)
	}
}
