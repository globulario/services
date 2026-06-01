# CLAUDE.md

This file is read automatically by Claude Code at the start of every session. It contains the rules, invariants, and operational knowledge needed to work safely with the Globular codebase.

---

## PRIME RULES FOR AI AGENTS

1. Walk the 4 layers before debugging: Repository → Desired → Installed → Runtime.
2. Never confuse platform release with package version. release-index.json is the platform truth.
3. Never confuse build_id (UUID) with build_number (integer).
4. Never duplicate package kind classification. Use the canonical package registry.
5. Know the encoding before changing a type: proto, JSON, or internal-only.
6. Know the route before calling an RPC: mesh-routable or direct-only.
7. GitHub is a provider, not the architecture. Controller and node-agent stay provider-neutral.
8. MinIO is for secondary user data only (files, search indexes). Packages live in /var/lib/globular/packages/ (POSIX CAS). ScyllaDB is the package index. Never look in MinIO for packages, certificates, or workflows.
9. Day-0 reads release-index.json. Day-1 joins from active BOM, not latest.
10. Do not infer truth from filenames when a manifest exists.
11. Keep scripts phase-oriented and idempotent.
12. Every fix should add or update a regression test.

Full rules: [docs/ai/ai-operating-rules.md](docs/ai/ai-operating-rules.md)

---

## HARD RULES — NEVER VIOLATE

These rules are non-negotiable. Every code change, every suggestion, every action must respect them.

### 1. etcd is the SOLE source of truth

- All cluster configuration, service endpoints, desired state, and node state lives in etcd
- **NO environment variables** for service configuration — ever
- **NO hardcoded addresses** — all endpoints resolved from etcd or service discovery
- **NO hardcoded gRPC service ports** — all ports come from etcd at runtime
- Standard protocol ports (443, 53, 2379) are OK — they are protocol definitions, not config
- If etcd can't provide a value, the service MUST error out — no silent fallbacks to defaults

### 2. The 4-layer state model is SACRED — never collapse

```
Layer 1: Repository (Artifact)     — "Does this version exist?"
Layer 2: Desired Release (Controller) — "What should be running?"
Layer 3: Installed Observed (Node Agent) — "What is actually installed?"
Layer 4: Runtime Health (systemd)  — "Is it running and healthy?"
```

- Each layer is INDEPENDENT with its own owner and data source
- Never assume Desired == Installed or Installed == Running
- Never skip layers when diagnosing or converging
- Repository → Desired → Installed → Runtime — this order is strict

### 3. NO localhost / 127.0.0.1 for remote addresses

- If the address could be remote, resolve it from etcd
- For bind/listen operations ONLY, use `0.0.0.0`
- `localhost` is acceptable only for: etcdctl pointing to local etcd, or local DNS resolver in resolv.conf
- All inter-service gRPC MUST use mTLS with the cluster CA

### 4. All state changes flow through workflows

- Every meaningful cluster mutation goes through the Workflow Service
- No hidden imperative shortcuts, no inline state changes
- Workflows MUST be idempotent (safe to replay)
- Workflows MUST reach a terminal state (SUCCEEDED or FAILED)
- The controller DECIDES, the Workflow Service COORDINATES, Node Agents EXECUTE

### 5. Founding node quorum — first 3 nodes MUST have all infrastructure

- **etcd**: runs on ALL nodes — no exceptions
- **ScyllaDB**: minimum 3 nodes for replication
- **MinIO**: minimum 3 nodes for erasure coding / redundancy
- The first 3 nodes of ANY cluster MUST have profiles: `core`, `control-plane`, `storage`
- This is enforced at join time in `enforceFoundingProfiles()` — cannot be bypassed
- Without MinIO on 3 nodes, it's a single point of failure that cascades: workflows fail → `completePublish` fails → artifacts stay VERIFIED → reconciler can't find them → services never upgrade
- `SetNodeProfiles` also enforces this — you cannot remove `storage` from a node if it would drop below 3 storage nodes

### 6. Security boundaries

