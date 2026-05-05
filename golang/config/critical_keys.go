package config

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
