// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=day0_bootstrap_completion_rule
// @awareness implements=globular.platform:intent.day0_day1_are_repeatable_ceremonies
// @awareness risk=high
package rules

import "github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"

const (
	systemConfigKey    = "/globular/system/config"
	nodesPrefixKey     = "/globular/nodes/"
	resourcesPrefixKey = "/globular/resources/"
)

// isLikelyDay0Bootstrap returns true when snapshot evidence strongly suggests
// initial Day-0 bootstrap seeding has not completed yet.
//
// Heuristic (conservative):
//   - cluster has at most one known node; and
//   - foundational registry entries are all absent:
//     /globular/system/config, /globular/nodes/, /globular/resources/
//
// This avoids paging hard-fail invariants while the authoritative controller
// has not yet published baseline cluster state.
func isLikelyDay0Bootstrap(snap *collector.Snapshot) bool {
	if snap == nil {
		return false
	}
	// Require explicit critical-key checks from collector. A nil/empty map in
	// unit tests or partial snapshots is not enough evidence for Day-0.
	if len(snap.CriticalKeyPresent) == 0 {
		return false
	}
	if len(snap.Nodes) > 1 {
		return false
	}
	return !snap.CriticalKeyPresent[systemConfigKey] &&
		!snap.CriticalKeyPresent[nodesPrefixKey] &&
		!snap.CriticalKeyPresent[resourcesPrefixKey]
}
