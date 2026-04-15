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
// ScyllaDB-backed durable event bus
//
// Semantics (these are the contract — every consumer must understand them):
//
//   Publish success = event is durably stored in ScyllaDB.
//   Publish success ≠ event was delivered to any subscriber.
//
//   Subscribers receive events via local poll + dispatch (best-effort live).
//   Correctness comes from durable replay, not from live delivery.
//
//   Each event service instance maintains a durable cursor in ScyllaDB.
//   On connect (including reconnect), the instance resumes from its last
//   cursor — NOT from "now". Events published during downtime are replayed.
//
//   Consumers must be idempotent. Duplicates are acceptable.
//   Silent loss is not acceptable.
//
//   Events have a TTL of 3600 seconds (1 hour). This is long enough to
//   survive short outages, restarts, and reconnect gaps. Events older than
//   1 hour are garbage-collected by ScyllaDB.
//
// Cursor identity:
//   One cursor per node hostname. Each node runs one event service instance
//   with its own local subscriber set. The cursor tracks how far THIS node's
//   event service has polled. If a node is replaced, the new instance gets
//   the cold-start replay policy (see below). If the same node restarts,
//   it resumes from its persisted cursor.
//
// Cold-start replay policy:
//   When no durable cursor exists (first boot or cursor expired), the
//   instance replays from eventTTL ago (1 hour). This is a bounded replay
//   — events older than the TTL are already garbage-collected by ScyllaDB,
//   so replaying from eventTTL is the maximum possible recovery window.
//   Events published before the TTL horizon are permanently lost to this
//   instance. This is an explicit, documented trade-off.
//
// Cursor commit boundary:
//   The cursor is saved AFTER events are dispatched to local subscribers
//   (queued to gRPC streams). If dispatch fails mid-batch, the cursor is
//   NOT advanced — events will be re-polled on the next tick (duplicates).
//   "Dispatched" means sent to the local gRPC OnEvent stream. It does NOT
//   mean the downstream consumer has processed the event. Consumers must
//   be idempotent because replay and duplicate delivery are expected.
// ---------------------------------------------------------------------------

const (
	eventKeyspace = "globular_events"
	pollInterval  = 200 * time.Millisecond
	eventTTL      = 3600 // seconds — 1 hour retention for operational events

	// ── Storm guardrails ─────────────────────────────────────
	// These limits bound CPU, memory, and I/O per poll tick. The goal is:
	// steady-state polls are cheap (1–2 buckets, handful of events), and
	// catch-up after outage converges progressively without spikes.

	// Maximum minute-buckets scanned in a SINGLE pollOnce() call.
	// In steady state this is 1–2. During catch-up, pollOnce returns
	// after this many buckets and the cursor advances incrementally.
	// Full catch-up from 60 buckets behind takes ~12 ticks (≈2.4s).
	maxBucketsPerPoll = 5

	// Maximum events returned by a single pollOnce() call.
	// Caps memory: 500 events × ~1 KB avg ≈ 500 KB per tick maximum.
	// If a bucket has more than this, remaining events are picked up
	// on the next tick (cursor only advances to last dispatched event).
	maxEventsPerPoll = 500

	// Maximum minute-buckets that the replay window can span.
	// This is an absolute ceiling: 60 buckets = 1 hour = matches TTL.
	// Events beyond this horizon are already garbage-collected by ScyllaDB.
	// Truncation is logged, never silent.
	maxReplayBuckets = 60
)

// createEventKeyspaceCQL returns the CQL for creating the event keyspace.
func createEventKeyspaceCQL(rf int) string {
	return fmt.Sprintf(`
CREATE KEYSPACE IF NOT EXISTS globular_events
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}
`, rf)
}

// Events are partitioned by a time bucket (minute-level) so ScyllaDB can
// efficiently scan only recent rows. The sequence is a TimeUUID for global
// ordering and deduplication.
const createEventsTableCQL = `
CREATE TABLE IF NOT EXISTS globular_events.events (
    bucket   text,
    seq      timeuuid,
    name     text,
    data     blob,
    PRIMARY KEY ((bucket), seq)
) WITH CLUSTERING ORDER BY (seq ASC)
  AND default_time_to_live = 3600
`

// Consumer cursors — one row per event service instance. Stores the last
// processed TimeUUID so reconnect/restart resumes from the right position.
const createCursorsTableCQL = `
CREATE TABLE IF NOT EXISTS globular_events.cursors (
    instance_id text PRIMARY KEY,
    last_seq    timeuuid,
    updated_at  timestamp
) WITH default_time_to_live = 604800
`
// 604800 = 7 days. Cursor must survive multi-day outages/rebuilds.
// After 7 days without update, the cursor expires and the instance
// falls back to cold-start replay (from TTL horizon).

// scyllaBus manages the ScyllaDB connection, polling, and cursor persistence.
type scyllaBus struct {
	session    *gocql.Session
	instanceID string     // unique per event service instance (from service ID)
	lastSeq    gocql.UUID // last processed TimeUUID — durable cursor
	stop       chan struct{}
	stopped    atomic.Bool
	logger     *slog.Logger
}