- `cluster_controller_server` MUST NOT use `os/exec`, `syscall`, or `systemctl`
- `node_agent_server` can only use `os/exec` within `internal/supervisor/`
- Run `make check-services` to verify
- No token/credential storage in etcd values — use file references
- **No tokens stored in the codebase** — never commit JWTs, API keys, or credentials to source. Tokens are ephemeral (generated at runtime or cached in `~/.config/globular/token` per user)
- All gRPC RPCs must have `(globular.auth.authz)` annotations

---

## ARCHITECTURE NOTES

### Project Structure

```
services/
├── proto/                     # 38 .proto files (authoritative API contracts)
├── golang/                    # All Go services (33 binaries)
│   ├── cluster_controller/    # Central control plane (bootstrap port 12000)
│   ├── node_agent/            # Node executor (bootstrap port 11000)
│   ├── workflow/              # Workflow engine
│   ├── cluster_doctor/        # Health analysis
│   ├── repository/            # Package registry (MinIO-backed)
│   ├── authentication/        # JWT tokens
│   ├── rbac/                  # Permission enforcement
│   ├── dns/                   # Authoritative DNS
│   ├── ai_memory/             # Persistent AI knowledge (ScyllaDB-backed)
│   ├── ai_executor/           # Diagnosis + remediation
│   ├── ai_watcher/            # Event monitoring
│   ├── ai_router/             # Dynamic routing
│   ├── domain/                # ACME cert management (runs in controller)
│   ├── compute/               # Batch jobs (not yet in build manifest)
│   ├── globularcli/           # CLI tool
│   ├── mcp/                   # MCP server (129+ tools)
│   ├── globular_service/      # Shared primitives (lifecycle, config, CLI helpers)
│   ├── interceptors/          # gRPC middleware (auth → RBAC → audit)
│   ├── config/                # etcd-backed config
│   └── security/              # TLS, PKI, JWT, Ed25519
├── typescript/                # gRPC-Web client library
├── docs/                      # Full documentation (49 files)
├── generateCode.sh            # Proto → Go/TypeScript + build services
└── build-all-packages.sh      # Package build pipeline
```

> **Note**: Service ports are runtime attributes resolved from etcd — never hardcode them. Query `service_config_list` for current values. Only `cluster_controller` (12000) and `node_agent` (11000) are fixed bootstrap ports used before etcd is available.

### Key File Paths (ACTUAL, VERIFIED)

