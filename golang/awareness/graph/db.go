package graph

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS nodes (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL,
    name          TEXT NOT NULL,
    path          TEXT NOT NULL DEFAULT '',
    summary       TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at    INTEGER NOT NULL DEFAULT 0,
    updated_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS edges (
    src           TEXT NOT NULL,
    kind          TEXT NOT NULL,
    dst           TEXT NOT NULL,
    phase         TEXT NOT NULL DEFAULT '',
    required      INTEGER NOT NULL DEFAULT 0,
    confidence    REAL NOT NULL DEFAULT 1.0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    PRIMARY KEY (src, kind, dst, phase)
);

CREATE TABLE IF NOT EXISTS invariants (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    summary       TEXT NOT NULL DEFAULT '',
    severity      TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT '',
    metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS failure_modes (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    summary          TEXT NOT NULL DEFAULT '',
    symptoms_json    TEXT NOT NULL DEFAULT '[]',
    root_cause       TEXT NOT NULL DEFAULT '',
    architecture_fix TEXT NOT NULL DEFAULT '',
    metadata_json    TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS agent_context_cache (
    cache_key        TEXT PRIMARY KEY,
    task             TEXT NOT NULL,
    context_markdown TEXT NOT NULL,
    created_at       INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS graph_builds (
    id          TEXT PRIMARY KEY,
    repo_root   TEXT NOT NULL,
    git_commit  TEXT NOT NULL DEFAULT '',
    release_id  TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL DEFAULT 0,
    stats_json  TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS incidents (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL,
    severity      TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT '',
    started_at    INTEGER NOT NULL DEFAULT 0,
    ended_at      INTEGER NOT NULL DEFAULT 0,
    summary       TEXT NOT NULL DEFAULT '',
    evidence_json TEXT NOT NULL DEFAULT '{}',
    created_at    INTEGER NOT NULL DEFAULT 0,
    updated_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS awareness_proposals (
    id              TEXT PRIMARY KEY,
    incident_id     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL,
    proposal_yaml   TEXT NOT NULL,
    validation_json TEXT NOT NULL DEFAULT '{}',
    created_by      TEXT NOT NULL DEFAULT '',
    created_at      INTEGER NOT NULL DEFAULT 0,
    promoted_at     INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS context_aliases (
    id         TEXT PRIMARY KEY,
    target_id  TEXT NOT NULL,
    alias      TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 1.0,
    source     TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS runtime_snapshots (
    id            TEXT PRIMARY KEY,
    captured_at   INTEGER NOT NULL,
    node_id       TEXT NOT NULL DEFAULT '',
    cluster_id    TEXT NOT NULL DEFAULT '',
    snapshot_json TEXT NOT NULL,
    created_at    INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_runtime_snapshots_captured ON runtime_snapshots(captured_at DESC);

CREATE INDEX IF NOT EXISTS idx_nodes_type ON nodes(type);
CREATE INDEX IF NOT EXISTS idx_nodes_name ON nodes(name);
CREATE INDEX IF NOT EXISTS idx_edges_src  ON edges(src);
CREATE INDEX IF NOT EXISTS idx_edges_dst  ON edges(dst);
CREATE INDEX IF NOT EXISTS idx_edges_kind ON edges(kind);
CREATE INDEX IF NOT EXISTS idx_edges_phase ON edges(phase);
CREATE INDEX IF NOT EXISTS idx_context_aliases_target ON context_aliases(target_id);

CREATE TABLE IF NOT EXISTS preflight_audits (
    id                   TEXT PRIMARY KEY,
    task                 TEXT NOT NULL DEFAULT '',
    timestamp            INTEGER NOT NULL DEFAULT 0,
    git_sha              TEXT NOT NULL DEFAULT '',
    files_json           TEXT NOT NULL DEFAULT '[]',
    forbidden_fixes_json TEXT NOT NULL DEFAULT '[]',
    invariants_json      TEXT NOT NULL DEFAULT '[]',
    code_smells_json     TEXT NOT NULL DEFAULT '[]',
    created_at           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_preflight_audits_ts  ON preflight_audits(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_preflight_audits_sha ON preflight_audits(git_sha);
`

// Graph is the central awareness graph handle backed by SQLite.
type Graph struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite awareness graph at path.
// The parent directory is created if it does not exist.
func Open(path string) (*Graph, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("awareness graph: mkdir %s: %w", filepath.Dir(path), err)
	}
	dsn := path + "?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("awareness graph: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite WAL allows one writer
	g := &Graph{db: db}
	if err := g.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return g, nil
}

// OpenMemory opens a fresh in-memory SQLite awareness graph.
// It is suitable for validation previews: changes are never persisted.
// Each call returns an independent, isolated in-memory database.
func OpenMemory() (*Graph, error) {
	// Each unique name gives an isolated in-memory DB within the process.
	// Using ?cache=shared with a unique name avoids cross-connection leakage.
	db, err := sql.Open("sqlite3", "file::memory:?mode=memory")
	if err != nil {
		return nil, fmt.Errorf("awareness graph (memory): open: %w", err)
	}
	db.SetMaxOpenConns(1)
	g := &Graph{db: db}
	if err := g.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return g, nil
}

// Close closes the underlying database.
func (g *Graph) Close() error { return g.db.Close() }

// DB returns the raw *sql.DB (for tests and bulk operations).
func (g *Graph) DB() *sql.DB { return g.db }

// migrate applies the schema DDL. Safe to call on an existing database.
func (g *Graph) migrate(_ context.Context) error {
	if _, err := g.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("awareness graph: migrate: %w", err)
	}
	return nil
}
