# CLAUDE.md

This file is read automatically by Claude Code at the start of every session. It contains the rules, invariants, and operational knowledge needed to work safely with the Globular codebase.

---

## SESSION PRELUDE — read before any tool call

Claude has no continuous memory between sessions. The rules below are loaded as text, but my training defaults will leak through unless I actively check them. Hooks enforce some of these; the rest require deliberate attention.

1. **MEMORY = ai-memory, not flat files.** For project `globular-services`, use `mcp__globular__memory_store` / `memory_query` / `memory_update`. A `PreToolUse` hook (`block-flat-memory.sh`) will deny Write/Edit to `~/.claude/projects/.../memory/` entry files. `MEMORY.md` (the index) is still editable for migration cleanup, but new entries must go to ai-memory.

2. **AWARENESS-FIRST in high-risk dirs.** Before any edit to `golang/{node_agent,cluster_controller,repository,rbac,security,cluster_doctor,mcp,ai_executor,services_manager}/`, `docs/awareness/`, or `docs/intent/` — call `mcp__awg__awareness_briefing(file=<path>)` FIRST. Two `PreToolUse` hooks enforce this:
   - `enforce-briefing.sh` — denies Edit/Write/MultiEdit on these paths without a briefing.
   - `enforce-briefing-bash.sh` — denies Bash commands that mutate these paths (`>`, `>>`, `tee`, `cp`, `mv`, `sed -i`, `python -c "...open('docs/awareness/...', 'w')..."` etc.) without a briefing. Best-effort: catches obvious patterns; the bypass `python3 /tmp/script.py` where the script writes is not detectable from the command line alone — don't rely on that bypass.

   No "simple fix" exemption.

3. **Ask the graph, don't grep.** When you need an invariant, intent, failure mode, or forbidden fix, use `mcp__awg__awareness_query` / `awareness_resolve` / `awareness_briefing`. Do NOT grep over `docs/intent/` or `docs/awareness/` — the YAML files are inputs to the graph, not the queryable surface.

4. **End non-trivial tasks with the AWG summary line**: `AWG: briefing(<target>) | invariants: X, Y | uncertainty: Z`. See the AWARENESS USAGE section for variants (degraded, empty+high-risk, empty+low-risk=omit).

5. **CONTRACT-FIRST before edits.** Before resolving any error / bug / failing test, identify the governing contract — `briefing → resolve IDs → related contracts → invariants → failure modes → intent → only then patch` — and complete the pre-edit checklist in the CONTRACT-FIRST RESOLUTION section below. **No checklist, no edit.** A green test with no identified contract is an oracle match, not a resolution.

If you find yourself defaulting to flat-file memory, grepping over awareness sources, or editing high-risk code without calling briefing — STOP. That is the drift this prelude exists to prevent. Today's session (2026-06-03) produced this prelude because the same drift kept happening.

---

## CONTRACT-FIRST RESOLUTION — identify the contract before you edit

AWG now treats every error as a **contract-discovery problem before it is treated as a code-editing problem.** Before resolving any error, bug, failing test, warning, or reported problem, identify the governing contract that defines what "correct" means. A fix is valid only when the governing contract is identified, respected, and evidence-backed — **a passing test with no identified contract is an oracle match, not a resolution.**

This is the operational form of two foundational AWG meta-principles — `meta.contract_must_be_explicit_before_resolution` and `meta.no_resolution_without_a_respected_contract` — authored in the awareness-graph corpus.

### The graph is the pre-repair search space

Traverse it before touching code, in this order:

```text
briefing → resolve IDs → follow related contracts → check invariants
         → check failure modes → check intent → only then patch
```

### Required pre-edit checklist (no checklist, no edit)

Before modifying any code, write down:

1. **Contract status** — found / inferred / missing / unknown
2. **Contract source** — the AWG node IDs or files that ground it
3. **Relevant invariants** — what must stay true
4. **Relevant failure modes** — how this path has broken before
5. **Forbidden fixes** — patches the graph already rules out
6. **Verification plan** — how respect for the contract will be proven

Then act on the contract status — this is the line between fixing within the rules and doing local patch surgery that damages the organism elsewhere:

- **found** → fix within the rule.
- **inferred** → propose, then verify carefully; flag it for promotion to an explicit contract.
- **missing** → extract a candidate invariant; do not apply a behavioral fix without approval unless the change is purely diagnostic or reversible.
- **unknown** → stop pretending. Emit a revision request or an architecture question. Do not say "fixed".

**Enforcement level:** this is **agent discipline** (prompt-level), *not yet* a mechanical CI gate. The principles are classified `review_only` in `docs/awareness/meta_principle_coverage.yaml`; mechanization is the Phase-2 `awg gate` step against a frozen contract set. Until then, this checklist is the gate.

