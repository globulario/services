package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
)

// getStore opens/returns the key-value store used for file/title associations.
func (srv *server) getStore(name, path string) (storage_store.Store, error) {
	if store, ok := srv.associations.Load(name); ok {
		return store.(storage_store.Store), nil
	}

	var store storage_store.Store
	switch srv.CacheType {
	case "BADGER":
		store = storage_store.NewBadger_store()
	case "LEVELDB":
		store = storage_store.NewLevelDB_store()
	case "SCYLLA":
		store = storage_store.NewScylla_store(srv.CacheAddress, name, srv.CacheReplicationFactor)
	default:
		store = storage_store.NewBadger_store()
	}

	if err := store.Open(`{"path":"` + path + `","name":"` + name + `"}`); err != nil {
		return nil, err
	}
	srv.associations.Store(name, store)
	logger.Info("associations store opened", "name", name, "path", path, "type", srv.CacheType)
	return store, nil
}

// getIndex opens or creates a Bleve index at path and caches it on the server.
func (srv *server) getIndex(path string) (bleve.Index, error) {
	if srv.indexs == nil {
		srv.indexs = make(map[string]bleve.Index, 0)
	}
	resolved, err := srv.resolveIndexPath(path)
	if err != nil {
		return nil, err
	}
	if idx, ok := srv.indexs[resolved]; ok && idx != nil {
		return idx, nil
	}
	index, err := bleve.Open(resolved)
	if err != nil {
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(resolved, mapping)
		if err != nil {
			return nil, err
		}
	}
	srv.indexs[resolved] = index
	logger.Info("index opened", "path", resolved)
	return index, nil
}

func (srv *server) resolveIndexPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("index path is required")
	}
	clean := strings.ReplaceAll(path, "\\", "/")
	if Utility.Exists(clean) {
		return clean, nil
	}
	dataDir := config.GetDataDir()
	fallback := filepath.Join(dataDir, strings.TrimPrefix(clean, "/"))
	if Utility.Exists(fallback) {
		return fallback, nil
	}
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		return "", err
	}
	return fallback, nil
}

// getAssociations returns the opened association store by id, if any.
func (srv *server) getAssociations(id string) storage_store.Store {
	if srv.associations != nil {
		if st, ok := srv.associations.Load(id); ok {
			return st.(storage_store.Store)
		}
	}
	return nil
}

func (srv *server) migrateAssociationKey(indexPath, oldKey, newKey, file string) {
	if indexPath == "" || oldKey == "" || newKey == "" || oldKey == newKey {
		return
	}
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return
	}
	store, cerr := srv.getStore(filepath.Base(indexPath), resolved)
	if cerr != nil {
		return
	}
	if data, err := store.GetItem(oldKey); err == nil && len(data) > 0 {
		_ = store.RemoveItem(oldKey)
		_ = store.SetItem(newKey, data)
		logger.Info("associations key migrated after metadata write",
			"old", oldKey, "new", newKey, "file", file)
	}
}
