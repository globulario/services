package storage_store

import (
	"errors"
	"strings"
)

// run serializes all DB operations through a single goroutine.
func (store *Badger_store) run() {
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
				err = errors.New("badger: getItem: key=" + action["key"].(string) + " err=" + err.Error())
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
			// Exit the goroutine cleanly.
			return
		}
	}
}

// NewBadger_store creates a new instance and starts its action loop.
func NewBadger_store() *Badger_store {
	s := &Badger_store{
		actions: make(chan map[string]interface{}),
	}
	go s.run()
	return s
}

//////////////////////// Synchronized (public) API /////////////////////////

// Open opens the store. Accepts either a raw path ("/var/lib/mydb")
// or a JSON options string: {"path":"/var/lib","name":"mydb"}.
func (store *Badger_store) Open(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	action := map[string]interface{}{
		"name":   "Open",
		"result": make(chan error),
		"path":   path,
	}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Close closes the store and stops the action loop.
func (store *Badger_store) Close() error {
	action := map[string]interface{}{
		"name":   "Close",
		"result": make(chan error),
	}
	store.actions <- action
	return <-action["result"].(chan error)
}

// SetItem sets a value for key.
func (store *Badger_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{
		"name":   "SetItem",
		"result": make(chan error),
		"key":    key,
		"val":    val,
	}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetItem retrieves a value for key.
func (store *Badger_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{
		"name":    "GetItem",
		"results": make(chan map[string]interface{}),
		"key":     key,
	}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["val"].([]byte), nil
}

// RemoveItem deletes a key.
func (store *Badger_store) RemoveItem(key string) error {
	action := map[string]interface{}{
		"name":   "RemoveItem",
		"result": make(chan error),
		"key":    key,
	}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear removes all data.
func (store *Badger_store) Clear() error {
	action := map[string]interface{}{
		"name":   "Clear",
		"result": make(chan error),
	}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop destroys the underlying data.
func (store *Badger_store) Drop() error {
	action := map[string]interface{}{
		"name":   "Drop",
		"result": make(chan error),
	}
	store.actions <- action
	return <-action["result"].(chan error)
}
