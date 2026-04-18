package main

// scylla_store.go — ScyllaDB-backed manifest metadata store.
//
// The repository service stores artifact metadata (manifests + publish state) in
// ScyllaDB for distributed consistency. Binary blobs stay in MinIO.
//
// Schema design:
//   - Primary table: manifests (artifact_key → manifest JSON + state)
//   - Designed for the repository's scale (hundreds/low-thousands of manifests)
//   - Queries: by key (primary), full scan for list/search (acceptable at scale)

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gocql/gocql"
	"github.com/globulario/services/golang/config"
)

const (
	scyllaKeyspace       = "repository"
	scyllaManifestsTable = "manifests"
	scyllaPort           = 9042
)

// schemaCreateTable — executed idempotently at startup (table structure only).
var schemaCreateTable = []string{
	`CREATE TABLE IF NOT EXISTS ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` (
		artifact_key          text,
		manifest_json         blob,
		publish_state         text,
		publisher_id          text,
		name                  text,
		version               text,
		platform              text,
		build_number          bigint,
		checksum              text,
		entrypoint_checksum   text,
		size_bytes            bigint,
		modified_unix         bigint,
		kind                  text,
		channel               text,
		created_at            timestamp,
		PRIMARY KEY (artifact_key)
	)`,
}

// schemaMigrations add columns to existing tables. Run AFTER table creation
// but BEFORE indexes. "Already exists" errors are silently ignored.
var schemaMigrations = []string{
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD entrypoint_checksum text`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD channel text`,
}

// schemaIndexes create secondary indexes. Run AFTER migrations so new columns
// exist before we index them.
var schemaIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_entrypoint_checksum ON ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` (entrypoint_checksum)`,
	`CREATE INDEX IF NOT EXISTS idx_channel ON ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` (channel)`,
}

// manifestRow is a single row from the manifests table.
type manifestRow struct {
	ArtifactKey        string
	ManifestJSON       []byte
	PublishState       string
	PublisherID        string
	Name               string
	Version            string
	Platform           string
	BuildNumber        int64
	Checksum           string
	EntrypointChecksum string
	SizeBytes          int64
	ModifiedUnix       int64
	Kind               string
	Channel            string
	CreatedAt          time.Time
}

// scyllaStore manages the ScyllaDB session for the repository service.
type scyllaStore struct {
	mu      sync.Mutex
	session *gocql.Session
	hosts   []string
	rf      int
}

// connectScylla establishes a connection to ScyllaDB and creates the schema.
// Follows the established pattern: fetch hosts from etcd, adapt RF to cluster
// size, create keyspace + tables idempotently.
func connectScylla() (*scyllaStore, error) {
	hosts, err := config.GetScyllaHosts()
	if err != nil {
		return nil, fmt.Errorf("scylla: %w", err)
	}

	rf := len(hosts)
	if rf > 3 {
		rf = 3
	}
	if rf < 1 {
		rf = 1
	}
	consistency := gocql.Quorum
	if rf < 2 {
		consistency = gocql.One
	}

	// Connect without keyspace first to create keyspace.
	cluster := gocql.NewCluster(hosts...)
	cluster.Port = scyllaPort
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second

	adminSession, err := retryScyllaConnect(cluster, 6)
	if err != nil {
		return nil, fmt.Errorf("scylla admin connect: %w", err)
	}

	// Create keyspace.
	ksQuery := fmt.Sprintf(
		`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`,
		scyllaKeyspace, rf,
	)
	if err := adminSession.Query(ksQuery).Exec(); err != nil {
		adminSession.Close()
		return nil, fmt.Errorf("scylla create keyspace: %w", err)
	}

	// Phase 1: Create tables.
	for _, ddl := range schemaCreateTable {
		if err := adminSession.Query(ddl).Exec(); err != nil {
			adminSession.Close()
			return nil, fmt.Errorf("scylla schema: %w", err)
		}
	}

	// Phase 2: Run migrations (add columns to existing tables).
	// Ignore "already exists" errors — these are idempotent.
	for _, migration := range schemaMigrations {
		if err := adminSession.Query(migration).Exec(); err != nil {
			errMsg := err.Error()
			if !strings.Contains(errMsg, "already exist") && !strings.Contains(errMsg, "conflicts with") {
				adminSession.Close()
				return nil, fmt.Errorf("scylla migration: %w", err)
			}
		}
	}

	// Phase 3: Create indexes (after migrations so columns exist).
	for _, idx := range schemaIndexes {
		if err := adminSession.Query(idx).Exec(); err != nil {
			adminSession.Close()
			return nil, fmt.Errorf("scylla index: %w", err)
		}
	}
	adminSession.Close()

	// Reconnect with keyspace set.
	cluster.Keyspace = scyllaKeyspace
	session, err := retryScyllaConnect(cluster, 6)
	if err != nil {
		return nil, fmt.Errorf("scylla connect (keyspace): %w", err)
	}

	slog.Info("scylladb connected", "hosts", hosts, "keyspace", scyllaKeyspace, "rf", rf)

	return &scyllaStore{
		session: session,
		hosts:   hosts,
		rf:      rf,
	}, nil
}

