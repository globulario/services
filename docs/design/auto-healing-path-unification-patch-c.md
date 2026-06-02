# Patch C — Unify Path B into Path A (design note, not yet implemented)

**Status:** design-only, awaiting approval. Patches A and B have shipped (see commit history) and removed the immediate risks. Patch C is the structural follow-up that closes the remaining gap.

**Author intent:** one finding → one policy → one execution gate → one etcd audit trail → one verification path. Today the cluster_doctor has two parallel mutation paths; this note proposes how to merge them safely without removing any current capability.

---

## 1. Current state after Patches A + B

- Default `HealerMode = "observe"` — background mutation requires an explicit operator opt-in.
- `release.stuck_resolved` is `HealPropose` — the direct-etcd-write hot path is no longer reachable from `executeAutoAction` policy.
- The remaining `HealAuto` actions (`delete_stale_cache`, `clear_resolved_drift`, `seed_ops_knowledge`) still dispatch through `Healer.executeAutoAction` → `RemoteOps.*`, bypassing the gated `ExecuteRemediation` path.
- This is acceptable as a temporary posture because the default mode no longer triggers these actions, but the architectural concern is still open: an operator who flips to enforce mode reintroduces an under-gated mutation path.

---

## 2. The end-state we want

```text
              (one steering wheel)

  Doctor finding
    │
    ▼
  Policy classifier (HealAuto | HealPropose | HealObserve)
    │
    │ if HealAuto
    ▼
  Healer constructs a RemediationAction proto for the finding
    │
    ▼
  ExecuteRemediation  ← single gate
    │   • leader-only
    │   • evidence-trust
    │   • hard-blocklist
    │   • approval (or trusted-system token)
    │   • cooldown
    │   • failure-rate
    │   • etcd audit (30d TTL)
    ▼
  ActionExecutor.Execute (typed handler per ActionType)
    │
    ▼
  Verification step (re-evaluate invariant)
    │
    ▼
  Audit close-out + (if regressed) propose escalation
```

No code path outside this funnel mutates cluster, node, package, release, file, or service state.

---

## 3. Two implementation options

### 3.1 Option C.1 — Expand `ActionExecutor` (preferred long-term)

Add typed `ActionType`s for the actions the healer currently dispatches:

```text
DELETE_CACHE_ARTIFACT
CLEAR_DRIFT_OBSERVATION
SEED_OPS_KNOWLEDGE
PATCH_RELEASE_PHASE   (still hard-blocked from auto unless opted in)
```

Each gets a typed handler in `ActionExecutor.Execute()`:

```go
case cluster_doctorpb.ActionType_DELETE_CACHE_ARTIFACT:
    return e.deleteCacheArtifact(ctx, params, dryRun)
case cluster_doctorpb.ActionType_CLEAR_DRIFT_OBSERVATION:
    return e.clearDriftObservation(ctx, params, dryRun)
// …
```

`Healer.executeAutoAction` is replaced with a constructor:

```go
func buildRemediationAction(autoAction string, f Finding) *cluster_doctorpb.RemediationAction {
    // returns a typed ActionType + Params map keyed off the finding
}
```

The healer loop then becomes:

```go
for _, f := range findings {
    rule := LookupPolicy(f.InvariantID)
    if rule.Disposition != HealAuto || rule.AutoAction == "" {
        continue
    }
    action := buildRemediationAction(rule.AutoAction, f)
    req := &cluster_doctorpb.ExecuteRemediationRequest{
        FindingId:     f.FindingID,
        StepIndex:     0,
        ApprovalToken: systemHealerToken,
        DryRun:        h.DryRun,
    }
    resp, err := h.executeRemediation(ctx, req)
    // record into the same etcd audit stream
}
```

#### Trade-offs
- **Pros:** single ActionType vocabulary, single executor, single audit, single set of gates. The healer becomes a thin policy classifier that emits typed actions — exactly the shape the docs already describe.
- **Cons:** larger refactor; adds 4 new `ActionType` enum values to the proto + regenerates pb; touches `actionExecutor` and every test that exhaustively asserts on action types.

### 3.2 Option C.2 — Route through the WorkflowService

Keep the existing `RemoteOps` actions but wrap each healer dispatch in a `RunRemediationWorkflow` call. The workflow engine's `ExecuteRemediation` actor callback already routes through the gated handler, so the gates fire transparently.

```go
for _, f := range findings {
    rule := LookupPolicy(f.InvariantID)
    if rule.Disposition != HealAuto {
        continue
    }
    _, err := s.RunRemediationWorkflow(ctx, f.FindingID, 0, systemHealerToken, h.DryRun)
}
```

The workflow service must be configured (`workflow_endpoint` resolved); otherwise the healer falls back to the existing path. This is a hard precondition — if absent in enforce mode, the healer must refuse to dispatch rather than silently bypass the gate.

