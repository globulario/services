# Remediation Contracts — Operator Runbook

This page covers the operator-facing surface of the remediation contracts
shipped in `docs/intent/remediation.*.yaml`,
`docs/intent/evidence.provenance_trust_levels.yaml`,
`docs/intent/audit.retention_and_correlation_policy.yaml`,
`docs/intent/runtime.identity_attestation.yaml`, and
`docs/intent/operator.override_intent.yaml`.

You need this page when:

- A remediation gets rejected with `approval_token invalid:` or
  `evidence trust=STALE`.
- You're issuing an operator override and the CLI demands `--override-*`
  flags you don't recognize.
- You want to tune the failure-rate breaker for an action class.
- You're auditing a past remediation and need to trace it across
  doctor → workflow → audit.
- You see a non-`TRUSTED` runtime attestation verdict in verifier output.

---

## 1. Approval tokens

Every MEDIUM/HIGH-risk remediation requires a signed approval token.
Bare `--approval some-string` no longer works — the server rejects any
token that isn't a valid Ed25519 JWT bound to this specific
(action_class, target, generation, finding_id) tuple.

### Mint a token from the CLI

```bash
# Refresh the doctor's finding cache first.
globular doctor report

# Mint a token bound to finding f-abc, step 1.
globular doctor mint-approval --finding f-abc --step 1
```

The CLI auto-resolves `action_class`, `target`, and the evidence-digest
`generation` from the doctor's current report so you don't have to
compute them. The token prints on stdout (diagnostics on stderr) so you
can pipe it directly:

```bash
globular doctor remediate f-abc --step 1 --approval "$(
  globular doctor mint-approval --finding f-abc --step 1
)"
```

### Token claims and what they mean

| Claim | What it binds to |
|-------|------------------|
| `action_class` | The exact `cluster_doctorpb.ActionType` (e.g. `SYSTEMCTL_STOP`) |
| `target` | The finding's `EntityRef` (node or service id) |
| `generation` | `sha256:<hex>` of the finding's current evidence |
| `finding_id` | The doctor finding id |
| `aud` | `remediation:<cluster_domain>` |
| `exp` | Token expiry (default 10m, max 1h) |
| `jti` | Single-use nonce — replay rejected via etcd-backed store |

If the doctor re-evaluates the finding between mint and use, the
`generation` digest changes and the token is rejected. Mint a fresh one.

### Why a token might be rejected

- `approval_token invalid: signature invalid` — token wasn't signed by
  the cluster's issuer key. Usually means it was minted on a different
  cluster.
- `approval_token invalid: token already used` — single-use enforcement
  tripped. Mint a new one.
- `approval_token invalid: ... mismatch` — the action_class/target/
  generation/finding_id you ran with doesn't match what was minted.
  Re-mint with the correct step or refresh the finding cache.
- `approval_token invalid: ... expired` — token passed `exp`. Re-mint.
- `approval_token invalid: etcd unavailable` — replay store can't reach
  etcd. The server fails closed: no replay record means no enforcement,
  which means no acceptance. Restore etcd before remediating.

---

## 2. Evidence trust gating

`cluster_doctor` classifies every finding's evidence and refuses live
remediation when the evidence is `STALE` or `UNTRUSTED`. Dry-run is
exempt — you can always inspect what *would* happen.

| Verdict | Source age | Authorizes remediation? |
|---------|-----------|-------------------------|
| `AUTHORITATIVE` | ≤ source's freshness window | Yes |
| `DEGRADED` | 1×–2× window | Yes (downgrade recorded in audit) |
| `STALE` | > 2× window | **No** |
| `UNTRUSTED` | Missing source/writer/timestamp | **No** |

Freshness windows by source (`golang/evidence/trust.go`):

| Source | Window |
|--------|--------|
| `etcd_contract` | 5 min |
| `workflow_receipt` | 10 min |
| `verifier_attestation` | 5 min |
| `controller_snapshot` | 2 min |
| `service_log` | 1 min |
| `telemetry` | 90 s |
| `operator_input` | 5 min |
| `inferred` | 30 s |

If you see `remediation blocked: evidence trust=STALE`, refresh the
finding (`globular doctor report`) and retry. The finding may auto-clear
between sweeps if the underlying condition resolved.

