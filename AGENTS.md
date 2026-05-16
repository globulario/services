# AGENTS.md — Globular Codex Operating Rules

This file is read automatically by Codex and other AI coding agents at session start.
It contains the decision rules, hard constraints, and operational knowledge required
to work safely in this codebase.

**This is not a style guide. It is a safety contract.**

Full architectural reference: `CLAUDE.md` and `docs/ai/ai-operating-rules.md`.
Awareness usage rules: `docs/awareness/agent_decision_rules.md`.

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
- `node_agent_server` can only use `os/exec` within `internal/supervisor/`
- All gRPC RPCs must have `(globular.auth.authz)` annotations

### 6. No tokens or credentials in source

- No JWTs, API keys, or credentials committed to source
- Tokens are ephemeral — generated at runtime, cached in `~/.config/globular/token`

### 7. No automatic rollback

- Crash → create incident record
- Rollback = explicit `Force=true` via CLI only
- Never generate code that rolls back automatically on error

### 8. Founding quorum is enforced

- etcd on ALL nodes — no exceptions
- ScyllaDB minimum 3 nodes
- MinIO minimum 3 nodes
- First 3 nodes MUST have profiles: `core`, `control-plane`, `storage`

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

---

## Package Kind — One Registry

Package kind (SERVICE / INFRASTRUCTURE / APPLICATION / COMMAND) must come from the
canonical registry. **Never hardcode a package kind list in application code.**

Sources of truth:
- `packages/metadata/<name>/package.json` → per-package spec
- Canonical registry (generated from specs) → runtime kind lookup

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
