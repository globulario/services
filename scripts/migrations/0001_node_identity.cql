-- Migration 0001: NodeIdentity projection tables
-- Idempotent: all statements use IF NOT EXISTS.
-- Run against the ScyllaDB cluster where cluster-controller projections are stored.
--
-- See: docs/architecture/projection-clauses.md
--      docs/architecture/cluster-introspection-plan.md (Phase 1)

CREATE KEYSPACE IF NOT EXISTS globular_projections
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1};

-- Main table: keyed by node_id.
CREATE TABLE IF NOT EXISTS globular_projections.node_identity (
    node_id      text PRIMARY KEY,
    hostname     text,
    macs         set<text>,
    ips          set<text>,
    labels       set<text>,
    observed_at  bigint
);

-- Reverse lookup: hostname → node_id.
CREATE TABLE IF NOT EXISTS globular_projections.node_identity_by_hostname (
    hostname     text PRIMARY KEY,
    node_id      text,
    observed_at  bigint
);

-- Reverse lookup: MAC → node_id.
CREATE TABLE IF NOT EXISTS globular_projections.node_identity_by_mac (
    mac          text PRIMARY KEY,
    node_id      text,
    observed_at  bigint
);

-- Reverse lookup: IP → node_id.
CREATE TABLE IF NOT EXISTS globular_projections.node_identity_by_ip (
    ip           text PRIMARY KEY,
    node_id      text,
    observed_at  bigint
);