// retryScyllaConnect attempts to connect with exponential backoff.
func retryScyllaConnect(cluster *gocql.ClusterConfig, maxRetries int) (*gocql.Session, error) {
	backoff := time.Second
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		session, err := cluster.CreateSession()
		if err == nil {
			if attempt > 0 {
				slog.Info("scylla: connection established after retries", "attempts", attempt+1)
			}
			return session, nil
		}
		lastErr = err
		if attempt < maxRetries {
			slog.Warn("scylla: connection attempt failed, retrying",
				"attempt", attempt+1,
				"next_retry_in", backoff.String(),
				"err", err)
			time.Sleep(backoff)
			backoff *= 2
		}
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries+1, lastErr)
}

// Ping verifies the ScyllaDB session is alive.
func (s *scyllaStore) Ping() error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()

	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}
	return sess.Query("SELECT now() FROM system.local").Consistency(gocql.One).Exec()
}

// Close shuts down the session.
func (s *scyllaStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
}

// Reconnect closes the existing session and establishes a new one.
func (s *scyllaStore) Reconnect() error {
	s.Close()

	hosts, err := config.GetScyllaHosts()
	if err != nil {
		return fmt.Errorf("scylla reconnect: %w", err)
	}

	rf := len(hosts)
	if rf > 3 {
		rf = 3
	}
	if rf < 1 {
		rf = 1
	}
	consistency := gocql.Quorum
	if rf < 2 {
		consistency = gocql.One
	}

	cluster := gocql.NewCluster(hosts...)
	cluster.Port = scyllaPort
	cluster.Consistency = consistency
	cluster.Timeout = 10 * time.Second
	cluster.ConnectTimeout = 10 * time.Second
	cluster.Keyspace = scyllaKeyspace

	session, err := retryScyllaConnect(cluster, 3)
	if err != nil {
		return fmt.Errorf("scylla reconnect: %w", err)
	}

	s.mu.Lock()
	s.session = session
	s.hosts = hosts
	s.rf = rf
	s.mu.Unlock()

	slog.Info("scylladb reconnected", "hosts", hosts, "keyspace", scyllaKeyspace, "rf", rf)
	return nil
}

// NodeCount returns the number of ScyllaDB hosts. Used to detect single-node
// clusters where local storage fallback is acceptable.
func (s *scyllaStore) NodeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.hosts)
}

// ── Manifest CRUD ──────────────────────────────────────────────────────────

