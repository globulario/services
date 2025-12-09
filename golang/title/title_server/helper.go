package main

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// getStore opens/returns the key-value store used for file/title associations.
func (srv *server) getStore(name, path string) (storage_store.Store, error) {
	safeName := sanitizeStoreName(name)
	if srv.associations == nil {
		srv.associations = new(sync.Map)
	} else if store, ok := srv.associations.Load(safeName); ok {
		return store.(storage_store.Store), nil
	}
	origName := name
	name = safeName

	var store storage_store.Store
	var openOptions string
	switch srv.CacheType {
	case "BADGER":
		store = storage_store.NewBadger_store()
		openOptions = fmt.Sprintf(`{"path":"%s","name":"%s"}`, path, name)
	case "LEVELDB":
		store = storage_store.NewLevelDB_store()
		openOptions = fmt.Sprintf(`{"path":"%s","name":"%s"}`, path, name)
	case "SCYLLADB":
		store = storage_store.NewScylla_store(srv.CacheAddress, name, srv.CacheReplicationFactor)
		opts, err := srv.buildScyllaOptions(name)
		if err != nil {
			return nil, fmt.Errorf("build scylla options: %w", err)
		}
		openOptions = opts
	default:
		store = storage_store.NewBadger_store()
		openOptions = fmt.Sprintf(`{"path":"%s","name":"%s"}`, path, name)
	}

	if err := store.Open(openOptions); err != nil {
		return nil, err
	}
	srv.associations.Store(name, store)
	logger.Info("associations store opened", "name", origName, "store", name, "path", path, "type", srv.CacheType)
	return store, nil
}

func (srv *server) buildScyllaOptions(name string) (string, error) {
	hosts := srv.scyllaHosts()
	replication := srv.CacheReplicationFactor
	if replication <= 0 {
		replication = 1
	}
	safeName := sanitizeStoreName(name)
	opts := map[string]interface{}{
		"hosts":              hosts,
		"keyspace":           safeName,
		"table":              safeName,
		"replication_factor": replication,
	}
	raw, err := json.Marshal(opts)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (srv *server) scyllaHosts() []string {
	var hosts []string
	if addr := strings.TrimSpace(srv.CacheAddress); addr != "" {
		for _, part := range strings.Split(addr, ",") {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				hosts = append(hosts, trimmed)
			}
		}
	}
	if len(hosts) == 0 {
		hosts = append(hosts, config.GetLocalIP())
	}
	return hosts
}

func sanitizeStoreName(s string) string {
	trimmed := strings.ToLower(strings.TrimSpace(s))
	if trimmed == "" {
		return "store"
	}
	var b strings.Builder
	changed := false
	for i, r := range trimmed {
		if i == 0 {
			if unicode.IsLetter(r) {
				b.WriteRune(r)
				continue
			}
			if unicode.IsDigit(r) {
				b.WriteRune('a')
				b.WriteRune(r)
				changed = true
				continue
			}
			b.WriteRune('a')
			changed = true
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
			changed = true
		}
	}
	result := b.String()
	if result == "" {
		result = "store"
		changed = true
	}
	if changed && result != trimmed {
		result = fmt.Sprintf("%s_%x", result, crc32.ChecksumIEEE([]byte(trimmed)))
	}
	return result
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
