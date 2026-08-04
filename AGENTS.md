# AGENTS.md — Globular Codex Operating Rules

This file is read automatically by Codex and other AI coding agents at session start.
It contains the decision rules, hard constraints, and operational knowledge required
to work safely in this codebase.

**This is not a style guide. It is a safety contract.**

Full architectural reference: `CLAUDE.md` and `docs/ai/ai-operating-rules.md`.
Awareness usage rules: `docs/awareness/agent_decision_rules.md`.
Runtime/deployment memory: AI Memory via `globular ops-knowledge`.

---

## Session Prelude — Read Before Tool Calls

AI agents do not have reliable continuous memory. Treat this file as the active
safety contract for every session.

1. **Memory is AI Memory, not flat files.** Use `globular ops-knowledge` or the
   available Globular memory MCP tools for durable project knowledge. Do not create
   ad hoc memory files in the repo or agent-private project directories.
2. **Awareness first for high-risk work.** Before edits to high-risk directories
   or authority surfaces, query AWG/awareness first. A "simple fix" is not an
   exemption when the file is high-risk.
3. **Ask the graph before grepping awareness sources.** When you need an invariant,
   intent, failure mode, or forbidden fix, prefer AWG query/resolve/briefing tools.
   `docs/awareness/` and `docs/intent/` are graph inputs, not the primary query
   surface. If AWG is unavailable, then use repository-local files as fallback and
   say the graph was unavailable.
4. **End non-trivial code tasks with awareness context.** Use a compact line such
   as `AWG: briefing(<target>) | invariants: X, Y | uncertainty: Z`. For degraded
   graph access, say what fallback evidence was checked. Omit the line only for
   low-risk/no-behavior edits.

---

## Contract-First Resolution — No Oracle Patching

Before resolving any error, bug, failing test, warning, or reported problem,
identify the governing contract that defines "correct". A passing test without a
contract is only an oracle match; it is not a validated repair.

Pre-edit checklist:

1. **Contract status** — found / inferred / missing / unknown
2. **Contract source** — AWG node IDs, docs, code, or tests that ground it
3. **Relevant invariants** — what must remain true
4. **Relevant failure modes** — how this path has broken before
5. **Forbidden fixes** — known-bad repairs the graph or docs rule out
6. **Verification plan** — tests/builds/probes that prove the contract is respected

Act by status:

- **found** — fix within the rule.
- **inferred** — fix cautiously and flag the candidate contract for promotion.
- **missing** — extract/record a candidate invariant before behavioral repair.
- **unknown** — stop and ask; do not pretend the issue is fixed.

---

## The Core Mental Model — Read This First

Globular is a distributed bare-metal cluster control plane. It is **not** a normal
monorepo app. Every file you touch may affect a live cluster. The cluster has 4 layers
of state, each with a different owner and authority source. Collapsing these layers is
the most common class of serious bug.

```
Layer 1  Repository     — "Does this version exist?"      etcd + Scylla + MinIO
Layer 2  Desired State  — "What should be running?"       etcd ServiceDesiredVersion
Layer 3  Installed      — "What is actually installed?"   node-agent heartbeat
Layer 4  Runtime        — "Is it running and healthy?"    systemd + health checks
```

**Before every change, ask:** which layer does this file own? Does my change respect
that layer's authority? Can it affect the layers above or below?

---

## Decision Framework — Before You Edit

```
OBSERVE → DIAGNOSE → RECOMMEND → [APPROVE] → EXECUTE → VERIFY
```

**Never jump to EXECUTE.** This sequence exists because:
- Most bugs are layer mismatches, not code bugs. Editing without diagnosing fixes
  the wrong layer.
- The right tool at the wrong layer makes things worse.
- Verification after edit catches regressions that reasoning doesn't.

### Trigger matrix

| Situation | Action before editing |
|-----------|----------------------|
| Touching a file in `high_risk_files.yaml` | Run `awareness_preflight` compact |
| Touching reconciler / desired-state / release-bridge / heartbeat / apply_package_release | Run `awareness_preflight` compact |
| Error you can't trace to an obvious code path | Run `awareness_failure_match_error` with exact text |
| About to change a type that lives in etcd | Identify encoding (proto? JSON? hand-struct?), check all readers/writers |
| Adding a field to a proto or etcd-serialized struct | Verify backward compatibility — old agents will read new data |
| Cross-layer change (desired state + reconciler + CLI) | Map the full propagation path before writing any code |
| Unfamiliar subsystem — "is this safe to change?" | Run `awareness_impact_file` on the target |
| Active incident / cluster not converging | Run `awareness_offline_diagnose` or `awareness_live_preflight` first |
| Audit, runtime/deployment change, packaging change, or day-0/day-1 change | Query AI Memory with `globular ops-knowledge list` and, when relevant, `globular ops-knowledge verify` |