// PutManifest writes a manifest row to ScyllaDB.
func (s *scyllaStore) PutManifest(ctx context.Context, row manifestRow) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}

	return sess.Query(`INSERT INTO manifests (
		artifact_key, manifest_json, publish_state, publisher_id, name,
		version, platform, build_number, checksum, entrypoint_checksum,
		size_bytes, modified_unix, kind, channel, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ArtifactKey, row.ManifestJSON, row.PublishState,
		row.PublisherID, row.Name, row.Version, row.Platform,
		row.BuildNumber, row.Checksum, row.EntrypointChecksum,
		row.SizeBytes, row.ModifiedUnix, row.Kind, row.Channel, row.CreatedAt,
	).WithContext(ctx).Exec()
}

// GetManifest reads a single manifest by artifact key.
func (s *scyllaStore) GetManifest(ctx context.Context, artifactKey string) (*manifestRow, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}

	row := &manifestRow{}
	err := sess.Query(`SELECT artifact_key, manifest_json, publish_state, publisher_id,
		name, version, platform, build_number, checksum, entrypoint_checksum,
		size_bytes, modified_unix, kind, channel, created_at
		FROM manifests WHERE artifact_key = ?`, artifactKey).
		WithContext(ctx).
		Scan(&row.ArtifactKey, &row.ManifestJSON, &row.PublishState,
			&row.PublisherID, &row.Name, &row.Version, &row.Platform,
			&row.BuildNumber, &row.Checksum, &row.EntrypointChecksum,
			&row.SizeBytes, &row.ModifiedUnix, &row.Kind, &row.Channel, &row.CreatedAt)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// ListManifests returns all manifest rows. At repository scale (hundreds of
// entries) a full table scan is efficient.
func (s *scyllaStore) ListManifests(ctx context.Context) ([]manifestRow, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}

	iter := sess.Query(`SELECT artifact_key, manifest_json, publish_state, publisher_id,
		name, version, platform, build_number, checksum, entrypoint_checksum,
		size_bytes, modified_unix, kind, channel, created_at
		FROM manifests`).WithContext(ctx).Iter()

	var rows []manifestRow
	var row manifestRow
	for iter.Scan(&row.ArtifactKey, &row.ManifestJSON, &row.PublishState,
		&row.PublisherID, &row.Name, &row.Version, &row.Platform,
		&row.BuildNumber, &row.Checksum, &row.EntrypointChecksum,
		&row.SizeBytes, &row.ModifiedUnix, &row.Kind, &row.Channel, &row.CreatedAt) {
		rows = append(rows, row)
		row = manifestRow{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scylla list manifests: %w", err)
	}
	return rows, nil
}

// DeleteManifest removes a manifest row by artifact key.
func (s *scyllaStore) DeleteManifest(ctx context.Context, artifactKey string) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}

	return sess.Query(`DELETE FROM manifests WHERE artifact_key = ?`, artifactKey).
		WithContext(ctx).Exec()
}

// FindByEntrypointChecksum queries the secondary index for manifests matching
// the given entrypoint_checksum. Returns all matches (caller filters by state/platform).
func (s *scyllaStore) FindByEntrypointChecksum(ctx context.Context, checksum string) ([]manifestRow, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}

	iter := sess.Query(`SELECT artifact_key, manifest_json, publish_state, publisher_id,
		name, version, platform, build_number, checksum, entrypoint_checksum,
		size_bytes, modified_unix, kind, channel, created_at
		FROM manifests WHERE entrypoint_checksum = ?`, checksum).
		WithContext(ctx).Iter()

	var rows []manifestRow
	var row manifestRow
	for iter.Scan(&row.ArtifactKey, &row.ManifestJSON, &row.PublishState,
		&row.PublisherID, &row.Name, &row.Version, &row.Platform,
		&row.BuildNumber, &row.Checksum, &row.EntrypointChecksum,
		&row.SizeBytes, &row.ModifiedUnix, &row.Kind, &row.Channel, &row.CreatedAt) {
		rows = append(rows, row)
		row = manifestRow{}
	}
	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("scylla find by entrypoint_checksum: %w", err)
	}
	return rows, nil
}

// UpdatePublishState updates only the publish_state column for an artifact.
func (s *scyllaStore) UpdatePublishState(ctx context.Context, artifactKey, state string) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}

	return sess.Query(`UPDATE manifests SET publish_state = ? WHERE artifact_key = ?`,
		state, artifactKey).WithContext(ctx).Exec()
}
