// @awareness namespace=globular.platform
// @awareness component=platform_globularcli.state_cmds_authority_pin
// @awareness file_role=architectural_pin_test_for_scanInstalledState_typed_rpc
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness enforces=globular.platform:invariant.etcd.path_has_single_owner
// @awareness risk=high
package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Architectural pin for the v1.2.176 refactor of
// state_cmds.go::scanInstalledState.
//
// Before v1.2.176 scanInstalledState scanned /globular/nodes/ (owned
// by node_agent) directly via clientv3. The refactor routes through
// two typed RPCs:
//
//	cluster_controller.ListNodes  → enumerate nodes + agent endpoints
//	node_agent.ListInstalledPackages — owner's view of L3 installed state
//
// This test fails if a future contributor reintroduces a direct etcd
// Get / Put / Delete against /globular/nodes/* in this file.
//
// Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	invariant:etcd.path_has_single_owner
//	forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc
//
// Other functions in state_cmds.go still use clientv3 (the
// audittrail-owned /globular/audit/desired_writes/ read in
// repairDesiredWriteProvenanceLegacy is a tracked follow-up — different
// ownership domain). This pin is scoped to the /globular/nodes/* prefix
// so it lights up only on the regression class scanInstalledState used
// to be.
func TestStateCmds_ScanInstalledState_NoDirectEtcdAgainstNodes(t *testing.T) {
	body, err := os.ReadFile("state_cmds.go")
	if err != nil {
		t.Fatalf("read state_cmds.go: %v", err)
	}

	// Scope to the scanInstalledState function body. Other functions
	// in this file still have unrelated /globular/nodes/* scans
	// (repairGhostNodes, resolveAgentEndpoint, …) — those are
	// separate tracked ratchets and out of v1.2.176's scope. This pin
	// fails specifically if scanInstalledState reintroduces the prior
	// bypass.
	const fnHeader = "func scanInstalledState(ctx context.Context, report *canonReport) error {"
	startIdx := strings.Index(string(body), fnHeader)
	if startIdx < 0 {
		t.Fatalf("scanInstalledState function header not found — has the signature changed? " +
			"If so, update this pin so the regression guard still tracks the function body.")
	}
	endIdx := findMatchingBrace(string(body), startIdx+len(fnHeader)-1)
	if endIdx <= startIdx {
		t.Fatalf("could not locate end of scanInstalledState function — pin cannot scope to body")
	}
	fnBody := body[startIdx:endIdx]

	re := regexp.MustCompile(`\.(Get|Put|Delete)\(\s*[^,)]+,\s*"/globular/`)
	if loc := re.FindIndex(fnBody); loc != nil {
		match := re.FindSubmatch(fnBody)
		t.Errorf("CRITICAL state_cmds.go::scanInstalledState contains a direct etcd %s against /globular/* "+
			"(near byte offset %d inside the function) — violates "+
			"invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. "+
			"L3 installed state is owned by node_agent; enumerate nodes via "+
			"cluster_controller.ListNodes then call node_agent.ListInstalledPackages per "+
			"node (the v1.2.176 scanInstalledState pattern). Reintroducing the etcd scan re-opens "+
			"the bypass vector closed in v1.2.176.",
			string(match[1]), loc[0])
	}
}

// findMatchingBrace returns the index of the closing '}' that
// balances the '{' at openIdx within src. Returns -1 if not found.
// Simple brace-counter; does not handle braces inside string literals
// — fine for Go source that does not embed `{` or `}` inside string
// constants at the locations this pin scans.
func findMatchingBrace(src string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
