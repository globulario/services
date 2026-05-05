# Globular Failure Patterns and Invariants

This document captures failure patterns discovered during Globular cluster simulations and converts them into architecture-level invariants.

The goal is not only to fix individual bugs. The goal is to make whole classes of failures impossible, visible, or safely recoverable.

## Core principle

```text
Missing state is not desired state.
```

For critical infrastructure, missing, stale, unreadable, or invalid state must not be interpreted as permission to destroy runtime state.

Critical infrastructure must fail conservative, not destructive.

When truth is missing:

```text
do not destroy
do not disable
do not wipe
do not overwrite with empty state
hold last known good
degrade loudly
ask the authority to republish
```

---

## Pattern 1: Critical State Under-Replicated

### Pattern name

```text
CRITICAL_STATE_UNDER_REPLICATED
```

### Description

Critical control-plane data is stored with insufficient replication for the current cluster topology.

Example: a multi-node cluster where critical ScyllaDB keyspaces still use `RF=1`.

### Failure symptom

A single node failure makes critical cluster data unavailable.

Examples:

```text
globular_dns rows unavailable
globular_projections rows unavailable
controller or DNS cannot read required state
```

### Required invariant

```text
No multi-node Globular cluster may run critical ScyllaDB keyspaces with RF=1.
```

### Enforcement rule

```text
if cluster_storage_nodes == 1:
    required_rf = 1

if cluster_storage_nodes == 2:
    required_rf = 2

if cluster_storage_nodes >= 3:
    required_rf = 3
```

### Required enforcement point

A controller subsystem should continuously enforce this policy.

Suggested component:

```text
scylla_schema_guard
```

Responsibilities:

```text
inspect critical keyspaces
compare actual RF with required RF
emit doctor finding if RF is too low
ALTER KEYSPACE when safe
mark repair required
trigger or request repair
```

### Critical keyspaces

At minimum:

```text
globular_dns
globular_projections
workflow state / receipts
repository ledger
event bus state
cluster operational status
any state required by controller, DNS, package reconciliation, or doctor
```

### Required tests

```text
5-node cluster with globular_dns RF=1 must raise CRITICAL finding
5-node cluster with globular_projections RF=1 must raise CRITICAL finding
schema guard must raise RF after cluster growth
schema guard must not lower RF automatically
repair-required status must be recorded after RF increase
```

---

## Pattern 2: Bootstrap State Escaped To Production

### Pattern name

```text
BOOTSTRAP_STATE_ESCAPED_TO_PRODUCTION
```

### Description

A setting that is safe during single-node bootstrap remains active after the cluster becomes multi-node.

Example:

```text
RF=1 is acceptable during Day-0 single-node bootstrap
RF=1 is unsafe after the cluster grows to 3 or 5 nodes
```

### Failure symptom

A Day-0 shortcut becomes a Day-2 production weakness.

### Required invariant

```text
Any bootstrap-only setting must have an automatic promotion rule when the cluster topology changes.
```

### Enforcement rule

```text
RF=1 is allowed only while cluster_size == 1.
Once cluster_size >= 2, RF=1 becomes a policy violation.
```

### Required enforcement points

Run safety-policy reconciliation on:

```text
node join
node leave
role change
controller leadership change
scheduled schema guard tick
cluster topology generation change
```

### Required tests

```text
single-node bootstrap allows RF=1
joining a second node requires RF=2
joining a third node requires RF=3
existing RF=1 keyspaces are upgraded after topology growth
policy violation remains visible until fixed
```

---

## Pattern 3: Absence As Destructive Intent

### Pattern name

```text
ABSENCE_AS_DESTRUCTIVE_INTENT
```

### Description

A missing desired-state key is interpreted as an instruction to disable or destroy runtime infrastructure.

Example:

```text
/globular/ingress/v1/spec is missing
node-agent treats missing spec as mode=disabled
keepalived stops
VIP disappears
```

### Failure symptom

Temporary control-plane blindness becomes a runtime outage.

### Required invariant

```text
Missing critical desired state is never a command to destroy runtime state.
```

### Enforcement rule

```text
missing desired state != disabled
missing desired state == unknown
unknown == hold last known good
```

Only explicit desired state may disable critical infrastructure.

For ingress:

```text
keepalived may stop only if:
    spec.mode == "disabled"
    spec.explicit_disabled == true
    spec.writer == "cluster-controller"
    spec.generation is valid
    spec.checksum is valid
```

