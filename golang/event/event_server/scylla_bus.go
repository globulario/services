package main

import (
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/gocql/gocql"
)

// ---------------------------------------------------------------------------
// ScyllaDB-backed event bus
//
// This replaces the in-memory-only pub/sub so that events published on any
// instance are visible to subscribers on every other instance.
//
// Design:
//   - Publish writes to ScyllaDB with a TTL (events auto-expire).
//   - Each instance polls ScyllaDB for new events and dispatches them to its
//     local gRPC subscriber streams.
//   - Subscribe/OnEvent/Quit remain 100% local (tied to gRPC stream affinity).
//   - QueryEvents reads directly from ScyllaDB.
// ---------------------------------------------------------------------------

const (
	eventKeyspace = "globular_events"
	pollInterval  = 200 * time.Millisecond
)

const createEventKeyspaceCQL = `
CREATE KEYSPACE IF NOT EXISTS globular_events
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 3}
`

// Events are partitioned by a time bucket (minute-level) so ScyllaDB can
// efficiently scan only recent rows. The sequence is a globally-unique
// monotonic counter (TimeUUID-based) to avoid collisions across instances.
const createEventsTableCQL = `
CREATE TABLE IF NOT EXISTS globular_events.events (
    bucket   text,
    seq      timeuuid,
    name     text,
    data     blob,
    PRIMARY KEY ((bucket), seq)
) WITH CLUSTERING ORDER BY (seq ASC)
  AND default_time_to_live = 300
`

// scyllaBus manages the ScyllaDB connection and polling loop.
type scyllaBus struct {
	session *gocql.Session
	lastSeq gocql.UUID // last seen TimeUUID for deduplication
	stop    chan struct{}
	stopped atomic.Bool
	logger  *slog.Logger
}

func newScyllaBus(logger *slog.Logger) *scyllaBus {
	return &scyllaBus{
		stop:   make(chan struct{}),
		logger: logger,
	}
}

func (sb *scyllaBus) connect() error {
	// Scylla hosts from etcd (Tier-0).
	hosts, err := config.GetScyllaHosts()
	if err != nil {
		return fmt.Errorf("scylla hosts: %w", err)
	}
	port := 9042

	// Create keyspace + table.
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	initSess, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla init connect: %w", err)
	}
	if err := initSess.Query(createEventKeyspaceCQL).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create keyspace: %w", err)
	}
	if err := initSess.Query(createEventsTableCQL).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create table: %w", err)
	}
	initSess.Close()

	// Reconnect with keyspace.
	cluster.Keyspace = eventKeyspace
	sb.session, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla session: %w", err)
	}

	// Seed lastSeq to now so we don't replay old events on startup.
	sb.lastSeq = gocql.TimeUUID()

	sb.logger.Info("scylla event bus connected", "hosts", hosts, "keyspace", eventKeyspace)
	return nil
}

func (sb *scyllaBus) close() {
	if sb.stopped.CompareAndSwap(false, true) {
		close(sb.stop)
	}
	if sb.session != nil {
		sb.session.Close()
		sb.session = nil
	}
}

// currentBucket returns the time bucket key (minute-level granularity).
func currentBucket() string {
	return time.Now().UTC().Format("2006-01-02T15:04")
}

// previousBucket returns the bucket for the previous minute, so we don't miss
// events published at the boundary.
func previousBucket() string {
	return time.Now().UTC().Add(-1 * time.Minute).Format("2006-01-02T15:04")
}

// publish inserts an event into ScyllaDB.
func (sb *scyllaBus) publish(name string, data []byte) error {
	if sb.session == nil {
		return fmt.Errorf("scylla not connected")
	}
	bucket := currentBucket()
	seq := gocql.TimeUUID()
	return sb.session.Query(
		`INSERT INTO events (bucket, seq, name, data) VALUES (?, ?, ?, ?)`,
		bucket, seq, name, data,
	).Exec()
}

// pollOnce reads all events newer than lastSeq from the current and previous
// time buckets. Returns a slice of events and updates lastSeq.
func (sb *scyllaBus) pollOnce() []pollEvent {
	if sb.session == nil {
		return nil
	}

	buckets := []string{previousBucket(), currentBucket()}
	var events []pollEvent

	for _, bucket := range buckets {
		iter := sb.session.Query(
			`SELECT seq, name, data FROM events WHERE bucket = ? AND seq > ?`,
			bucket, sb.lastSeq,
		).Iter()

		var seq gocql.UUID
		var name string
		var data []byte
		for iter.Scan(&seq, &name, &data) {
			events = append(events, pollEvent{
				seq:  seq,
				name: name,
				data: append([]byte(nil), data...), // defensive copy
			})
			if seq.Time().After(sb.lastSeq.Time()) {
				sb.lastSeq = seq
			}
		}
		if err := iter.Close(); err != nil {
			sb.logger.Warn("poll events error", "bucket", bucket, "err", err)
		}
	}

	return events
}

// queryEvents reads events from ScyllaDB for the QueryEvents RPC.
func (sb *scyllaBus) queryEvents(nameFilter string, afterSeq gocql.UUID, limit int) ([]pollEvent, gocql.UUID) {
	if sb.session == nil {
		return nil, afterSeq
	}
	if limit <= 0 {
		limit = 100
	}

	// Scan the last 10 minutes of buckets.
	now := time.Now().UTC()
	var events []pollEvent
	latestSeq := afterSeq

	for i := 10; i >= 0; i-- {
		bucket := now.Add(-time.Duration(i) * time.Minute).Format("2006-01-02T15:04")
		iter := sb.session.Query(
			`SELECT seq, name, data FROM events WHERE bucket = ? AND seq > ?`,
			bucket, afterSeq,
		).Iter()

		var seq gocql.UUID
		var name string
		var data []byte
		for iter.Scan(&seq, &name, &data) {
			if nameFilter != "" && !strings.HasPrefix(name, nameFilter) {
				continue
			}
			events = append(events, pollEvent{
				seq:  seq,
				name: name,
				data: append([]byte(nil), data...),
			})
			if seq.Time().After(latestSeq.Time()) {
				latestSeq = seq
			}
			if len(events) >= limit {
				break
			}
		}
		_ = iter.Close()
		if len(events) >= limit {
			break
		}
	}

	return events, latestSeq
}

type pollEvent struct {
	seq  gocql.UUID
	name string
	data []byte
}
