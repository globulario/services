// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.etcd_authority_pin
// @awareness file_role=architectural_pin_tests_for_cluster_doctor_observer_only_rule
// @awareness enforces=globular.platform:invariant.cluster_doctor.observer_only_never_writes_etcd
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=critical
package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Architectural pin tests for the four-layer authority invariants applied
// to cluster_doctor. The principle is anchored in
// invariant:cluster_doctor.observer_only_never_writes_etcd and
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. The
// running code must not violate them. These tests fail loudly if a future
// contributor reintroduces the violations the v1.2.166 commit removed.
//
// The tests walk the cluster_doctor source tree (excluding tests, excluding
// the explicit allowlist of grandfathered read sites whose proper fix is a
// new typed RPC on the cluster_controller) and assert no etcd .Put or .Get
// of /globular/* exists.

// TestClusterDoctor_NoNewEtcdDataWrites is the architectural pin for the
// write side, scoped to DATA writes against /globular/* paths.
//
// "Data write" means writing application state (audit records, gate state,
// findings, run state) to a /globular/* key. Coordination primitives that
// happen to use etcd (leader election via concurrency.NewElection, lease
// grants on the doctor's own leader lock, transient registration entries)
// are tracked separately by the "concurrency primitives" allowlist below —
// they are flagged so a future move to a non-etcd primitive (gossip,
// controller-side leader election) is a small change, not a hunt.
//
// As of v1.2.166 the two known data-write violations were converted to
// no-ops:
//   - executor.persistAuditToEtcd
//   - remediation_gate.{persist,load,delete}
//
// Additional data-write/read violations the audit + this pin caught but
// did NOT fix in this commit (tracked in the follow-up scope):
//   - approval_replay_etcd.go — replays approval tokens persisted in etcd
//   - remediation_history.go — history records in etcd
//   - rules/etcd_helpers.go — generic helpers for rule etcd access
//   - collector/sweep_requests.go — sweep request queue in etcd
//   - collector/verification.go — fetchDesiredServiceTargets direct etcd read
//
// Each of these needs its own typed-RPC migration. This test currently
// allow-lists them by file path so v1.2.166 lands clean; the allowlist
// must shrink over time.
func TestClusterDoctor_NoNewEtcdDataWrites(t *testing.T) {
	root := mustClusterDoctorRoot(t)

	// Matches .Put(ctx, "/globular/...", ...) — data writes to a
	// /globular path. Includes the wrapper helper remediationGateEtcdKey
	// so the wrapper's reintroduction would also fire.
	dataWriteRE := regexp.MustCompile(`\.Put\(\s*[^,)]+,\s*("/globular/|remediationGateEtcdKey)`)

	// File-level allowlist. Each entry is keyed by relative path under
	// cluster_doctor/ and carries the awareness-anchored reason it is
	// currently necessary plus the migration target. This list must
	// shrink over time. Each entry was vetted by querying the awareness
	// graph for the file's anchors before adding it here.
	legitimateEtcdUsage := map[string]string{
		// CATEGORY 1 — legitimate coordination primitives (etcd is the
		// standard distributed coordination layer; not a four-layer
		// data write).
		"cluster_doctor_server/leader_election.go": "concurrency.NewElection / lease. Anchored by invariant:doctor.remediation_requires_leader and forbidden_fix:execute_remediation_on_follower_instance. Etcd here is a coordination primitive, not a data write. Remains legitimate unless leader election moves to a controller-managed surface.",
		"cluster_doctor_server/collector/collector.go": "etcd service-discovery primitive for dialling other services. Reading the service registry to find endpoint addresses, not reading another layer's state.",

		// CATEGORY 2 — security/policy state that legitimately needs to
		// persist across leader failover. Currently in etcd. Migration
		// target: ai-memory typed history RPC (the doctor's own
		// persistence surface that honours the layer model).
		"cluster_doctor_server/approval_replay_etcd.go": "Approval-token replay table. Anchored by failure_mode:doctor.approval_token_replay_across_failover — the table MUST survive leader change or a replayed token re-authorises an old action. Migration target: ai-memory typed RPC. Tracked follow-up.",
		"cluster_doctor_server/remediation_history.go":  "Remediation history. Anchored by invariant:remediation.must_not_retry_without_changed_evidence_or_policy_budget — the budget check needs durable history. Migration target: ai-memory typed RPC. Tracked follow-up.",

		// CATEGORY 3 — grandfathered direct L2/L3 reads from the rules
		// package. Each file has a TODO comment pointing at the migration
		// (new typed cluster_controller RPC + collector.Snapshot field).
		// The read pin test (TestClusterDoctor_NoNewEtcdReads) names the
		// specific allowed function inside each file.
		"cluster_doctor_server/rules/etcd_helpers.go":              "Generic accessors used by the two grandfathered rule readers below.",
		"cluster_doctor_server/rules/package_version_authority.go": "readDesiredVersions — grandfathered direct read (see TODO in file).",
		"cluster_doctor_server/rules/repository_dns_invariants.go": "readDesiredBuildIDs — grandfathered direct read (see TODO in file).",

		// CATEGORY 4 — collector machinery with no awareness anchors yet.
		// Inspect-and-anchor needed before refactor. Tracked follow-up.
		"cluster_doctor_server/collector/sweep_requests.go": "Sweep request queue. No awareness anchors yet — needs the inspect-and-anchor pass before migration.",
		"cluster_doctor_server/collector/verification.go":   "fetchDesiredServiceTargets — direct etcd read of L2. Tracked under TestClusterDoctor_NoNewEtcdReads allowlist below.",
	}

	walkClusterDoctorGoFiles(t, root, func(path string, body []byte) {
		rel, _ := filepath.Rel(root, path)
		if _, allowed := legitimateEtcdUsage[rel]; allowed {
			return
		}
		if dataWriteRE.Match(body) {
			t.Errorf("CRITICAL %s contains a .Put(ctx, \"/globular/...\", ...) call — "+
				"violates invariant:cluster_doctor.observer_only_never_writes_etcd. "+
				"cluster-doctor must not write data to etcd. Dispatch a typed "+
				"RemediationAction (→ node-agent / controller) or persist via "+
				"the ai-memory typed RPC. This pin's allowlist names the existing "+
				"tracked exceptions — new violations must not be added.", path)
		}
	})
}