### Required tests

```text
delete ingress spec and verify keepalived stays active when LKG exists
invalid ingress spec must not disable keepalived
stale ingress spec must not disable keepalived without explicit disable
explicit controller-written disabled spec must disable keepalived
```

---

## Pattern 4: No Last-Known-Good For Critical Runtime

### Pattern name

```text
NO_LAST_KNOWN_GOOD_FOR_CRITICAL_RUNTIME
```

### Description

Critical runtime components do not persist or honor the last known good configuration when desired state is temporarily unavailable.

### Failure symptom

A temporary read failure causes critical runtime infrastructure to disappear or reset.

### Required invariant

```text
Every critical runtime renderer must persist and honor last-known-good config.
```

### Runtime configs requiring LKG

```text
keepalived / VIP
Envoy bootstrap / xDS last applied snapshot
MinIO rendered topology and environment
DNS zone snapshot
systemd rendered units
repository storage endpoint config
```

### Enforcement rule

```text
if desired state cannot be read:
    use last-known-good
    mark node degraded
    emit doctor finding
    request controller republish
    do not destructively change runtime
```

### Required implementation behavior

LKG files should be:

```text
written atomically
versioned by generation
protected by checksum
validated before use
stored in /var/lib/globular/<subsystem>/last-known-good.json
```

### Required tests

```text
missing desired state uses LKG
invalid desired state uses LKG
unreadable etcd uses LKG
corrupt LKG is rejected safely
first boot with no LKG waits for authoritative spec instead of disabling explicitly
```

---

## Pattern 5: Orphaned Critical State

### Pattern name

```text
ORPHANED_CRITICAL_STATE
```

### Description

A critical state key exists, but no authoritative controller loop owns it, republishes it, validates it, or restores it when missing.

### Failure symptom

A critical key disappears and no authority recreates it.

### Required invariant

```text
Every critical state key must have exactly one authoritative writer and one guardian loop.
```

### Required metadata for critical keys

```text
key
owner
schema_version
writer_identity
generation
checksum
recovery_source
delete_policy
consumer_fallback
doctor_invariant
```

### Example

```text
key: /globular/ingress/v1/spec
owner: cluster-controller
consumer: node-agent
fallback: hold_last_known_good
delete_policy: explicit audited tombstone only
doctor: ingress.spec_missing
```

### Required tests

```text
deleting critical key triggers controller restore
malformed critical key triggers doctor finding
unauthorized writer is rejected or overwritten
controller periodically republishes owned critical state
```

---

## Pattern 6: Missing State Without Intent Marker

### Pattern name

```text
MISSING_STATE_WITHOUT_INTENT_MARKER
```

### Description

The system cannot distinguish accidental missing state from intentional disable/delete.

Example ambiguity:

```text
Was ingress disabled intentionally?
Or was /globular/ingress/v1/spec lost?
```

### Failure symptom

Accidental deletion and intentional disable look the same.

### Required invariant

```text
Destructive state transitions require explicit intent markers.
```

### Enforcement rule

Critical deletes or disables require:

```text
approved workflow id
actor identity
timestamp
previous generation
target generation
reason
audit record
```

Example tombstone path:

```text
/globular/ingress/v1/delete_approval/<generation>
```

Without approval:

```text
controller restores the key
consumer holds last-known-good
doctor emits critical finding
```

### Required tests

```text
unapproved delete is restored
approved disable is honored
approved delete contains audit record
missing key without tombstone is not treated as disabled
```

---

## Pattern 7: Global Reconcile Starvation

### Pattern name

```text
GLOBAL_RECONCILE_STARVATION
```

### Description

One blocked reconcile phase prevents unrelated recovery or desired-state publication work from running.

Example:

```text
projection reconcile gets stuck
controller logs previous run still active
release reconcile and ingress publication do not run
```

### Failure symptom

One slow phase freezes the whole control plane.

### Required invariant

```text
No non-critical reconcile phase may block critical reconcile lanes.
```

### Required architecture

Split reconciliation into isolated lanes:

```text
critical-state-publisher lane
package-release lane
projection lane
doctor lane
telemetry lane
```

Each lane requires:

```text
independent lock
independent timeout
independent circuit breaker
independent metrics
last_success timestamp
degraded status
```

### Enforcement rule

Projection failure must not block:

```text
ingress republication
objectstore desired-state publication
package repair planning
DNS repair
controller leadership duties
```

### Required tests

