package storage_store

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	badger "github.com/dgraph-io/badger/v3"
)

// pkgLogger is a package-level logger. By default it discards output.
// Use SetLogger to wire your own logger.
var pkgLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

// Badger_store is a simple key/value store built on BadgerDB.
type Badger_store struct {
	path    string
	db      *badger.DB
	options string
	isOpen  bool

	// Synchronized action channel used by the public API.
	actions chan map[string]interface{}
}

// open opens the store. It supports two input formats:
//  1. A raw filesystem path (e.g. "/var/lib/mydb")
//  2. A JSON string like: {"path":"/var/lib","name":"mydb"}
//
// If "name" is provided, the DB path becomes "<path>/<name>".
func (store *Badger_store) open(optionsStr string) error {
	var (
		path    string
		optsMap map[string]interface{}
	)

	// Try JSON first. If it fails, treat optionsStr as a raw path.
	if err := json.Unmarshal([]byte(optionsStr), &optsMap); err == nil {
		if p, ok := optsMap["path"].(string); ok {
			path = p
			if n, ok := optsMap["name"].(string); ok && n != "" {
				path = filepath.ToSlash(filepath.Join(path, n))
			}
		} else {
			return errors.New("badger: open: missing 'path' in JSON options")
		}
	} else {
		// Fallback: optionsStr is a raw path.
		path = optionsStr
	}

	// Ensure directory exists
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}

	// Base options
	opts := badger.DefaultOptions(path)

	// Optional JSON flags:
	// {
	//   "syncWrites": true,
	//   "truncate": true,
	//   "readOnly": false
	// }
	if v, ok := getBool(optsMap, "syncWrites"); ok {
		opts = opts.WithSyncWrites(v)
	}
	if v, ok := getBool(optsMap, "readOnly"); ok {
		opts = opts.WithReadOnly(v)
	}
	// optional: expose more knobs if you want
	if v, ok := getInt(optsMap, "numVersionsToKeep"); ok {
		opts = opts.WithNumVersionsToKeep(v)
	}
	if v, ok := getBool(optsMap, "compactL0OnClose"); ok {
		opts = opts.WithCompactL0OnClose(v)
	}

	// (You can add more mappings here later if you need them.)

	db, err := badger.Open(opts)
	if err != nil {
		return err
	}

	store.db = db
	store.path = path
	store.isOpen = true
	pkgLogger.Info("badger open", "path", path, "syncWrites", opts.SyncWrites, "readOnly", opts.ReadOnly)
	return nil
}

// close closes the DB.
func (store *Badger_store) close() error {
	if store.db == nil {
		return errors.New("badger: close: db is not open")
	}
	pkgLogger.Info("badger close", "path", store.path)
	store.isOpen = false
	return store.db.Close()
}

// setItem writes a value for a key.
func (store *Badger_store) setItem(key string, val []byte) error {
	if store.db == nil {
		return errors.New("badger: setItem: db is not open")
	}
	return store.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), val)
	})
}

// getItem reads the value for a key.
func (store *Badger_store) getItem(key string) ([]byte, error) {
	if store.db == nil {
		return nil, errors.New("badger: getItem: db is not open")
	}

	var out []byte
	err := store.db.View(func(txn *badger.Txn) error {
		entry, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		val, err := entry.ValueCopy(nil)
		if err != nil {
			return err
		}
		out = val
		return nil
	})
	return out, err
}

// removeItem deletes a key.
func (store *Badger_store) removeItem(key string) error {
	if store.db == nil {
		return errors.New("badger: removeItem: db is not open")
	}
	return store.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// clear removes all data (same as DropAll).
func (store *Badger_store) clear() error {
	if store.db == nil {
		return errors.New("badger: clear: db is not open")
	}
	pkgLogger.Info("badger clear", "path", store.path)
	return store.db.DropAll()
}

// drop destroys the data (DropAll).
func (store *Badger_store) drop() error {
    if store.db == nil {
        return errors.New("badger: drop: db is not open")
    }
    pkgLogger.Info("badger drop", "path", store.path)
    if err := store.db.DropAll(); err != nil {
        return err
    }
    // release KEYREGISTRY and other files
    err := store.db.Close()
    store.db = nil
    store.isOpen = false
    return err
}