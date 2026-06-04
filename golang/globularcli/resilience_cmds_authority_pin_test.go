// @awareness namespace=globular.platform
// @awareness component=platform_globularcli.resilience_cmds_authority_pin
// @awareness file_role=architectural_pin_test_for_ingress_cli_typed_rpc_routing
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestResilienceCmds_Ingress_NoDirectEtcd pins the v1.2.184 refactor
// of runIngressStatus + runIngressRepublish. Before v1.2.184 both
// functions scanned /globular/ingress/v1/* in etcd directly. That
// prefix is owned by the cluster_controller's ingress spec guard, so
// CLI consumers reading raw etcd violated
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage.
//
// Routes through cluster_controller.GetIngressStatus +
// RequestIngressRepublish.
//
// Scope: both function bodies. Other functions in this file (scylla
// schema guard helpers) still use clientv3 against different
// prefixes; those are separate tracked ratchets.
func TestResilienceCmds_Ingress_NoDirectEtcd(t *testing.T) {
	body, err := os.ReadFile("resilience_cmds.go")
	if err != nil {
		t.Fatalf("read resilience_cmds.go: %v", err)
	}

	for _, fn := range []string{
		"func runIngressStatus(cmd *cobra.Command, args []string) error {",
		"func runIngressRepublish(cmd *cobra.Command, args []string) error {",
	} {
		startIdx := strings.Index(string(body), fn)
		if startIdx < 0 {
			t.Errorf("function header not found: %s", fn)
			continue
		}
		endIdx := findMatchingBrace(string(body), startIdx+len(fn)-1)
		if endIdx <= startIdx {
			t.Errorf("could not locate end of %s", fn)
			continue
		}
		fnBody := body[startIdx:endIdx]

		re := regexp.MustCompile(`\.(Get|Put|Delete|Watch)\(\s*[^,)]+,\s*[^,)]*"/globular/ingress/`)
		if loc := re.FindIndex(fnBody); loc != nil {
			match := re.FindSubmatch(fnBody)
			t.Errorf("CRITICAL %s contains a direct etcd %s against /globular/ingress/* "+
				"(near byte offset %d inside the function) — violates "+
				"invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. "+
				"Route through cluster_controller.GetIngressStatus / "+
				"RequestIngressRepublish (the v1.2.184 pattern).",
				strings.TrimSuffix(strings.TrimPrefix(fn, "func "), "(cmd *cobra.Command, args []string) error {"),
				string(match[1]), loc[0])
		}
	}
}