```text
hung projection scan does not block ingress publisher
hung projection scan does not block release reconcile
one lane timeout does not hold global lock
previous-run-active is lane-scoped, not global
```

---

## Pattern 8: Unbounded Critical Path Query

### Pattern name

```text
UNBOUNDED_CRITICAL_PATH_QUERY
```

### Description

A control-plane query runs without a strict timeout, retry budget, fallback, or failure classification.

### Failure symptom

A database scan or network call blocks control-plane progress indefinitely.

### Required invariant

```text
No control-plane query may run without timeout, degraded fallback, and lane isolation.
```

### Enforcement rule

Every critical query must have:

```text
context timeout
max retry budget
failure classification
degraded fallback
metric
structured error
```

### Required tests

```text
Scylla query hang times out
DNS reload query hang times out
repository metadata query hang times out
controller continues other lanes after timeout
query timeout emits doctor finding or degraded status
```

---

## Pattern 9: Derived State Blocks Authority

### Pattern name

```text
DERIVED_STATE_BLOCKS_AUTHORITY
```

### Description

Derived or rebuildable state becomes a hard dependency for authoritative control-plane work.

Examples of derived state:

```text
projections
indexes
cached node summaries
search views
aggregated status
UI convenience tables
```

### Failure symptom

A broken projection, index, or cache blocks actual cluster repair.

### Required invariant

```text
Derived state may degrade, but it must never block authoritative state repair.
```

### Enforcement rule

```text
if derived state is broken:
    mark stale/degraded
    continue critical reconcile
    rebuild asynchronously
```

### Required tests

```text
projection failure does not block package repair
projection failure does not block ingress spec publication
index failure does not block controller leadership work
derived state can be rebuilt after backend recovery
```

---

## Pattern 10: Recovery Depends On Degraded Discovery

### Pattern name

```text
RECOVERY_DEPENDS_ON_DEGRADED_DISCOVERY
```

### Description

The cluster needs a degraded discovery system, such as DNS, in order to repair that same discovery system or other critical systems.

### Failure symptom

The system needs DNS to repair DNS.

### Required invariant

```text
Emergency control-plane recovery must not depend exclusively on DNS.
```

### Required fallback paths

Critical repair clients must support:

```text
direct IP fallback
etcd-discovered endpoints
last-known endpoint cache
local node-agent endpoint
static bootstrap peers
```

### Commands that must work without DNS

```text
globular doctor
globular ingress
globular scylla
globular repository
globular node-agent
globular controller
```

### Required tests

```text
DNS down but doctor can target direct IP
DNS down but ingress status can be read
DNS down but schema guard status can be read
DNS down but controller can reach node-agent through fallback endpoint
```

---

## Pattern 11: Unguarded Runtime Destructive Action

### Pattern name

```text
UNGUARDED_RUNTIME_DESTRUCTIVE_ACTION
```

### Description

A local component performs a destructive runtime action based on uncertainty, missing state, stale state, or local inference.

### Destructive actions include

```text
stop keepalived
wipe MinIO data
remove etcd member
remove Scylla data
disable DNS zone
stop gateway / ingress
delete repository artifact
change active release pointer
```

### Failure symptom

Local uncertainty triggers a cluster-visible outage.

### Required invariant

```text
Destructive runtime actions require authoritative desired state, not local uncertainty.
```

### Enforcement rule

If an action is destructive, require:

```text
valid desired state
generation
checksum
explicit intent
audit record
safe fallback
```

### Required tests

```text
missing desired state cannot trigger destructive action
invalid desired state cannot trigger destructive action
explicit audited desired state can trigger destructive action
all destructive actions emit audit events
```

---

## Pattern 12: Topology Safety Drift

### Pattern name

```text
TOPOLOGY_SAFETY_DRIFT
```

### Description

The cluster topology changes, but safety policies do not update to match the new topology.

Examples:

```text
cluster grows to 5 nodes but Scylla RF stays 1
VIP participants do not match current control-plane nodes
MinIO topology does not match desired objectstore pool
etcd membership changes but quorum assumptions remain stale
```

### Failure symptom

The physical cluster changes, but durability, quorum, and failover policy remain stale.

### Required invariant

```text
Cluster topology changes must trigger safety-policy reconciliation.
```

### Policies affected by topology

```text
Scylla RF
MinIO erasure set / write quorum
etcd membership / quorum
DNS authority replicas
VIP participant set
controller eligible leaders
backup schedule
repair schedule
```

