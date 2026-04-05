# Cluster Introspection Infrastructure — Implementation Plan

**Status:** Draft
**Owner:** Dave + Claude
**Created:** 2026-04-05

## Motivation

Debugging Globular today requires the operator (or AI agent) to correlate data
across etcd / MinIO / ScyllaDB / service configs / journalctl logs, using
implicit schemas and conventions that are not discoverable from the outside.
Today's claude-as-SERVICE mess and the 4h install-loop debug session both
started with "I don't know which source of truth owns this fact, or how to
look it up fast." This plan introduces five read-projections and one proto
extension to give operators and ai-executor a small, stable, queryable surface
over existing cluster state — **without creating any new primary data stores
except a time-series snapshot archive.**

## Goals

1. **Single tool-call answers** for common questions: "what's this node's
   hostname/MAC/IP", "what kind is package X", "what's failing and how do I
   fix it"
2. **Stable query API** that doesn't break when underlying etcd schemas move
3. **AI-friendly** — every projection is exposed via MCP and typed return
   values
4. **No new primary sources of truth** — every projection is derivable from
   existing state; if the projection is lost or wrong, we re-derive

## Non-goals

- Replacing any existing etcd key. Sources of truth stay where they are.
- Caching for performance. If the underlying service is healthy, direct
  queries remain supported as fallback.
- Replacing cluster-doctor findings with structured-only remediation — text
  remediation stays as the human-readable form.

---

## Architectural discipline (mandatory for all phases)

Every projection table we add MUST follow this contract:

```
 [writer]──▶[source of truth]──▶[projector]──▶[ScyllaDB view]
                    ▲                              │
             [reconciler] ◀────────────────────────┘
                    ▲
             [reader fallback]
```

1. **Single writer path**: one service owns writing to the source of truth.
2. **Synchronous projector**: writes the ScyllaDB row AFTER the source write,
   in the same handler.
3. **Background reconciler**: re-derives the full projection from the source
   every 5 min. Catches projector bugs and missed events.
4. **Reader fallback**: every consumer can query the source of truth directly.
   ScyllaDB is for speed, not correctness.
5. **No cross-service writes to projections**: if service A's data ends up in
   projection B, service A owns the projector, not B.

Violations fail code review.

---

## Phase 1 — Node Identity Resolver

**Effort:** 1 day
**Unlocks:** every other phase (they all need to resolve identifiers)

> **Contract**: strictly governed by the 12 clauses in
> [projection-clauses.md](./projection-clauses.md). This is the first
> projection — it sets the pattern. No deviations, no "just this once".

### Surface

| Interface | Signature |
|-----------|-----------|
| MCP tool | `node_resolve(identifier: string) → NodeIdentity` |
| CLI | `globular node resolve <identifier>` |
| gRPC (cluster-controller) | `ResolveNode(ResolveNodeRequest) → ResolveNodeResponse` |

Where `identifier` is any of: `node_id` (uuid), `hostname`, `mac`, `ip`.

### Return shape (strict, per projection-clauses.md)

The **NodeIdentity** projection answers exactly one question:
*"Who is this node?"*

```json
{
  "node_id":     "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
  "hostname":    "globule-ryzen",
  "ips":         ["10.0.0.63"],
  "macs":        ["e0:d4:64:f0:86:f6"],
  "labels":      ["control-plane", "core", "gateway"],
  "source":      "cluster-controller",
  "observed_at": 1712345678
}
```

**MUST NOT include**: services, packages, metrics, logs, health status,
heartbeat age, install state, or anything else. Callers chain into other
projections (`node_health`, `pkg_info`, …) when they need those.

### Field authority

| Field | Authoritative source | Written by |
|-------|---------------------|------------|
| `node_id` | cluster-controller state (etcd) | join workflow |
| `hostname` | node-agent `ReportStatus.identity.hostname` | node-agent |
| `macs` | node-agent `ReportStatus.identity.mac` (array) | node-agent |
| `ips` | node-agent `ReportStatus.identity.ips` | node-agent |
| `labels` | cluster-controller `SetNodeProfiles` (stored as profiles) | operator/UI |

The projector reflects these as received. No transformation, no enrichment.

### Freshness & origin (Clause 4)

The `source` and `observed_at` fields ARE the projection's freshness stamp.
Per the clauses, they live inside the projection — not in a wrapper envelope.

