package storage_store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
)

// package-level logger; quiet by default. inject via SetScyllaLogger.
var scyLogger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))

// SetScyllaLogger allows the host service to wire a slog logger.
func SetScyllaLogger(l *slog.Logger) {
	if l != nil {
		scyLogger = l
	}
}

type ScyllaStore struct {
	cluster *gocql.ClusterConfig
	session *gocql.Session

	// serialized action loop
	actions chan map[string]interface{}

	// resolved config
	keyspace string
	table    string
}

// open initializes keyspace/table and connects.
// Accepts either a raw (ignored) string or JSON options:
// {
//   "hosts": ["127.0.0.1","10.0.0.12"],   // default: [localIP, "127.0.0.1"]
//   "port": 9042,                          // default: 9042
//   "keyspace": "cache",                   // default: "cache"
//   "table": "kv",                         // default: "kv"
//   "replicationFactor": 3                 // default: 3
// }
func (s *ScyllaStore) open(optionsStr string) error {
	opts := struct {
		Hosts              []string `json:"hosts"`
		Port               int      `json:"port"`
		Keyspace           string   `json:"keyspace"`
		Table              string   `json:"table"`
		ReplicationFactor  int      `json:"replicationFactor"`
	}{}

	// defaults
	local := config.GetLocalIP()
	if local == "" {
		local = "127.0.0.1"
	}
	opts.Hosts = []string{local, "127.0.0.1"}
	opts.Port = 9042
	opts.Keyspace = "cache"
	opts.Table = "kv"
	opts.ReplicationFactor = 3

	// parse user options if provided
	if strings.TrimSpace(optionsStr) != "" {
		_ = json.Unmarshal([]byte(optionsStr), &opts) // best-effort; keep defaults if fields absent
	}
	if len(opts.Hosts) == 0 {
		opts.Hosts = []string{local, "127.0.0.1"}
	}
	if opts.Port <= 0 {
		opts.Port = 9042
	}
	if strings.TrimSpace(opts.Keyspace) == "" {
		opts.Keyspace = "cache"
	}
	if strings.TrimSpace(opts.Table) == "" {
		opts.Table = "kv"
	}
	if opts.ReplicationFactor <= 0 {
		opts.ReplicationFactor = 3
	}

	s.keyspace = opts.Keyspace
	s.table = opts.Table

	// 1) admin connection (system keyspace) to ensure keyspace exists
	admin := gocql.NewCluster(opts.Hosts...)
	admin.Port = opts.Port
	admin.Keyspace = "system"
	admin.Timeout = 10 * time.Second
	admin.Consistency = gocql.Quorum

	adminSession, err := admin.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla: admin session: %w", err)
	}
	defer adminSession.Close()

	// SimpleStrategy for single DC; adapt if you need NetworkTopologyStrategy
	createKeyspace := fmt.Sprintf(`
		CREATE KEYSPACE IF NOT EXISTS "%s"
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}
	`, opts.Keyspace, opts.ReplicationFactor)

	if err := adminSession.Query(createKeyspace).Consistency(gocql.All).Exec(); err != nil {
		return fmt.Errorf("scylla: create keyspace: %w", err)
	}

	// 2) app connection to target keyspace
	cluster := gocql.NewCluster(opts.Hosts...)
	cluster.Port = opts.Port
	cluster.Keyspace = opts.Keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.RetryPolicy = &gocql.SimpleRetryPolicy{NumRetries: 3}
	cluster.ProtoVersion = 4

	session, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla: create session: %w", err)
	}
	s.cluster = cluster
	s.session = session

	// 3) ensure table exists
	createTable := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS "%s"."%s" (
			key text PRIMARY KEY,
			value blob
		)
	`, opts.Keyspace, opts.Table)

	if err := s.session.Query(createTable).Consistency(gocql.All).Exec(); err != nil {
		s.session.Close()
		s.session = nil
		return fmt.Errorf("scylla: create table: %w", err)
	}

	scyLogger.Info("scylla connected",
		"hosts", opts.Hosts, "port", opts.Port,
		"keyspace", opts.Keyspace, "table", opts.Table,
		"rf", opts.ReplicationFactor)

	return nil
}

func (s *ScyllaStore) setItem(key string, val []byte) error {
	if s.session == nil {
		return errors.New("scylla: setItem on nil session")
	}
	return s.session.Query(
		fmt.Sprintf(`INSERT INTO "%s"."%s" (key, value) VALUES (?, ?)`, s.keyspace, s.table),
		key, val,
	).Consistency(gocql.Quorum).Exec()
}

func (s *ScyllaStore) getItem(key string) ([]byte, error) {
	if s.session == nil {
		return nil, errors.New("scylla: getItem on nil session")
	}
	var value []byte
	if err := s.session.Query(
		fmt.Sprintf(`SELECT value FROM "%s"."%s" WHERE key = ?`, s.keyspace, s.table),
		key,
	).Consistency(gocql.Quorum).Scan(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func (s *ScyllaStore) removeItem(key string) error {
	if s.session == nil {
		return errors.New("scylla: removeItem on nil session")
	}
	return s.session.Query(
		fmt.Sprintf(`DELETE FROM "%s"."%s" WHERE key = ?`, s.keyspace, s.table),
		key,
	).Consistency(gocql.Quorum).Exec()
}

func (s *ScyllaStore) clear() error {
	if s.session == nil {
		return errors.New("scylla: clear on nil session")
	}
	return s.session.Query(
		fmt.Sprintf(`TRUNCATE "%s"."%s"`, s.keyspace, s.table),
	).Consistency(gocql.All).Exec()
}

func (s *ScyllaStore) drop() error {
	if s.session == nil {
		return errors.New("scylla: drop on nil session")
	}
	return s.session.Query(
		fmt.Sprintf(`DROP TABLE IF EXISTS "%s"."%s"`, s.keyspace, s.table),
	).Consistency(gocql.All).Exec()
}

func (s *ScyllaStore) close() error {
	if s.session == nil {
		return nil
	}
	s.session.Close()
	s.session = nil
	scyLogger.Info("scylla closed")
	return nil
}
