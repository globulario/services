# Globular Cluster Warning Analysis

## Context

The cluster produced several warnings related to:

- service desired/applied drift,
- missing PKI CA metadata,
- installed-state/runtime mismatches,
- missing objectstore desired state.

These warnings are not isolated. They point to a common architectural theme:

> Globular has desired state, installed state, applied state, and runtime state, but some guardians/publishers are missing or not strict enough.

The cluster is effectively saying:

```text
I know what should exist.
I know what was installed.
But I cannot prove what is actually running.
And some critical desired-state keys are missing.
```

---

# 1. `cluster.services.drift`

Example:

```text
WARN cluster.services.drift
services state hash mismatch (desired ≠ applied)
applied_hash=services:0990...
desired_hash=services:b95...
can_apply_privileged=true
```

## Meaning

The controller has a desired services state, but the node has not applied that exact state.

So the state model says:

```text
Desired services hash != Applied services hash
```

This means one of the following is likely true:

```text
1. node-agent did not apply the new service spec
2. node-agent applied it but did not report the new hash
3. controller desired-state changed but dispatch did not happen
4. privileged apply is available, but not being triggered correctly
5. apply failed and the failure was not promoted to a stronger doctor finding
```

## What should be fixed

### Fix A: Drift must trigger an apply workflow automatically

If:

```text
desired_hash != applied_hash
AND can_apply_privileged == true
```

then the controller should not only warn. It should dispatch something like:

```text
node.apply_services_desired_state
```

or equivalent.

## Required invariant

```text
Invariant: services desired/applied drift must either converge or become a blocking finding with a reason.
```

## Required state fields

For each node, record:

```json
{
  "desired_hash": "...",
  "applied_hash": "...",
  "last_apply_attempt": "...",
  "last_apply_result": "OK | FAILED | SKIPPED | BLOCKED",
  "last_apply_error": "...",
  "apply_generation": 123,
  "node_ack_generation": 122
}
```

## Better doctor behavior

The current warning should age into stronger severity.

Suggested behavior:

```text
WARN     if drift age < 2 minutes
ERROR    if drift age > 5 minutes
CRITICAL if drift affects critical services or ingress/objectstore/dns
```

## Test to add

```text
change desired service state
node applied_hash remains old
controller dispatches apply
node applies
applied_hash becomes desired_hash
warning clears
```

---

# 2. `pki.ca_not_published`

Example:

```text
WARN pki.ca_not_published
No CA metadata published at /globular/pki/ca
controller has not advertised the cluster CA fingerprint.
Node agents cannot detect CA rotation without this key.
Restart the cluster controller to publish.
```

## Meaning

This is a critical authoritative-state publisher missing.

The CA may exist locally, but the controller has not published the cluster CA metadata into etcd:

```text
/globular/pki/ca
```

That means node-agents cannot know:

```text
current CA fingerprint
CA generation
CA rotation status
valid issuer
trust bundle version
```

This is dangerous because CA/PKI is Class A critical state.

## What should be fixed

### Fix A: Controller must publish CA metadata on every leader cycle

Do not rely on “restart the controller.”

The controller should have a function like:

```text
ensurePKICAMetadataPublished()
```

called on:

```text
controller start
leader election
periodic critical-state publisher loop
CA rotation
doctor check
```

## Required etcd key

```text
/globular/pki/ca
```

Suggested value:

```json
{
  "cluster_id": "...",
  "ca_fingerprint_sha256": "...",
  "ca_subject": "...",
  "not_before": "...",
  "not_after": "...",
  "generation": 1,
  "published_at": "...",
  "writer": "cluster-controller",
  "writer_node_id": "...",
  "schema_version": "v1"
}
```

## Required invariant

```text
Invariant: CA metadata must always be published by the controller while the cluster has a valid CA.
```

## Required consumer behavior

Node-agent should not rotate or reject certs blindly if this key is missing.

It should report:

```text
PKI_METADATA_MISSING
```

and continue using last-known-good trust metadata if available.

## Test to add

```text
delete /globular/pki/ca
controller republishes it
node-agent detects current CA generation
doctor warning clears
```

This warning belongs to the same family as:

```text
ORPHANED_CRITICAL_STATE
NO_LAST_KNOWN_GOOD_FOR_CRITICAL_RUNTIME
MISSING_STATE_WITHOUT_INTENT_MARKER
```

---

# 3. `installed_state_runtime_mismatch`

Examples:

```text
Package yt-dlp has installed_state=2026.2.21
but runtime not converged: runtime unit missing (globular-yt-dlp.service)

Package claude has installed_state=2.1.80
but runtime unit missing (globular-claude.service)

Package cli has installed_state=1.2.10
but runtime unit missing (globular-cli.service)

Package keepalived has installed_state=0.0.1
but runtime unit missing (globular-keepalived.service)

Package sha256sum has installed_state=9.4.0
but runtime unit missing (globular-sha256sum.service)
```

