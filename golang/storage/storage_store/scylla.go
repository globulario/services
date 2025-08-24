package storage_store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"github.com/gocql/gocql"
)

type ScyllaStore struct {
	cluster *gocql.ClusterConfig
	session *gocql.Session

	// Sychronized action channel.
	actions chan map[string]interface{}
}

func NewScylla_store(address string, keySpace string, replicationFactor int) *ScyllaStore {

	// If no address is provided, get the local IP address.
	if len(address) == 0 {
		address = config.GetLocalIP() // Get your local IP address.
	}

	// If no keyspace is provided, use "cache".
	if len(keySpace) == 0 {
		keySpace = "cache" // Set your keyspace name here
	}

	createKeyspaceQuery := `
	CREATE KEYSPACE IF NOT EXISTS ` + keySpace + `
	WITH replication = {
		'class': 'SimpleStrategy',
		'replication_factor': ` + Utility.ToString(replicationFactor) + `
	}
	`

	adminCluster := gocql.NewCluster()                  // Replace with your SCYLLA cluster IP address
	adminCluster.Hosts = []string{address, "127.0.0.1"} // add local host as well.
	adminCluster.Keyspace = "system"                    // Use the 'system' keyspace for administrative tasks
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		fmt.Println("Error creating admin session:", err)
	}
	defer adminSession.Close()

	if err := adminSession.Query(createKeyspaceQuery).Exec(); err != nil {
		fmt.Println("Error creating keyspace:", err)
	}

	// The cluster address...
	cluster := gocql.NewCluster(address) // Set your SCYLLA cluster address here
	cluster.Keyspace = keySpace          // Set your keyspace name here
	cluster.Consistency = gocql.Quorum
	cluster.Hosts = []string{address, "127.0.0.1"}
	cluster.Port = 9042
	session, err := cluster.CreateSession()
	if err != nil {
		panic(err)
	}

	s := &ScyllaStore{
		cluster: cluster,
		session: session,
		actions: make(chan map[string]interface{}),
	}

	go func(store *ScyllaStore) {
		store.run()
	}(s)

	return s
}

// Manage the concurent access of the db.
func (store *ScyllaStore) run() {
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

func (s *ScyllaStore) open(optionsStr string) error {

	fmt.Println("SCYLLA store open")
	options := make(map[string]interface{})

	if err := json.Unmarshal([]byte(optionsStr), &options); err != nil {
		return err
	}

	// Define the CQL query to create the table.
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS kv (
			key text PRIMARY KEY,
			value blob
		)
	`

	// Execute the CREATE TABLE query.
	if err := s.session.Query(createTableQuery).Exec(); err != nil {
		fmt.Println("Error creating table:", err)
		return err
	}

	return nil
}

func (s *ScyllaStore) setItem(key string, val []byte) error {
	query := s.session.Query(`INSERT INTO kv (key, value) VALUES (?, ?)`, key, val)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) getItem(key string) ([]byte, error) {
	var value []byte
	if err := s.session.Query(`SELECT value FROM kv WHERE key = ?`, key).Scan(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *ScyllaStore) removeItem(key string) error {
	query := s.session.Query(`DELETE FROM kv WHERE key = ?`, key)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) clear() error {
	query := s.session.Query(`TRUNCATE kv`)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) drop() error {
	query := s.session.Query(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, "kv"))
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) close() error {
	s.session.Close()
	return nil
}

//////////////////////// Synchronized LevelDB access ///////////////////////////

// Open the store with a give file path.
func (store *ScyllaStore) Open(path string) error {
	path = strings.ReplaceAll(path, "\\", "/")
	action := map[string]interface{}{"name": "Open", "result": make(chan error), "path": path}
	store.actions <- action
	err := <-action["result"].(chan error)
	return err
}

// Close the store.
func (store *ScyllaStore) Close() error {
	action := map[string]interface{}{"name": "Close", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Set item
func (store *ScyllaStore) SetItem(key string, val []byte) error {
	action := map[string]interface{}{"name": "SetItem", "result": make(chan error), "key": key, "val": val}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Get item with a given key.
func (store *ScyllaStore) GetItem(key string) ([]byte, error) {
	action := map[string]interface{}{"name": "GetItem", "results": make(chan map[string]interface{}), "key": key}
	store.actions <- action
	results := <-action["results"].(chan map[string]interface{})
	if results["err"] != nil {
		return nil, results["err"].(error)
	}

	return results["val"].([]byte), nil
}

// Remove an item
func (store *ScyllaStore) RemoveItem(key string) error {
	action := map[string]interface{}{"name": "RemoveItem", "result": make(chan error), "key": key}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Clear the data store.
func (store *ScyllaStore) Clear() error {
	action := map[string]interface{}{"name": "Clear", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}

// Drop the data store.
func (store *ScyllaStore) Drop() error {
	action := map[string]interface{}{"name": "Drop", "result": make(chan error)}
	store.actions <- action
	return <-action["result"].(chan error)
}