---

## 3. Failure-rate breakers (cluster-wide)

Auto-remediation throttling is cluster-wide and per-action-class. The
policy lives at `/globular/cluster_doctor/failure_rate_policy` in etcd;
absent key means defaults apply.

### Default budgets (from `golang/remediation/policy.go`)

| Action class | Threshold | Window |
|--------------|-----------|--------|
| `SYSTEMCTL_RESTART` | 5 | 30 min |
| `SYSTEMCTL_STOP` | 2 | 60 min |
| `SYSTEMCTL_DISABLE` | 2 | 60 min |
| `PACKAGE_REINSTALL` | 2 | 24 h |
| `PACKAGE_REPAIR` | 3 | 60 min |
| `FILE_DELETE` | 1 | 24 h |
| `OBJECTSTORE_REPAIR` | 1 | 24 h |
| (default) | 3 | 30 min |

### Publish a custom policy

```bash
# Tighter SYSTEMCTL_STOP budget; everything else stays default.
cat > /tmp/policy.json <<'EOF'
{
  "default": {"threshold": 3, "window": 1800000000000},
  "class_policies": {
    "SYSTEMCTL_STOP": {"threshold": 1, "window": 3600000000000}
  }
}
EOF

etcdctl put /globular/cluster_doctor/failure_rate_policy "$(cat /tmp/policy.json)"
```

Window values are `time.Duration` nanoseconds:
`1800000000000` = 30 min, `3600000000000` = 60 min, `86400000000000` = 24 h.

Defaults merge in for any action class your override doesn't name — a
partial JSON can never silently disable enforcement.

### When the breaker trips

You'll see:
```
auto-remediation escalated: failure-rate breaker open for SYSTEMCTL_RESTART:
5 failures within 30m0s ≥ threshold 5 — operator approval required
```

That's the contract working. Mint an approval token (§1) to bypass on a
single finding, or issue a structured override (§4) to bypass the
breaker policy itself for a bounded window.

---

## 4. Operator overrides — structured force

A bare `--force` flag is forbidden by the override contract. Commands
that previously took `--force` now require structured override flags:

```bash
globular node recover full-reseed \
  --node-id globule-dell \
  --reason "disk corruption — manual partition repair complete" \
  --force \
  --override-actor "alice@cluster" \
  --override-reason "manual partition repair verified; bypassing storage-node count check" \
  --override-policy "node.recovery.cluster_safety_checks" \
  --override-scope "node:globule-dell" \
  --override-lifetime 15m
```

The CLI refuses to send the gate-bypassing RPC unless all override
fields are well-formed:

- `--override-actor` non-empty
- `--override-reason` ≥ 10 characters (vague reasons like "force" fail)
- `--override-policy` names the gate being bypassed
- `--override-scope` narrows the override (typically a node id)
- `--override-lifetime` defaults to 15 m, max 1 h

The validated override is attached to the request's audit note so the
server records who bypassed which policy, why, and when.

### Commands currently gated on Override

Today: `node recover full-reseed`. Other commands (`backup restore`,
`pkg override`, `services desired set`, `repo verify repair`) still
accept bare `--force` — they'll be migrated incrementally. The pattern
is documented in `golang/globularcli/override_flags.go`.

---

## 5. Audit correlation and retention

Every remediation produces a `RemediationAudit` written under
`/globular/cluster_doctor/audit/{audit_id}` in etcd. Records are leased
for **30 days** (`RemediationAuditRetention` constant in
`golang/cluster_doctor/cluster_doctor_server/executor.go`).

### Joining audits across services

Audits carry these correlation fields:

| Field | Source |
|-------|--------|
| `audit_id` | Generated per-call (`rem-<unix>`) |
| `correlation_id` | Propagated via `x-globular-correlation-id` gRPC metadata; falls back to a deterministic id derived from finding+step |
| `workflow_run_id` | Propagated via `x-globular-workflow-run-id` when the call originated from a workflow |
| `token_jti` | jti of the approval token that authorized the action (never the token itself) |
| `evidence_trust` | `AUTHORITATIVE`/`DEGRADED`/`STALE`/`UNTRUSTED` at decision time |
| `evidence_digest` | sha256 of the finding's evidence — same `generation` the approval token bound to |

