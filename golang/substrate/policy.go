// Package substrate implements the coordination-store survival contracts:
//
//	The coordination store (etcd) must be recreatable from durable authority,
//	bounded desired-state backup, and live observation. Loss of its quorum
//	may suspend convergence, but must not destroy the information required
//	to reconstruct convergence.
//
//	Restored desired state is evidence, not immediate authority. It must be
//	reconciled against newer durable and observed truth before destructive
//	convergence is permitted.
//
// The package provides the recovery ladder behind `globular substrate ...`:
//
//	rung 1 — restart a stopped existing member (no data touched)
//	rung 2 — rebuild from a surviving member (force-new-cluster, data intact)
//	rung 3 — recreate from a classified logical dump of /globular
//
// A dump captures the FULL keyspace; classification is applied at RESTORE
// time (and recorded in the manifest), so a mis-classified prefix is fixable
// in code without having lost data.
package substrate

import "strings"

// RestorePolicy says what a restore does with a key, per the classification
// contract: etcd may coordinate the system, but every value must be either
// reproducible from repository/release authority, reproducible from
// installed/runtime observation, recoverable from this bounded dump, or
// disposable ephemeral coordination state. Nothing else is allowed to exist.
type RestorePolicy string

const (
	// RestoreAuthoritative — cluster identity, trust anchors, operator-provided
	// secrets/policies, monotonic counters, and append-only audit history:
	// facts nothing in the cluster can recompute and whose restoration cannot
	// drive destructive convergence by itself.
	RestoreAuthoritative RestorePolicy = "RESTORE_AUTHORITATIVE"

	// RestoreAsUnverified — desired state (Layer 2) and durable intent. It is
	// restored, but the restore marker stays RESTORED_UNVERIFIED until
	// convergence has re-observed reality; controllers must not take
	// destructive actions from restored-only evidence.
	RestoreAsUnverified RestorePolicy = "RESTORE_AS_UNVERIFIED"

	// RebuildFromObservation — observed state whose owner re-publishes it
	// (installed packages, discovery endpoints, host lists). Never restored:
	// resurrecting stale observations poisons the 4-layer model.
	RebuildFromObservation RestorePolicy = "REBUILD_FROM_OBSERVATION"

	// Discard — leases, locks, leader keys, heartbeats, queues, transient run
	// state, findings, and stale approvals. Restoring any of these recreates
	// yesterday's weather; stale destructive approvals are actively dangerous.
	Discard RestorePolicy = "DISCARD"
)

// RestoreMarkerKey is written by every restore/recovery operation. Its status
// is RESTORED_UNVERIFIED until an operator (or a future controller
// verification pass) flips it to RESTORED_VERIFIED; the controller-side gate
// that suspends destructive convergence while unverified is tracked as P2.
const RestoreMarkerKey = "/globular/recovery/v1/restore"

// PrefixPolicy binds an etcd key prefix to its restore policy.
type PrefixPolicy struct {
	Prefix string
	Policy RestorePolicy
	Note   string
}

