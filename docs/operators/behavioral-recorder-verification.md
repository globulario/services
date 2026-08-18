# Verifying behavioral-recorder delivery on bare metal

**Scope**: proves the behavioral-memory learning loop is *observable* — that a
delivery failure and a subsequent recovery both become operator-visible on a
real cluster. Written for the 1.2.295 release check
([#238](https://github.com/globulario/services/issues/238)).

**What this does not prove**: that findings are *semantically* correct. It
proves the delivery path reports the truth about itself.

---

## Why a live run is needed at all

The unit tests prove classification and projection through a deterministic seam.
They deliberately do not touch etcd, DNS, or a running AI-memory — depending on
ambient state is exactly what made the previous test worthless, and the fix was
to stop depending on it.

That leaves one thing unproven in CI by construction: that on a real host the
recorder's *endpoint resolution* reaches a real service, and that a real outage
lands in the operator's journal rather than in silence. That is what this runbook
covers, and it is the whole of what it covers.

## Preconditions

- 1.2.295 installed on the cluster.
- `globular-cluster-doctor` active on at least two nodes (one leader, one
  follower). The follower matters: recorder health is **not** leader-gated, and
  step 4 checks that a follower reports its own recorder.
- Recorder health is projected on its own 60s ticker, independent of
  `healer_enabled` and of leadership, so this runbook works on a doctor with
  healing turned off. Every step below waits **two ticks** before concluding
  anything.

## Safety

Stopping AI-memory degrades **learning only**. The doctor's own verdict, the
four-layer convergence model, and remediation are unaffected — that is required
behaviour 6, and step 3 is where you confirm it on live data instead of trusting
the test. Nothing here writes etcd, changes desired state, or touches package
versions.

Service control goes through the node agent's supervisor path (`globular node
control`), never a hand-run systemd command — the supervisor is the single
allowlisted, auditable route, and an out-of-band stop would leave no receipt.

Bounded: total wall time is roughly `6 x 60s` plus service restart time. If any
step does not reach its expected state within **two ticks**, stop and treat it
as a finding — do not extend the wait and do not cut the release.

## The observation surface

Recorder health is a node-local log projection emitted **on transition**, from
its own 60s ticker, on every doctor instance:

```bash
journalctl -u globular-cluster-doctor -f | grep -i "behavioral recorder"
```

Classes you will see in the `health` field:

| Class | Meaning |
|---|---|
| `no_delivery_attempted` | recorder exists, nothing tried yet — **not** health |
| `healthy` | most recent outcome was a successful write |
| `recovered` | failed before, has succeeded since |
| `queue_pressure` | bundles dropped before delivery |
| `delivery_failing` | bundles accepted and then terminally lost |
| `recorder_unavailable` | this doctor has no recorder at all |

Emission is edge-triggered, so a sustained failure is announced **once**. Absence
of a repeat line is not evidence of recovery; only a `recovered` / `healthy`
transition is.

---

## Procedure

### 1. Establish successful delivery

```bash
# Force a fresh report so the recorder is actually asked to deliver something.
globular doctor report cluster --fresh
journalctl -u globular-cluster-doctor --since "-5min" | grep -i "behavioral recorder"
```

**Expect**: a transition to `healthy`, with `persisted > 0` and a
`last_success_at` timestamp.

**Record**: `persisted`, `last_success_at`, and the report's finding count.

> A `no_delivery_attempted` line here means the doctor produced no findings to
> record. An idle cluster cannot demonstrate delivery, and reading idle as
> success is the exact confusion this work removed. Produce a finding first.

### 2. Make delivery unavailable

Stop AI-memory on every node that runs it, through the node agent:

```bash
globular node control --unit ai-memory --action stop --node <node-ip>:11000
```

Do **not** stop etcd, the controller, or the doctor. The point is an AI-memory
outage, not a cluster outage.

### 3. Confirm the primary report is unaffected

Before looking at the recorder at all:

```bash
globular doctor report cluster --fresh
```

**Expect**: a complete report, the same finding count as step 1, no added
latency beyond normal variance, exit status 0.

This is required behaviour 6 on live data. If the report degrades here, **stop**
— learning has inverted into a dependency of reporting, and that is a release
blocker regardless of what the rest of this runbook shows.

### 4. Produce one observation and prove the failure is visible

```bash
globular doctor report cluster --fresh   # accepted into the queue, then lost
# wait two projection ticks (~2 min)
journalctl -u globular-cluster-doctor --since "-5min" | grep -i "behavioral recorder"
```

**Expect**, on the leader **and** on a follower:

- `health=delivery_failing`, `previous=healthy`
- `failed > 0`, non-empty `last_error`, and a `last_failure_at` later than
  step 1's `last_success_at`
- log level WARN, and **no** cluster-level finding created — recorder health is
  node-scoped and must never enter the cluster delta cache

**Record**: `failed`, `last_failure_at`, `last_error`, and which nodes reported.

> The bundle in this step was **accepted** by the queue and lost afterwards.
> Nothing rejected an enqueue. Seeing it at all is the defect being closed: the
> old code read Stats only when `Enqueue` itself returned false.

### 5. Restore AI-memory

```bash
globular node control --unit ai-memory --action start --node <node-ip>:11000
globular node control --unit ai-memory --action status --node <node-ip>:11000
```

Wait for it to report active before continuing. Restoring the service is not
itself recovery of the recorder — step 6 is.

### 6. Produce another observation and prove recovery

```bash
globular doctor report cluster --fresh
# wait two projection ticks (~2 min)
journalctl -u globular-cluster-doctor --since "-5min" | grep -i "behavioral recorder"
```

**Expect**:

- `health=recovered`, `previous=delivery_failing`
- `last_success_at` strictly **later** than the `last_failure_at` from step 4

Recovery is justified by an observed success later than the failure it clears,
never by elapsed time and never by AI-memory merely being back up. `recovered`
without a newer `last_success_at` is a bug — report it.

Then confirm the observations actually landed:

```bash
globular ai memory query --project globular-services --type architecture --limit 5
```

**Expect**: rows corresponding to the step-6 findings.

---

## Result template

Fill this in and attach it to the release record:

```text
cluster / nodes:        ...
version under test:     1.2.295
projection interval:    60s

step 1 healthy:         persisted=...  last_success_at=...  findings=...
step 3 report intact:   findings=...   (matches step 1? yes/no)  latency=...
step 4 failing:         failed=...     last_failure_at=...  last_error="..."
                        reported by: leader=...  follower=...
step 6 recovered:       last_success_at=...  (> step 4 last_failure_at? yes/no)
step 6 persisted rows:  ...

verdict:                pass / fail
remaining uncertainty:  ...
```

A run is a **pass** only when steps 1, 3, 4 and 6 each met their expected state
within two ticks. A step that needed a longer wait, a retry, or an explanation
is a fail — it means the surface is not dependable, which is the condition #238
exists to remove.
