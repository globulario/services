# EX-1 — Execution Surface Audit (Tier G spike)

The execute-side mirror of [RT-1](rt1-direct-write-surface-audit.md) (writes) and
[OT-1](ot1-observe-truth-surface-audit.md) (reads). RT fenced **who may write**
owner state; OT made **reads tell the truth** about owners; **EX governs the act of
taking an action with side effects** — the third leg:

> Writes go through owners. Reads tell the truth about owners. **Actions go through
> the governed execute path.**

## 0. North star

An action with side effects (process control, remediation, a cluster mutation,
a workflow step that changes the world) is governed when it is:

1. **Routed through the governed execute path** — plan → validate → check-approval →
   execute; the MCP governor is the only agent-facing execution surface.
2. **Leader-fenced** where it mutates cluster state (only the elected authority acts).
3. **Approval-gated** for high-risk / Tier-2 actions (a human approves before execute).
4. **Evidence-trust-gated** for autonomous remediation (stale/unverifiable evidence
   blocks execution — the OT→EX bridge).
5. **Audited** — every action leaves a durable receipt.

## 1. Contract backbone (from the awareness graph)

| Contract | Class / status | What it requires of execution |
|---|---|---|
| `mcp.governor.plan_validate_approve_execute_is_only_execute_path` | intent / seed | The governor is the *only* execution surface; agent actions follow plan→validate→check-approval→execute |
| `doctor.evidence_trust_must_be_authoritative_for_execution` | invariant / **active, CRITICAL** | Stale or unverifiable evidence must **block** autonomous remediation — backed by `CheckEvidenceTrust`, `remediation_gate.go`, and the ratchet `test.doctor_evidence_stale_blocks_execution` |
| `remediation.must_go_through_workflow` | intent / seed | Remediation is a workflow, not a hidden command |
| `deployment.automatic_rollback_is_forbidden` | intent / extracted_candidate | Destructive actions are operator-chosen, never automatic |

Named **forbidden fixes** already protect the evidence gate:
`disable_evidence_trust_gate_before_remediation` (don't remove `CheckEvidenceTrust`
before dispatch) and `treating_peer_estimates_as_authoritative_evidence`.

**Headline:** unlike RT and OT at audit time, the EX surface is **already
substantially governed.** The governor, the three-tier permission model, the
leader-fence, the approval-token path, and the evidence-trust gate all *exist and
are wired*. EX-1's job is therefore less "build the governance" and more "find the
one surface where the contract is incoherent, and the smaller durability gaps."

## 2. Surface A — governed execution paths (credit what exists)

Each verified against live code (file:line) this audit, not assumed.

| Surface | Gate | Verdict |
|---|---|---|
| **MCP governor** (`mcp/tools_governor.go`) | `ValidateCommand(req)` → `StatusNeedsConfirmation` → `ExecuteCommand(req)` (:209–219); `CheckApproval(cmdPath)` (:438); read-only `validate`/`check_approval` tools | ✅ Governed — the gated execute path |
| **AI executor three-tier** (`ai_executor_server/remediator.go`) | Tier 0 → `ACTION_SKIPPED`; Tier 1 → dispatch via **event bus** (`PublishEvent`, not direct exec); Tier 2 → `ACTION_PENDING`; unknown tier → defaults to approval-required (fail-safe) | ✅ Governed |
| **AI executor Tier-2 approval** (`ai_executor_server/handlers.go:117`, `job_store.go:126,152`) | `jobStore.approve(incidentId, approvedBy)` → `JOB_APPROVED` state machine; idempotent; deny-then-approve rejected (tested in `job_store_test.go`) | ✅ Governed via a real approval RPC |
| **cluster_doctor remediation** (`cluster_doctor_server/handler_remediation.go`) | Leader-fenced `if !s.isAuthoritative.Load()` (:69 and re-checked :108); approval token `security.ValidateApprovalToken(... approvalReplayStore)` single-use (:239); `heal_mode=observe` default (no auto-exec); hard blocklist `ETCD_PUT/DELETE/NODE_REMOVE` never auto-executable | ✅ Governed — leader + approval + evidence + blocklist |
| **Evidence-trust gate** (`remediation_gate.go`, `CheckEvidenceTrust`) | Stale/unverifiable evidence blocks autonomous remediation (the CRITICAL active invariant) | ✅ Governed — the OT→EX bridge |
| **Workflow engine** (`workflow/engine/actors.go`) | Typed registered actor handlers; no free-form shell; one read-only `systemctl is-active` fallback | ✅ Governed |
| **Cluster controller** | No `os/exec` — delegates to node_agent via gRPC; `check-controller-no-exec` CI gate forbids `os/exec` import + `exec.Command` in the controller dir | ✅ Governed + enforced |

## 3. Surface B — the node_agent execution boundary (the real gap)

