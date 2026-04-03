// Package shared_index provides a cluster-aware Bleve search index.
//
// Any instance can enqueue index/delete operations via ScyllaDB. A single
// writer (elected via etcd lease) processes the queue, indexes locally with
// Bleve/Scorch at full native speed, and pushes segment snapshots to MinIO.
// All instances poll MinIO for new segments and serve searches from their
// local copy — same mmap performance as a standalone Bleve index.
package shared_index

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

const (
	queueKeyspace = "shared_index"
	queueTable    = "index_queue"
)

// QueueItem represents a pending index or delete operation.
type QueueItem struct {
	ID        gocql.UUID
	IndexName string
	DocID     string
	JsonStr   string
	Data      string // raw payload to store via SetInternal
	IDField   string
	Fields    []string
	Operation string // "index" or "delete"
	CreatedAt time.Time
}

// indexQueue manages the ScyllaDB-backed operation queue.
type indexQueue struct {
	session *gocql.Session
	logger  *slog.Logger
}

func newIndexQueue(logger *slog.Logger) *indexQueue {
	return &indexQueue{logger: logger}
}

func (q *indexQueue) connect(hosts []string) error {
	if len(hosts) == 0 {
		hosts = []string{"127.0.0.1"}
	}
	if h := os.Getenv("SCYLLA_HOSTS"); h != "" {
		hosts = strings.Split(h, ",")
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = 9042
	cluster.Consistency = gocql.One // start with ONE for bootstrap
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	// Create keyspace + table.
	initSess, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla connect: %w", err)
	}

	// Detect actual cluster size to set appropriate replication factor and consistency.
	var peerCount int
	if err := initSess.Query(`SELECT count(*) FROM system.peers`).Consistency(gocql.One).Scan(&peerCount); err != nil {
		peerCount = 0
	}
	nodeCount := peerCount + 1 // system.peers excludes local node

	rf := 3
	if nodeCount < 3 {
		rf = nodeCount
	}
	if rf < 1 {
		rf = 1
	}

	// Work queue uses ONE: items are idempotent and processed by a single
	// elected writer, so strong consistency is unnecessary. ONE keeps the
	// queue functional during partial outages (gocql may not discover all
	// peers immediately after startup).
	cluster.Consistency = gocql.One

	q.logger.Info("index queue: cluster probed", "nodes", nodeCount, "replication_factor", rf, "consistency", cluster.Consistency)

	cql := fmt.Sprintf(`CREATE KEYSPACE IF NOT EXISTS %s
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`, queueKeyspace, rf)
	if err := initSess.Query(cql).Consistency(gocql.One).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create keyspace: %w", err)
	}

	cql = fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
		id         timeuuid,
		index_name text,
		doc_id     text,
		json_str   text,
		data       text,
		id_field   text,
		fields     list<text>,
		operation  text,
		created_at timestamp,
		PRIMARY KEY ((index_name), id)
	) WITH CLUSTERING ORDER BY (id ASC)
	  AND default_time_to_live = 3600`, queueKeyspace, queueTable)
	if err := initSess.Query(cql).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create table: %w", err)
	}
	initSess.Close()

	// Reconnect with keyspace.
	cluster.Keyspace = queueKeyspace
	q.session, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla session: %w", err)
	}

	q.logger.Info("index queue connected", "hosts", hosts, "keyspace", queueKeyspace)
	return nil
}

func (q *indexQueue) close() {
	if q.session != nil {
		q.session.Close()
		q.session = nil
	}
}

// Enqueue adds an index or delete operation to the queue.
func (q *indexQueue) Enqueue(indexName, docID, jsonStr, data, idField string, fields []string, operation string) error {
	if q.session == nil {
		return fmt.Errorf("queue not connected")
	}
	id := gocql.TimeUUID()
	return q.session.Query(
		fmt.Sprintf(`INSERT INTO %s (id, index_name, doc_id, json_str, data, id_field, fields, operation, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, queueTable),
		id, indexName, docID, jsonStr, data, idField, fields, operation, time.Now(),
	).Exec()
}

// DequeueBatch reads up to `limit` pending items for a given index.
// Returns items ordered by time (oldest first).
func (q *indexQueue) DequeueBatch(indexName string, limit int) ([]QueueItem, error) {
	if q.session == nil {
		return nil, fmt.Errorf("queue not connected")
	}
	if limit <= 0 {
		limit = 100
	}

	iter := q.session.Query(
		fmt.Sprintf(`SELECT id, index_name, doc_id, json_str, data, id_field, fields, operation, created_at
			FROM %s WHERE index_name = ? LIMIT ?`, queueTable),
		indexName, limit,
	).Iter()

	var items []QueueItem
	var item QueueItem
	for iter.Scan(&item.ID, &item.IndexName, &item.DocID, &item.JsonStr, &item.Data,
		&item.IDField, &item.Fields, &item.Operation, &item.CreatedAt) {
		items = append(items, item)
		item = QueueItem{}
	}
	if err := iter.Close(); err != nil {
		return items, fmt.Errorf("dequeue scan: %w", err)
	}
	return items, nil
}

// DequeueAllIndexNames returns distinct index names that have pending items.
func (q *indexQueue) DequeueAllIndexNames() ([]string, error) {
	if q.session == nil {
		return nil, fmt.Errorf("queue not connected")
	}
	iter := q.session.Query(
		fmt.Sprintf(`SELECT DISTINCT index_name FROM %s`, queueTable),
	).Iter()
	var names []string
	var name string
	for iter.Scan(&name) {
		names = append(names, name)
	}
	if err := iter.Close(); err != nil {
		return names, fmt.Errorf("list index names: %w", err)
	}
	return names, nil
}

// DeleteProcessed removes a batch of items by their IDs.
func (q *indexQueue) DeleteProcessed(indexName string, ids []gocql.UUID) error {
	if q.session == nil {
		return fmt.Errorf("queue not connected")
	}
	for _, id := range ids {
		if err := q.session.Query(
			fmt.Sprintf(`DELETE FROM %s WHERE index_name = ? AND id = ?`, queueTable),
			indexName, id,
		).Exec(); err != nil {
			return fmt.Errorf("delete %s: %w", id, err)
		}
	}
	return nil
}
