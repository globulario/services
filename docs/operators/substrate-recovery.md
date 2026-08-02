# Substrate Recovery — etcd dump, restore, and the recovery ladder

**The contract**: the coordination store (etcd) must be recreatable from
durable authority, bounded desired-state backup, and live observation. Loss of
its quorum may suspend convergence, but must not destroy the information
required to reconstruct convergence. No correctness-critical fact may exist
only in etcd.

**The restore law**: restored desired state is evidence, not immediate
authority. It must be reconciled against newer durable and observed truth
before destructive convergence is permitted.

Implementation: `golang/substrate/` + `globular substrate ...`
(origin: the 2026-07-10 quorum-loss incident, where a recoverable node
failure disabled the mechanism responsible for recovery).

## Dumps

```bash
globular substrate dump            # writes /var/lib/globular/backup/etcd/globular-dump-<ts>-rev<N>.json
```

- Captures the **full** `/globular` keyspace at one consistent revision using
  **serializable (local) reads** — this works on a quorum-less member, which
  is what makes rung-2 recovery possible.
- The manifest records `cluster_uid`, `desired_epoch`, source revision, and a
  payload SHA-256. `desired_epoch` is the max ModRevision across
  desired-state keys only — heartbeats, leases, and observations do not
  advance it, so dump selection orders by desired state, never by timestamp.
- Files are 0600: dumps contain operator secrets.
- **Schedule it.** Run from cron or a systemd timer on every control-plane
  node, e.g. `*/30 * * * * root /usr/lib/globular/bin/globular substrate dump`.
  A dump is the rung-3 floor; without one, the floor is Day-0.5 re-bootstrap
  from `release-index.json`.

## Classification (applied at restore, recorded per prefix in `golang/substrate/policy.go`)

| Class | Meaning | Examples |
|---|---|---|
| `RESTORE_AUTHORITATIVE` | Identity, trust anchors, secrets, monotonic counters, audit — nothing can recompute these | cluster id, PKI, secrets, audit/ledger, epochs, bootstrap markers |
| `RESTORE_AS_UNVERIFIED` | Desired state / durable intent — restored, but gated by the marker | DesiredService, releases, workflows definitions, domain/ingress specs, controller state |
| `REBUILD_FROM_OBSERVATION` | Owners re-publish; restoring stale copies poisons the 4-layer model | installed packages, host lists, discovery endpoints |
| `DISCARD` | Ephemera; stale approvals are actively dangerous | leases, locks, leader keys, heartbeats, run state, delete approvals |

Structural overrides: any `/locks/` segment or `/lock` suffix → DISCARD;
service `instances/`/`runtime` subtrees → DISCARD. Unknown prefixes restore
as UNVERIFIED and are **reported** — an unknown prefix is a
classification-table gap, fix `policy.go`.

## The recovery ladder

Try each rung in order; every rung is bounded and evidence-gated.

### Rung 1 — restart a stopped existing member

```bash
globular substrate recover --restart-members
```

Starts `globular-etcd` / `globular-node-agent` only if the unit file exists
and (for etcd) the data dir is non-empty — an existing member restarting with
its own identity. Never installs, never bootstraps, never mutates membership.
This is the fix for the 2026-07-10 class of incident: a member stopped by an
interrupted operation.

### Rung 2 — rebuild from the surviving member

```bash
globular substrate recover --from-survivor          # on the surviving node
```

When the rest of the membership is unrecoverable: takes a dump first
(evidence before mutation; refuses to proceed without one unless `--force`),
copies the data dir aside as the rollback point, runs etcd once with
`force-new-cluster` (drops dead members, keeps **all** data), proves
single-voter quorum with a linearizable read, hands back to the normal unit,
and writes the `RESTORED_UNVERIFIED` marker. Other nodes then rejoin fresh
via the normal join path.

### Rung 3 — recreate from a dump

```bash
globular substrate recover --from-dump               # best dump by desired_epoch
globular substrate recover --from-dump /path/x.json  # explicit file
globular substrate recover --from-dump --dry-run     # classify + report only
```

Into a fresh (or freshly re-bootstrapped) etcd. Guards: cluster-UID mismatch
refused; keys that already exist live are never overwritten (live evidence
wins); lease-bound keys never restored. `--force` overrides both guards.

### Rung 4 — Day-0.5

No dump either: re-bootstrap from `release-index.json` + node re-registration
against already-installed machines. (Not a command; this is the normal Day-0
path run against non-pristine machines after `clean`.)

## The marker

Every rung-2/3 recovery writes `/globular/recovery/v1/restore` with status
`RESTORED_UNVERIFIED`. Until it is flipped, restored desired state is
evidence only — controllers must not take destructive convergence actions
from it (controller-side enforcement of this gate is tracked as P2; until
then it is operator discipline).

```bash
globular substrate status          # show the marker
globular substrate mark-verified --note "convergence re-observed, drift resolved"
```

Verify before attesting: nodes heartbeating, installed state re-synced,
doctor findings collapsed, no unexpected drift actions queued.

## What this deliberately does not do

- No witness nodes, no learner choreography as survival requirements — the
  dump replaces membership preservation.
- No automatic rung-2/3: they mutate membership or seed state and require an
  explicit operator command until field evidence earns automation.
- No restore of observations, runs, locks, or approvals — yesterday's weather
  stays in yesterday.
