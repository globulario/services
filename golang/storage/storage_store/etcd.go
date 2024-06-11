package storage_store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/config"
	"go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

type Etcd_store struct {

	// The actual client.
	client *clientv3.Client

	address string

	// Sychronized action channel.
	actions chan map[string]interface{}
}

// Open a connection to the store at a given address, with the port.
func NewEtcd_store() *Etcd_store {

	s := new(Etcd_store)
	s.actions = make(chan map[string]interface{})

	// Start the run loop.
	go func(store *Etcd_store) {
		store.run()
	}(s)

	return s
}

// Manage the concurent access of the db.
func (store *Etcd_store) run() {
	for {
		select {
		case action := <-store.actions:
			if action["name"].(string) == "Open" {
				action["result"].(chan error) <- store.open(action["address"].(string))
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
			} else if action["name"].(string) == "Close" {
				action["result"].(chan error) <- store.close()
				break // exit here.
			}
		}
	}
}

func (s *Etcd_store) open(address string) error {

	var err error

	if len(address) == 0 {
		// in that case I will use the address from the config file.
		path := config.GetConfigDir() + "/etcd.yml"

		if len(path) == 0 {
			return errors.New("config file not found")
		}

		// read the config file.
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// parse the config file.
		config_ := make(map[string]interface{})
		err = yaml.Unmarshal(data, &config_)
		if err != nil {
			return err
		}

		// get the address.
		address = config_["initial-advertise-peer-urls"].(string)
		if len(address) == 0 {
			return errors.New("address not found")
		}
	}


	s.client, err = clientv3.New(clientv3.Config{
		Endpoints: []string{address},
	})

	if err != nil {
		return err
	}

	fmt.Println("Connected to etcd server:", address)

	s.address = address

	return nil
}

func (s *Etcd_store) setItem(key string, val []byte) error {

	ctx, cancel :=  context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Set item:", key, string(val))

	rsp, err := s.client.Put(ctx, key, string(val))
	if err != nil {
		return err
	}

	fmt.Println("Set Response:", rsp)

	return nil
}

func (s *Etcd_store) getItem(key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rsp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var value []byte
	for _, ev := range rsp.Kvs {
		value = ev.Value
	}

	return value, nil
}

func (s *Etcd_store) removeItem(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rsp, err := s.client.Delete(ctx, key)
	if err != nil {
		return err
	}

	fmt.Println("Delete Response:", rsp)

	return nil

}

func (s *Etcd_store) close() error {
	return s.client.Close()
}

//////////////////////// Synchronized LevelDB access ///////////////////////////

// Open the store with a give file path.
func (store *Etcd_store) Open(address string) error {
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "address": address}
	store.actions <- action
	err := <-action["result"].(chan error)
	return err
}

// Close the store.
func (store *Etcd_store) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Set item
func (store *Etcd_store) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Get item with a given key.
func (store *Etcd_store) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}

	return results["val"].([]byte), nil
}

// Remove an item
func (store *Etcd_store) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear the data store.
func (store *Etcd_store) Clear() error {
	return errors.New("not supported")
}

// Drop the data store.
func (store *Etcd_store) Drop() error {
	return errors.New("not supported")
}