- `source`: one of `scylla` | `cluster-controller` | `node-agent`
  - `scylla` — answer came from the read projection
  - `cluster-controller` — answer came from the RPC fallback
  - `node-agent` — answer came from asking the node directly (last resort)
- `observed_at`: unix timestamp (seconds) when the underlying data was last
  written at the source

A caller that sees `now - observed_at > 60` SHOULD re-query with a
fallback hint. The tool never lies about how fresh the data is.

**Size budget** (Clause 6): one row fits comfortably under 1 KB. A cluster
of 30 nodes with 4 IPs each and 6 labels still clears 3 KB only when
someone asks for the full list (which is scoped by filter, per Clause 5).

### Data model

```sql
-- ScyllaDB keyspace: globular
CREATE TABLE node_identity (
    node_id      text PRIMARY KEY,
    hostname     text,
    macs         set<text>,
    ips          set<text>,
    labels       set<text>,
    observed_at  timestamp
);

-- Reverse lookups are plain denormalized tables. No SASI, no secondary
-- indexes, no clever tricks — exact-match PK only. Boring is good.
CREATE TABLE node_identity_by_hostname (
    hostname    text PRIMARY KEY,
    node_id     text
);

CREATE TABLE node_identity_by_mac (
    mac         text PRIMARY KEY,
    node_id     text
);

CREATE TABLE node_identity_by_ip (
    ip          text PRIMARY KEY,
    node_id     text
);
```

All four tables are written atomically-ish from the same projector call
(batched LOGGED). Reconciler rebuilds them in lockstep from the source every
5 min, so a dropped write corrects within one cycle.

### Source of truth

`/globular/clustercontroller/state` (etcd) — the in-memory `ClusterState`
serialized by cluster-controller.

### Writer + projector

**cluster-controller** `handlers_status.go:ReportStatus` handler:
- After updating `node.LastSeen = reportedAt`
- After `srv.persistState()` (etcd write)
- Call `srv.nodeIdentityProjector.Upsert(node)` (new method)

The projector:
```go
type nodeIdentityProjector struct {
    scylla *gocql.Session
}

func (p *nodeIdentityProjector) Upsert(n *Node) error {
    // Synchronous write to both tables; log & continue on scylla error
    // (never fail ReportStatus because of a projection failure)
}
```

### Reconciler

Background goroutine in cluster-controller, runs every 5 minutes:
1. Read all nodes from in-memory state
2. Compute expected ScyllaDB rows
3. Upsert each; delete ScyllaDB rows whose node_id no longer exists

### Consumers

**MCP tool** (`golang/mcp/tools_node.go:add_node_resolve_tool`):
```
node_resolve(identifier) →
  1. If identifier looks like UUID: query by node_id
  2. If contains ':': query by mac
  3. If dotted-quad: query by ip (via node_identity_by_ip)
  4. Else: query by hostname
  5. Not found in ScyllaDB → fall back to cluster-controller.ResolveNode RPC
```

**CLI** (`golang/globularcli/node_cmds.go` — new `resolve` sub-command):
```bash
globular node resolve globule-nuc
# node_id:   814fbbb9-607f-5144-be1a-a863a0bea1e1
# hostname:  globule-nuc
# mac:       00:1f:c6:9c:d3:34
# ips:       10.0.0.8, 10.0.0.214
# profiles:  control-plane, core, gateway, storage
# status:    ready
# last_seen: 2s ago
```

### Files to create/modify

- `proto/cluster_controller.proto`
  - `ResolveNode(ResolveNodeRequest) → ResolveNodeResponse` RPC
  - `NodeIdentity` message
- `golang/cluster_controller/cluster_controller_server/projections/`
  - `node_identity.go` — projector + reconciler
- `golang/cluster_controller/cluster_controller_server/handlers_status.go`
  - Hook projector after `persistState()`
- `golang/cluster_controller/cluster_controller_server/handlers_node.go` (or new file)
  - `ResolveNode` handler implementation
- `golang/cluster_controller/cluster_controller_server/server.go`
  - Wire up scylla session + projector + reconciler goroutine
- `golang/globularcli/node_cmds.go`
  - New `resolve` sub-command
- `golang/mcp/tools_node.go` (new)
  - `node_resolve` tool
- `scripts/migrations/0001_node_identity.cql` (new)
  - Schema migration

### Tests

