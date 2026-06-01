package main

// manifest_cache.go — in-memory TTL cache for artifact manifests and directory listings.
//
// Manifests are immutable once published, so caching is safe. Writes (upload,
// promote, delete, state change) invalidate the affected entry. The cache
// eliminates ~95% of MinIO/storage reads from the reconcile-loop hot path
// (GetArtifactManifest called ~6/s across the cluster, every 30s reconcile).

import (
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// manifestCacheEntry holds a parsed manifest, its publish state, and the
// storage key it was loaded from.
type manifestCacheEntry struct {
	key       string
	state     repopb.PublishState
	manifest  *repopb.ArtifactManifest
	expiresAt time.Time
}

// dirCacheEntry holds a directory listing result.
type dirCacheEntry struct {
	names     []string
	expiresAt time.Time
}

// manifestCache is a concurrency-safe, TTL-based cache for manifest reads
// and directory listings. It sits between the gRPC handlers and the storage
// backend, absorbing repeated reads from the reconcile loop.
type manifestCache struct {
	mu          sync.RWMutex
	manifests   map[string]*manifestCacheEntry // keyed by storage key (e.g. "artifacts/pub%name%ver%plat%bn")
	manifestTTL time.Duration

	dirMu  sync.RWMutex
	dirs   map[string]*dirCacheEntry
	dirTTL time.Duration
}

const (
	defaultManifestTTL = 2 * time.Minute
	defaultDirTTL      = 30 * time.Second
)

func newManifestCache() *manifestCache {
	return &manifestCache{
		manifests:   make(map[string]*manifestCacheEntry),
		manifestTTL: defaultManifestTTL,
		dirs:        make(map[string]*dirCacheEntry),
		dirTTL:      defaultDirTTL,
	}
}

// getManifest returns a cached manifest entry if present and not expired.
func (c *manifestCache) getManifest(storageKey string) (string, repopb.PublishState, *repopb.ArtifactManifest, bool) {
	c.mu.RLock()
	e, ok := c.manifests[storageKey]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return "", repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil, false
	}
	return e.key, e.state, e.manifest, true
}

// putManifest stores a manifest in the cache.
func (c *manifestCache) putManifest(storageKey, key string, state repopb.PublishState, m *repopb.ArtifactManifest) {
	c.mu.Lock()
	c.manifests[storageKey] = &manifestCacheEntry{
		key:       key,
		state:     state,
		manifest:  m,
		expiresAt: time.Now().Add(c.manifestTTL),
	}
	c.mu.Unlock()
}

// invalidateManifest removes a single manifest entry from the cache.
func (c *manifestCache) invalidateManifest(storageKey string) {
	c.mu.Lock()
	delete(c.manifests, storageKey)
	c.mu.Unlock()
}

// invalidatePrefix removes all manifest entries whose key starts with prefix.
// Used when a delete or upload could affect build-number resolution.
func (c *manifestCache) invalidatePrefix(prefix string) {
	c.mu.Lock()
	for k := range c.manifests {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.manifests, k)
		}
	}
	c.mu.Unlock()
	// Also invalidate directory cache since the listing changed.
	c.invalidateDir(artifactsDir)
}

// getDir returns a cached directory listing if present and not expired.
func (c *manifestCache) getDir(path string) ([]string, bool) {
	c.dirMu.RLock()
	e, ok := c.dirs[path]
	c.dirMu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.names, true
}

// putDir stores a directory listing in the cache.
func (c *manifestCache) putDir(path string, names []string) {
	c.dirMu.Lock()
	c.dirs[path] = &dirCacheEntry{
		names:     names,
		expiresAt: time.Now().Add(c.dirTTL),
	}
	c.dirMu.Unlock()
}

// invalidateDir removes a directory listing from the cache.
func (c *manifestCache) invalidateDir(path string) {
	c.dirMu.Lock()
	delete(c.dirs, path)
	c.dirMu.Unlock()
}
