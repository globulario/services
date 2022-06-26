package storage_store

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type LevelDB_store struct {
	path    string
	db      *leveldb.DB
	options string
	isOpen  bool

	// Sychronized action channel.
	actions chan map[string]interface{}
}

// Manage the concurent access of the db.
func (store *LevelDB_store) run() {
	for {
		select {
		case action := <-store.actions:
			if action["name"].(string) == "Open" {
				action["result"].(chan error) <- store.open(action["path"].(string))
			} else if action["name"].(string) == "SetItem" {
				if action["val"] != nil {
					action["result"].(chan error) <- store.setItem(action["key"].(string), action["val"].([]byte))
				} else {
					action["result"].(chan error) <- store.setItem(action["key"].(string), nil)
				}
			} else if action["name"].(string) == "GetItem" {
				val, err := store.getItem(action["key"].(string))
				if err != nil {
					err = errors.New("item not found  key:" + action["key"].(string) + " error: " + err.Error())
				}
				action["results"].(chan map[string]interface{}) <- map[string]interface{}{"val": val, "err": err}
			} else if action["name"].(string) == "RemoveItem" {
				action["result"].(chan error) <- store.removeItem(action["key"].(string))
			} else if action["name"].(string) == "Clear" {
				action["result"].(chan error) <- store.clear()
			} else if action["name"].(string) == "Drop" {
				action["result"].(chan error) <- store.drop()
			} else if action["name"].(string) == "Close" {
				action["result"].(chan error) <- store.close()
				break // exit here.
			}

		}
	}
}

func NewLevelDB_store() *LevelDB_store {
	s := new(LevelDB_store)
	s.actions = make(chan map[string]interface{})
	go func(store *LevelDB_store) {
		store.run()
	}(s)
	return s
}

// In that case the parameter contain the path.
func (store *LevelDB_store) open(optionsStr string) error {
	if store.isOpen {
		return nil // the connection is already open.
	}

	store.options = optionsStr
	if len(store.path) == 0 {
		if len(optionsStr) == 0 {
			return errors.New("store path and store name must be given as options")
		}

		options := make(map[string]interface{})
		json.Unmarshal([]byte(optionsStr), &options)

		if options["path"] == nil {
			return errors.New("no store path was given in connection option")
		}

		if options["name"] == nil {
			return errors.New("no store name was given in connection option")
		}

		store.path = options["path"].(string) + string(os.PathSeparator) + options["name"].(string)

	}
	store.isOpen = true
	return nil
}

// Close the store.
func (store *LevelDB_store) close() error {
	if store.db == nil {
		store.isOpen = false
		return nil
	}

	if !store.isOpen {
		return nil
	}
	store.isOpen = false
	return nil
}

func (store *LevelDB_store) getDb() (*leveldb.DB, error) {
	var err error
	store.db, err = leveldb.OpenFile(store.path, nil)
	if err != nil {
		return nil, err
	}

	return store.db, nil
}

// Set item
func (store *LevelDB_store) setItem(key string, val []byte) error {

	db, err := store.getDb()
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Put([]byte(key), val, nil)
}

// Get item with a given key.
func (store *LevelDB_store) getItem(key string) ([]byte, error) {
	db, err := store.getDb()

	if err != nil {
		return nil, err
	}
	defer db.Close()
	// Here I will make a little trick to give more flexibility to the tool...
	if strings.HasSuffix(key, "*") {
		// I will made use of iterator to ket the values
		values := "["
		iter := db.NewIterator(util.BytesPrefix([]byte(key[:len(key)-2])), nil)

		for ok := iter.Last(); ok; ok = iter.Prev() {
			if values != "[" {
				values += ","
			}
			values += string(iter.Value())
		}

		values += "]"

		iter.Release()
		return []byte(values), nil // I will return the stringnify value

	}

	return db.Get([]byte(key), nil)
}

// Remove an item or a range of items with same path
func (store *LevelDB_store) removeItem(key string) error {
	db, err := store.getDb()
	if err != nil {
		return err
	}
	defer db.Close()
	if strings.HasSuffix(key, "*") {
		// I will made use of iterator to ket the values
		iter := db.NewIterator(util.BytesPrefix([]byte(key[:len(key)-1])), nil)
		for ok := iter.Last(); ok; ok = iter.Prev() {
			db.Delete([]byte(iter.Key()), nil)
		}
		iter.Release()

	}
	return db.Delete([]byte(key), nil)
}

// Clear the data store.
func (store *LevelDB_store) clear() error {
	err := store.Drop()
	if err != nil {
		return err
	}

	// Recreate the db files and connection.
	return nil
}

// Drop the data store.
func (store *LevelDB_store) drop() error {
	// Close the db
	err := store.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(store.path)
}

//////////////////////// Synchronized LevelDB access ///////////////////////////

// Open the store with a give file path.
func (store *LevelDB_store) Open(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "path": path}
	store.actions <- action
	err := <-action["result"].(chan error)
	return err
}

// Close the store.
func (store *LevelDB_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Set item
func (store *LevelDB_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Get item with a given key.
func (store *LevelDB_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}

	return results["val"].([]byte), nil
}

// Remove an item
func (store *LevelDB_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear the data store.
func (store *LevelDB_store) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop the data store.
func (store *LevelDB_store) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}
