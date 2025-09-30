package storage_store

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// package-level logger; no-op by default. inject via SetLevelDBLogger.
var levelLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

// SetLevelDBLogger lets callers provide a slog logger.
func SetLevelDBLogger(l *slog.Logger) {
	if l != nil {
		levelLogger = l
	}
}

type LevelDB_store struct {
	path    string
	db      *leveldb.DB
	options string
	isOpen  bool

	// Synchronized action channel.
	actions chan map[string]interface{}
	mu      sync.Mutex
}

// open initializes store path from a JSON options string:
// {"path":"/data","name":"kv"}  -> "/data/kv"
// or from a raw path if you've pre-set store.path externally.
// It ensures the parent directory exists. The DB itself is opened lazily per op.
func (store *LevelDB_store) open(optionsStr string) error {
	if store.isOpen {
		return nil
	}
	store.options = optionsStr

	if store.path == "" {
		if optionsStr == "" {
			return errors.New("leveldb: open: missing options; expected JSON with 'path' and 'name'")
		}
		var opts map[string]interface{}
		if err := json.Unmarshal([]byte(optionsStr), &opts); err != nil {
			return err
		}
		p, okP := opts["path"].(string)
		n, okN := opts["name"].(string)
		if !okP || strings.TrimSpace(p) == "" {
			return errors.New("leveldb: open: no store 'path' in options")
		}
		if !okN || strings.TrimSpace(n) == "" {
			return errors.New("leveldb: open: no store 'name' in options")
		}
		store.path = filepath.ToSlash(filepath.Join(p, n))
	}

	// Ensure parent dir exists; DB files are created on first OpenFile().
	if err := os.MkdirAll(filepath.Dir(store.path), 0o755); err != nil {
		return err
	}

	store.isOpen = true
	levelLogger.Info("leveldb configured", "path", store.path)
	return nil
}

// close marks the store closed (DB handles are opened/closed per op).
func (st *LevelDB_store) close() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.db != nil {
		err := st.db.Close()
		st.db = nil
		return err
	}
	return nil
}

// getDb opens the DB handle (callers must Close()).
func (store *LevelDB_store) getDb() (*leveldb.DB, error) {
	if !store.isOpen {
		return nil, errors.New("leveldb: db is not open")
	}
	if store.db == nil {
		db, err := leveldb.OpenFile(store.path, nil)
		if err != nil {
			return nil, err
		}
		store.db = db
	}
	return store.db, nil
}

// setItem writes key->val.
func (store *LevelDB_store) setItem(key string, val []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	db, err := store.getDb()
	if err != nil {
		return err
	}
	return db.Put([]byte(key), val, nil)
}

// getItem returns value for exact key; if key ends with "*" it returns a JSON
// array (as []byte) of values for all keys with that prefix (without "*").
func (store *LevelDB_store) getItem(key string) ([]byte, error) {

	store.mu.Lock()
	defer store.mu.Unlock()

	db, err := store.getDb()
	if err != nil {
		return nil, err
	}

	// Wildcard prefix: "prefix*"
	if strings.HasSuffix(key, "*") {
		prefix := []byte(key[:len(key)-1])
		iter := db.NewIterator(util.BytesPrefix(prefix), nil)
		defer iter.Release()

		values := make([]string, 0, 16)
		for iter.First(); iter.Valid(); iter.Next() {
			values = append(values, string(iter.Value()))
		}
		// Return JSON array of stringified values (preserving prior behavior)
		out, _ := json.Marshal(values)
		return out, nil
	}

	return db.Get([]byte(key), nil)
}

// removeItem deletes an exact key; if key ends with "*" it deletes all keys with that prefix.
func (store *LevelDB_store) removeItem(key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	db, err := store.getDb()
	if err != nil {
		return err
	}

	if strings.HasSuffix(key, "*") {
		prefix := []byte(key[:len(key)-1])
		iter := db.NewIterator(util.BytesPrefix(prefix), nil)
		defer iter.Release()

		batch := new(leveldb.Batch)
		for iter.First(); iter.Valid(); iter.Next() {
			k := append([]byte(nil), iter.Key()...) // copy key
			batch.Delete(k)
		}
		if batch.Len() > 0 {
			return db.Write(batch, nil)
		}
		return nil
	}

	return db.Delete([]byte(key), nil)
}

// clear erases all data by deleting the DB directory and recreating it.
func (st *LevelDB_store) clear() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	db, err := st.getDb()
	if err != nil {
		return err
	}
	it := db.NewIterator(nil, nil)
	defer it.Release()
	wb := new(leveldb.Batch)
	for it.First(); it.Valid(); it.Next() {
		wb.Delete(it.Key())
	}
	return db.Write(wb, nil)
}

// drop removes the DB files from disk.
func (st *LevelDB_store) drop() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	path := st.path
	db := st.db
	st.db = nil

	if db != nil {
		_ = db.Close()
	}
	if strings.TrimSpace(path) != "" {
		return os.RemoveAll(path)
	}
	return nil
}

// Get all keys in the store.
func (store *LevelDB_store) getAllKeys() ([]string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	
	db, err := store.getDb()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	keys := make([]string, 0, 16)
	for iter.First(); iter.Valid(); iter.Next() {
		keys = append(keys, string(iter.Key()))
	}
	return keys, nil
}
