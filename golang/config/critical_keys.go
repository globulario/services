package config

import (
	"fmt"
	"strings"
)

// CriticalEtcdKeys lists the etcd keys that must be present for a healthy
// cluster. These are checked by the cluster doctor on every collection cycle.
// Add entries here when introducing new authoritative cluster state keys.
//
// Invariant (Case 05: CRITICAL_STATE_REGISTRY_AND_OWNERSHIP): every critical
// key must have exactly one authoritative owner and be continuously checked
// for presence by the doctor collector.
var CriticalEtcdKeys = []string{
	"/globular/system/config",
	"/globular/ingress/v1/spec",
	"/globular/ingress/v1/spec_backup",
	"/globular/pki/ca",
	"/globular/objectstore/config",
}

// CriticalEtcdPrefixes lists key prefixes where at least one key must exist.
// The doctor flags a violation if no key with the given prefix exists.
var CriticalEtcdPrefixes = []string{
	"/globular/resources/",
	"/globular/nodes/",
	"/globular/scylla/schema_guard/",
}

// CriticalKeyPolicy carries the ownership metadata for a single critical key.
// This is the subset of the controller's CriticalKeyRecord that is accessible
// to the config package and the cluster doctor without importing the
// controller's main package.
//
// Every entry in CriticalEtcdKeys and CriticalEtcdPrefixes must have a
// corresponding CriticalKeyPolicy in CriticalKeyPolicies.
//
// Invariant: critical_state.registry_ownership_required
type CriticalKeyPolicy struct {
	// Key is the full etcd key path, or a prefix ending in "/" if IsPrefix=true.
	Key      string
	IsPrefix bool
	// Owner is the component that is the sole authoritative writer for this key.
	Owner string
	// SchemaVersion identifies the JSON schema version for the key's value.
	SchemaVersion string
	// DeletePolicyName is a human-readable delete governance name.
	// One of: "never_automatic", "allowed_on_node_remove", "allowed_by_operator".
	DeletePolicyName string
	// DoctorInvariant is the doctor finding ID emitted when the key is absent.
	DoctorInvariant string
}

// CriticalKeyPolicies is the shared ownership table for all critical etcd keys
// and prefixes. Every entry in CriticalEtcdKeys and CriticalEtcdPrefixes must
// have a corresponding entry here. The cluster doctor uses this table to verify
// ownership governance is complete on every collection cycle.
//
// When adding a new key to CriticalEtcdKeys or CriticalEtcdPrefixes, add a
// matching CriticalKeyPolicy entry here. TestRegistryKeyHasCompletePolicy will
// fail the CI build if any key lacks a policy.
//
// Invariant: critical_state.registry_ownership_required
var CriticalKeyPolicies = []CriticalKeyPolicy{
	{
		Key:              "/globular/system/config",
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "never_automatic",
		DoctorInvariant:  "system.config_missing",
	},
	{
		Key:              "/globular/ingress/v1/spec",
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "never_automatic",
		DoctorInvariant:  "ingress.spec_missing",
	},
	{
		Key:              "/globular/ingress/v1/spec_backup",
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "never_automatic",
		DoctorInvariant:  "ingress.spec_backup_missing",
	},
	{
		Key:              "/globular/pki/ca",
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "never_automatic",
		DoctorInvariant:  "pki.ca_not_published",
	},
	{
		Key:              "/globular/objectstore/config",
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "never_automatic",
		DoctorInvariant:  "objectstore.no_desired_state",
	},
	{
		Key:              "/globular/resources/",
		IsPrefix:         true,
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "allowed_by_operator",
		DoctorInvariant:  "desired_state.key_missing",
	},
	{
		Key:              "/globular/nodes/",
		IsPrefix:         true,
		Owner:            "node-agent",
		SchemaVersion:    "v1",
		DeletePolicyName: "allowed_on_node_remove",
		DoctorInvariant:  "node.heartbeat_missing",
	},
	{
		Key:              "/globular/scylla/schema_guard/",
		IsPrefix:         true,
		Owner:            "cluster-controller",
		SchemaVersion:    "v1",
		DeletePolicyName: "allowed_on_node_remove",
		DoctorInvariant:  "scylla.keyspace.rf_policy_violation",
	},
}

// OwnerForKey returns the authoritative owner of the given etcd key by
// matching against CriticalKeyPolicies (exact key or prefix). Returns an
// error if the key has no registered owner in the policy table.
func OwnerForKey(key string) (string, error) {
	for _, p := range CriticalKeyPolicies {
		if p.IsPrefix {
			if strings.HasPrefix(key, p.Key) {
				return p.Owner, nil
			}
		} else if key == p.Key {
			return p.Owner, nil
		}
	}
	return "", fmt.Errorf("no registered owner for critical key %q", key)
}

// ValidateCriticalKeyOwner returns an error if writerID is not the registered
// owner of key. Unknown keys (not in CriticalKeyPolicies) pass through without
// error — governance only applies to declared critical keys.
func ValidateCriticalKeyOwner(key, writerID string) error {
	for _, p := range CriticalKeyPolicies {
		matched := false
		if p.IsPrefix {
			matched = strings.HasPrefix(key, p.Key)
		} else {
			matched = key == p.Key
		}
		if !matched {
			continue
		}
		if p.Owner != writerID {
			return fmt.Errorf("critical key %q owned by %q: writer %q is not authorized", key, p.Owner, writerID)
		}
		return nil
	}
	return nil // not a registered critical key — no restriction
}

// PolicyGapsForKeys returns the list of keys (from the keys and prefixes
// slices) that have no entry in CriticalKeyPolicies. An empty result means
// all keys are fully governed. Used by the cluster doctor collector to detect
// missing policy entries without requiring an etcd query.
func PolicyGapsForKeys(keys, prefixes []string) []string {
	var gaps []string
	for _, key := range keys {
		if _, err := OwnerForKey(key); err != nil {
			gaps = append(gaps, key)
		}
	}
	for _, prefix := range prefixes {
		found := false
		for _, p := range CriticalKeyPolicies {
			if p.IsPrefix && p.Key == prefix {
				found = true
				break
			}
		}
		if !found {
			gaps = append(gaps, prefix)
		}
	}
	return gaps
}
