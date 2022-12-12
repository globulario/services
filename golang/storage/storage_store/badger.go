package storage_store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	badger "github.com/dgraph-io/badger/v3"
)

type Badger_store struct {
	path    string
	db      *badger.DB
	options string
	isOpen  bool

	// Sychronized action channel.
	actions chan map[string]interface{}
}

// Manage the concurent access of the db.
func (store *Badger_store) run() {
	for {
		select {
		case action := <-store.actions:
			if action["name"].(string) == "Open" {
				action["result"].(chan error) <- store.open( `{ "path":"` +action["path"].(string) + `"}`)
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

func NewBadger_store() *Badger_store {
	fmt.Println("create new badger store")
	s := new(Badger_store)
	s.actions = make(chan map[string]interface{})
	go func(store *Badger_store) {
		store.run()
	}(s)
	return s
}

// Open the store
func (store *Badger_store) open(optionsStr string) error {

	options := make(map[string] interface{})
	err := json.Unmarshal([]byte(optionsStr), &options)
    if err != nil {
		return err
	}

	path := options["path"].(string)
	if options["name"] != nil {
		path += "/" +  options["name"].(string)
	}

	path =strings.ReplaceAll(path, "\\", "/")

	// TODO give access to more option at the moment 
	// Open the Badger database located in the optionsStr directory.
	// It will be created if it doesn't exist.
	store.db, err = badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return err
	}

	//fmt.Println("store at path ", options["path"], "is now open")
	
	return nil
}

// Close the store
func (store *Badger_store) close() error {
	return store.db.Close()
}


// Set item
func (store *Badger_store) setItem(key string, val []byte) error {

	err := store.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), val)
	})

	if err != nil {
		fmt.Println("failed to set key", key, err)
		return err
	}

	//fmt.Println("Key was set ", key, err)
	return nil
}

// Get item with a given key.
func (store *Badger_store) getItem(key string) (val []byte, err error) {
	err = store.db.View(func (txn *badger.Txn) error{
		
		entry, err := txn.Get([]byte(key))
		if err != nil{
			return err
		}

		// copy the value to val
		val, err = entry.ValueCopy(nil)
		return err
	})

	return 
}

// Remove an item
func (store *Badger_store) removeItem(key string) (err error) {
	err = store.db.Update(func (txn *badger.Txn) error{
		return txn.Delete([]byte(key))
	})
	return
}

// Clear the data store.
func (store *Badger_store) clear() error {
	return store.db.DropAll() // same as drop
}

// Drop the data store.
func (store *Badger_store) drop() error {
	return store.db.DropAll()
}

//////////////////////// Synchronized LevelDB access ///////////////////////////

// Open the store with a give file path.
func (store *Badger_store) Open(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "path": path}
	store.actions <- action
	err := <-action["result"].(chan error)
	return err
}

// Close the store.
func (store *Badger_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Set item
func (store *Badger_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Get item with a given key.
func (store *Badger_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}

	return results["val"].([]byte), nil
}

// Remove an item
func (store *Badger_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear the data store.
func (store *Badger_store) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop the data store.
func (store *Badger_store) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}