// TestClusterDoctor_NoNewEtcdReads pins the read side. Two grandfathered
// reads are allow-listed by exact file+function name — they have TODO
// markers pointing at the migration plan (typed RPC on the controller +
// Snapshot field). Any NEW direct etcd .Get of /globular/resources/* or
// /globular/nodes/* prefixes added under cluster_doctor/rules/ will fail
// this test.
//
// Allowed reads inside the allowlist:
//   rules/repository_dns_invariants.go: readDesiredBuildIDs
//   rules/package_version_authority.go: readDesiredVersions
//
// These will be removed when the typed RPC + Snapshot field land. The
// allowlist is intentionally narrow so further reads can't slip in
// under the same justification.
func TestClusterDoctor_NoNewEtcdReads(t *testing.T) {
	root := mustClusterDoctorRoot(t)

	getRE := regexp.MustCompile(`\.Get\(\s*[^,)]+,\s*"(/globular/resources|/globular/nodes/[^"]*packages)`)

	// Allowlist: relative path → set of function names whose direct etcd
	// reads are grandfathered. NEW reads in any other function fail the
	// test. The allowlist must shrink over time, never grow.
	// Allowlist keyed by rel path under cluster_doctor/ (which is what
	// `filepath.Rel(root, path)` returns inside the walk).
	allowed := map[string]map[string]bool{
		"cluster_doctor_server/rules/repository_dns_invariants.go": {
			"readDesiredBuildIDs": true,
		},
		"cluster_doctor_server/rules/package_version_authority.go": {
			"readDesiredVersions": true,
		},
		"cluster_doctor_server/collector/verification.go": {
			"fetchDesiredServiceTargets": true,
		},
	}

	walkClusterDoctorGoFiles(t, root, func(path string, body []byte) {
		matches := getRE.FindAllIndex(body, -1)
		if len(matches) == 0 {
			return
		}
		rel, _ := filepath.Rel(root, path)
		funcs, isAllowedFile := allowed[rel]
		for _, m := range matches {
			// Find the enclosing function name by scanning backward for
			// the most recent `func ... (` header. Good-enough heuristic
			// for the test; the production code path is not load-bearing
			// on this parse.
			fn := enclosingFuncName(body, m[0])
			if isAllowedFile && funcs[fn] {
				continue
			}
			t.Errorf("CRITICAL %s:%s contains a direct etcd .Get of /globular/{resources,nodes}/* — "+
				"violates invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. "+
				"Call the owner's typed RPC instead "+
				"(cluster_controller.GetDesiredState / GetServiceRelease / "+
				"node_agent.ListInstalledPackages). If a typed RPC for the exact "+
				"shape does not exist yet, add it on the owner — do not 'temporarily' "+
				"scan etcd.",
				path, fn)
		}
	})
}

// ── helpers ───────────────────────────────────────────────────────────────

func mustClusterDoctorRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// Test runs from cluster_doctor/cluster_doctor_server. Walk upward to
	// the directory above, which contains all of cluster_doctor/.
	root := filepath.Dir(wd)
	if filepath.Base(root) != "cluster_doctor" {
		t.Fatalf("unexpected test cwd: wd=%s root=%s (want cluster_doctor)", wd, root)
	}
	return root
}

func walkClusterDoctorGoFiles(t *testing.T, root string, visit func(path string, body []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		visit(path, body)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

// enclosingFuncName returns the most recent `func ... name(` preceding offset.
// Best-effort. If no function header found before offset, returns "<top-level>".
func enclosingFuncName(body []byte, offset int) string {
	funcRE := regexp.MustCompile(`(?m)^func(?:\s*\([^)]*\))?\s+(\w+)\s*\(`)
	// Find the last match before offset.
	matches := funcRE.FindAllSubmatchIndex(body[:offset], -1)
	if len(matches) == 0 {
		return "<top-level>"
	}
	last := matches[len(matches)-1]
	return string(body[last[2]:last[3]])
}
