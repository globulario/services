package storage_store

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/allegro/bigcache/v3"
)

// package-level logger; no-op by default. wire your own via SetLogger.
var bcLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))


// BigCache_store is an in-memory KV store backed by bigcache.
type BigCache_store struct {
	cache   *bigcache.BigCache
	closed  bool // set true after Close is called
	actions chan map[string]interface{}
	mu      sync.Mutex
	isOpen  bool
}

// open initializes bigcache. Accepts either a raw JSON options string or "" for defaults.
// Supported JSON options (all optional):
// {
//   "shards": 1024,
//   "lifeWindowSec": 600,
//   "maxEntriesInWindow": 600000,
//   "maxEntrySize": 1024,
//   "hardMaxCacheSizeMB": 8192,
//   "verbose": true
// }
func (store *BigCache_store) open(optionsStr string) error {
	if store.closed {
		return errors.New("bigcache: open on closed store")
	}
	if store.cache != nil {
		return nil // idempotent
	}

	cfg := bigcache.Config{
		Shards:             1024,
		LifeWindow:         10 * time.Minute,
		MaxEntriesInWindow: 1000 * 10 * 60,
		MaxEntrySize:       1000,
		HardMaxCacheSize:   8192,
		Verbose:            true,
		OnRemove:           nil, // no-op
	}

	// allow lightweight tuning via JSON
	if optionsStr != "" {
		var opts map[string]interface{}
		if err := json.Unmarshal([]byte(optionsStr), &opts); err == nil {
			if v, ok := getInt(opts, "shards"); ok && v > 0 {
				cfg.Shards = v
			}
			if v, ok := getInt(opts, "lifeWindowSec"); ok && v > 0 {
				cfg.LifeWindow = time.Duration(v) * time.Second
			}
			if v, ok := getInt(opts, "maxEntriesInWindow"); ok && v > 0 {
				cfg.MaxEntriesInWindow = v
			}
			if v, ok := getInt(opts, "maxEntrySize"); ok && v > 0 {
				cfg.MaxEntrySize = v
			}
			if v, ok := getInt(opts, "hardMaxCacheSizeMB"); ok && v >= 0 {
				cfg.HardMaxCacheSize = v
			}
			if vb, ok := getBool(opts, "verbose"); ok {
				cfg.Verbose = vb
			}
		}
	}

	cache, err := bigcache.NewBigCache(cfg)
	if err != nil {
		return err
	}
	store.cache = cache
	bcLogger.Info("bigcache open",
		"shards", cfg.Shards,
		"lifeWindow", cfg.LifeWindow.String(),
		"hardMaxCacheSizeMB", cfg.HardMaxCacheSize,
		"verbose", cfg.Verbose,
	)
	
	store.isOpen = true

	return nil
}

// close shuts down bigcache and marks the store as closed.
func (store *BigCache_store) close() error {
	if store.cache == nil {
		// allow idempotent close
		store.closed = true
		return nil
	}
	err := store.cache.Close()
	store.cache = nil
	store.closed = true
	bcLogger.Info("bigcache close")
	return err
}

// helpers to parse JSON ints/bools safely
func getInt(m map[string]interface{}, k string) (int, bool) {
	if v, ok := m[k]; ok {
		switch t := v.(type) {
		case float64:
			return int(t), true
		case int:
			return t, true
		}
	}
	return 0, false
}

func getBool(m map[string]interface{}, k string) (bool, bool) {
	if v, ok := m[k]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b, true
		}
	}
	return false, false
}
