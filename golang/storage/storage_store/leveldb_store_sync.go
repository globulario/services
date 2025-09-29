package storage_store

import (
	"errors"
	"strings"
)

// Manage the concurrent access of the db via a single goroutine.
func (store *LevelDB_store) run() {
	for {
		action := <-store.actions
		switch action["name"] {
		case "Open":
			action["result"].(chan error) <- store.open(action["path"].(string))

		case "SetItem":
			if action["val"] != nil {
				action["result"].(chan error) <- store.setItem(action["key"].(string), action["val"].([]byte))
			} else {
				action["result"].(chan error) <- store.setItem(action["key"].(string), nil)
			}

		case "GetItem":
			val, err := store.getItem(action["key"].(string))
			if err != nil {
				err = errors.New("leveldb: item not found key=" + action["key"].(string) + " err=" + err.Error())
			}
			action["results"].(chan map[string]interface{}) <- map[string]interface{}{"val": val, "err": err}

		case "RemoveItem":
			action["result"].(chan error) <- store.removeItem(action["key"].(string))

		case "Clear":
			action["result"].(chan error) <- store.clear()

		case "Drop":
			action["result"].(chan error) <- store.drop()

		case "Close":
			action["result"].(chan error) <- store.close()
			return // exit run loop cleanly

		case "GetAllKeys":
			keys, err := store.getAllKeys()
			action["results"].(chan map[string]interface{}) <- map[string]interface{}{"keys": keys, "err": err}

		default:
			// Unknown action
			bcLogger.Error("LevelDB_store.run: unknown action", "action", action["name"])
			if action["result"] != nil {
				action["result"].(chan error) <- errors.New("LevelDB_store.run: unknown action " + action["name"].(string))
			}
		}
		
	}
}

// NewLevelDB_store constructs a new store and starts its run loop.
func NewLevelDB_store() *LevelDB_store {
	s := &LevelDB_store{
		actions: make(chan map[string]interface{}),
	}
	go s.run()
	return s
}

//////////////////////// Synchronized LevelDB access ///////////////////////////

// Open configures the store using either a JSON options string
// {"path":"/data","name":"kv"} or a pre-filled store.path (ignored here).
func (store *LevelDB_store) Open(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "path": path}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Close marks the store closed and stops the run loop.
func (store *LevelDB_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// SetItem writes key->val.
func (store *LevelDB_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetItem returns value for key; supports "prefix*" wildcard returning a JSON array.
func (store *LevelDB_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["val"].([]byte), nil
}

// RemoveItem deletes a key; supports "prefix*" wildcard.
func (store *LevelDB_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear removes all data (by dropping DB files and recreating parent dir).
func (store *LevelDB_store) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop deletes the DB files from disk.
func (store *LevelDB_store) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetAllKeys returns all keys in the store.
func (store *LevelDB_store) GetAllKeys() ([]string, error) {
	action := map[string]interface{}{"name": "GetAllKeys", "results": make(chan map[string]interface{})}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["keys"].([]string), nil
}