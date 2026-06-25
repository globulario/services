package config

import (
	"context"
	"testing"
)

// TestValidateCriticalKeyOwner_MultiWriter proves /globular/nodes/ admits both the
// node-agent owner and the cluster-controller authorized writer, and rejects a
// third writer; controller-owned keys still admit only the controller.
func TestValidateCriticalKeyOwner_MultiWriter(t *testing.T) {
	cases := []struct {
		key, writer string
		wantOK      bool
	}{
		{"/globular/nodes/n1/packages/svc/echo", "node-agent", true},         // owner
		{"/globular/nodes/n1/packages/svc/echo", "cluster-controller", true}, // authorized (release/convergence)
		{"/globular/nodes/n1/packages/svc/echo", "rogue", false},             // neither
		{"/globular/resources/DesiredService/echo", "cluster-controller", true},
		{"/globular/resources/DesiredService/echo", "node-agent", false}, // not authorized
		{"/globular/not/a/critical/key", "anyone", true},                 // non-critical → unrestricted
	}
	for _, tc := range cases {
		err := ValidateCriticalKeyOwner(tc.key, tc.writer)
		if (err == nil) != tc.wantOK {
			t.Errorf("ValidateCriticalKeyOwner(%q, %q) err=%v, wantOK=%v", tc.key, tc.writer, err, tc.wantOK)
		}
	}
}

// TestLocalWriterIdentityGuard proves the critical-write primitive enforces the
// registered process identity: fail-open when unset, allow owner + authorized
// writers, reject a writer not permitted for the key.
func TestLocalWriterIdentityGuard(t *testing.T) {
	kv := &fakeKV{}
	restore := SetWriteKVForTest(kv)
	defer restore()
	t.Cleanup(func() { SetLocalWriterIdentity("") })

	ctx := context.Background()
	nodeKey := "/globular/nodes/n1/packages/svc/echo"
	resourceKey := "/globular/resources/DesiredService/echo"

	// 1. Unregistered identity → fail-open: any critical write is allowed.
	SetLocalWriterIdentity("")
	if err := PutRuntimeWithClass(ctx, nodeKey, []byte("v"), CriticalWrite); err != nil {
		t.Fatalf("unregistered identity should be unguarded, got: %v", err)
	}

	// 2. Registered node-agent → owns /globular/nodes/, must be allowed.
	SetLocalWriterIdentity("node-agent")
	if err := PutRuntimeWithClass(ctx, nodeKey, []byte("v"), CriticalWrite); err != nil {
		t.Fatalf("node-agent writing its own key should be allowed, got: %v", err)
	}
	// node-agent is NOT authorized for /globular/resources/ → rejected.
	if err := PutRuntimeWithClass(ctx, resourceKey, []byte("v"), CriticalWrite); err == nil {
		t.Fatal("node-agent writing a controller-owned key should be rejected")
	}

	// 3. Registered cluster-controller → authorized writer of /globular/nodes/
	//    (release/convergence commits) and owner of /globular/resources/.
	SetLocalWriterIdentity("cluster-controller")
	if err := PutRuntimeWithClass(ctx, nodeKey, []byte("v"), CriticalWrite); err != nil {
		t.Fatalf("cluster-controller is an authorized installed-state writer, got: %v", err)
	}
	if err := PutRuntimeWithClass(ctx, resourceKey, []byte("v"), CriticalWrite); err != nil {
		t.Fatalf("cluster-controller writing its own key should be allowed, got: %v", err)
	}

	// 4. Delete primitive is guarded identically.
	SetLocalWriterIdentity("node-agent")
	if _, err := DeleteRuntimeWithClass(ctx, resourceKey, CriticalWrite); err == nil {
		t.Fatal("node-agent deleting a controller-owned key should be rejected")
	}
	if _, err := DeleteRuntimeWithClass(ctx, nodeKey, CriticalWrite); err != nil {
		t.Fatalf("node-agent deleting its own key should be allowed, got: %v", err)
	}
}
