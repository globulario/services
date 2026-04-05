// Package projections holds the read-only views over cluster-controller state
// that back Globular's introspection tools (MCP, CLI, UI).
//
// Governed by docs/architecture/projection-clauses.md — in particular:
//   - Single Source of Truth (Clause 1): this package NEVER invents data;
//     every row is re-derivable from cluster-controller's in-memory state.
//   - Reader-Fallback (Clause 3): callers must fall back to the source on
//     miss / stale / error. This package is best-effort; its failures are
//     logged and swallowed, never propagated to the request path.
//   - Deterministic Projection (Clause 10): projectors are pure functions
//     of their input state.
package projections

// Keyspace used by all cluster-controller projections. Shared across the
// node_identity projection (Phase 1) and whatever future projections follow.
// Fresh keyspaces would fragment the schema for little gain — each projection
// owns its own table(s), and the keyspace is just the shared container.
const Keyspace = "globular_projections"

// createKeyspaceCQL creates the projection keyspace if it doesn't exist.
// Replication factor is parameterised; callers pass 1 for single-node dev
// clusters and 3 for production.
const createKeyspaceCQL = `CREATE KEYSPACE IF NOT EXISTS %s
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`

// ── node_identity projection ─────────────────────────────────────────────────
//
// Question answered: "Who is this node?"
// Source of truth:   /globular/clustercontroller/state (etcd, owned by cc)
// Fields:            node_id, hostname, ips[], macs[], labels[], observed_at
// Reverse lookups:   hostname / mac / ip → node_id (plain denormalized tables)
//
// The main table is keyed by node_id. The three `_by_*` tables exist ONLY
// so reverse lookups are a single partition read. No secondary indexes,
// no SASI, no materialized views — boring exact-match PK lookups only.

const createNodeIdentityCQL = `CREATE TABLE IF NOT EXISTS globular_projections.node_identity (
    node_id      text PRIMARY KEY,
    hostname     text,
    macs         set<text>,
    ips          set<text>,
    labels       set<text>,
    observed_at  bigint
)`

const createNodeIdentityByHostnameCQL = `CREATE TABLE IF NOT EXISTS globular_projections.node_identity_by_hostname (
    hostname     text PRIMARY KEY,
    node_id      text,
    observed_at  bigint
)`

const createNodeIdentityByMacCQL = `CREATE TABLE IF NOT EXISTS globular_projections.node_identity_by_mac (
    mac          text PRIMARY KEY,
    node_id      text,
    observed_at  bigint
)`

const createNodeIdentityByIpCQL = `CREATE TABLE IF NOT EXISTS globular_projections.node_identity_by_ip (
    ip           text PRIMARY KEY,
    node_id      text,
    observed_at  bigint
)`

// nodeIdentityTables returns every CREATE TABLE statement the node_identity
// projection requires. Callers exec them in order after keyspace creation.
func nodeIdentityTables() []string {
	return []string{
		createNodeIdentityCQL,
		createNodeIdentityByHostnameCQL,
		createNodeIdentityByMacCQL,
		createNodeIdentityByIpCQL,
	}
}
