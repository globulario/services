# Operational Knowledge YAML Schema

Every YAML file in `stages/`, `runbooks/`, and `service-roles/` follows this schema. The build-time validator (`globular ops-knowledge validate`) enforces it and computes per-entry `seed_sha256` hashes.

## Top-level structure

```yaml
schema_version: 1
file_kind: stage | runbook | service-role
metadata:
  title: "Human-readable title for the file"
  description: "One-paragraph context"
  source_documents:                   # links to authoritative human docs this seed condenses
    - docs/operators/day-0-1-2-operations.md
entries:                              # required; one or more entries
  - id: ...
    ...
```

## Entry schema

```yaml
- id: ops.<stage>.<topic>.<verb-noun>      # required; kebab-cased, dotted, stable across releases
  type: ARCHITECTURE | DECISION | REFERENCE | SKILL | DEBUG  # required; maps to AI Memory MemoryType
  title: "Short title (~60 chars)"          # required
  tags:                                     # required; first tag MUST be the lifecycle stage
    - day-0 | day-1 | day-2 | always
    - <subsystem>                           # e.g. objectstore, keepalived, etcd, ai-memory
    - <topic>                               # e.g. topology, contract, integrity
  applies_when:                             # required; AI session-start filters by these
    cluster_phases: [day-0, day-1, day-2]   # which phases the entry is valid in
    services_present: []                    # services that must be running for entry to apply (empty = always)
    services_healthy: []                    # services that must be healthy (subset of services_present)
  content: |                                # required; the body
    Markdown-formatted.
    Use as much detail as needed but keep it scoped to one concept.
  links:                                    # optional but encouraged
    awareness_invariants: []                # ids from docs/awareness/invariants.yaml
    awareness_failure_modes: []             # ids from docs/awareness/failure_modes.yaml
    runbooks: []                            # paths to other runbook files in this dir
    cli_commands: []                        # representative CLI commands the entry references
  related_ids: []                           # other seed-entry ids this links to (bidirectional in AI Memory)
  provenance:                               # required; build tool fills these
    source: seed
    seed_version: ""                        # filled by build (semver of the bundle)
    seed_sha256: ""                         # filled by build (SHA256 of canonical-form entry)
    immutable: true                         # always true for seed entries
```

## Validation rules

The validator MUST reject a YAML file if any of:

| Rule | Reason |
|---|---|
| `id` is missing, empty, or duplicates another id in any seed file | ids must be globally unique and stable |
| `id` does not start with `ops.` | namespace prevents collision with non-seed memories |
| `type` not in {ARCHITECTURE, DECISION, REFERENCE, SKILL, DEBUG} | must map to a real `MemoryType` (per `docs/ai/ai-services.md`) |
| First `tag` is not one of `day-0`, `day-1`, `day-2`, `always` | every entry must declare its lifecycle scope as the first tag |
| `applies_when.cluster_phases` is empty | cannot determine when the entry is relevant |
| `applies_when.services_healthy` references a service not in `services_present` | inconsistent declaration |
| `links.awareness_invariants` references an id not in `docs/awareness/invariants.yaml` | dangling reference |
| `links.awareness_failure_modes` references an id not in `docs/awareness/failure_modes.yaml` | dangling reference |
| `links.runbooks` references a file not in `docs/operational-knowledge/runbooks/` | dangling reference |
| `provenance.source` is anything other than `seed` | only seed entries belong in this directory |
| Total content size of one entry exceeds 16 KiB | seed entries must be concise; long-form belongs in `docs/` |

## Stamping rules (build tool)

When the awareness bundle is built, the tool walks every entry and:

1. Removes `provenance.seed_version` and `provenance.seed_sha256` (so they don't influence the hash)
2. Re-serializes the rest in canonical YAML form (sorted keys, no trailing newlines, UTF-8)
3. Computes `SHA256` over the canonical bytes
4. Writes the hash back into `provenance.seed_sha256`
5. Stamps `provenance.seed_version` with the awareness bundle's semver

The seeder workflow (day-1) verifies the hash on every load. ai-memory rejects writes whose computed hash doesn't match the manifest hash.

## File-kind specifics

### `file_kind: stage`

Stage files describe **what is true at this lifecycle stage**. They are usually `ARCHITECTURE` or `REFERENCE` entries — not procedures.

Examples of good stage entries:
- "After day-0 bootstrap, etcd is single-node and the bootstrap flag is still set"
- "After day-1 keepalived enable, the VIP is held by one node and propagates via VRRP"
- "MinIO topology contract: the controller is the sole authority for `/globular/objectstore/config`"

### `file_kind: runbook`

Runbook files describe **how to do a procedure or recover from a known incident**. They are usually `SKILL` entries.

Schema mirrors `docs/runbooks/objectstore-nfs-remediation.yaml`:

```yaml
entries:
  - id: ops.runbook.minio.add-node-to-pool
    type: SKILL
    title: "Add a node to the MinIO pool"
    tags: [day-1, objectstore, runbook]
    applies_when:
      cluster_phases: [day-1, day-2]
      services_present: [cluster-controller, repository, ai-memory]
    content: |
      # Long-form runbook content uses the phased structure below.
    procedure:                                    # only for SKILL/runbook entries
      - phase: observe
        description: "Confirm preconditions"
        commands:
          - cmd: "globular cluster nodes list"
            expect: "target node has 'storage' profile"
      - phase: plan
        description: "Generate proposal"
        commands:
          - cmd: "globular objectstore disk approve --node {{node_id}} --node-ip {{stable_ip}} --path /mnt/data"
      - phase: execute
        description: "Apply the topology"
        warning: "Destructive — wipes .minio.sys on all pool nodes"
        commands:
          - cmd: "globular objectstore topology apply --proposal {{proposal_id}} --i-understand-data-reset"
      - phase: verify
        description: "Confirm convergence"
        commands:
          - cmd: "globular objectstore topology status"
            expect: "Overall: ✓ CONVERGED"
    success_criteria:
      - "ai-memory reports the new node in the verified pool"
      - "doctor finding objectstore_topology_mismatch is cleared"
```

### `file_kind: service-role`

Service-role files describe **what one service IS for and what it owns**. They are usually `ARCHITECTURE` entries — short, definitional.

Examples:
- "cluster-controller: sole authority for `/globular/objectstore/config`; never directly executes shell commands"
- "node-agent: only service allowed to use `os/exec`, scoped to `internal/supervisor/`"

## Querying a loaded seed

Once a seed entry is in AI Memory, query like any other memory:

```bash
# All seed entries for day-1:
globular memory query --project globular-services --type SKILL --tags day-1,seed

# What does the cluster know about MinIO topology?
globular memory query --project globular-services --tags objectstore,topology

# Show only seed entries (filter by provenance):
globular memory query --project globular-services --metadata source=seed
```

Or, in Claude / MCP:

```
mcp__globular__memory_query(project="globular-services", tags="day-1,objectstore,seed")
```

## Versioning

Seed `id`s are stable across releases. The `provenance.seed_version` field tracks WHICH release a given memory was last seeded from. When the bundle's seed_version increments, the seeder workflow upserts: same id + new sha → update content; same id + same sha → noop.

A seed `id` may be retired but never reused. Once retired, the seeder writes a tombstone entry (content = "retired in vX.Y.Z, see <new id>") so prior references resolve cleanly.