#### Trade-offs
- **Pros:** smallest code change; existing `RemoteOps` implementations can stay (the workflow engine resolves them through the actor callback). All gates apply because the workflow's `ExecuteAction` step routes through `ExecuteRemediation`.
- **Cons:** every healer tick now incurs a workflow-service round-trip; the cluster gains a hard dependency on workflow-service availability for any auto-heal mutation. Pile-up risk if workflow-service is itself unhealthy and the doctor keeps proposing remediation for it.

### 3.3 Hybrid (recommended)

- Use **C.2** for actions whose mutation already crosses a service boundary (`clear_resolved_drift` → workflow service, `patch_release_available` → etcd-via-controller). Workflow is the natural transport.
- Use **C.1** for actions that are intrinsically local executor operations (`delete_stale_cache` → node_agent RPC, `seed_ops_knowledge` → ai-memory RPC). They naturally fit the executor's typed-action model.

This minimizes refactor size while putting every mutation behind the same gate set.

---

## 4. The "system healer token" problem

If the healer is to call `ExecuteRemediation`, it must satisfy the approval gate when the rule's action would normally require it. Options:

1. **A system-scoped approval token** issued at doctor startup, bound to a fixed actor identity (`healer@cluster_doctor`), with a narrow allowlist of `(invariant_id, action_class)` pairs. Refreshed on leadership transitions.
2. **A new `ExecuteRemediationRequest.SystemHealer` flag** that says "this is an internal auto-heal dispatch; apply the cooldown/failure-rate/evidence gates but skip the approval-token check IFF the action_class is on the system-healer allowlist." Auditable, narrow, no token machinery.

Option 2 is preferred because it surfaces the elevated trust in the proto contract instead of hiding it in a token claim. The action class allowlist becomes the durable security boundary, and operator approval is still required for anything outside that allowlist.

---

## 5. Required tests before Patch C ships

| # | Test | Purpose |
|---|---|---|
| C-T1 | `TestHealer_AllAutoActionsRouteThroughExecuteRemediation` | every HealAuto evaluation produces an `ExecuteRemediation` call (or a workflow dispatch that re-enters `ExecuteRemediation`) |
| C-T2 | `TestHealer_EveryExecutedActionWritesEtcdAudit` | for every HealAuto execution, a corresponding `/globular/cluster_doctor/audit/rem-*` record is written |
| C-T3 | `TestHealer_BypassPath_FailsClosed` | a healer with `executeRemediation == nil` (or workflow client nil) must REFUSE to mutate in enforce mode |
| C-T4 | `TestExecuteRemediation_SystemHealerActor_BoundedByAllowlist` | a system-healer dispatch for an action class outside the allowlist is rejected with the same machinery as a non-token operator call |
| C-T5 | `TestRemediation_SingleAuditPerAction` | dual-path regression guard — exactly one audit record per executed action; never both healer-ring AND etcd audit firing for the same finding |
| C-T6 | `TestHealer_DryRun_NoMutations` (already exists) | continues to pass after the refactor |

---

## 6. Migration plan

1. **Decide on hybrid vs. pure C.1 vs. pure C.2.** Recommend hybrid.
2. **Add `SystemHealer` field to `ExecuteRemediationRequest`** + actor allowlist in cluster-doctor config. Land that proto bump first as a self-contained PR.
3. **Switch one HealAuto action at a time** through `ExecuteRemediation`, starting with the most-firing rule (`artifact.cache_digest_mismatch` is the safest first migration: small blast radius, well-tested action).
4. **Each migration ships with:** a test from §5 + a deprecation log on the old `Healer.executeAutoAction` dispatch case so operators see the path is being retired.
5. **After all HealAuto actions migrate, delete `actionPatchReleaseAvailable` + `RemoteOps.PatchReleasePhase`.** The unreachable code from Patch B is removed.
6. **After deletion**, default `HealerMode` can safely return to `"enforce"` IF the user wants — but the default stays `"observe"` per Patch A unless the operator opts in.

---

## 7. Out of scope for Patch C

- Adding new HealAuto invariants.
- Changing the failure-rate / cooldown / evidence-trust policies.
- Workflow-service architectural changes.
- Any change to non-doctor services.

Patch C is purely about routing existing capability through one gate. Capability expansion is a later round.

---

## 8. Decision points awaiting operator input

- **Hybrid vs. one option?** (Recommend hybrid — see §3.3.)
- **`SystemHealer` proto flag vs. token-bound actor?** (Recommend flag — see §4.)
- **Migration order — start with `artifact.cache_digest_mismatch` or `ops_knowledge.seed_deferred`?** (Recommend cache_digest_mismatch — narrower allowlist needed, executor handler is trivial.)
- **What happens to the unreachable `actionPatchReleaseAvailable` code today?** Leave in place until Patch C ships, or delete now and re-add via the new path? (Lean: leave in place; deletion is a separate cleanup PR if at all.)
