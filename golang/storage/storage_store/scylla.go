package storage_store

import (
	"encoding/json"
	"fmt"

	"github.com/davecourtois/Utility"
	"github.com/gocql/gocql"
)

type ScyllaStore struct {
	cluster *gocql.ClusterConfig
	session *gocql.Session
}

func NewScylla_store(address string, keySpace string) *ScyllaStore {

	// If no address is provided, get the local IP address.
	if len(address) == 0 {
		address = Utility.MyLocalIP() // Get your local IP address.
	}

	// If no keyspace is provided, use "cache".
	if len(keySpace) == 0 {
		keySpace = "cache" // Set your keyspace name here
	}

	createKeyspaceQuery := `
	CREATE KEYSPACE IF NOT EXISTS ` + keySpace + `
	WITH replication = {
		'class': 'SimpleStrategy',
		'replication_factor': 3
	}
	`

	fmt.Println("ScyllaDB store create keyspace")
	adminCluster := gocql.NewCluster("127.0.0.1") // Replace with your ScyllaDB cluster IP address
	adminCluster.Keyspace = "system"              // Use the 'system' keyspace for administrative tasks
	adminSession, err := adminCluster.CreateSession()
	if err != nil {
		fmt.Println("Error creating admin session:", err)
	}
	defer adminSession.Close()

	if err := adminSession.Query(createKeyspaceQuery).Exec(); err != nil {
		fmt.Println("Error creating keyspace:", err)
	}

	fmt.Println("ScyllaDB store create session")

	// The cluster address...
	cluster := gocql.NewCluster(address) // Set your ScyllaDB cluster address here
	cluster.Keyspace = keySpace          // Set your keyspace name here
	cluster.Consistency = gocql.Quorum

	session, err := cluster.CreateSession()
	if err != nil {
		panic(err)
	}

	return &ScyllaStore{
		cluster: cluster,
		session: session,
	}
}

func (s *ScyllaStore) Open(optionsStr string) error {

	fmt.Println("ScyllaDB store open")
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

func (s *ScyllaStore) SetItem(key string, val []byte) error {
	query := s.session.Query(`INSERT INTO kv (key, value) VALUES (?, ?)`, key, val)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) GetItem(key string) ([]byte, error) {
	var value []byte
	if err := s.session.Query(`SELECT value FROM kv WHERE key = ?`, key).Scan(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *ScyllaStore) RemoveItem(key string) error {
	query := s.session.Query(`DELETE FROM kv WHERE key = ?`, key)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) Clear() error {
	query := s.session.Query(`TRUNCATE kv`)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) Drop() error {
	query := s.session.Query(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, "kv"))
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

func (s *ScyllaStore) Close() error {
	s.session.Close()
	return nil
}
