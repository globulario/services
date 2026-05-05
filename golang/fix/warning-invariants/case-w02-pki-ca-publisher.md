# Case W02: PKI CA Metadata Critical State Registry Gap

## Pattern
`pki.ca_not_published` warning fires but `/globular/pki/ca` is not in the critical state registry.
The doctor sees it as critical, the controller publishes it, but governance is missing.

## Root Cause
`/globular/pki/ca` is absent from:
- `config.CriticalEtcdKeys`
- `critical_state_registry.go`

It therefore has no official owner record, restore strategy, delete policy, or doctor invariant ID
binding — violating Case 05 (CRITICAL_STATE_REGISTRY_AND_OWNERSHIP).

## Required Invariant
CA metadata must always be published by the controller while the cluster has a valid CA.
The critical state registry must be the single source of truth for this governance.

## Implementation

### W02-A: Add to config.CriticalEtcdKeys
Add `/globular/pki/ca` to `config.CriticalEtcdKeys` in `config/critical_keys.go`.

### W02-B: Add to critical_state_registry
Add entry with:
- Owner: `cluster-controller`
- SchemaVersion: `v1`
- Restore: `RestoreFromState` (controller recomputes from CA cert)
- Delete: `DeleteNeverAutomatic`
- DoctorInvariant: `pki.ca_not_published`
- GuardedBy: `persist-state-locked`

## Files / Components
- `config/critical_keys.go`: add `/globular/pki/ca`
- `cluster_controller/cluster_controller_server/critical_state_registry.go`: add entry
- `cluster_doctor/cluster_doctor_server/rules/pki_health.go`: already implemented
- `cluster_doctor/cluster_doctor_server/collector/collector.go`: already collects CAMetadata

## Tests
- Unit: `pki.ca_not_published` fires when CAMetadata is nil and nodes exist
- Unit: `pki.ca_not_published` does not fire when CAMetadata is present
- Unit: `pki.ca_not_published` does not fire on empty cluster (no joined nodes)
- Unit: `pki.ca_expiry_warning` fires at WARN for 30-day expiry
- Unit: `pki.ca_expiry_warning` fires at ERROR for 7-day expiry
- Unit: `pki.ca_expiry_warning` fires at CRITICAL for expired CA
- Unit: registry entry for `/globular/pki/ca` is complete and owner-validated

## Remaining To Reach DoD
- Integration: delete `/globular/pki/ca` → controller republishes on next persist cycle

## DoD
PKI CA metadata is governed by the same critical-state registry as all other Class A keys.