### Skip awareness when

- Fixing a compile error (type mismatch, missing import, syntax)
- Renaming a variable within a single file, no semantic change
- Adding a CLI flag with no cross-layer effect
- Writing tests for isolated logic (no etcd, no gRPC, no state mutation)
- Updating documentation or comments
- Bumping version numbers in `zz_version_generated.go`

---

## Hard Rules — Never Violate

These are non-negotiable. Every suggestion, every edit, every generated code must
respect them.

### 1. etcd is the sole source of truth

- All cluster configuration, service endpoints, desired state, and node state
  live in etcd
- **No environment variables** for service configuration — ever
- **No hardcoded addresses** — all endpoints resolved from etcd or service discovery
- **No hardcoded gRPC service ports** — all ports from etcd at runtime
- Standard protocol ports (443, 53, 2379) are OK — they are protocol definitions, not config
- If etcd can't provide a value, the service MUST error out — no silent fallbacks

### 2. No localhost / 127.0.0.1 for remote addresses

- If the address could be remote, resolve it from etcd
- For bind/listen operations ONLY, use `0.0.0.0`
- `localhost` is acceptable only for: etcdctl pointing to local etcd, or local DNS resolver

### 3. The 4-layer model is sacred — never collapse

- Never assume Desired == Installed or Installed == Running
- Never skip layers when diagnosing or converging
- Each layer has its own owner: Repository (repository service), Desired (controller),
  Installed (node-agent), Runtime (systemd + health)

### 4. All cluster state changes flow through workflows

- Every meaningful mutation goes through the Workflow Service
- Workflows MUST be idempotent (safe to replay)
- Workflows MUST reach a terminal state (SUCCEEDED or FAILED)
- No hidden imperative shortcuts, no inline state changes

### 5. cluster_controller security boundary

- `cluster_controller_server` MUST NOT use `os/exec`, `syscall`, or `systemctl`
- `node_agent_server` is the system executor. Read-only probes may use
  `os/exec` for domain tools and inspection (`systemctl is-active/status/show`,
  `journalctl`, `nodetool status`, `restic`, `sctool`, `cqlsh`, `mc`,
  `openssl`). Mutating systemd actions (`start`, `stop`, `restart`, `enable`,
  `disable`, `daemon-reload`, `kill`, `mask`, `unmask`) MUST go through
  `internal/supervisor/`. The sanctioned exception is `workflow_day0.go`,
  because Day-0 bootstrap runs before supervisor/etcd are available.
- Run `make check-services` and `make check-nodeagent-exec-boundary` for these
  constraints when touching executor or controller code.
- All gRPC RPCs must have `(globular.auth.authz)` annotations

### 6. No tokens or credentials in source

- No JWTs, API keys, or credentials committed to source
- Tokens are ephemeral — generated at runtime, cached in `~/.config/globular/token`
- No token/credential storage in etcd values — store file references, not secrets

### 7. No automatic rollback

- Crash → create incident record
- Rollback = explicit `Force=true` via CLI only
- Never generate code that rolls back automatically on error

### 8. Quorum is capacity, not an admission floor

- etcd on ALL nodes — no exceptions
- Quorum describes observed survival/write-safety quality of the current
  distributed system size; it is not a hidden controller admission constraint.
- A one-node cluster has one-node survivability, two nodes have two-node
  survivability, and larger clusters can provide better fault tolerance.
- Do not auto-promote profiles, block Day-1 rollout, or refuse bootstrap/join
  only because ScyllaDB/MinIO/storage count is below a preferred size.
- Destructive storage/topology operations still require explicit component
  safety checks and operator approval when data safety is at risk.

### 9. Repository artifacts are POSIX CAS + Scylla, not MinIO

- Packages live in `/var/lib/globular/packages/` as POSIX CAS.
- ScyllaDB is the package index.
- MinIO is for secondary user data only: files, search indexes, web objects.
- Never look in MinIO for packages, certificates, or workflows.

---

## Encoding Rules — Know Before Changing a Type

Globular has three encoding classes. **Identify which one before adding or changing any field.**

| Class | Identifier | Rule |
|-------|-----------|------|
| Protobuf-generated | `*.pb.go` file, `proto3` tag in struct | Run `generateCode.sh` after changing `.proto`. Never hand-edit `*.pb.go`. |
| Hand-written JSON/etcd | `// +globular:encoding:json` comment, `json:"..."` tags, stored at `/globular/...` key | Maintain backward compatibility — old agents read new data. New fields MUST be `omitempty`. |
| Internal-only struct | No storage annotation, not in etcd, not serialized over wire | Safe to change schema freely. |

When in doubt: grep for the type name in `etcd_get` output or trace the `cli.Put` call.

---

## RPC Routing — Know the Route Before Calling

Not all RPCs are mesh-routable through Envoy.

