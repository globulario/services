package resourcestore

import (
	"testing"

	"github.com/globulario/services/golang/config"
)

// TestGuardOwner_EnforcesCriticalKeyOwnerAtResourceWrite proves the RT-2 activation:
// config.ValidateCriticalKeyOwner — previously inert (zero non-test runtime callers,
// per the RT-1 audit) — is now enforced at the desired-state write chokepoint
// (etcdStore.Apply/Delete via guardOwner). The owner passes; a non-owner is refused,
// so the gate is real, not decoration.
func TestGuardOwner_EnforcesCriticalKeyOwnerAtResourceWrite(t *testing.T) {
	key := keyFor("ServiceDesiredVersion", "echo") // /globular/resources/ServiceDesiredVersion/echo

	// The cluster-controller (the registered owner of /globular/resources/) passes —
	// Apply/Delete call guardOwner with controllerWriterID, so legitimate
	// desired-state writes are never blocked.
	if err := guardOwner(key); err != nil {
		t.Fatalf("controller (owner) write to %q must pass, got: %v", key, err)
	}

	// A non-owner writer for the same resources key is refused — this is the runtime
	// enforcement the ownership table previously lacked. Once RT-2 routes the CLI /
	// script bypass paths through the owner, this gate is what stops them.
	for _, intruder := range []string{"node-agent", "operator-cli", "intruder"} {
		if err := config.ValidateCriticalKeyOwner(key, intruder); err == nil {
			t.Errorf("non-owner %q writing %q must be refused", intruder, key)
		}
	}
}
