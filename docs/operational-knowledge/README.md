# Operational Knowledge — Day-0 Seed for AI Memory

This directory holds the **operational knowledge baseline** that ships with every Globular release. It is the chicken-and-egg breaker for AI Memory: before any incident has happened, the cluster already knows what each service is for, what the normal lifecycle stages look like, what the standard runbooks are, and which patterns recur often enough to be named.

## Why this exists

`AI Memory` is designed to learn from lived experience: incident → diagnosis → action → outcome → memory. That works once the cluster has run a while. It does not work on day-0, when:

- the cluster is being installed for the first time
- AI services are not yet healthy (memory backend ScyllaDB needs quorum)
- there is no prior session, no prior incident, no prior fix
- the operator (and any AI assistant) is fully blind to project-level context

The result, today, is that every fresh AI session repeats the same discovery work that previous sessions did — and writes nothing back. This directory fixes that by providing a **signed, versioned, immutable** seed that day-0/day-1 install pushes into AI Memory.

## What lives here

```
docs/operational-knowledge/
├── README.md                # this file
├── SCHEMA.md                # YAML schema reference
├── packages.md              # canonical reference: what a package is, files, install, validation
├── dns-records.md           # canonical reference: managing DNS records (A, AAAA, MX, TXT, SRV, CAA, etc.)
├── awg-operator-guide.md    # canonical reference: using AWG (standalone sidecar) — 7 tools, CLI, write path
├── build-and-release.md     # canonical reference: local dist build (build-release.sh) + GitHub CI (ci/docs/release.yml)
├── stages/                  # lifecycle-stage seed entries (what's true at this stage)
│   ├── day-0-bootstrap.yaml           ✓ shipped
│   ├── day-1-join.yaml                ✓ shipped (join procedure, stale dep-block recovery)
│   ├── day-1-keepalived.yaml          ✓ shipped
│   ├── day-1-objectstore.yaml         ✓ shipped
│   ├── day-1-deploy-pipeline.yaml     ✓ shipped
│   ├── package-system.yaml            ✓ shipped (anatomy, identity, install, validation, anti-patterns)
│   ├── profile-system.yaml            ✓ shipped (intent, invariants, normalization, enforcement)
│   ├── security-system.yaml           ✓ shipped (token/cert/key policy and lifecycle boundaries)
│   ├── grpc-service-backbone.yaml     ✓ shipped (service/client/interceptor architecture blueprint)
│   ├── service-version-management.yaml ✓ shipped (zz_version_generated + ldflags contract)
│   ├── installed-artifact-system.yaml ✓ shipped (filesystem layout, authority boundary, permission model,
│   │                                              StableIP/VIP template rule)
│   ├── awareness-graph-operations.yaml ✓ shipped (build pipeline, cluster refresh, annotation authoring)
│   └── day-2-maintenance.yaml         ✓ shipped (incl. etcd NOSPACE alarm recovery)
├── runbooks/                # codified procedures
│   ├── add-node-to-minio-pool.yaml              ✓ shipped
│   ├── recover-stuck-topology-apply.yaml        ✓ shipped
│   ├── repartition-shared-disk.yaml             ✓ shipped
│   ├── deploy-controller-fix-via-repository.yaml ✓ shipped
│   ├── recover-day-0-bootstrap-failure.yaml     ✓ shipped
│   ├── recover-keepalived-vip-loss.yaml         ✓ shipped
│   ├── recover-etcd-member-eviction.yaml        ✓ shipped
│   ├── rotate-pki-certificates.yaml             ✓ shipped
│   ├── configure-google-workspace-mail.yaml     ✓ shipped (MX + SPF + DKIM + DMARC end-to-end)
│   ├── fix-doctor-error-findings.yaml           ✓ shipped (etcd 0700, scylla-manager registration,
│   │                                              services drift, unknown dirs, legacy alias dirs,
│   │                                              service.old_pid_after_upgrade)
│   ├── recover-failed-platform-upgrade.yaml     (TODO)
│   └── restore-from-backup.yaml                 (TODO)
└── service-roles/           # canonical "what is this service for" entries
    ├── awareness-graph.yaml         ✓ shipped (standalone sidecar; 7 RPCs + 7 mcp__awg__* tools,
    │                                            tool decision tree, preflight/edit-check/metadata semantics,
    │                                            contract-first write path, coverage philosophy)
    ├── cluster-controller.yaml      ✓ shipped
    ├── cluster-doctor.yaml          ✓ shipped (role, invariants catalog, event-amplification bug)
    ├── node-agent.yaml              ✓ shipped
    ├── repository.yaml              ✓ shipped
    ├── mcp.yaml                     ✓ shipped (untracked service, 148+ tools, auth, failure modes)
    ├── ai-memory.yaml               ✓ shipped
    ├── workflow.yaml                ✓ shipped
    ├── ai-watcher.yaml              ✓ shipped
    ├── ai-executor.yaml             ✓ shipped
    ├── ai-router.yaml               ✓ shipped
    ├── dns.yaml                     ✓ shipped
    └── ingress.yaml                 ✓ shipped
```

