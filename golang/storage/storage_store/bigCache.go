package storage_store

import (
	"errors"
	"time"
	"github.com/allegro/bigcache/v3"
)

// Implement the storage service with big store.
type BigCache_store struct {
	cache *bigcache.BigCache // The actual cache.

	// Sychronization.
	actions chan map[string]interface{}
}

func (store *BigCache_store) run() {
	for {
		select {
		case action := <-store.actions:

			if action["name"].(string) == "Open" {
				action["result"].(chan error) <- store.open(action["options"].(string))
			} else if action["name"].(string) == "SetItem" {
				action["result"].(chan error) <- store.cache.Set(action["key"].(string), action["val"].([]byte))
			} else if action["name"].(string) == "GetItem" {
				val, err := store.cache.Get(action["key"].(string))
				if err != nil {
					err = errors.New("item not found  key:" + action["key"].(string) + " error: " + err.Error())
				}
				action["results"].(chan map[string]interface{}) <- map[string]interface{}{"val": val, "err": err}
			} else if action["name"].(string) == "RemoveItem" {
				action["result"].(chan error) <- store.cache.Delete(action["key"].(string))
			} else if action["name"].(string) == "Clear" {
				action["result"].(chan error) <- store.cache.Reset()
			} else if action["name"].(string) == "Drop" {
				action["result"].(chan error) <- store.cache.Reset()
			} else if action["name"].(string) == "Close" {
				action["result"].(chan error) <- store.cache.Close()
				break // exit here.
			}

		}
	}
}

// Use it to use the store.
func NewBigCache_store() *BigCache_store {
	s := new(BigCache_store)
	s.actions = make(chan map[string]interface{})

	go func() {
		s.run()
	}()

	return s
}

func (store *BigCache_store) Open(options string) error {
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "options": options}
	store.actions <- action
	return <-action["result"].(chan error)
}

func (store *BigCache_store) open(optionsStr string) error {

	var err error
	config := bigcache.Config{
		// number of shards (must be a power of 2)
		Shards: 1024,
		// time after which entry can be evicted
		LifeWindow: 10 * time.Minute,
		// rps * lifeWindow, used only in initial memory allocation
		MaxEntriesInWindow: 1000 * 10 * 60,
		// max entry size in bytes, used only in initial memory allocation
		MaxEntrySize: 1000,
		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 8192,
		// prints information about additional memory allocation
		Verbose: true,
		// callback fired when the oldest entry is removed because of its
		// expiration time or no space left for the new entry. Default value is nil which
		// means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: func(key string, data []byte) {
			/** Nothing here **/
		},
	}

	// init the underlying cache.
	//store.cache, err = bigcache.NewBigCache(config)
	store.cache, err = bigcache.NewBigCache(config)
	if err != nil {
		panic(err)
	}
	return err
}

// Close the store.
func (store *BigCache_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Set item
func (store *BigCache_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Get item with a given key.
func (store *BigCache_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}

	return results["val"].([]byte), nil
}

// Remove an item
func (store *BigCache_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear the data store.
func (store *BigCache_store) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop the data store.
func (store *BigCache_store) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}
