package main

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/globulario/services/golang/storage/storage_store"
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
	if idx, ok := srv.indexs[path]; ok && idx != nil {
		return idx, nil
	}
	index, err := bleve.Open(path)
	if err != nil {
		mapping := bleve.NewIndexMapping()
		index, err = bleve.New(path, mapping)
		if err != nil {
			return nil, err
		}
	}
	srv.indexs[path] = index
	logger.Info("index opened", "path", path)
	return index, nil
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
