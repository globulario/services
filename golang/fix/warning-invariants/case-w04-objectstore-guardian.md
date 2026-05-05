# Case W04: Objectstore Desired State Critical State Registry Gap

## Pattern
`objectstore.no_desired_state` fires but `/globular/objectstore/config` is not in the critical
state registry. Same governance gap as Case W02 for PKI CA.

## Root Cause
`/globular/objectstore/config` is absent from:
- `config.CriticalEtcdKeys`
- `critical_state_registry.go`

The controller publishes it via `publishObjectStoreDesiredStateLocked()` and the doctor checks for it,
but the key has no official ownership, restore strategy, or delete policy — violating Case 05.

## Required Invariant
Objectstore topology must have controller-owned desired state before any node starts or reconfigures MinIO.
The critical state registry must be the single source of truth for this governance.

## Implementation

### W04-A: Add to config.CriticalEtcdKeys
Add `/globular/objectstore/config` to `config.CriticalEtcdKeys` in `config/critical_keys.go`.

### W04-B: Add to critical_state_registry
Add entry with:
- Owner: `cluster-controller`
- SchemaVersion: `v1`
- Restore: `RestoreFromState` (controller recomputes from objectstore topology)
- Delete: `DeleteNeverAutomatic`
- DoctorInvariant: `objectstore.no_desired_state`
- GuardedBy: `persist-state-locked`

## Files / Components
- `config/critical_keys.go`: add `/globular/objectstore/config`
- `cluster_controller/cluster_controller_server/critical_state_registry.go`: add entry
- `cluster_doctor/cluster_doctor_server/rules/objectstore_health.go`: already implemented
- Tests already exist in `objectstore_health_test.go`

## Tests
- Unit: `objectstore.no_desired_state` fires when ObjectStoreDesired is nil and storage nodes exist
- Unit: `objectstore.no_desired_state` does not fire with no storage nodes
- Unit: registry entry for `/globular/objectstore/config` is complete and owner-validated
- (Existing tests in objectstore_health_test.go already cover some of this)

## Remaining To Reach DoD
- Integration: delete `/globular/objectstore/config` → node-agents hold LKG, controller republishes
- Verify node-agent does not infer topology or restart MinIO when key is absent

## DoD
Objectstore topology is governed by the same critical-state registry as all other Class A keys.
Node-agent holds last-known-good config when the key is absent.
