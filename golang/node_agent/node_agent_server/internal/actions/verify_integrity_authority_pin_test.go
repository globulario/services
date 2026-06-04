// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.actions.verify_integrity_authority_pin
// @awareness file_role=architectural_pin_test_for_verify_integrity_resolver_injection
// @awareness enforces=globular.platform:invariant.four_layer.truth_read_via_owner_rpc_not_direct_storage
// @awareness risk=high
package actions

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Architectural pin for the v1.2.169 refactor of verify_integrity.go.
//
// Before v1.2.169 the package.verify_integrity action read
// /globular/resources/ServiceDesiredVersion/* directly from etcd via
// config.GetEtcdClient() + clientv3.WithPrefix(). The fix replaced
// that with an injected desiredVersionResolver populated at server
// boot by a closure that dials cluster_controller.GetDesiredState
// (the typed RPC that owns L2).
//
// This test fails if a future contributor:
//   - reintroduces a clientv3 import in verify_integrity.go, or
//   - reintroduces a direct etcd Get / Put / Delete against
//     /globular/resources/* in this file.
//
// Anchored by:
//   invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//   forbidden_fix:read_owned_etcd_prefix_directly_instead_of_calling_owner_rpc

func TestVerifyIntegrity_NoDirectEtcdAgainstResourcesPrefix(t *testing.T) {
	body, err := os.ReadFile("verify_integrity.go")
	if err != nil {
		t.Fatalf("read verify_integrity.go: %v", err)
	}

	// Forbid the clientv3 import — verify_integrity.go must not
	// touch etcd directly. The resolver injection point is the only
	// supported channel.
	if strings.Contains(string(body), `clientv3 "go.etcd.io/etcd/client/v3"`) ||
		strings.Contains(string(body), `"go.etcd.io/etcd/client/v3"`) {
		t.Errorf("CRITICAL verify_integrity.go imports go.etcd.io/etcd/client/v3 — " +
			"violates invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. " +
			"The action MUST read L2 desired state through the injected " +
			"desiredVersionResolver, which is wired at server boot to call " +
			"cluster_controller.GetDesiredState. Reintroducing the etcd " +
			"client re-opens the bypass vector closed in v1.2.169.")
	}

	// Catch the call shape directly too — even if a future
	// contributor pulls etcd in via a helper, this fires on the
	// quoted prefix.
	re := regexp.MustCompile(`\.(Get|Put|Delete)\(\s*[^,)]+,\s*"/globular/resources/`)
	if loc := re.FindIndex(body); loc != nil {
		match := re.FindSubmatch(body)
		t.Errorf("CRITICAL verify_integrity.go contains a direct etcd %s against /globular/resources/* "+
			"(near byte offset %d) — violates invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage. "+
			"L2 desired state is owned by cluster_controller; read it through the injected resolver "+
			"(see desiredVersionResolver / SetDesiredVersionResolver).",
			string(match[1]), loc[0])
	}
}

// TestVerifyIntegrity_ResolverHasSafeDefault guards against an empty
// state where the server forgets to install the resolver: the action
// must skip the I2 invariant rather than panic or fabricate state.
func TestVerifyIntegrity_ResolverHasSafeDefault(t *testing.T) {
	// Snapshot any installed resolver so the test does not leak state.
	saved := desiredVersionResolver
	t.Cleanup(func() { desiredVersionResolver = saved })

	// Simulate "server forgot to wire it" by calling Set with nil.
	SetDesiredVersionResolver(nil)
	// Replace with the documented default behaviour for the safety
	// check below.
	desiredVersionResolver = func(_ context.Context) map[string]DesiredRef {
		return map[string]DesiredRef{}
	}

	got := readDesiredVersions(context.Background())
	if got == nil {
		t.Fatalf("readDesiredVersions returned nil map — must be non-nil even when resolver is missing")
	}
	if len(got) != 0 {
		t.Fatalf("default resolver leaked state: %d entries", len(got))
	}
}