Full protocol — contract statuses, per-status output templates, the contract-extraction duty, and forbidden behaviors: [docs/design/contract-first-resolution-protocol.md](docs/design/contract-first-resolution-protocol.md).

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

### 7. Call awareness.briefing BEFORE writing any code in high-risk directories

**No exceptions. No judgment calls. "Simple fix" is not an exemption.**

Call `awareness.briefing(file=<target>)` before touching any file under:

- `golang/node_agent/`
- `golang/cluster_controller/`
- `golang/repository/`
- `golang/rbac/`
- `golang/security/`
- `golang/cluster_doctor/`
- `golang/mcp/`
- `golang/ai_executor/`
- `golang/services_manager/`
- `docs/awareness/`
- `docs/intent/`

**Why this is a hard rule, not a guideline:**

Two reasons, both non-negotiable:

1. **Bug prevention.** The cases where you are most confident a fix is "too simple to need awareness" are exactly the cases where you are most likely to miss a critical invariant. In 2026-06, a one-function wrapper (`DownloadArtifactToDir`) looked mechanical — the briefing would have caught `repository.fallback_requires_manifest_and_checksum` (critical) before the code was written. Instead, v1.2.141 shipped with an unverified download path; v1.2.142 was required to fix it. Individual bugs are recoverable.

2. **Architecture drift prevention.** This is the more important reason. Invariants erode one "simple fix" at a time. Intent gets forgotten. Boundaries blur. Each small deviation from the architecture is invisible in isolation — the damage is cumulative and only becomes visible when the system starts failing in ways that are expensive to trace and hard to reverse. The briefing is the only mechanism that connects a local code change to the global architectural intent. Skipping it for "obvious" changes is how infrastructure dies slowly.

The briefing is lightweight (13ms, ~500 tokens). There is no cost to justify skipping it.

Call `awareness.briefing` first, then write code. Handle the result per the empty-briefing policy:
- **Low-risk / no-behavior edit**: empty briefing is fine — proceed quietly, omit the AWG summary line.
- **High-risk target, minor edit**: treat empty as DEGRADED — announce it, check `high_risk_files.yaml`, use code/tests/docs as fallback.
- **Behavior change in high-risk code**: empty briefing triggers escalation — run `impact`, `briefing(task=)`, or query related domains before editing. If still empty, continue only with explicit uncertainty and tests.
- **Degraded (service unavailable)**: do not proceed with architectural changes without explicit user approval.

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

**Workflow:**
1. `awareness.briefing` with `file` or `task` — start every non-trivial task here (~500 tokens, ~13ms).
2. `awareness.impact` on each target file when briefing's coverage is thin.
3. `awareness.resolve` on any `referenced_id` you need expanded.
4. Read the actual code. Patch. Run required tests from awareness output + nearby regressions.
5. End the response with the AWG summary line (below).

**Status handling:**
- `ok` — follow invariants, forbidden fixes, required tests. Resolve any high/critical referenced_id.
- `empty` — handle per risk tier (see empty-briefing policy below).
- `degraded` — do NOT proceed with high-risk architectural changes without user approval. Use code/tests/docs as fallback evidence and say so.

**Empty-briefing policy** (empty ≠ safe, but empty ≠ always noisy):

| Risk tier | Example | Action |
|-----------|---------|--------|
| **Low-risk / no-behavior** | Typo fix, formatting, comment, import reorder | Proceed quietly. Omit AWG line from summary. |
| **High-risk target, minor edit** | Rename in node_agent, log message in repository | Treat as DEGRADED. Announce. Check `high_risk_files.yaml`. Use code/tests/docs as fallback. |
| **Behavior change in high-risk code** | New convergence path, auth check change, workflow resume logic | Escalate: run `impact(file=)`, `briefing(task=)`, or query related domains. If still empty, continue only with explicit uncertainty and tests. |

**AWG summary line** (append to every non-trivial code task — replaces the old 9-field template):

```
AWG: briefing(<target>) | invariants: X, Y | uncertainty: Z
```

Variants:
- Degraded: `AWG: DEGRADED — fallback: <what was checked>`
- Empty + high-risk: `AWG: DEGRADED — empty briefing for high-risk target; proceeded with fallback reasoning/tests`
- Empty + low-risk: omit the AWG line entirely.

Awareness explains *why* code exists, *what* it protects, *which fixes are forbidden*. It cannot prove current etcd state, cluster membership, runtime health, or installed-package state — verify those with live tools.

### PRINCIPLE EXTRACTION PROTOCOL — when processing errors

When documenting an error (incident, failure mode, finding), don't just record WHAT broke — classify WHY against the generative meta-principles. This turns error documentation into architectural learning.

