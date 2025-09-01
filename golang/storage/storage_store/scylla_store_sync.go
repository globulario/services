package storage_store

import "errors"

// NewScylla_store creates a ScyllaStore and starts the action loop.
// address/keyspace/replicationFactor params are kept for backward compatibility
// but are now superseded by JSON options passed to Open(). You can still seed
// defaults by calling this with address/keyspace; they'll be part of Open options
// if you craft them that way.
func NewScylla_store(address string, keySpace string, replicationFactor int) *ScyllaStore {
	s := &ScyllaStore{
		actions: make(chan map[string]interface{}),
	}
	go s.run()

	// (legacy constructor kept intact; leave opts to Open() call)
	_ = address
	_ = keySpace
	_ = replicationFactor
	return s
}

// run serializes all DB operations via a single goroutine.
func (store *ScyllaStore) run() {
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
				err = errors.New("scylla: item not found key=" + action["key"].(string) + " err=" + err.Error())
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
			return // exit the loop cleanly
		}
	}
}

///////////////////// Public (synchronized) API /////////////////////

// Open accepts a JSON options string (see scylla_store.go open()).
func (store *ScyllaStore) Open(path string) error {
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "path": path}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Close shuts down the session and terminates the action loop.
func (store *ScyllaStore) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// SetItem writes key -> val.
func (store *ScyllaStore) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetItem returns val for key.
func (store *ScyllaStore) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["val"].([]byte), nil
}

// RemoveItem deletes key.
func (store *ScyllaStore) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear truncates the table.
func (store *ScyllaStore) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop drops the table.
func (store *ScyllaStore) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}