| What | Path |
|------|------|
| Service certificate | `/var/lib/globular/pki/issued/services/service.crt` |
| Service private key | `/var/lib/globular/pki/issued/services/service.key` |
| CA certificate | `/var/lib/globular/pki/ca.crt` |
| CA private key | `/var/lib/globular/pki/ca.key` |
| Ed25519 signing keys | `/var/lib/globular/keys/<id>_private` |
| etcd config | `/var/lib/globular/config/etcd.yaml` |
| ACME certs (Let's Encrypt) | `/var/lib/globular/domains/{domain}/fullchain.pem` |
| xDS ACME symlink | `/var/lib/globular/config/tls/acme/{domain}/` |
| Bootstrap flag | `/var/lib/globular/bootstrap.enabled` |
| RBAC cluster roles | `/var/lib/globular/policy/rbac/cluster-roles.json` |
| Keepalived config | `/etc/keepalived/keepalived.conf` (managed by node agent) |
| MCP config | `/var/lib/globular/mcp/config.json` |

**WARNING**: `/etc/globular/creds/` does NOT exist. All certs are under `/var/lib/globular/pki/`.

### etcd Key Schema

```
/globular/system/config                              — global settings
/globular/services/{service_id}/config               — service endpoint + config
/globular/services/{service_id}/instances/{node}     — per-node instance
/globular/resources/DesiredService/{name}             — desired state
/globular/resources/ServiceRelease/{name}             — release tracking
/globular/nodes/{node_id}/packages/{kind}/{name}     — installed packages
/globular/nodes/{node_id}/status                     — node heartbeat
/globular/ingress/v1/spec                            — keepalived VIP config
/globular/ingress/v1/status/{node_id}                — VRRP state
/globular/domains/v1/{fqdn}                          — external domain spec
/globular/providers/v1/{name}                        — DNS provider config
/globular/ai/jobs/{incident_id}                      — AI executor job records
```

### Service Implementation Pattern

Every service uses shared primitives from `globular_service/`:

```go
// main()
globular_service.HandleInformationalFlags("name", "version")  // --version, --help, --describe
serviceID, configPath := globular_service.ParsePositionalArgs()
lm := globular_service.NewLifecycleManager(srv, port)
lm.RegisterService(func(gs *grpc.Server) { pb.RegisterMyServiceServer(gs, srv) })
lm.Serve()  // blocks, handles TLS, interceptors, health, graceful shutdown
```

Config fallback chain: etcd → local seed file → global config → hardcoded defaults.

### Current Cluster (5 nodes)

> **Note**: Node profiles are runtime attributes managed by the cluster or set manually. Query `cluster_list_nodes` for authoritative current state.

| Node | IP | Profiles |
|------|-----|----------|
| globule-ryzen | 10.0.0.63 | control-plane, core, storage |
| globule-nuc | 10.0.0.8 | control-plane, core, storage |
| globule-dell | 10.0.0.20 | control-plane, core, storage |
| globule-hp-01 | 10.0.0.9 | control-plane, core, storage |
| globule-lenovo | 10.0.0.102 | control-plane, core, storage |

- **VIP**: 10.0.0.100 (keepalived, floats between ryzen and nuc)
- **DMZ**: Router forwards all external traffic to VIP
- **Public IP**: 96.20.133.54
- **Domain**: globular.io (Let's Encrypt wildcard cert: `*.globular.io`)
- **Internal domain**: globular.internal

---

## BUILD COMMANDS

```bash
cd golang && go build ./...                          # Build all
cd golang && go build ./echo/echo_server             # Build specific service
cd golang && go test ./... -race                     # Run all tests
cd golang && go test ./echo/echo_server -v           # Test specific package
./generateCode.sh                                    # Proto → Go/TypeScript
./build-all-packages.sh                              # Full package build
make check-services                                  # Security constraints
```

---

## DOCUMENTATION

Full docs in `docs/` (49 files, 16k+ lines). Key references:

- `docs/index.md` — Navigation hub
- `docs/ai/ai-rules.md` — Strict AI agent rules (12 rules)
- `docs/ai/ai-services.md` — AI Memory, Executor, Watcher, Router
- `docs/operators/ports-reference.md` — All ports and firewall rules
- `docs/operators/known-issues.md` — CLI gaps, infrastructure limitations
- `docs/operators/dns-and-pki.md` — Internal/external certs, ACME, DNS zones
- `docs/operators/keepalived-and-ingress.md` — VIP failover, DMZ
- `docs/developers/local-first.md` — Run services without a cluster

---

## DAY-1 NODE OPERATIONS

### Joining a node

1. On the controller — create a join token:
   ```
   globular cluster token create
   ```
2. On the new node — run the gateway join script:
   ```
   curl -sfL https://<controller-ip>:8443/join -k | sudo bash -s -- --token <JOIN_TOKEN>
   ```
   Add `--repair-etcd` only if a prior join attempt left stale etcd WAL.

**Never** use `globular cluster join` directly — the script handles TLS, user creation, unit files, and etcd add ordering that the bare command skips.

### Cleaning a node before rejoin

Before rejoining a node that had a previous Globular install (or a failed join), wipe its state completely. The clean script is served by the gateway:

```bash
# Interactive (asks for confirmation):
curl -sfL https://<controller-ip>:8443/clean -k | sudo bash

# Non-interactive / AI agents — MUST use --force:
curl -sfL https://<controller-ip>:8443/clean -k | sudo bash -s -- --force
```

The script stops all Globular and ScyllaDB services, removes unit files, wipes `/var/lib/globular`, removes ScyllaDB packages (so node-agent owns the install on rejoin), cleans PKI from the system trust store, removes user certs, and drops the globular user/group.

The canonical script is at `scripts/clean-node.sh` (also embedded in the gateway binary at build time from `Globular/internal/gateway/handlers/cluster/clean-node.sh`).

**AI agents**: always use `--force` to avoid blocking on stdin. Query `ops.day-1.node.clean` in ai-memory for the full procedure.

---

## KNOWN ISSUES (check before assuming things work)

1. **DNS zones persist to ScyllaDB** — if zones appear missing, the CLI may have auth issues. Use grpcurl directly to `localhost:10006` to set domains.
2. **Split-horizon DNS not supported** — `/etc/hosts` override needed for hairpin NAT
3. **ACME cert path mismatch** — reconciler writes to `/var/lib/globular/domains/{d}/`, xDS reads from `/var/lib/globular/config/tls/acme/{d}/`. Symlink required.
4. **compute_server not in build** — code exists but not compiled or packaged
5. **Service versions come from the deploy pipeline** — never hardcode versions in source code; the repository allocates versions via `--bump`, and the build injects them via ldflags

---

## AWARENESS USAGE

Awareness is the compact gRPC map of project intent, invariants, failure modes, incident patterns, required tests, and forbidden fixes. It does NOT replace reading code, running tests, or checking runtime state — it shows which floorboards are fragile before you walk in.

**Call awareness before non-trivial edits to:** service lifecycle, package publish/install, repository/discovery, cluster state, etcd state, RBAC/security/token logic, filesystem/upload paths, remediation/recovery code, install scripts, or tests that encode operational contracts.

**Also call awareness before edits to:** ScyllaDB join/topology/DDL preflight paths, objectstore/MinIO topology, awareness-graph itself (yaml2nt, scanner, briefing/impact/query handlers), or repository publish/artifact-state/release-index paths.

**Workflow:**
1. `awareness.briefing` with `file` or `task` — start every non-trivial task here. Reads ~500 tokens by default.
2. `awareness.impact` on each target file when briefing's coverage is thin — direct anchors only (inferred fields are reserved in v0; see `docs/awareness/decisions/inference-v0-direct-anchors-only.md`).
3. `awareness.resolve` on any `referenced_id` you need expanded.
4. Read the actual code. Patch. Run the required tests from awareness output + nearby regressions.
5. End the response with the awareness template (below).

**Status handling:**
- `ok` — treat returned rules as active context. Follow invariants and forbidden fixes; run required tests. Resolve any referenced_id marked high or critical.
- `empty` — NOT proof of safety. Say "no direct awareness anchors were found" and continue cautiously. Check `high_risk_files.yaml`.
- `degraded` or transport error — report degraded awareness. Do NOT proceed with high-risk architectural changes (state authority, security, package install, repository identity, convergence) unless the user explicitly approves. Use code/tests/docs/runtime as fallback evidence and say so.

**Final response template** (append to every non-trivial code task):

```text
Awareness used:
- briefing:
- impact:
- resolved IDs:
- invariants touched:
- failure modes considered:
- forbidden fixes avoided:
- required tests:
- tests run:
- remaining uncertainty:
```

If awareness was unavailable: `Awareness status: DEGRADED — fallback evidence: …`
If awareness was empty: `Awareness status: EMPTY for the target — code/docs checked manually: …`

Awareness explains *why* code exists, *what* it protects, *which fixes are forbidden*. Awareness cannot prove current etcd state, cluster membership, runtime health, or installed-package state — verify those with live tools.

---

## AWARENESS ACTIVATION RULES

Full machine-readable rules: [`docs/awareness/activation_rules.yaml`](docs/awareness/activation_rules.yaml)

Awareness is **mandatory** for the following. No exceptions.

| Rule | Trigger | Tools required |
|---|---|---|
| **R1 Annotated file** | File has `@awareness` annotations or CodeSymbol entries in graph | `briefing(file=)` |
| **R2 High-risk directory** | Edit targets `golang/node_agent/`, `golang/cluster_controller/`, `golang/services_manager/`, `golang/repository/`, `golang/rbac/`, `golang/security/`, `golang/mcp/`, `golang/cluster_doctor/`, `golang/ai_executor/`, `docs/awareness/`, `docs/intent/` | `briefing(file=)` then `impact(file=)` |
| **R3 State authority** | Task involves desired state, installed state, runtime state, convergence, reconciliation, service health, package install/publish, repository identity, etcd truth, node-agent authority, cluster controller | `briefing(task=)` then `impact(file=)` when files are known |
| **R4 Awareness internals** | Modifying yaml2nt, annotation-scanner, CodeSymbol importer, RDF vocabulary, Oxigraph load/seed, or MCP awareness tools | `briefing(file=)` then `resolve()` for any high/critical referenced_id |
| **R5 Shortcut fix** | Proposed fix changes authority boundary, weakens validation, ignores stale data, treats missing data as healthy, bypasses gateway/pool, or removes strict mode | `impact(file=)` and check forbidden fixes |
| **R6 Final report** | Every non-trivial code task | Append awareness template to response |

Awareness is **optional** (but always allowed) for:
- Typo-only README or doc edits (no code, no annotation blocks)
- Formatting-only changes (gofmt, import reorder)
- Comment-only changes outside `@awareness` annotation blocks
- Low-risk pure-helpers with no state, security, or runtime behavior
- Test data fixtures with no production code path

---

## AI RULES (for AI agents operating on this codebase)

### Observe before acting
Always diagnose before prescribing. Sequence: OBSERVE → DIAGNOSE → RECOMMEND → [APPROVE] → EXECUTE → VERIFY.

### Never invent state
Reason only from observable, verifiable evidence. If you need to know something, query the API — don't assume from memory or partial data. Stale memories must be verified against current state.

### Typed actions only
Never construct shell commands. Use typed gRPC RPCs. If an action doesn't have a typed API, it shouldn't be done by AI.

### Audit everything
Every action must produce a durable record. Use AI Memory for knowledge, etcd job store for actions.

### Fail safe
If AI services are down, the cluster must continue operating through its deterministic convergence model. AI is supplementary, never required.

### Respect RBAC
AI service accounts have scoped permissions. Do not attempt to escalate. Do not bypass the interceptor chain.

### Three-tier permissions
- Tier 0 (OBSERVE): Read-only diagnosis — always safe
- Tier 1 (AUTO_REMEDIATE): Pre-approved actions (restart, clear cache)
- Tier 2 (REQUIRE_APPROVAL): Human must approve before execution

---

## AI MEMORY SERVICE

If MCP tools `mcp__globular__memory_*` are available, use them instead of flat-file memory. Project: `"globular-services"`.

| Tool | Purpose |
|------|---------|
| `memory_store` | Save knowledge (type, title, content, tags, metadata) |
| `memory_query` | Search by type, tags, text |
| `memory_get` | Retrieve by ID |
| `memory_update` | Merge-update fields |
| `memory_delete` | Remove |
| `memory_list` | Lightweight summaries |
| `session_save` | Persist conversation context |
| `session_resume` | Resume prior conversation |

Types: feedback, architecture, decision, debug, session, user, project, reference, scratch, skill.

---

## COMMON MISTAKES TO AVOID

- Using `/etc/globular/creds/` (doesn't exist — use `/var/lib/globular/pki/`)
- Hardcoding port 10000 for controller (it's 12000)
- Using `os.Getenv()` for service config (use etcd)
- Calling `os/exec` in cluster_controller (forbidden)
- Using `127.0.0.1` for inter-service addresses (resolve from etcd)
- Assuming desired state == installed state (check all 4 layers)
- Writing env var sections in READMEs (etcd is the only config source)
- Referencing `clustercontroller` directory (it's `cluster_controller` with underscore)
- Assuming DNS zones persist across restarts (they're in-memory, re-register after restart)
- Storing tokens/JWTs/credentials in source code, config files, or etcd values — tokens are ephemeral, generated at runtime