- Unit: projector emits correct rows for known inputs
- Unit: reconciler removes stale entries
- Integration: end-to-end — report status → query via all 4 identifier types
- Fallback: ScyllaDB down → CLI still returns correct data via RPC fallback

### Rollout

1. Add schema migration (idempotent `CREATE TABLE IF NOT EXISTS`)
2. Deploy new cluster-controller with projector (non-breaking — writers unchanged)
3. Run reconciler once to backfill
4. Deploy new CLI + MCP tool
5. Monitor: every existing test still passes, RPC fallback never triggered in healthy state

---

## Phase 2 — Package Info Aggregator

**Effort:** 1-2 days
**Unlocks:** prevents kind-mismatch bugs like today's claude-as-SERVICE mess

### Surface

| Interface | Signature |
|-----------|-----------|
| MCP tool | `pkg_info(name: string) → PackageInfo` |
| CLI | `globular pkg info <name>` |
| gRPC (repository) | `DescribePackage(DescribePackageRequest) → PackageInfo` |

### Response shape

```go
type PackageInfo struct {
    Name          string
    Kind          ArtifactKind   // SERVICE, COMMAND, INFRASTRUCTURE, ...
    Publisher     string
    Versions      []ArtifactVersion  // all published versions + build numbers
    Desired       DesiredState       // kind-appropriate: ServiceDesiredVersion | InfrastructureRelease | nil
    Installed     []NodeInstallation // [{node_id, hostname, version, status, installed_at}, ...]
    Failing       []NodeFailure      // nodes where install_state=FAILED
    Spec          *PackageSpec       // raw manifest if available
}
```

### Source of truth

- **Kind / versions**: repository service (MinIO catalog)
- **Desired state**: etcd `/globular/resources/{ServiceDesiredVersion,InfrastructureRelease}/...`
- **Installed state**: etcd `/globular/nodes/<id>/packages/<KIND>/<name>`

### Implementation

Live aggregator — NO ScyllaDB table. Each call does 3 queries in parallel and
merges. Response cache: in-process LRU with 10s TTL.

```go
func (s *repositoryServer) DescribePackage(ctx, req) (*PackageInfo, error) {
    // Parallel:
    //   a) ListArtifactVersions(publisher, name) → kind + versions
    //   b) etcd get ServiceDesiredVersion/<name> OR InfrastructureRelease/<pub>/<name>
    //   c) etcd prefix scan /globular/nodes/*/packages/*/<name>
    // Merge into PackageInfo
}
```

### Files to create/modify

- `proto/repository.proto`
  - `DescribePackage` RPC + `PackageInfo` / `NodeInstallation` / `DesiredState` messages
- `golang/repository/repository_server/describe_package.go` (new)
- `golang/globularcli/pkg_cmds.go`
  - New `info` sub-command
- `golang/mcp/tools_repository.go`
  - `pkg_info` tool

### Integration with Phase 1

`pkg_info` returns `node_id` for each installed/failing entry. UI and AI chain
those into `node_resolve` for friendly display.

---

## Phase 3 — Structured Remediation Actions

**Effort:** 2-3 days
**Unlocks:** ai-watcher Tier 2 (autonomous low-risk fixes); UI "Execute fix"
buttons; CLI-driven repair workflows

### Surface

Extend `cluster_doctor.proto`:

```protobuf
message RemediationStep {
    int32 order = 1;
    string description = 2;   // existing human-readable text
    string cli_command = 3;   // existing
    RemediationAction action = 4;  // NEW: structured action (optional)
}

message RemediationAction {
    ActionType action_type = 1;
    ActionRisk risk = 2;
    map<string, string> params = 3;
    bool idempotent = 4;              // safe to retry?
}

enum ActionType {
    ACTION_UNSPECIFIED = 0;
    SYSTEMCTL_RESTART = 1;      // params: {unit: "globular-node-agent", node_id: "..."}
    SYSTEMCTL_STOP = 2;
    SYSTEMCTL_START = 3;
    FILE_DELETE = 4;            // params: {path: "/var/lib/globular/bin/x.tmp", node_id: "..."}
    ETCD_DELETE = 5;            // params: {key: "/globular/..."}
    ETCD_PUT = 6;               // params: {key, value}
    PACKAGE_REINSTALL = 7;      // params: {package, node_id, version}
    NODE_REMOVE = 8;            // params: {node_id}
    // ...
}

enum ActionRisk {
    RISK_UNSPECIFIED = 0;
    RISK_LOW = 1;     // idempotent, auto-executable (restart, retry, cleanup tmp)
    RISK_MEDIUM = 2;  // requires UI approval (package install, service stop)
    RISK_HIGH = 3;    // CLI-only, human operator (node removal, data deletion)
}
```