func newScyllaBus(logger *slog.Logger) *scyllaBus {
	return &scyllaBus{
		stop:   make(chan struct{}),
		logger: logger,
	}
}

// connect establishes the ScyllaDB session and resumes from the last durable
// cursor. If no cursor exists (first boot or cursor expired after 7 days),
// replays from eventTTL ago (1 hour) — the maximum recovery window.
func (sb *scyllaBus) connect() error {
	hosts, err := config.GetScyllaHosts()
	if err != nil {
		return fmt.Errorf("scylla hosts: %w", err)
	}
	port := 9042

	rf := len(hosts)
	if rf > 3 {
		rf = 3
	}
	consistency := gocql.Quorum
	if rf < 2 {
		consistency = gocql.One
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = port
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	// Create keyspace + tables.
	initSess, err := cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla init connect: %w", err)
	}
	if err := initSess.Query(createEventKeyspaceCQL(rf)).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create keyspace: %w", err)
	}
	if err := initSess.Query(createEventsTableCQL).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create events table: %w", err)
	}
	if err := initSess.Query(createCursorsTableCQL).Exec(); err != nil {
		initSess.Close()
		return fmt.Errorf("create cursors table: %w", err)
	}
	initSess.Close()

	// Reconnect with keyspace.
	cluster.Keyspace = eventKeyspace
	sb.session, err = cluster.CreateSession()
	if err != nil {
		return fmt.Errorf("scylla session: %w", err)
	}

	// Derive instance ID from hostname (unique per event service instance).
	if sb.instanceID == "" {
		hostname, _ := config.GetHostname()
		sb.instanceID = "event-" + hostname
	}

	// Resume from durable cursor. If none exists (first boot or cursor
	// expired), replay from eventTTL ago — the maximum recovery window.
	// Anything older is already garbage-collected by ScyllaDB.
	//
	// NOTE: gocql.UUID{} (zero UUID) does NOT have a zero Go time — its
	// Time() returns the UUID epoch (1582-10-15). We detect "no cursor"
	// by comparing to the zero UUID directly, not via Time().IsZero().
	sb.lastSeq = sb.loadCursor()
	if sb.lastSeq == (gocql.UUID{}) {
		replayHorizon := time.Duration(eventTTL) * time.Second
		sb.lastSeq = gocql.UUIDFromTime(time.Now().Add(-replayHorizon))
		sb.logger.Info("no durable cursor found, replaying from TTL horizon",
			"instance", sb.instanceID, "horizon", replayHorizon)
	} else {
		sb.logger.Info("resumed from durable cursor", "instance", sb.instanceID, "cursor_time", sb.lastSeq.Time().Format(time.RFC3339))
	}

	sb.logger.Info("scylla event bus connected", "hosts", hosts, "keyspace", eventKeyspace)
	return nil
}

func (sb *scyllaBus) close() {
	// Persist cursor before shutdown so next connect resumes correctly.
	sb.saveCursor()
	if sb.stopped.CompareAndSwap(false, true) {
		close(sb.stop)
	}
	if sb.session != nil {
		sb.session.Close()
		sb.session = nil
	}
}

// loadCursor reads the last durable cursor from ScyllaDB.
// Returns zero UUID if no cursor exists.
func (sb *scyllaBus) loadCursor() gocql.UUID {
	if sb.session == nil {
		return gocql.UUID{}
	}
	var lastSeq gocql.UUID
	if err := sb.session.Query(
		`SELECT last_seq FROM cursors WHERE instance_id = ?`, sb.instanceID,
	).Scan(&lastSeq); err != nil {
		return gocql.UUID{} // No cursor yet.
	}
	return lastSeq
}

// saveCursor persists the current cursor position to ScyllaDB.
// Called after each poll dispatch and on shutdown.
func (sb *scyllaBus) saveCursor() {
	if sb.session == nil || sb.instanceID == "" {
		return
	}
	if err := sb.session.Query(
		`INSERT INTO cursors (instance_id, last_seq, updated_at) VALUES (?, ?, ?)`,
		sb.instanceID, sb.lastSeq, time.Now().UTC(),
	).Exec(); err != nil {
		sb.logger.Warn("save cursor failed", "err", err)
	}
}

// currentBucket returns the time bucket key (minute-level granularity).
func currentBucket() string {
	return time.Now().UTC().Format("2006-01-02T15:04")
}

