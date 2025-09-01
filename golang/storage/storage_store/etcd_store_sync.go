package storage_store

import "errors"

// NewEtcd_store creates the store and starts its run loop.
func NewEtcd_store() *Etcd_store {
	s := &Etcd_store{
		actions: make(chan map[string]interface{}),
	}
	go s.run()
	return s
}

// run serializes all operations through a single goroutine.
// IMPORTANT: on "Close", it returns to stop the goroutine cleanly.
func (store *Etcd_store) run() {
	for {
		action := <-store.actions
		switch action["name"] {
		case "Open":
			action["result"].(chan error) <- store.open(action["address"].(string))

		case "SetItem":
			if action["val"] != nil {
				action["result"].(chan error) <- store.setItem(action["key"].(string), action["val"].([]byte))
			} else {
				action["result"].(chan error) <- store.setItem(action["key"].(string), nil)
			}

		case "GetItem":
			val, err := store.getItem(action["key"].(string))
			if err != nil {
				err = errors.New("etcd: item not found key=" + action["key"].(string) + " err=" + err.Error())
			}
			action["results"].(chan map[string]interface{}) <- map[string]interface{}{"val": val, "err": err}

		case "RemoveItem":
			action["result"].(chan error) <- store.removeItem(action["key"].(string))

		case "Close":
			action["result"].(chan error) <- store.close()
			return // stop loop cleanly
		}
	}
}

//////////////////////// Public (synchronized) API /////////////////////////

// Open connects to etcd. Address can be empty (load from etcd.yml) or a
// comma-separated list of endpoints: "host1:2379,host2:2379".
func (store *Etcd_store) Open(address string) error {
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "address": address}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Close shuts down the client and terminates the run loop.
func (store *Etcd_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// SetItem sets key -> val.
func (store *Etcd_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// GetItem fetches val for key.
func (store *Etcd_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}
	return results["val"].([]byte), nil
}

// RemoveItem deletes key.
func (store *Etcd_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear is not supported by etcd KV (would require range delete).
func (store *Etcd_store) Clear() error { return errors.New("etcd: clear not supported") }

// Drop is not supported by etcd KV.
func (store *Etcd_store) Drop() error { return errors.New("etcd: drop not supported") }
