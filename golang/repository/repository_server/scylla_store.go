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

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/gocql/gocql"
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
	// Phase CLI-B — trusted publisher registry.
	`CREATE TABLE IF NOT EXISTS ` + scyllaKeyspace + `.trusted_publishers (
		publisher_id      text,
		public_key_id     text,
		public_key_pem    blob,
		algorithm         text,
		trust_state       text,
		valid_from_unix   bigint,
		valid_until_unix  bigint,
		created_by        text,
		created_unix      bigint,
		notes             text,
		PRIMARY KEY (publisher_id, public_key_id)
	)`,
	// Phase CLI-B — detached artifact signatures.
	`CREATE TABLE IF NOT EXISTS ` + scyllaKeyspace + `.artifact_signatures (
		artifact_key      text,
		public_key_id     text,
		digest            text,
		algorithm         text,
		signature_bytes   blob,
		signed_by         text,
		signed_at_unix    bigint,
		provenance_ref    text,
		PRIMARY KEY (artifact_key, public_key_id)
	)`,
	// Phase F — package config receipts (one row per node-agent action on
	// one config file). Compound PK with clustering by timestamp DESC so
	// `pkg config conflicts` is one query.
	`CREATE TABLE IF NOT EXISTS ` + scyllaKeyspace + `.config_receipts (
		publisher_id    text,
		name            text,
		platform        text,
		timestamp_unix  bigint,
		node_id         text,
		path            text,
		config_kind     text,
		merge_strategy  text,
		checksum_before text,
		checksum_after  text,
		action          text,
		snapshot_id     text,
		workflow_run_id text,
		reason          text,
		sensitive       boolean,
		build_number    bigint,
		PRIMARY KEY ((publisher_id, name, platform), timestamp_unix, node_id, path)
	) WITH CLUSTERING ORDER BY (timestamp_unix DESC, node_id ASC, path ASC)`,
	// Phase CLI-C — installed-package revision history.
	`CREATE TABLE IF NOT EXISTS ` + scyllaKeyspace + `.installed_revisions (
		publisher_id        text,
		name                text,
		platform            text,
		installed_at_unix   bigint,
		revision_id         text,
		kind                text,
		version             text,
		build_id            text,
		build_number        bigint,
		checksum            text,
		node_id             text,
		previous_revision_id text,
		config_snapshot_id  text,
		service_status_before text,
		service_status_after  text,
		workflow_run_id     text,
		action              text,
		PRIMARY KEY ((publisher_id, name, platform), installed_at_unix, revision_id)
	) WITH CLUSTERING ORDER BY (installed_at_unix DESC, revision_id ASC)`,
}

// schemaMigrations add columns to existing tables. Run AFTER table creation
// but BEFORE indexes. "Already exists" errors are silently ignored.
var schemaMigrations = []string{
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD entrypoint_checksum text`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD channel text`,

	// Repository pipeline state machine — durable observability for the
	// publish pipeline (see artifact_state.go). Independent of publish_state.
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD artifact_state text`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD artifact_state_reason text`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD artifact_state_updated_unix bigint`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD artifact_state_workflow_run_id text`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD blob_key text`,
	`ALTER TABLE ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` ADD build_id text`,
}