This is the EX-1 headline, and it required a **verify-first correction** of the
initial surface sweep, which reported "40+ CRITICAL boundary violations" measured
against CLAUDE.md HARD RULE #6 ("node_agent_server may use `os/exec` only within
`internal/supervisor/`"). That framing is wrong, but it points at a real gap.

**What's actually true (all personally verified this audit):**

- The **documented** contract (HARD RULE #6) says exec is confined to
  `internal/supervisor/`.
- The **enforced** contract (`make check-nodeagent-exec-boundary`) says the
  opposite: its own comment declares "the node agent is a system executor by
  design; exec is legitimate in `internal/`, `*_provider.go`, `*_handler.go`,
  `repair_actions.go`, `workflow_day0.go`, `heartbeat.go`, …" and it only flags
  exec in **generated/type files** (`*.pb.go`, `*_grpc.pb.go`, `types*.go`) — which
  by definition never contain exec. **The gate is vacuous: it passes
  unconditionally.**
- The **code** does a third thing: **103 direct `exec.Command` sites across 33
  files** outside `internal/`, mixing read-only probes (`etcd_probe`, `scylla_probe`,
  `minio_probe`, `status`, `hardware`, `search_logs_handler`, …) with **genuine
  mutations** — e.g. `event_publisher.go:511,514` runs `systemctl stop` + `disable`
  on a flapping unit; `minio_systemd_reconcile.go:579` runs `daemon-reload`;
  `restore_provider.go`, `grpc_scylla_remove_node.go` (`nodetool removenode`),
  `keepalived.go`, `infrastructure_actions.go`, `package_actions.go` mutate too.

**The gap is not "40 files to refactor" — it is that the node_agent has no coherent,
enforced execution contract.** Three sources of truth disagree (doc says
supervisor-only; check sanctions almost-anywhere; code does 103 mixed sites), and
the CI gate that should govern it is a no-op. For an execution-governance tier this
is the central finding: the one large execution surface with side effects on the
host is governed in *intent* but not in *enforcement*.

## 4. Verify-first corrections (overstated / retracted)

- ⛔ **"Day-0 bootstrap (`workflow_day0.go`) is an ungoverned violation" — RETRACTED.**
  It is an **explicitly sanctioned exception**: `check-nodeagent-exec-boundary`
  lists `workflow_day0.go` by name as "Day-0 bootstrap that orchestrates system
  setup." Day-0 runs before etcd/the governor exist, so it *cannot* route through
  them (bootstrap-boundary). Ungoverned-by-design and accepted — noted, not scoped.
- ⛔ **"AI executor Tier-2 has no approval validation; approval source unclear" —
  RETRACTED.** There is a real governed approval path: `handlers.go:117`
  `jobStore.approve(...)` → `JOB_APPROVED`, with idempotence and deny-then-approve
  rejection covered by `job_store_test.go`. Tier-2 defers to a real approve RPC, not
  a dead-end.

## 5. Confirmed smaller gap

- ⚠️ **Remediation-gate escalation state is in-memory only** (`remediation_gate.go`
  ~:34–49, `remediationGatePersistFn` is a no-op). The escalation counter
  (repeated cooldown rejections → require approval) resets on doctor restart, so a
  previously-escalated target silently de-escalates. Self-documented
  (`failure_mode: doctor.escalation_state_lost_on_restart`). **MEDIUM.**

## 6. Scoping → EX-2 / EX-3

1. **EX-2 — give the node_agent a coherent, enforced execution contract (HIGH).**
   Reconcile the three disagreeing sources: (a) decide the real rule — almost
   certainly "read-only probes are allowlisted; **mutating** actions
   (`systemctl start/stop/restart/enable/disable/daemon-reload`, `nodetool
   removenode`, etc.) must route through `internal/supervisor/`"; (b) make
   `check-nodeagent-exec-boundary` actually enforce it (scan real source for
   mutating verbs outside `internal/`, not just generated files); (c) update
   HARD RULE #6 / CLAUDE.md so the doc matches the enforced rule. The migration of
   the handful of mutating call sites (`event_publisher`, `minio_systemd_reconcile`,
   …) onto the supervisor is the concrete work; the **non-vacuous gate is the
   ratchet** that keeps it closed. Mirrors RT-3 (route writes through the guarded
   primitive + a coverage ratchet).
2. **EX-3 — persist the remediation-gate escalation state (MEDIUM).** Back the
   escalation counter with etcd/ai-memory so a restart cannot silently de-escalate
   a target that has been repeatedly auto-rejected.
3. *(Noted, not scoped)* Day-0 bootstrap is ungoverned-by-design; acceptable under
   the bootstrap-boundary intent. If ever tightened, it belongs in a Day-0 hardening
   track, not EX.

## 7. One-line close

RT fenced the hands; OT cleared the eyes; EX finds the governor, the tiers, the
leader-fence, and the evidence gate **already standing** — and one surface, the node
agent's host execution, where the contract is written three different ways and the
gate that should enforce it does nothing. EX-2 makes that one contract coherent and
its gate real.