### Enforcement rule

On topology change:

```text
recompute safety policy
compare runtime against policy
emit doctor findings
dispatch repair workflows
record policy generation
```

### Required tests

```text
node join triggers RF policy recompute
node leave triggers degraded safety evaluation
role change updates VIP participant policy
objectstore topology mismatch emits finding
policy generation changes after topology update
```

---

# Invariant Coverage Matrix

Use this table to track whether each invariant is implemented, enforced, and proven by tests.

| Pattern | Status | Enforcement Point | Doctor Finding | Failure Test |
|---|---|---|---|---|
| CRITICAL_STATE_UNDER_REPLICATED | VIOLATED / UNPROVEN | scylla_schema_guard | scylla.keyspace.rf_policy_violation | required |
| BOOTSTRAP_STATE_ESCAPED_TO_PRODUCTION | VIOLATED / UNPROVEN | topology safety reconciler | topology.safety_policy_drift | required |
| ABSENCE_AS_DESTRUCTIVE_INTENT | VIOLATED | node-agent ingress reconcile | ingress.spec_missing | required |
| NO_LAST_KNOWN_GOOD_FOR_CRITICAL_RUNTIME | VIOLATED / UNPROVEN | node-agent runtime renderers | runtime.lkg_missing_or_invalid | required |
| ORPHANED_CRITICAL_STATE | VIOLATED / UNPROVEN | critical state guardian | critical_state.owner_missing | required |
| MISSING_STATE_WITHOUT_INTENT_MARKER | UNPROVEN | tombstone/audit workflow | critical_state.intent_missing | required |
| GLOBAL_RECONCILE_STARVATION | VIOLATED | reconcile lanes | reconcile.lane_timeout | required |
| UNBOUNDED_CRITICAL_PATH_QUERY | UNPROVEN | query wrappers / context policy | control_plane.query_timeout | required |
| DERIVED_STATE_BLOCKS_AUTHORITY | VIOLATED / UNPROVEN | projection lane isolation | derived_state.blocks_authority | required |
| RECOVERY_DEPENDS_ON_DEGRADED_DISCOVERY | UNPROVEN | direct endpoint fallback | recovery.discovery_dependency | required |
| UNGUARDED_RUNTIME_DESTRUCTIVE_ACTION | UNPROVEN | destructive action guard | runtime.destructive_action_unapproved | required |
| TOPOLOGY_SAFETY_DRIFT | VIOLATED / UNPROVEN | topology safety reconciler | topology.safety_policy_drift | required |

Status meanings:

```text
VIOLATED   - observed or strongly indicated by failure simulation
UNPROVEN   - implementation may exist, but no failure test proves the invariant
PREVENTIVE - not observed yet, but same class of risk should be guarded
COVERED    - implemented, enforced, and proven by failure test
```

---

# Subsystem Checklist

Every Globular subsystem should answer these questions.

```text
1. What is the authoritative state?
2. Where is it stored?
3. Who is allowed to write it?
4. Is it replicated enough for the current topology?
5. What happens if the state is missing?
6. What happens if the state is stale?
7. What happens if the state is invalid?
8. Is there last-known-good?
9. Is delete different from explicit disable?
10. Can this subsystem block global reconcile?
11. Does it have timeout, circuit breaker, and degraded mode?
12. Does doctor detect the problem before outage?
13. Is there a simulation test for this failure?
```

If a subsystem cannot answer these questions, the invariant is not yet proven.

---

# Recommended Documentation Placement

Suggested file path:

```text
docs/architecture/failure-patterns-and-invariants.md
```

Suggested cross-links:

```text
docs/architecture/state-criticality-model.md
docs/architecture/day0-day1-day2.md
docs/operations/scylla-rf-policy.md
docs/operations/ingress-vip-recovery.md
docs/operations/doctor-invariants.md
```

---

# Summary

The last failure should not be documented as a single incident where one node went down.

It should be documented as a class of systemic risks:

```text
critical state under-replicated
missing truth treated as destructive command
critical config without last-known-good
reconcile starvation
bootstrap assumptions escaping into production
topology safety drift
```

The permanent fix is invariant-driven engineering.

For every critical subsystem, Globular must define:

```text
owner
replication policy
generation
checksum
last-known-good behavior
explicit delete/disable semantics
bounded reconcile
doctor invariant
failure simulation
```

That is how Globular moves from reactive hardening to operational awareness.