**Step 1 — CLASSIFY** the error against `invariant:meta.*` (7 categories). Source of truth: the shared corpus now lives in the awareness-graph repo at `docs/awareness/generic/state_authority_invariants.yaml` (moved out of services 2026-06-13 so the knowledge is portable); re-query the graph if this table feels stale. Edit meta-principles there, not in services.

**Authority** (4) — "who owns this truth, and is this code that owner?"
| Principle | What it catches |
|-----------|----------------|
| `meta.storage_is_not_semantic_authority` | Wrong actor writes/reads truth |
| `meta.identity_computation_must_be_invariant` | Right actor, wrong meaning across contexts |
| `meta.competing_writers_must_converge_or_be_fenced` | Two writers with different state fight by timing |
| `meta.structure_must_not_be_stripped_in_projection` | Value projected to a primitive carries the lie of universality — scope/subject/source were part of the meaning |

**Signal** (5) — "is the truth arriving intact, or degraded / silent / absorbed?"
| Principle | What it catches |
|-----------|----------------|
| `meta.fallback_must_degrade_semantics` | Fallback returns same shape as truth |
| `meta.authority_must_express_uncertainty` | Owner can't say "unknown" → callers fabricate certainty |
| `meta.absence_scope_must_be_explicit` | Not-found-where treated as does-not-exist |
| `meta.connection_errors_must_not_be_absorbed` | TLS/auth error absorbed into generic timeout |
| `meta.assertions_must_carry_their_scope` | Positive or negative assertion aggregated without naming its scope (which node, which moment) — strips truth |

**Lifecycle** (7) — "will this operation complete, and what happens if it fails?"
| Principle | What it catches |
|-----------|----------------|
| `meta.write_creates_completion_obligation` | Write without cleanup path → permanent stall |
| `meta.half_done_must_not_look_done` | Intermediate state satisfies completeness check |
| `meta.silence_is_not_valid_for_unexpected` | Unhandled case is silent no-op |
| `meta.failure_response_must_contract_not_amplify` | Unbounded retry/re-enqueue turns one failure into a cascade |
| `meta.diagnostic_output_must_be_bounded` | One error → N log lines fills disk → cascade to healthy services |
| `meta.binding_outlives_evidence_until_invalidated` | Decision durable, but the evidence it was bound to has moved — binding authorizes the wrong present |
| `meta.state_mutations_must_be_durably_committed_before_side_effects` | Intent must commit before action; retry is the ONLY response; no alternative path once committed (audit lies if "X failed so I tried Y") |

**Dependency** (2) — "what breaks if a non-critical thing fails? what if A needs B needs A?"
| Principle | What it catches |
|-----------|----------------|
| `meta.critical_path_no_non_critical_dependency` | Critical path blocked/flooded by non-critical service |
| `meta.circular_dependency_must_have_break_glass` | Self-deploying system stuck — deploy pipeline can't deploy fix for itself |

**Perception** (16) — "is the screen telling the truth about the system?" (GUI meta-principles; Nielsen/WCAG/Material/ISO 9241-110/Shneiderman/GOV.UK/EEMUA 191 re-grounded in operator truth)
| Principle | What it catches |
|-----------|----------------|
| `meta.ui.screen_claim_must_bind_to_authority` | Desired/cached/optimistic/confirmed state collapsed into one visual meaning |
| `meta.ui.state_certainty_must_be_visually_distinct` | Loading/stale/unknown/optimistic rendered like confirmed truth |
| `meta.ui.same_truth_same_language` | Same operational state rendered differently across screens |
| `meta.ui.destructive_action_requires_confirmed_authority` | Destructive control offered without confirmed RBAC + visible risk |
| `meta.ui.failure_must_preserve_diagnostic_context` | Error path blanks selection, context, last-known data |
| `meta.ui.provenance_over_recall` | Claim shown without which-node/as-of-when/which-version |
| `meta.ui.meaning_must_survive_presentation_loss` | Warning/meaning exists only in color, hover, or desktop layout |
| `meta.ui.decoration_must_not_impersonate_authority` | Placeholder/cosmetic/AI-generated content confusable with live state |
| `meta.ui.simplicity_must_not_hide_operational_truth` | "Clean" UI hides node, generation, stale state, failed receipt, authority source |
| `meta.ui.operator_must_remain_in_control` | Automation/auto-refresh/optimistic update/AI changes state without visible consent |
| `meta.ui.control_must_be_reversible_or_guarded` | Irreversible action without undo/dry-run/preflight; shortcut bypasses the guard |
| `meta.ui.workflow_must_yield_closure` | Operation ends with no receipt, terminal state, or next action; fake progress animation |
| `meta.ui.ai_assistance_must_be_explainable_and_bounded` | AI summary asserts state without source, timestamp, confidence, evidence boundary |
| `meta.ui.task_path_must_match_operator_goal` | Screen mirrors proto/RPC schema instead of the operator's task |
| `meta.ui.notification_volume_must_match_operator_capacity` | Alarm flood, N symptom toasts for one cause, standing red badges nobody acts on |
| `meta.ui.interactive_element_must_have_stable_identity` | Anonymous styled span carrying a count/status — no id/data-* — invisible to tests, a11y, targeted refresh |

