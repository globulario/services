// @awareness namespace=globular.platform
// @awareness component=platform_controller.reconciler
// @awareness file_role=critical_state_registry_for_convergence_gate
// @awareness enforces=globular.platform:invariant.state.unknown_must_not_default_to_healthy
// @awareness risk=high
package main

import "fmt"

// critical_state_registry.go — Case 05: CRITICAL_STATE_REGISTRY_AND_OWNERSHIP
//
// Every critical etcd key must have exactly one authoritative owner, a known
// schema version, a restore strategy, LKG consumer behavior, and a delete
// approval policy. This registry is the single source of truth for key
// governance. The cluster doctor uses it for key-missing checks.
//
// Invariant: every critical key must have exactly one authoritative writer
// and one guardian loop. If a key is missing without a delete-approval
// tombstone, the owning guardian must restore it.

// DeletePolicy governs when a critical key may be permanently removed.
type DeletePolicy int

const (
	// DeleteNeverAutomatic: key must only be deleted via explicit audited tombstone.
	DeleteNeverAutomatic DeletePolicy = iota
	// DeleteAllowedOnNodeRemove: key may be deleted when the associated node is removed.
	DeleteAllowedOnNodeRemove
	// DeleteAllowedByOperator: key may be deleted via explicit operator command with audit record.
	DeleteAllowedByOperator
)

// RestoreStrategy describes how the owner guardian restores a missing key.
type RestoreStrategy int

const (
	// RestoreFromBackup: restore from a backup key written by the same guardian.
	RestoreFromBackup RestoreStrategy = iota
	// RestoreFromState: recompute from controller in-memory state.
	RestoreFromState
	// RestoreWaitOperator: cannot auto-restore, wait for operator to set.
	RestoreWaitOperator
)

// CriticalKeyRecord describes a single critical etcd key.
type CriticalKeyRecord struct {
	// Key is the full etcd key path (or prefix if IsPrefix=true).
	Key string
	// IsPrefix indicates Key is a prefix and multiple keys may match.
	IsPrefix bool
	// Owner is the component that must maintain this key.
	Owner string
	// SchemaVersion identifies the JSON schema for the key's value.
	SchemaVersion string
	// RestoreStrategy describes how the owner restores a missing key.
	Restore RestoreStrategy
	// LKGConsumerBehavior describes what consumers do when the key is missing.
	LKGConsumerBehavior string
	// DeletePolicy governs when this key may be permanently removed.
	Delete DeletePolicy
	// DoctorInvariant is the doctor finding code emitted when the key is missing.
	DoctorInvariant string
	// GuardedBy names the controller goroutine or subsystem that maintains this key.
	GuardedBy string
}

// criticalStateRegistry is the cluster-wide registry of owned critical keys.
// Add new entries here when introducing new critical etcd keys.
var criticalStateRegistry = []CriticalKeyRecord{
	{
		Key:                 "/globular/ingress/v1/spec",
		Owner:               "cluster-controller",
		SchemaVersion:       "v1",
		Restore:             RestoreFromBackup,
		LKGConsumerBehavior: "hold last-known-good keepalived config",
		Delete:              DeleteNeverAutomatic,
		DoctorInvariant:     "ingress.spec_missing",
		GuardedBy:           "ingress-spec-guard",
	},
	{
		Key:                 "/globular/ingress/v1/spec_backup",
		Owner:               "cluster-controller",
		SchemaVersion:       "v1",
		Restore:             RestoreFromState,
		LKGConsumerBehavior: "not a consumer-facing key",
		Delete:              DeleteNeverAutomatic,
		DoctorInvariant:     "ingress.spec_backup_missing",
		GuardedBy:           "ingress-spec-guard",
	},
	{
		Key:           "/globular/scylla/schema_guard/",
		IsPrefix:      true,
		Owner:         "cluster-controller",
		SchemaVersion: "v1",
		Restore:       RestoreFromState,
		LKGConsumerBehavior: "re-run schema guard on next tick",
		Delete:              DeleteAllowedOnNodeRemove,
		DoctorInvariant:     "scylla.keyspace.rf_policy_violation",
		GuardedBy:           "scylla-schema-guard",
	},
	{
		Key:                 "/globular/system/config",
		Owner:               "cluster-controller",
		SchemaVersion:       "v1",
		Restore:             RestoreFromState,
		LKGConsumerBehavior: "use built-in defaults",
		Delete:              DeleteNeverAutomatic,
		DoctorInvariant:     "system.config_missing",
		GuardedBy:           "controller-reconcile",
	},
	{
		Key:           "/globular/nodes/",
		IsPrefix:      true,
		Owner:         "node-agent",
		SchemaVersion: "v1",
		Restore:       RestoreFromState,
		LKGConsumerBehavior: "mark node stale/unreachable if heartbeat absent >5min",
		Delete:              DeleteAllowedOnNodeRemove,
		DoctorInvariant:     "node.heartbeat_missing",
		GuardedBy:           "controller-health-monitor",
	},
	{
		Key:           "/globular/resources/",
		IsPrefix:      true,
		Owner:         "cluster-controller",
		SchemaVersion: "v1",
		Restore:       RestoreFromState,
		LKGConsumerBehavior: "node-agent waits for desired state",
		Delete:              DeleteAllowedByOperator,
		DoctorInvariant:     "desired_state.key_missing",
		GuardedBy:           "reconcile-nodes",
	},
	{
		Key:                 "/globular/pki/ca",
		Owner:               "cluster-controller",
		SchemaVersion:       "v1",
		Restore:             RestoreFromState,
		LKGConsumerBehavior: "node-agent holds stale CA metadata; cannot detect rotation",
		Delete:              DeleteNeverAutomatic,
		DoctorInvariant:     "pki.ca_not_published",
		GuardedBy:           "persist-state-locked",
	},
	{
		Key:                 "/globular/objectstore/config",
		Owner:               "cluster-controller",
		SchemaVersion:       "v1",
		Restore:             RestoreFromState,
		LKGConsumerBehavior: "node-agent holds last-known-good MinIO topology; does not infer",
		Delete:              DeleteNeverAutomatic,
		DoctorInvariant:     "objectstore.no_desired_state",
		GuardedBy:           "persist-state-locked",
	},
}

// LookupCriticalKey returns the registry entry for the given etcd key, or nil
// if the key is not in the registry.
func LookupCriticalKey(key string) *CriticalKeyRecord {
	for i := range criticalStateRegistry {
		r := &criticalStateRegistry[i]
		if r.IsPrefix {
			if len(key) >= len(r.Key) && key[:len(r.Key)] == r.Key {
				return r
			}
		} else if key == r.Key {
			return r
		}
	}
	return nil
}

// ValidateCriticalKeyWrite checks that writerID matches the registered owner for
// the given etcd key. Returns an error if the key is registered and the writer
// is not the authoritative owner. Unknown keys (not in the registry) are allowed
// through without error — the registry only governs known critical keys.
func ValidateCriticalKeyWrite(key, writerID string) error {
	rec := LookupCriticalKey(key)
	if rec == nil {
		return nil
	}
	if rec.Owner != writerID {
		return fmt.Errorf("critical key %q owned by %q, write rejected from %q", key, rec.Owner, writerID)
	}
	return nil
}