## Meaning

Globular says:

```text
Installed state says the package exists.
Runtime says the systemd unit does not exist.
```

So the installed-state table is not enough. The runtime proof failed.

This can happen if:

```text
1. package installed but unit was never rendered
2. unit was rendered but deleted
3. package is not supposed to be a daemon but doctor expects a service
4. package metadata incorrectly marks command/tool packages as services
5. node-agent install pipeline wrote installed_state too early
6. upgrade/install failed after installed_state was persisted
```

The suspicious packages here are:

```text
yt-dlp
claude
cli
sha256sum
```

These may be command/tool packages, not long-running services.

But `keepalived` should likely have a systemd unit if it is used for VIP.

So this warning may mix two different issues:

```text
A. real runtime missing unit
B. wrong package kind / wrong runtime expectation
```

## What should be fixed

### Fix A: Package kind must define runtime expectation

Every package must declare one of:

```text
SERVICE
INFRASTRUCTURE
COMMAND
LIBRARY
ASSET
PLUGIN
```

Then doctor should only expect a systemd unit for packages that are supposed to run.

Example for a command package:

```json
{
  "package": "yt-dlp",
  "kind": "COMMAND",
  "runtime": {
    "type": "none"
  }
}
```

Example for an infrastructure package:

```json
{
  "package": "keepalived",
  "kind": "INFRASTRUCTURE",
  "runtime": {
    "type": "systemd",
    "unit": "globular-keepalived.service"
  }
}
```

## Required invariant

```text
Invariant: installed_state must not imply systemd runtime unless package metadata declares a systemd runtime.
```

### Fix B: Installed state must be written after runtime proof, not before

For service/infra packages, the install pipeline should not mark package fully installed until it proves:

```text
binary exists
config rendered
systemd unit exists
systemd daemon-reload completed
unit enabled if required
unit active or intentionally held
runtime hash recorded
```

Better installed state:

```json
{
  "package": "keepalived",
  "version": "0.0.1",
  "installed": true,
  "runtime_expected": true,
  "runtime_converged": false,
  "runtime_reason": "unit_missing",
  "installed_phase": "BINARY_INSTALLED",
  "next_action": "RENDER_SYSTEMD_UNIT"
}
```

### Fix C: Runtime mismatch must trigger repair

For packages where runtime is expected:

```text
installed_state exists
runtime unit missing
```

should dispatch:

```text
node.repair_runtime_unit
```

or:

```text
node.reapply_package_runtime
```

## Required statuses

```text
INSTALLED_RUNTIME_CONVERGED
INSTALLED_RUNTIME_MISSING
INSTALLED_RUNTIME_HELD
INSTALLED_RUNTIME_NOT_EXPECTED
INSTALLED_RUNTIME_REPAIRING
```

## Tests to add

For service/infra packages:

```text
install keepalived
delete globular-keepalived.service
doctor detects runtime missing
node-agent re-renders unit
systemd daemon-reload
unit restored
warning clears
```

For command packages:

```text
install yt-dlp as COMMAND
no systemd unit exists
doctor reports OK because runtime_expected=false
```

This warning maps to existing failure-pattern families:

```text
DERIVED_STATE_BLOCKS_AUTHORITY
UNGUARDED_RUNTIME_DESTRUCTIVE_ACTION
TOPOLOGY_SAFETY_DRIFT
```

But it also reveals a more specific new pattern:

```text
INSTALLED_STATE_WITHOUT_RUNTIME_PROOF
```

---

# 4. `objectstore.no_desired_state`

Example:

```text
WARN objectstore.no_desired_state
No objectstore desired state found at /globular/objectstore/config
controller has not published the authoritative MinIO topology.
Node agents may be using stale local configs.
```

## Meaning

This is serious.

It means the authoritative MinIO topology key is missing:

```text
/globular/objectstore/config
```

That is exactly the same class of problem as the ingress/VIP spec disappearance.

If the objectstore desired state is missing, node-agents may fall back to stale local configs, or worse, infer topology locally.

For MinIO, local inference is dangerous.

## What should be fixed

### Fix A: Controller must publish objectstore desired state continuously

The controller needs a critical-state publisher:

```text
ensureObjectStoreDesiredStatePublished()
```

It should run on:

```text
controller start
leader election
objectstore topology change
node join/leave
periodic critical-state publisher loop
doctor repair
```

## Required etcd key

```text
/globular/objectstore/config
```

Suggested value:

```json
{
  "generation": 12,
  "cluster_id": "...",
  "mode": "distributed",
  "pool": [
    {
      "node_id": "...",
      "address": "10.0.0.x",
      "drives": ["/var/lib/globular/minio"]
    }
  ],
  "write_policy": "no_round_robin_for_writes",
  "rendered_hash": "...",
  "published_at": "...",
  "writer": "cluster-controller",
  "writer_node_id": "...",
  "schema_version": "v1"
}
```