### Hand-grenade rules for mutating actions

`ETCD_PUT`, `ETCD_DELETE`, `NODE_REMOVE`, `FILE_DELETE` outside well-known
trash paths — these are permanently excluded from auto-execution regardless
of risk tag. They can appear in findings (as the right answer to a problem),
but executing them always requires:

1. A named rule that produced the action, with narrow invariants declared
2. A successful dry-run first (the rule emits the *intended* result and the
   executor checks the post-condition would match)
3. An explicit operator approval token (never LOW-auto)
4. An audit log entry naming rule + invariants + operator

Even if a rule author tags an `ETCD_DELETE` as `RISK_LOW`, the executor
rejects auto-mode. The tag is advisory; the blocklist is enforcement.

Safe-by-default auto-executable action types (LOW + no approval required):
- `SYSTEMCTL_RESTART` on Globular-managed units
- `SYSTEMCTL_START` (restarting a stopped Globular-managed unit)
- `FILE_DELETE` when path matches `/usr/lib/globular/bin/*.tmp` or
  `/usr/lib/globular/bin/*.bak` (stale install artifacts)

Everything else: at minimum `RISK_MEDIUM` + approval token.

### Execution API

New RPC on cluster-doctor:

```protobuf
rpc ExecuteRemediation(ExecuteRemediationRequest) returns (ExecuteRemediationResponse);

message ExecuteRemediationRequest {
    string finding_id = 1;
    int32 step_index = 2;
    string approval_token = 3;  // required for MEDIUM+; opaque token issued by UI
    bool dry_run = 4;
}
```

The handler:
1. Looks up finding
2. Loads step by index
3. Verifies risk level + approval
4. Dispatches to node-agent (via cluster-controller routing) for node-local actions
5. Logs execution audit

### Work items

- **Proto additions**: `cluster_doctor.proto`, regenerate
- **Risk + approval middleware** in cluster-doctor
- **Executor dispatcher** — per ActionType handler
- **Migrate ~20 existing rules** to emit structured actions alongside text
- **UI** — render "Execute" button for LOW/MEDIUM actions
- **ai-watcher integration** — auto-execute LOW, create incident for MEDIUM

### Non-breaking rollout

Existing `RemediationStep.description` and `cli_command` stay as-is. Adding
`action` as an optional field. Old clients see no change. New clients see both
text and (optionally) structured action.

---

## Phase 4a — Schema Reference

**Effort:** 1-2 days
**Unlocks:** operators + AI can discover "who writes this key, what does it
mean" without reading source

### Approach

Go struct annotations + `go generate` → both markdown doc and ScyllaDB rows.

Example annotation:

```go
// +globular:schema:key="/globular/resources/ServiceDesiredVersion/{name}"
// +globular:schema:writer="globular-cluster-controller"
// +globular:schema:readers="globular-node-agent,globular-repository"
// +globular:schema:invariants="Only set for packages with kind=SERVICE"
type ServiceDesiredVersion struct { ... }
```

A tool (`cmd/schema-extractor/main.go`) parses all Go files for these
annotations and emits:

1. `docs/schema.md` — human-readable reference
2. `scripts/migrations/schema_seed.cql` — rows for `cluster_schema` table

### Data model

```sql
CREATE TABLE cluster_schema (
    key_pattern  text PRIMARY KEY,
    writer       text,
    readers      set<text>,
    description  text,
    invariants   text,
    since_version text,
    source_file  text,
    source_line  int
);
```

### MCP tool

`schema_describe(pattern_or_name: string) → SchemaEntry`

### No new source of truth

The Go code IS the source. ScyllaDB table + markdown are both generated
artifacts. If the code changes and the generator isn't re-run, CI fails.

### Hard-enforcement — no opt-out

Comment pragmas drift into magic dust if the neat parts get documented and
the cursed parts stay in the attic. We prevent that with two CI checks:

1. **Re-generate and diff check** — CI runs `go generate ./...` then `git
   diff --exit-code` on `docs/schema.md` and the seed CQL. If annotations
   changed but generated artifacts weren't committed, build fails.