| Pattern | How to call |
|---------|------------|
| Mesh-routed public RPCs | Use `globular.internal` FQDN — Envoy handles routing |
| Direct controller/node RPCs | Resolve address from etcd service registry |
| Bootstrap RPCs (before mesh ready) | Write directly to etcd — `upsertServiceDesiredVersion` pattern |

**Never hardcode `localhost:port` for an inter-service call.**
**Never assume an RPC reachable during bootstrap is reachable at runtime.**

---

## Version Identity — Never Confuse These

| Identity | Type | Example | Authority |
|----------|------|---------|-----------|
| Platform release | BOM tag | `v1.0.87` | `release-index.json` |
| Package version | Semver | `1.2.43` | Package manifest |
| Build number | int64 | `7` | Repository service |
| Build ID | UUID | `abc123...` | Repository service |
| Artifact digest | SHA-256 | `sha256:aa...` | MinIO / local file |

Rules:
- Do NOT stamp every package with the platform release version
- `release-index.json` is the BOM truth — GitHub asset list is not authoritative
- Same (publisher, name, version, platform) + different digest = identity conflict → reject
- build_id and build_number are never interchangeable
- GitHub is a release provider, not the architecture. Repository, controller, and
  node-agent code must stay provider-neutral.
- Day-0 reads `release-index.json`; Day-1 joins from the active platform BOM, not
  "latest" and not a guessed filename.

---

## Package Kind — One Registry

Package kind (SERVICE / INFRASTRUCTURE / APPLICATION / COMMAND) must come from the
canonical registry. **Never hardcode a package kind list in application code.**

Sources of truth:
- `packages/registry.yaml` → authored package classification authority
- Generated registry projections → runtime kind lookup
- `packages/metadata/<name>/package.json` and specs → validated mirrors

Anti-pattern (do not reproduce):
```
// BAD — duplicated kind list
var infraPackages = []string{"etcd", "scylla", "minio", "envoy"}
```

---

## Awareness Tools — When to Use

If the Globular MCP server is available in your session, follow the same rules as
`docs/awareness/agent_decision_rules.md`. Summary:

| When | Tool |
|------|------|
| Before editing high-risk file | `awareness_preflight` compact |
| Error you can't explain from code | `awareness_failure_match_error` |
| "Is this safe to change?" | `awareness_impact_file` |
| Cross-layer design question | `awareness_path` or `awareness_neighborhood` |
| Active incident | `awareness_offline_diagnose` |
| After fixing a non-obvious invariant | `awareness_learn_from_fix` |

`UNKNOWN_IMPACT` from preflight ≠ safe. If graph is unavailable, grep
`docs/awareness/failure_modes.yaml` and `docs/awareness/invariants.yaml` directly.

Awareness explains why code exists, what it protects, and which fixes are
forbidden. It cannot prove current etcd state, cluster membership, runtime
health, or installed-package state; verify those with live tools.

---

## AI Memory — Runtime and Deployment Context

Before audits, packaging changes, day-0/day-1 changes, runtime-dir changes, installer
changes, or deployment pipeline changes, query behavioral memory for operational
context:

```bash
globular --timeout 3s ops-knowledge list
globular --timeout 3s ops-knowledge verify
```

Use AI Memory to recover learned runtime/deployment constraints, recurring failure
modes, package authoring checklists, and cluster-specific operational notes. Treat
those entries as operational context, not source-of-truth code: confirm any invariant
against the repository, AWG, or live cluster state before editing.

If AI Memory is unavailable, continue with AWG and repository-local docs, but state
that memory context could not be queried.

---

## Service and Application Framework Rules

### Go services

Every service should use the shared `globular_service` primitives for common
lifecycle behavior: informational flags, positional config parsing, TLS,
interceptors, health, registration, and graceful shutdown.

Required pattern:

```go
globular_service.HandleInformationalFlags("name", "version")
serviceID, configPath := globular_service.ParsePositionalArgs()
lm := globular_service.NewLifecycleManager(srv, port)
lm.RegisterService(func(gs *grpc.Server) { pb.RegisterMyServiceServer(gs, srv) })
lm.Serve()
```

Do not bypass shared lifecycle/start/stop helpers unless the governing contract
explicitly says the service is special.

### Web applications and frontends

Globular applications are web frontends served through the Envoy gateway using
gRPC-Web protocol translation. Any frontend framework is acceptable (React, Vue,
Angular, vanilla JS, etc.) only if it respects the platform boundary:

- Use generated gRPC-Web clients for backend communication.
- Do not invent local package, node, service, RBAC, or runtime authority in the UI.
- UI state must name its authority: backend RPC, workflow state, RBAC result, or
  local-only view state.
- Success states must be based on backend/workflow/runtime confirmation, not a
  local click or optimistic dispatch.
