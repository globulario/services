package actions

// registry_isolation_test.go — test-only protection for the process-global
// action registry.
//
// Register() and Decorate() mutate package-level maps. In production that is
// safe: every one of the 21 call sites is inside func init(), and Go serializes
// package initialization, so there is no live race and no runtime mutation.
//
// In tests it is not safe. A test that calls Register(&fakeHandler{name:
// "package.uninstall"}) replaces the real handler for the REST OF THE PROCESS.
// That is not hypothetical: it silently broke TestServiceUninstall_* — those
// tests passed alone and failed when run after the OCI tests, and the failure
// was invisible in CI because the only broad `go test ./...` step is
// continue-on-error and the focused OCI workflow filtered them out with -run.
//
// Any test that registers or decorates an action MUST call this first.

import "testing"

// swapActionRegistryForTest snapshots the action registry and decorator table
// and restores both when the test ends, so handler substitution cannot leak
// into any later test.
func swapActionRegistryForTest(t *testing.T) {
	t.Helper()

	savedRegistry := make(map[string]Handler, len(registry))
	for name, handler := range registry {
		savedRegistry[name] = handler
	}
	savedDecorators := make(map[string][]Decorator, len(decorators))
	for name, chain := range decorators {
		savedDecorators[name] = append([]Decorator(nil), chain...)
	}

	t.Cleanup(func() {
		registry = savedRegistry
		decorators = savedDecorators
	})
}

// TestActionRegistryIsRestoredBetweenTests is a ratchet: it fails if the
// isolation helper stops working, before the damage reaches an unrelated suite.
func TestActionRegistryIsRestoredBetweenTests(t *testing.T) {
	realUninstall := Get("package.uninstall")
	if realUninstall == nil {
		t.Fatal("package.uninstall must be registered by init()")
	}

	t.Run("substitute", func(t *testing.T) {
		swapActionRegistryForTest(t)
		Register(&fakeHandler{name: "package.uninstall"})
		if Get("package.uninstall") == realUninstall {
			t.Fatal("substitution did not take effect inside the subtest")
		}
	})

	if Get("package.uninstall") != realUninstall {
		t.Fatal("the real package.uninstall handler was not restored — registry isolation is broken")
	}
}