## Required invariant

```text
Invariant: Objectstore topology must have controller-owned desired state before any node starts or reconfigures MinIO.
```

### Fix B: Node-agent must hold last-known-good

If `/globular/objectstore/config` is missing:

```text
do not infer topology
do not wipe data
do not start stray local MinIO
do not rewrite minio.env
```

Instead:

```text
hold last-known-good objectstore config
mark OBJECTSTORE_DESIRED_STATE_MISSING
request controller republish
```

### Fix C: Destructive transitions require approval

Any MinIO action like:

```text
wipe .minio.sys
change pool
remove node from pool
change distributed endpoints
```

must require:

```text
TopologyTransition approval
generation match
node/path match
audit marker
```

## Test to add

```text
delete /globular/objectstore/config
controller republishes it
node-agent holds LKG until republished
MinIO does not restart with inferred topology
no data wipe occurs
```

This maps directly to:

```text
ORPHANED_CRITICAL_STATE
NO_LAST_KNOWN_GOOD_FOR_CRITICAL_RUNTIME
ABSENCE_AS_DESTRUCTIVE_INTENT
```

---

# Bigger Pattern From All Warnings

These warnings reveal a missing architecture layer:

```text
Critical State Publisher / Guardian Layer
```

The controller should not only reconcile packages. It must continuously guarantee the presence of critical state keys:

```text
/globular/pki/ca
/globular/objectstore/config
/globular/ingress/v1/spec
/globular/scylla/schema_guard/...
/globular/release/active
/globular/cluster/membership
```

---

# Proposed New Subsystem

Add:

```text
critical_state_guard
```

or split by domain:

```text
pki_guard
objectstore_guard
ingress_guard
services_state_guard
runtime_state_guard
```

But they should share one model.

Suggested interface:

```go
type CriticalStateGuardian interface {
    Key() string
    Owner() string
    Class() StateClass
    Check(ctx context.Context) (*CriticalStateStatus, error)
    Publish(ctx context.Context) error
    Repair(ctx context.Context) error
    ConsumerFallback() string
}
```

State classes:

```text
CLASS_A_AUTHORITATIVE
CLASS_B_REPLICATED
CLASS_C_DERIVED
CLASS_D_RUNTIME_RENDERED
```

---

# What To Modify First

## P0. Publish missing critical keys

Implement controller publishers for:

```text
/globular/pki/ca
/globular/objectstore/config
/globular/ingress/v1/spec
```

Because missing keys caused or could cause destructive behavior.

## P0. Change consumers from destructive fallback to last-known-good

For node-agent:

```text
missing objectstore config -> hold LKG
missing ingress spec -> hold LKG
missing PKI CA metadata -> hold LKG / degraded
```

No missing critical key should become:

```text
disabled
empty config
local inference
wipe
stop
```

## P0. Fix installed-state/runtime proof

For package runtime doctor:

```text
COMMAND packages should not require systemd units
SERVICE/INFRA packages must repair missing units
installed_state must include runtime expectation
```

## P0. Make drift actionable

For:

```text
desired_hash != applied_hash
can_apply_privileged=true
```

the controller should dispatch apply/reapply, not only warn.

---

# Suggested New Invariants From These Warnings

```text
Invariant 1:
Critical controller-owned etcd keys must be periodically republished.

Invariant 2:
Missing critical desired state must never trigger destructive runtime changes.

Invariant 3:
Every critical runtime config must have last-known-good behavior.

Invariant 4:
Installed state is not converged unless runtime proof matches package metadata.

Invariant 5:
Desired/applied hash drift must either converge automatically or escalate with a reason.

Invariant 6:
Package kind determines runtime expectation. COMMAND packages must not be treated as daemons.

Invariant 7:
Objectstore topology must never be inferred locally when authoritative desired state is missing.

Invariant 8:
PKI CA metadata must be published and versioned so node-agents can detect CA rotation.
```

---

# Simple Summary

The cluster is warning about this:

```text
controller did not publish some critical truth
node-agent has applied state different from desired state
installed_state says packages exist but runtime proof says some units are missing
objectstore and PKI authoritative keys are missing
```

So the fix is not only “restart controller.”

The real fix is:

```text
controller must become guardian of critical state
node-agent must hold last-known-good instead of guessing
doctor must distinguish command packages from daemon packages
runtime repair must be automatically dispatched when installed_state lies
```

This is exactly Globular’s state model becoming sharper:

```text
Artifact
  ↓
Desired
  ↓
Installed
  ↓
Applied
  ↓
Runtime
```

Right now, the warnings show mismatches between those layers. Good news: the doctor is seeing the ghosts. Now the controller needs to become the ghostbuster.