Every entry becomes one row in AI Memory with `provenance.source = "seed"`.

## Lifecycle

```
build  ── go test ──> validate YAML against SCHEMA.md
   │
   └── awareness bundle build ──> compute SHA256 per file
                              ──> stamp manifest with seed_version + seed_sha256
                              ──> ship in the signed awareness bundle (kind=AWARENESS_BUNDLE)

day-0  ── node-agent installs awareness bundle ──> /var/lib/globular/operational-knowledge/
       └── files on disk are immediately readable by operator and any AI assistant
           (no AI services required)

day-1  ── once ai-memory.service reaches verified state ──>
          seeder workflow ingests YAML files into AI Memory
          - provenance.source = "seed"
          - provenance.seed_version, provenance.seed_sha256 stamped per entry
          - immutable = true
          - idempotent: skip if (id, seed_sha256) already exists

ongoing ── cluster-doctor runs ops_knowledge.seed_integrity invariant ──>
           flags any drift between ai-memory and the bundle's manifest
```

## Integrity model

Operational knowledge is signed-on-disk and immutable-in-memory. Three layers:

1. **Bundle integrity (existing)** — the operational-knowledge YAML files ship inside the on-disk bundle (`/var/lib/globular/awareness/awareness-bundle-*.tar.gz`), which is SHA256-manifest-signed. The installer verifies that manifest at install time, catching tampering. (This is the ops-knowledge bundle, distinct from the AWG graph, which since the 2026-06 extraction is a standalone sidecar no longer shipped as a Globular package.)
2. **AI Memory immutability (new)** — entries with `provenance.source = "seed"` carry `immutable: true` at the storage layer. `ai-memory` rejects UPDATE/DELETE on these unless the caller's gRPC principal is `seeder` (the day-1 workflow account). Direct memory_store/update/delete RPCs from any other principal are rejected with `PERMISSION_DENIED`.
3. **Doctor invariant `ops_knowledge.seed_integrity` (new)** — runs on every doctor cycle. For each seed entry in AI Memory: read the manifest, re-compute SHA256, fail if drift. Severity: `warning` (advisory, never gating per `convergence.no_infinite_retry`).

## What is NOT seed knowledge

The seed is for the things every cluster shares. It is not:

- per-cluster runtime state (that's the live AI Memory's organic growth)
- per-incident diagnoses (those come from ai-watcher → ai-executor → ai-memory)
- experimental procedures (those go through `awareness.experience` ledger first, get promoted only if proven across N sessions)
- Claude's chat scrollback (no AI session writes seed entries — only the build pipeline does)

If you want to add to the seed, you write a YAML file here and it ships in the next release. There is no runtime path to mutate the seed.

## Authoring rules (for the build pipeline, not for runtime)

- Every entry has a stable `id` (kebab-cased, dotted, e.g. `ops.day-1.minio.topology-apply-flow`)
- Every entry MUST have `provenance.source: seed` (the build tool stamps `seed_sha256` automatically)
- Every entry SHOULD link to: an awareness invariant if any, the runbook YAML if it's a procedure, related failure_modes by id
- Every entry MUST declare `applies_when` (cluster phase, services that must be present)
- See `SCHEMA.md` for the full schema and validation rules.

## Loading this knowledge into AI Memory (operator's view)

```bash
# Day-1, after ai-memory is healthy, run once per cluster:
globular ops-knowledge seed --bundle /var/lib/globular/awareness/current/

# Verify post-load:
globular ops-knowledge verify
# Reports any drift between the on-disk bundle and what's in ai-memory.

# Inspect what's loaded:
globular ops-knowledge list --stage day-1
```

(These commands are part of the implementation plan, not yet built. See the design status section below.)

## Design status

| Layer | Status |
|---|---|
| Directory + README + SCHEMA | this PR |
| First seed YAML files (stages, key runbooks, service-roles) | this PR |
| New awareness invariants and failure_modes | this PR |
| `globular ops-knowledge` CLI (validate / seed / verify / list) | follow-up |
| Awareness bundle build inclusion (SHA256 stamping) | follow-up |
| ai-memory immutability layer + `seeder` principal | follow-up |
| Day-1 seeder workflow | follow-up |
| Doctor invariant `ops_knowledge.seed_integrity` runtime check | follow-up |

## Related

- `docs/runbooks/objectstore-nfs-remediation.yaml` — the existing runbook YAML pattern this design mirrors
- `docs/operators/day-0-1-2-operations.md` — the canonical lifecycle reference
- `docs/ai/ai-services.md` — AI Memory schema and the watcher/executor/memory pipeline
- `docs/awareness/failure_modes.yaml` — code-time failure modes (distinct from operational seed)
- `docs/awareness/invariants.yaml` — code-time invariants (distinct from operational seed)