// schemaIndexes create secondary indexes. Run AFTER migrations so new columns
// exist before we index them.
var schemaIndexes = []string{
	`CREATE INDEX IF NOT EXISTS idx_entrypoint_checksum ON ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` (entrypoint_checksum)`,
	`CREATE INDEX IF NOT EXISTS idx_channel ON ` + scyllaKeyspace + `.` + scyllaManifestsTable + ` (channel)`,
}

// manifestLedger is the interface that abstracts the ScyllaDB manifest store.
// Defining it here (where the concrete type lives) keeps the interface narrow
// and allows tests to inject a fake implementation without needing a real
// ScyllaDB cluster.
type manifestLedger interface {
	GetManifest(ctx context.Context, artifactKey string) (*manifestRow, error)
	ListManifests(ctx context.Context) ([]manifestRow, error)
	PutManifest(ctx context.Context, row manifestRow) error
	UpdatePublishState(ctx context.Context, artifactKey, state string) error
	DeleteManifest(ctx context.Context, artifactKey string) error
	FindByEntrypointChecksum(ctx context.Context, checksum string) ([]manifestRow, error)

	// Repository artifact pipeline state — durable observability columns added
	// alongside publish_state. See artifact_state.go for the state machine.
	UpdateArtifactState(ctx context.Context, artifactKey string, s scyllaArtifactState) error
	GetArtifactState(ctx context.Context, artifactKey string) (string, error)
}

// scyllaArtifactState holds the columns persisted by UpdateArtifactState.
// Mirrors artifact_state.ArtifactStateFields plus the state header itself.
type scyllaArtifactState struct {
	State         string
	Reason        string
	UpdatedUnix   int64
	WorkflowRunID string
	BlobKey       string
	Checksum      string
	SizeBytes     int64
	BuildID       string
	BuildNumber   int64
	PublisherID   string
	Name          string
	Version       string
	Platform      string
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

	// Repository pipeline state — independent of PublishState.
	// Empty for legacy rows that predate the state machine; backfill /
	// sync lifts those into a concrete state. See artifact_state.go.
	ArtifactState string
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
// Uses connectScylla() so the keyspace and schema are recreated if ScyllaDB
// was restarted from a clean state (e.g. after a node was removed without
// decommissioning, which can cause the remaining node to wipe and re-bootstrap).
func (s *scyllaStore) Reconnect() error {
	s.Close()

	newStore, err := connectScylla()
	if err != nil {
		return fmt.Errorf("scylla reconnect: %w", err)
	}

	s.mu.Lock()
	s.session = newStore.session
	s.hosts = newStore.hosts
	s.rf = newStore.rf
	s.mu.Unlock()

	slog.Info("scylladb reconnected", "hosts", newStore.hosts, "keyspace", scyllaKeyspace, "rf", newStore.rf)
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
		size_bytes, modified_unix, kind, channel, created_at, artifact_state
		FROM manifests WHERE artifact_key = ?`, artifactKey).
		WithContext(ctx).
		Scan(&row.ArtifactKey, &row.ManifestJSON, &row.PublishState,
			&row.PublisherID, &row.Name, &row.Version, &row.Platform,
			&row.BuildNumber, &row.Checksum, &row.EntrypointChecksum,
			&row.SizeBytes, &row.ModifiedUnix, &row.Kind, &row.Channel, &row.CreatedAt,
			&row.ArtifactState)
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
		size_bytes, modified_unix, kind, channel, created_at, artifact_state
		FROM manifests`).WithContext(ctx).Iter()

	var rows []manifestRow
	var row manifestRow
	for iter.Scan(&row.ArtifactKey, &row.ManifestJSON, &row.PublishState,
		&row.PublisherID, &row.Name, &row.Version, &row.Platform,
		&row.BuildNumber, &row.Checksum, &row.EntrypointChecksum,
		&row.SizeBytes, &row.ModifiedUnix, &row.Kind, &row.Channel, &row.CreatedAt,
		&row.ArtifactState) {
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
		size_bytes, modified_unix, kind, channel, created_at, artifact_state
		FROM manifests WHERE entrypoint_checksum = ?`, checksum).
		WithContext(ctx).Iter()

	var rows []manifestRow
	var row manifestRow
	for iter.Scan(&row.ArtifactKey, &row.ManifestJSON, &row.PublishState,
		&row.PublisherID, &row.Name, &row.Version, &row.Platform,
		&row.BuildNumber, &row.Checksum, &row.EntrypointChecksum,
		&row.SizeBytes, &row.ModifiedUnix, &row.Kind, &row.Channel, &row.CreatedAt,
		&row.ArtifactState) {
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

// UpdateArtifactState writes the durable artifact_state columns. All columns
// are written together so a single state transition is atomic at the row
// level. Empty fields in the input still overwrite — callers should populate
// fields they care about (the transition helper carries identity through).
func (s *scyllaStore) UpdateArtifactState(ctx context.Context, artifactKey string, st scyllaArtifactState) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}
	return sess.Query(`UPDATE manifests SET
		artifact_state = ?,
		artifact_state_reason = ?,
		artifact_state_updated_unix = ?,
		artifact_state_workflow_run_id = ?,
		blob_key = ?,
		checksum = ?,
		size_bytes = ?,
		build_id = ?,
		build_number = ?,
		publisher_id = ?,
		name = ?,
		version = ?,
		platform = ?
		WHERE artifact_key = ?`,
		st.State, st.Reason, st.UpdatedUnix, st.WorkflowRunID,
		st.BlobKey, st.Checksum, st.SizeBytes, st.BuildID, st.BuildNumber,
		st.PublisherID, st.Name, st.Version, st.Platform, artifactKey,
	).WithContext(ctx).Exec()
}

