package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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
	case "SCYLLADB":
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
		if Utility.Exists(resolved) {
			return idx, nil
		}
		_ = idx.Close()
		delete(srv.indexs, resolved)
		logger.Warn("index handle reset after directory removal", "path", resolved)
	}
	if err := Utility.CreateIfNotExists(resolved, 0o755); err != nil {
		return nil, fmt.Errorf("ensure index directory %s: %w", resolved, err)
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

	dataDir := filepath.Clean(config.GetDataDir())
	clean := filepath.Clean(path)

	var resolved string
	if strings.HasPrefix(clean, dataDir) {
		resolved = clean
	} else {
		trimmed := strings.TrimLeft(clean, string(filepath.Separator))
		resolved = filepath.Join(dataDir, trimmed)
	}
	resolved = filepath.Clean(resolved)

	if Utility.Exists(resolved) {
		return resolved, nil
	}

	parent := filepath.Dir(resolved)
	if err := Utility.CreateIfNotExists(parent, 0o755); err != nil {
		return "", fmt.Errorf("ensure parent dir %s: %w", parent, err)
	}

	// Caller (Bleve/store) will create the final path; just return it.
	return resolved, nil
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

func (srv *server) getMetadataStore(indexPath, collection string) (storage_store.Store, error) {
	resolved, err := srv.resolveIndexPath(indexPath)
	if err != nil {
		return nil, err
	}
	base := filepath.Base(resolved)
	name := fmt.Sprintf("%s_%s_meta", base, collection)

	root := filepath.Join(config.GetDataDir(), "title_metadata")
	if srv.Domain != "" {
		root = filepath.Join(root, srv.Domain)
	}
	root = filepath.Join(root, base, collection)
	if err := Utility.CreateIfNotExists(root, 0o755); err != nil {
		return nil, fmt.Errorf("ensure metadata dir %s: %w", root, err)
	}
	return srv.getStore(name, root)
}

func (srv *server) persistMetadata(indexPath, collection, key string, msg proto.Message) error {
	store, err := srv.getMetadataStore(indexPath, collection)
	if err != nil {
		return err
	}
	raw, err := protojson.Marshal(msg)
	if err != nil {
		return err
	}
	return store.SetItem(key, raw)
}

func (srv *server) removeMetadata(indexPath, collection, key string) {
	store, err := srv.getMetadataStore(indexPath, collection)
	if err != nil {
		return
	}
	_ = store.RemoveItem(key)
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