**Composition** (7) — "does the layout make truth easy to perceive?" (visual composition; Gestalt/typography/color theory/design tokens — composition is evidence architecture: pretty cards, wrong order is the failure)
| Principle | What it catches |
|-----------|----------------|
| `meta.ui.visual_hierarchy_must_match_decision_hierarchy` | Decorative summary louder than drift/risk; safety evidence below the fold |
| `meta.ui.visual_grouping_must_match_semantic_grouping` | Mixed-authority values blended in one card read as one confirmed truth |
| `meta.ui.spacing_must_encode_relationships` | Uniform spacing flattens real relationships or fabricates false ones |
| `meta.ui.proportion_must_reflect_operational_weight` | Destructive button more prominent than its own risk evidence |
| `meta.ui.color_must_have_semantic_contract` | Success color on desired/optimistic state; one hue serving conflicting roles; no unknown/stale roles |
| `meta.ui.typography_must_express_information_hierarchy` | Warning typographically indistinguishable from metadata |
| `meta.ui.theme_tokens_must_encode_roles_not_preferences` | Raw color/size literals in components instead of semantic tokens |

**Structure** (10) — "is this unit shaped to be reused, inspected, and outlive its implementation?" (code structure/reusability; Parnas, coupling/cohesion, Ousterhout, Hyrum; applies to Go AND TypeScript)
| Principle | What it catches |
|-----------|----------------|
| `meta.code.reusable_unit_must_have_a_stable_semantic_boundary` | Shared unit whose consumers import internals and break on refactor (anchor) |
| `meta.code.contract_must_outlive_implementation_fashion` | Callers depend on private structure; observable surface undeliberate (Hyrum) |
| `meta.code.complexity_must_be_hidden_behind_honest_boundaries` | Boundary absorbs errors/staleness/authority — encapsulation became concealment |
| `meta.code.reuse_must_follow_semantic_cohesion` | GenericMegaTable: merged because markup looked similar, not same concept |
| `meta.code.composition_must_prefer_standard_protocols` | Units coordinate via framework-private global state instead of platform contracts |
| `meta.code.debuggability_is_part_of_correctness` | Abstraction/build layer with no path from runtime behavior to source intent |
| `meta.code.local_state_must_not_become_hidden_authority` | Component answers permission/completion/health from its own state |
| `meta.code.extension_points_must_be_explicit` | Consumer customizes by reaching into shadow root / private internals |
| `meta.code.abstraction_must_be_deeper_than_its_interface` | Pass-through wrappers, layer stacking, single-caller helpers (Ousterhout) |
| `meta.code.framework_dependency_must_be_earned` | Framework adopted on convenience; no written cost case or exit path |

If one fits → add `related_invariants: [meta.<id>]` to the error entry.
If none fits → flag as **UNCLASSIFIABLE** (potential new principle — zoom out with human).

**Step 2 — CHECK COVERAGE**: does the principle's enforcement already cover this case? If not, propose a forbidden_fix, required_test, or enforcement update.

**Step 3 — SEARCH FOR SIBLINGS**: use the principle as a lens to find similar violations in code that hasn't broken yet. The principle predicts where the same bug class is hiding.

**Step 4 — REPORT**: end error analysis with classification, coverage gap, sibling count, principle instance count.

**Rules**: Do NOT force-fit errors into principles. Bad classification is worse than none. Unclassifiable errors are where the next principle is hiding.

---

## AWARENESS ACTIVATION RULES

Full machine-readable rules: [`docs/awareness/activation_rules.yaml`](docs/awareness/activation_rules.yaml)

Two rules. Hook enforcement unchanged.

| Rule | Trigger | Enforcement | Tool |
|------|---------|-------------|------|
| **AUTO** | Edit in high-risk dir (`node_agent`, `cluster_controller`, `services_manager`, `repository`, `rbac`, `security`, `mcp`, `cluster_doctor`, `ai_executor`, `docs/awareness/`, `docs/intent/`) | Hook-enforced | `briefing(file=)` |
| **MANUAL** | Task touches authority surface: desired state, installed state, runtime evidence, convergence, security/RBAC, repository publish/installability, workflow resume/receipts, cluster-doctor remediation, awareness-graph internals | Agent judgment | `briefing(task=)` then `impact(file=)` |

**Low-risk exemption**: if the edit changes no behavior, awareness is optional. The hook still fires in high-risk dirs (it can't know intent), but the agent satisfies it with the briefing call and moves on quietly.

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