2. **Coverage check** — a linter (`cmd/schema-lint/main.go`) walks every
   Go package under `golang/` and finds:
   - Types with `etcd.Put` / `etcd.Get` / `clientv3.KV` references in
     their methods
   - Types used as values serialized to keys under `/globular/` prefixes

   Any such type without a `+globular:schema:key=...` annotation fails
   the build with:

   ```
   error: type FooState appears to be an etcd-backed resource but has no
   +globular:schema:key annotation. Add one or justify exclusion with
   //go:schemalint:ignore.
   ```

   Opt-out exists but requires a named justification comment.

This ensures operators and AI see the same schema whether the subsystem is
neat (cluster-controller state) or cursed (legacy service config blobs).

---

## Phase 4b — Cluster Snapshots

**Effort:** 1-2 days
**Unlocks:** `compare to known-good state`, time-travel debugging

### Data model (append-only)

```sql
CREATE TABLE cluster_snapshot (
    snapshot_id timeuuid PRIMARY KEY,
    captured_at timestamp,
    trigger     text,            -- "manual", "scheduled", "post-operation"
    etcd_dump   blob,            -- JSON dump of /globular/ prefix
    node_count  int,
    service_count int,
    metadata    map<text, text>
);

CREATE INDEX ON cluster_snapshot (captured_at);
```

### CLI

```bash
globular cluster snapshot capture            # take a snapshot
globular cluster snapshot list               # list recent
globular cluster snapshot compare <id1> <id2>
globular cluster snapshot diff-to-healthy    # compare to last snapshot tagged "healthy"
```

### Retention

- 7 days full dumps
- 30 days hourly sampled
- 90 days daily sampled
- Older → pruned or exported to MinIO cold storage

### Source of truth note

This IS a new primary data store — but it is strictly append-only archival
data, not derived state. No drift concern because nothing else writes to it
and no operational logic reads from it to make decisions (only for
comparison/audit).

---

## Dependencies between phases

```
  Phase 1 (node_identity)
     │
     ├─▶ Phase 2 (pkg_info uses node_id→hostname)
     │
     ├─▶ Phase 3 (remediation targets nodes by id)
     │
     └─▶ Phase 4b (snapshots reference node_id)

  Phase 4a (schema) is independent, but its MCP tool benefits from 1+2.
```

## Testing strategy

- **Unit tests** for each projector and reconciler (pure functions where
  possible)
- **Integration tests** that simulate source-of-truth changes and verify
  projection consistency within 1s
- **Chaos tests** — kill ScyllaDB mid-write, verify reconciler recovers
- **Cross-service tests** — projector in service A, reader in service B,
  verify transactional correctness

## Migration considerations

- Every new ScyllaDB table ships with a `CREATE TABLE IF NOT EXISTS` migration
- Migrations are idempotent; safe to re-run on existing clusters
- Projectors skip writes if the table doesn't exist yet (never block the
  hot path)
- Reconcilers bootstrap empty tables on first run

## Open questions

1. **Approval token format** for Phase 3 — JWT, opaque UUID, signed? Needs
   RBAC scope.
2. **Per-node ScyllaDB vs cluster-wide** — projections live in the cluster
   keyspace; any node can read. Is there a per-node override needed?
3. **Schema migration ordering** — who runs `CREATE TABLE`? cluster-controller
   on first-writer-elect, or a separate migrations service?
4. **Phase 4a annotation format** — `+globular:schema:` comment pragmas vs
   struct tags vs a sidecar `schema.yaml`?

## Success criteria

After all phases land:

- [ ] `globular node resolve <any-id>` returns all 4 identifiers in <50ms
- [ ] `globular pkg info <name>` shows kind, versions, per-node install status
  in one screen
- [ ] ai-executor can auto-restart a crash-looping service when risk=LOW
- [ ] cluster-doctor findings include at least one structured action per rule
- [ ] `schema_describe("/globular/resources/ServiceDesiredVersion/{name}")`
  returns writer/readers/invariants
- [ ] `cluster snapshot compare` highlights drift between two points in time
- [ ] **No new etcd keys added** — only ScyllaDB projections + proto extensions

## Next action

Build Phase 1 end-to-end as a concrete proof-point for the architectural
discipline. Target: fully working `node_resolve` tool in a single working
session, with projector, reconciler, CLI, MCP tool, and tests.