// putConfigReceipt persists a single config receipt row.
func (s *scyllaStore) putConfigReceipt(ctx context.Context, r *repopb.PackageConfigReceipt) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}
	return sess.Query(`INSERT INTO config_receipts
		(publisher_id, name, platform, timestamp_unix, node_id, path,
		 config_kind, merge_strategy, checksum_before, checksum_after,
		 action, snapshot_id, workflow_run_id, reason, sensitive, build_number)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.GetPublisherId(), r.GetName(), r.GetPlatform(),
		r.GetTimestampUnix(), r.GetNodeId(), r.GetPath(),
		r.GetConfigKind().String(), r.GetMergeStrategy().String(),
		r.GetChecksumBefore(), r.GetChecksumAfter(),
		r.GetAction().String(), r.GetSnapshotId(),
		r.GetWorkflowRunId(), r.GetReason(), r.GetSensitive(), r.GetBuildNumber(),
	).WithContext(ctx).Exec()
}

// listConfigReceipts returns receipts for one (publisher, name, platform)
// in newest-first order.
func (s *scyllaStore) listConfigReceipts(ctx context.Context, publisherID, name, platform string) ([]*repopb.PackageConfigReceipt, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}
	iter := sess.Query(`SELECT timestamp_unix, node_id, path, config_kind, merge_strategy,
		checksum_before, checksum_after, action, snapshot_id, workflow_run_id,
		reason, sensitive, build_number FROM config_receipts
		WHERE publisher_id = ? AND name = ? AND platform = ?`,
		publisherID, name, platform).WithContext(ctx).Iter()
	var (
		ts, buildNum                                                                   int64
		nodeID, path, kindStr, mergeStr, before, after, actionStr, snap, runID, reason string
		sensitive                                                                      bool
	)
	var out []*repopb.PackageConfigReceipt
	for iter.Scan(&ts, &nodeID, &path, &kindStr, &mergeStr, &before, &after,
		&actionStr, &snap, &runID, &reason, &sensitive, &buildNum) {
		ck := repopb.ConfigKind_CONFIG_KIND_UNSPECIFIED
		if v, ok := repopb.ConfigKind_value[kindStr]; ok {
			ck = repopb.ConfigKind(v)
		}
		ms := repopb.MergeStrategy_MERGE_STRATEGY_UNSPECIFIED
		if v, ok := repopb.MergeStrategy_value[mergeStr]; ok {
			ms = repopb.MergeStrategy(v)
		}
		ac := repopb.ConfigReceiptAction_CONFIG_RECEIPT_ACTION_UNSPECIFIED
		if v, ok := repopb.ConfigReceiptAction_value[actionStr]; ok {
			ac = repopb.ConfigReceiptAction(v)
		}
		out = append(out, &repopb.PackageConfigReceipt{
			NodeId: nodeID, PublisherId: publisherID, Name: name, Platform: platform,
			BuildNumber: buildNum, Path: path, ConfigKind: ck, MergeStrategy: ms,
			ChecksumBefore: before, ChecksumAfter: after, Action: ac,
			SnapshotId: snap, WorkflowRunId: runID, TimestampUnix: ts,
			Reason: reason, Sensitive: sensitive,
		})
	}
	return out, iter.Close()
}

// GetArtifactState returns the artifact_state column. Empty string on
// missing row or unset column (legacy artifacts).
func (s *scyllaStore) GetArtifactState(ctx context.Context, artifactKey string) (string, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return "", fmt.Errorf("scylla: not connected")
	}
	var state string
	err := sess.Query(`SELECT artifact_state FROM manifests WHERE artifact_key = ?`,
		artifactKey).WithContext(ctx).Scan(&state)
	if err != nil {
		// Surface the error so callers can distinguish "missing row" from
		// "unavailable backend" — both should fall back to in-memory cache
		// at the call site.
		return "", err
	}
	return state, nil
}