// PrefixPolicies is the classification table. Longest matching prefix wins,
// so a specific entry (e.g. a bootstrap marker) overrides its parent subtree.
// Unknown prefixes classify as RESTORE_AS_UNVERIFIED and are reported — a
// silent drop would violate the no-irreducible-truth audit, and a silent
// authoritative restore would violate the restore law.
var PrefixPolicies = []PrefixPolicy{
	// ── Identity, trust, secrets, counters, audit ────────────────────────────
	{"/globular/system/cluster/id", RestoreAuthoritative, "immutable cluster membership UUID"},
	{"/globular/system/config", RestoreAuthoritative, "cluster name/domain/version"},
	{"/globular/system/rbac/generation", RestoreAuthoritative, "monotonic generation — must not regress"},
	{"/globular/clustercontroller/epoch", RestoreAuthoritative, "controller fencing epoch — must not regress"},
	{"/globular/routing/refresh-generation", RestoreAuthoritative, "monotonic generation — must not regress"},
	{"/globular/pki/", RestoreAuthoritative, "cluster trust anchor material"},
	{"/globular/pki/locks/", Discard, "PKI operation locks are transient"},
	{"/globular/pki/ca_delete_approval/", Discard, "stale destructive approvals must not survive restore"},
	{"/globular/security/public_keys", RestoreAuthoritative, "cluster public key set"},
	{"/globular/auth/root", RestoreAuthoritative, "root auth record"},
	{"/globular/credentials/", RestoreAuthoritative, "operator-provided credentials"},
	{"/globular/secrets/", RestoreAuthoritative, "operator-provided secrets"},
	{"/globular/tokens/", Discard, "tokens are ephemeral by policy — regenerate"},
	{"/globular/acme/certs/", RestoreAuthoritative, "issued certs — re-issue is rate-limited upstream"},
	{"/globular/audit/", RestoreAuthoritative, "append-only history; preserved, never drives convergence"},
	{"/globular/ops/ledger/", RestoreAuthoritative, "append-only operations ledger"},
	{"/globular/repository/", RestoreAuthoritative, "repository authority and policy surface"},
	{"/globular/migrations/", RestoreAuthoritative, "which migrations ran — prevents double-apply"},
	{"/globular/resources/bootstrap_marker", RestoreAuthoritative, "prevents accidental re-Day-0"},
	{"/globular/nodes/bootstrap_marker", RestoreAuthoritative, "prevents accidental re-Day-0"},
	{"/globular/scylla/schema_guard/", RestoreAuthoritative, "schema-apply guard incl. bootstrap marker"},
	{"/globular/scylla/schema_guard/enforce_request", Discard, "transient trigger"},

	// ── Desired state / durable intent (Layer 2) ─────────────────────────────
	{"/globular/resources/", RestoreAsUnverified, "DesiredService / releases / install policies"},
	{"/globular/platform/", RestoreAsUnverified, "active/desired platform release anchors"},
	{"/globular/releases/local_overrides/", RestoreAsUnverified, "operator version overrides"},
	{"/globular/services/", RestoreAsUnverified, "per-service desired config (instances/runtime carved out below)"},
	{"/globular/workflows/", RestoreAsUnverified, "workflow definitions — durable intent"},
	{"/globular/compute/definitions/", RestoreAsUnverified, "compute job type definitions"},
	{"/globular/objectstore/", RestoreAsUnverified, "objectstore desired topology and admissions"},
	{"/globular/ingress/v1/spec", RestoreAsUnverified, "ingress desired spec"},
	{"/globular/ingress/v1/spec_backup", RestoreAsUnverified, "ingress spec backup"},
	{"/globular/domains/v1/", RestoreAsUnverified, "external domain specs"},
	{"/globular/providers/v1/", RestoreAsUnverified, "DNS/infra provider configs"},
	{"/globular/dns/v1/zones", RestoreAsUnverified, "configured DNS zones"},
	{"/globular/cluster/minio/config", RestoreAsUnverified, "MinIO config"},
	{"/globular/cluster/public-dirs/", RestoreAsUnverified, "public directory mappings"},
	{"/globular/applications/", RestoreAsUnverified, "application registrations"},
	{"/globular/backup/artifacts/", RestoreAsUnverified, "backup artifact metadata — points at real backups"},
	{"/globular/ai/claude-md", RestoreAsUnverified, "AI operating context"},
	{"/globular/clustercontroller/state", RestoreAsUnverified, "controller state incl. admitted node records"},
	{"/globular/system/controller-target-build", RestoreAsUnverified, "target controller build"},
	{"/globular/system/acc/config", RestoreAsUnverified, "acc service config"},
	{"/globular/bootstrap/", RestoreAsUnverified, "bootstrap parameters (gateway host)"},

	// ── Observations — owners re-publish these ───────────────────────────────
	{"/globular/nodes/", RebuildFromObservation, "installed packages (Layer 3) — node-agents re-sync"},
	{"/globular/cluster/dns/", RebuildFromObservation, "derived from live membership"},
	{"/globular/cluster/minio/hosts", RebuildFromObservation, "derived host list"},
	{"/globular/cluster/scylla/hosts", RebuildFromObservation, "derived host list"},
	{"/globular/system/etcd_endpoints", RebuildFromObservation, "reflects live etcd topology"},
	{"/globular/mcp/nodes/", RebuildFromObservation, "MCP node registrations"},

	// ── Ephemeral coordination state ─────────────────────────────────────────
	{"/globular/system/posture", Discard, "recomputed every posture tick"},
	{"/globular/system/etcd_endpoint_reconcile/", Discard, "reconcile timestamps"},
	{"/globular/clustercontroller/leader", Discard, "leader election — lease-bound"},
	{"/globular/controller/", Discard, "lanes, pending updates, removal requests, findings"},
	{"/globular/convergence/", Discard, "convergence action queue"},
	{"/globular/workflow/", Discard, "workflow run execution state — interrupted runs do not resume across a restore"},
	{"/globular/compute/jobs/", Discard, "transient job records"},
	{"/globular/compute/heartbeats/", Discard, "heartbeats"},
	{"/globular/compute/leases/", Discard, "leases"},
	{"/globular/platform_upgrade/", Discard, "upgrade run records"},
	{"/globular/locks/", Discard, "distributed locks"},
	{"/globular/sharedindex/", Discard, "writer locks"},
	{"/globular/verification/", Discard, "recomputed verification results"},
	{"/globular/approvals/", Discard, "stale destructive approvals must not survive restore"},
	{"/globular/ingress/v1/status/", Discard, "per-node runtime status"},
	{"/globular/ingress/v1/republish_request", Discard, "transient trigger"},
	{"/globular/ingress/v1/delete_approval/", Discard, "stale destructive approvals must not survive restore"},
	{"/globular/objectstore/delete_approval/", Discard, "stale destructive approvals must not survive restore"},
	{"/globular/objectstore/reconcile/", Discard, "reconcile timestamps"},
	{"/globular/objectstore/restart_in_progress", Discard, "restart lock flag"},
	{"/globular/objectstore/last_restart_result", Discard, "transient result"},
	{"/globular/cluster/alerts/", Discard, "recomputed alerts"},
	{"/globular/cluster_doctor/", Discard, "doctor recomputes findings and leadership"},
	{"/globular/dns/v1/status", Discard, "runtime status"},
	{"/globular/ai/jobs/", Discard, "AI job queue"},
	{"/globular/recovery/", Discard, "old recovery state incl. prior restore markers"},
	{"/globular/etcd_joins/", Discard, "transient join requests"},
	{"/globular/backup/jobs/", Discard, "transient job records"},
	{"/globular/backup/locks/", Discard, "locks"},
	{"/globular/runtime", Discard, "runtime info"},
}

