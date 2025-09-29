package storage_store

import (
	"errors"
)

// run serializes operations; exits cleanly on "Close".
func (store *BigCache_store) run() {
	for {
		action := <-store.actions
		switch action["name"] {
		case "Open":
			action["result"].(chan error) <- store.open(action["options"].(string))

		case "SetItem":
			if store.cache == nil {
				action["result"].(chan error) <- errors.New("bigcache: setItem on closed store")
				continue
			}
			action["result"].(chan error) <- store.cache.Set(action["key"].(string), action["val"].([]byte))

		case "GetItem":
			if store.cache == nil {
				action["results"].(chan map[string]interface{}) <- map[string]interface{}{
					"val": nil, "err": errors.New("bigcache: getItem on closed store"),
				}
				continue
			}
			val, err := store.cache.Get(action["key"].(string))
			if err != nil {
				err = errors.New("bigcache: item not found key=" + action["key"].(string) + " err=" + err.Error())
			}
			action["results"].(chan map[string]interface{}) <- map[string]interface{}{"val": val, "err": err}

		case "RemoveItem":
			if store.cache == nil {
				action["result"].(chan error) <- errors.New("bigcache: removeItem on closed store")
				continue
			}
			action["result"].(chan error) <- store.cache.Delete(action["key"].(string))

		case "Clear":
			if store.cache == nil {
				action["result"].(chan error) <- errors.New("bigcache: clear on closed store")
				continue
			}
			action["result"].(chan error) <- store.cache.Reset()

		case "Drop":
			if store.cache == nil {
				action["result"].(chan error) <- errors.New("bigcache: drop on closed store")
				continue
			}
			// in-memory only; Drop == Clear
			action["result"].(chan error) <- store.cache.Reset()

		case "Close":
			// send result first (to avoid goroutine leaks), then return to stop the loop
			action["result"].(chan error) <- store.close()
			return

		case "GetAllKeys":
			if store.cache == nil {
				action["results"].(chan map[string]interface{}) <- map[string]interface{}{
					"keys": nil, "err": errors.New("bigcache: GetAllKeys on closed store"),
				}
				continue
			}
			iterator := store.cache.Iterator()
			var keys []string
			for iterator.SetNext() {
				entry, err := iterator.Value()
				if err == nil {
					keys = append(keys, entry.Key())
				}
			}
			action["results"].(chan map[string]interface{}) <- map[string]interface{}{"keys": keys, "err": nil}

		default:
			// Unknown action
			bcLogger.Error("BigCache_store.run: unknown action", "action", action["name"])
			if action["result"] != nil {
				action["result"].(chan error) <- errors.New("BigCache_store.run: unknown action " + action["name"].(string))
			}
		}
	}
}

// NewBigCache_store constructs a store and starts its action loop.
func NewBigCache_store() *BigCache_store {
	s := &BigCache_store{
		actions: make(chan map[string]interface{}),
	}
	go s.run()
	return s
}

// Open initializes the cache with optional JSON config (see open()).
func (store *BigCache_store) Open(options string) error {
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "options": options}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Close shuts down the cache and terminates the action loop.
func (store *BigCache_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// SetItem writes key->val.
func (store *BigCache_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetItem returns value for key.
func (store *BigCache_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["val"].([]byte), nil
}

// RemoveItem deletes key.
func (store *BigCache_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear removes all entries.
func (store *BigCache_store) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop is equivalent to Clear for in-memory cache.
func (store *BigCache_store) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetAllKeys returns all keys in the store.
func (store *BigCache_store) GetAllKeys() ([]string, error) {
	action := map[string]interface{}{"name": "GetAllKeys", "results": make(chan map[string]interface{})}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["keys"].([]string), nil
}