Querying audits by correlation:

```bash
etcdctl get --prefix /globular/cluster_doctor/audit/ \
  | jq 'select(.correlation_id | startswith("wf-run-789"))'
```

### Redaction

`RemediationAudit.Redacted()` strips approval-token-like material from
the action's `Params` before persisting. Both key-pattern matching
(`token`, `secret`, `password`, `api_key`) and value-shape matching
(JWT-shaped values) apply. The audit record never contains a raw
token — only the `token_jti` (a public identifier).

---

## 6. Runtime attestation verdicts

The verifier emits an `AttestationVerdict` per (service, node) target,
distinct from the coarser `ProofStatus`:

| Verdict | Meaning | Operator action |
|---------|---------|-----------------|
| `TRUSTED` | PID + exe + hash + service id + build id + launch authority all bind, observed-after-start | None |
| `UNVERIFIED` | Required fields missing (no PID, no exe, no service id) | Re-run `GetServiceRuntimeProof`; check node_agent health |
| `MISMATCH` | A binding failed — running binary hash differs from expected, or service id doesn't match | Inspect `node_agent`; possibly old PID after upgrade |
| `ORPHAN` | Process has no recorded launch authority (no systemd unit) | Investigate cgroup-escaped process; consider kill + reinstall |
| `STALE_OBSERVATION` | Attestation's `ObservedAt` predates `ProcessStartTime` | Attestation was replayed from a previous PID generation — re-collect proof |

Wrapper services (`keepalived`, `scylladb` upstream) skip the binary-
hash binding because the manifest checksum is synthetic. They can still
return any of the other verdicts.

Build-id binding is currently skipped when the service doesn't expose a
`/version` endpoint (most services today). The reason field will say
"build_id not reported by service" — operators see the gap explicitly.

---

## 7. Common worked examples

### "The doctor keeps trying to restart echo and failing"

```bash
# 1. See the latest report and the failing finding.
globular doctor report --json | jq '.findings[] | select(.invariant_id=="runtime.desired_enabled_not_alive")'

# 2. Check the breaker state — likely tripped after 5 failures.
etcdctl get --prefix /globular/cluster_doctor/audit/ \
  | jq 'select(.action_type=="SYSTEMCTL_RESTART" and .executed==false)' \
  | wc -l

# 3. Mint an approval token to bypass the breaker for one fresh attempt.
TOKEN=$(globular doctor mint-approval --finding <id> --step 0)
globular doctor remediate <id> --step 0 --approval "$TOKEN"
```

### "I need to force a node reseed even though we only have 2 storage nodes"

```bash
globular node recover full-reseed \
  --node-id globule-dell \
  --reason "disk corruption — confirmed safe to lose this node's data" \
  --force \
  --override-actor "alice@cluster" \
  --override-reason "tested cluster survives N-1 storage nodes for this recovery window" \
  --override-policy "node.recovery.cluster_safety_checks" \
  --override-scope "node:globule-dell" \
  --override-lifetime 30m
```

### "I want to audit who approved a remediation last week"

```bash
# Find audits for a specific finding.
etcdctl get --prefix /globular/cluster_doctor/audit/ \
  | jq -s '[.[] | select(.finding_id=="<id>")] | sort_by(.timestamp)'

# Find every audit that bypassed a specific policy via override.
# (Override entries are tagged in the note field today; first-class
#  Override audit shape lands when the server proto adds the fields.)
etcdctl get --prefix /globular/cluster_doctor/audit/ \
  | jq 'select(.note | contains("policy=node.recovery.cluster_safety_checks"))'
```

---

## See also

- `docs/intent/remediation.token_contract.yaml`
- `docs/intent/evidence.provenance_trust_levels.yaml`
- `docs/intent/remediation.failure_rate_policy.yaml`
- `docs/intent/audit.retention_and_correlation_policy.yaml`
- `docs/intent/runtime.identity_attestation.yaml`
- `docs/intent/operator.override_intent.yaml`
- `docs/intent/workflow.remediation_truth_consistency.yaml`
- `docs/intent/service.dependency_degradation_modes.yaml`
- `docs/operators/cluster-self-healing.md` — the broader self-healing model