// bucketsFrom returns all minute buckets from the given time to now (inclusive).
// This is the replay window — covers any gap between cursor and current time.
// If the gap exceeds maxReplayBuckets, we start from (now - maxReplayBuckets)
// instead, so the loop is bounded. Truncation is logged (not silent).
func bucketsFrom(from time.Time) []string {
	now := time.Now().UTC()
	from = from.UTC().Truncate(time.Minute)
	if from.After(now) {
		return []string{currentBucket()}
	}

	// Clamp the start time so we never iterate more than maxReplayBuckets.
	// Without this, a zero UUID (epoch 1582) or very old cursor would spin
	// through hundreds of millions of minutes and freeze the CPU.
	earliest := now.Add(-time.Duration(maxReplayBuckets-1) * time.Minute)
	if from.Before(earliest) {
		slog.Warn("event replay truncated: cursor too old, clamping to max replay window",
			"cursor_time", from.Format(time.RFC3339),
			"clamped_to", earliest.Format(time.RFC3339),
			"max_buckets", maxReplayBuckets)
		from = earliest
	}

	var buckets []string
	for t := from; !t.After(now); t = t.Add(time.Minute) {
		buckets = append(buckets, t.Format("2006-01-02T15:04"))
	}
	return buckets
}

// publish inserts an event into ScyllaDB.
// Returns nil on success, meaning the event is DURABLY STORED.
// This does NOT mean any subscriber has received it.
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

// pollOnce reads events newer than lastSeq, advancing the cursor incrementally.
// Returns events and advances lastSeq. After dispatch, the caller MUST call
// saveCursor() to persist.
//
// ── Bounded catch-up ──────────────────────────────────────────────────────
// pollOnce scans at most maxBucketsPerPoll buckets and returns at most
// maxEventsPerPoll events per invocation. In steady state (cursor is current),
// this means 1–2 buckets and a handful of events — very cheap. After an outage
// (cursor is behind), catch-up happens progressively: each tick advances the
// cursor by up to maxBucketsPerPoll buckets. A 60-bucket gap converges in
// ~12 ticks (≈2.4 seconds at 200ms interval) with bounded CPU and memory.
//
// ── Ordering ──────────────────────────────────────────────────────────────
// Events within a bucket are ordered by TimeUUID (ASC). Across buckets, we
// iterate chronologically. The `seq > lastSeq` filter ensures global ordering
// as long as TimeUUIDs are monotonic (they are — gocql uses system clock).
func (sb *scyllaBus) pollOnce() []pollEvent {
	if sb.session == nil {
		return nil
	}

	// Get all buckets from cursor to now (clamped to maxReplayBuckets).
	allBuckets := bucketsFrom(sb.lastSeq.Time())

	// Scan at most maxBucketsPerPoll buckets per tick.
	// Remaining buckets are caught up on subsequent ticks.
	scanBuckets := allBuckets
	if len(scanBuckets) > maxBucketsPerPoll {
		scanBuckets = scanBuckets[:maxBucketsPerPoll]
		sb.logger.Info("catch-up mode: scanning partial bucket range",
			"scanning", len(scanBuckets),
			"remaining", len(allBuckets)-len(scanBuckets))
	}

	var events []pollEvent
	hitEventLimit := false

	for _, bucket := range scanBuckets {
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
			if len(events) >= maxEventsPerPoll {
				hitEventLimit = true
				break
			}
		}
		if err := iter.Close(); err != nil {
			sb.logger.Warn("poll events error", "bucket", bucket, "err", err)
		}
		if hitEventLimit {
			sb.logger.Info("poll hit event limit, deferring rest to next tick",
				"events", len(events), "limit", maxEventsPerPoll)
			break
		}
	}

	// If we scanned catch-up buckets but found no events (TTL'd away),
	// advance the cursor past the scanned range so the next poll moves
	// forward. Without this, an old cursor loops forever over empty buckets.
	// IMPORTANT: never advance past completed (past) buckets. The current
	// minute's bucket may still receive new events, so the cursor must NOT
	// skip it — otherwise events written after the poll but within the same
	// minute are permanently lost.
	if len(events) == 0 && len(scanBuckets) > 0 {
		nowBucket := time.Now().UTC().Truncate(time.Minute)
		lastBucket := scanBuckets[len(scanBuckets)-1]
		t, err := time.Parse("2006-01-02T15:04", lastBucket)
		if err == nil {
			advanced := t.Add(time.Minute)
			// Only advance past buckets that are fully in the past.
			// The current bucket is still open for writes.
			if advanced.Before(nowBucket) && advanced.After(sb.lastSeq.Time()) {
				sb.lastSeq = gocql.MinTimeUUID(advanced)
			}
		}
	}

	// NOTE: cursor is NOT saved here. The caller must call saveCursor()
	// AFTER dispatching events to local subscribers. This ensures that if
	// dispatch fails, events will be re-polled on the next tick.
	return events
}

// queryEvents reads events from ScyllaDB for the QueryEvents RPC.
// afterSeq is a true cursor: returns all events strictly after afterSeq,
// scanning from the cursor's time bucket to now. Not "recent-ish" — exact.
func (sb *scyllaBus) queryEvents(nameFilter string, afterSeq gocql.UUID, limit int) ([]pollEvent, gocql.UUID) {
	if sb.session == nil {
		return nil, afterSeq
	}
	if limit <= 0 {
		limit = 100
	}

	// Scan from the cursor's time to now — exact replay.
	buckets := bucketsFrom(afterSeq.Time())
	var events []pollEvent
	latestSeq := afterSeq

	for _, bucket := range buckets {
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