// Classification is the result of classifying one key.
type Classification struct {
	Policy RestorePolicy
	Known  bool // false → no table entry matched; defaulted to RESTORE_AS_UNVERIFIED
}

// Classify returns the restore policy for an etcd key. Longest matching
// prefix in PrefixPolicies wins. Two structural overrides run first:
//
//   - lock-shaped keys (a "/locks/" segment or a trailing "/lock") are always
//     DISCARD — a restored lock is a deadlock nobody holds;
//   - per-service runtime subtrees ("/instances/" segment or trailing
//     "/runtime" under /globular/services/) are always DISCARD — they are
//     Layer-4 evidence republished by living processes.
//
// Unknown keys default to RESTORE_AS_UNVERIFIED with Known=false so callers
// can surface them: an unclassified prefix is a classification-table gap,
// not a reason to silently drop or silently trust data.
func Classify(key string) Classification {
	if strings.Contains(key, "/locks/") || strings.HasSuffix(key, "/lock") {
		return Classification{Policy: Discard, Known: true}
	}
	if strings.HasPrefix(key, "/globular/services/") &&
		(strings.Contains(key, "/instances/") || strings.HasSuffix(key, "/runtime")) {
		return Classification{Policy: Discard, Known: true}
	}

	best := -1
	for i := range PrefixPolicies {
		p := PrefixPolicies[i].Prefix
		if strings.HasPrefix(key, p) && (best == -1 || len(p) > len(PrefixPolicies[best].Prefix)) {
			best = i
		}
	}
	if best == -1 {
		return Classification{Policy: RestoreAsUnverified, Known: false}
	}
	return Classification{Policy: PrefixPolicies[best].Policy, Known: true}
}