- Critical operator meaning must not depend only on color, hover, animation, or
  layout.

---

## Build and Verify

After every code change:

```bash
# Build
cd golang && go build ./...

# Test the affected package
cd golang && go test ./<changed-package>/... -race

# Build specific service
cd golang && go build ./<service>/<service>_server
```

Do not report a task complete without a clean build. A passing `go vet` is a minimum
bar — `go test` on the affected package is required for any logic change.

**Do not run `go build` with `-trimpath` stripped or without ldflags — that's the CI
pipeline's job.** Report the change; let CI inject version/build metadata.

---

## Git Discipline

- **Do not push to remote** unless explicitly instructed
- **Do not amend published commits** — create new commits
- **Do not force-push `master`** — ever
- **Do not skip hooks** (`--no-verify`) — if a hook fails, fix the root cause
- Stage specific files — never `git add -A` or `git add .` (risk of committing
  credentials or generated files)
- Commit messages: lead with the subsystem prefix (`cluster_controller:`, `override:`,
  `node_agent:`, etc.) and explain WHY, not what

---

## Common Mistakes — Do Not Repeat

| Mistake | Correct |
|---------|---------|
| `/etc/globular/creds/` | `/var/lib/globular/pki/` |
| Port 10000 for controller | Port 12000 |
| `os.Getenv()` for service config | Read from etcd |
| `os/exec` in cluster_controller | Forbidden — use workflow + node_agent |
| `127.0.0.1` for inter-service | Resolve from etcd |
| Assuming desired == installed | Check all 4 layers |
| Hardcoded version in source | `Version=""`, injected via ldflags at build |
| Deleting desired state to fix drift | Roll forward — never delete desired state |
| `pkill -f <binary>` | `pkill -x <binary>` — `-f` matches parent shell argv → self-SIGKILL |

---

## When Uncertain — Stop and Ask

If you cannot identify:
- Which layer a file owns
- Whether a type is proto, JSON, or internal-only
- Whether an RPC is mesh-routable
- Whether a change is safe without a live cluster

**Stop. Ask. Do not guess and write code.**

The cost of asking is one turn. The cost of a wrong-layer edit in a live cluster is
hours of incident recovery.


## Sensei

This project uses [Sensei](https://github.com/globulario/sensei) to prevent
architectural drift. Before editing files, consult the awareness graph — it holds
the invariants, failure modes, forbidden fixes, and required tests that no diff
shows.

Sensei also installs the **Sensei Architect** skill. For architecture-sensitive
planning, implementation, debugging, review, recovery, migration, security, or
state/convergence work, load `.sensei/skills/sensei-architect/SKILL.md` before
planning or editing. Prefer native discovery from `.agents/skills/sensei-architect/`
when your agent supports Agent Skills. Use MCP tools when configured and CLI
fallbacks otherwise. Stay proportional to the risk and preserve durable lessons
with `sensei propose`.

### Behavioral guidelines

General discipline for every change (paraphrased from Andrej Karpathy's
observations on how LLMs fail at coding — the popular
[`andrej-karpathy-skills`](https://github.com/forrestchang/andrej-karpathy-skills)
file). Sensei adds the *repo-specific* rules below; these are the *general* ones:

1. **Think before coding.** State assumptions out loud. If the request is
   ambiguous, ask. If a simpler approach exists, push back. When confused, stop
   and name what is unclear — do not just pick one interpretation and run.
2. **Simplicity first.** Write the minimum code that solves the problem — no
   speculative abstractions, no flexibility nobody asked for.
3. **Surgical changes.** Touch only what the task requires. Do not improve
   neighboring code or refactor what is not broken; every changed line traces
   back to the request.
4. **Goal-driven execution.** Turn a vague instruction into a verifiable target
   first — "add validation" becomes "write tests for invalid inputs, then pass
   them."

### Rules

1. **Consult before editing high-risk files.** Run `sensei briefing --file <path>`
   before modifying any file listed in `docs/awareness/high_risk_files.yaml`. It
   returns the invariants, failure modes, forbidden fixes, and required tests that
   govern that file.
2. **Respect forbidden fixes.** The briefing lists patterns that look correct but
   are known-broken. Do not use them — revise rather than force one through.
3. **Run required tests.** The briefing lists tests that must pass when touching
   protected files. Run them before committing.
4. **Record new scars.** When you fix a durable failure class, add it back with
   `sensei propose` so the next agent inherits it.

### Commands

```bash
sensei briefing --file <path>     # context for a file edit
sensei briefing --task "desc"     # context for a task
sensei edit-check --file <path>   # does proposed content violate a rule?
sensei gate --diff <range>        # gate a diff (CI or pre-commit)
```

Requires a running server: `sensei serve` (defaults to `localhost:10120`). If a
Sensei MCP server is configured, prefer the `mcp__sensei__*` tools.
