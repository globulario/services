package globular_client

import (
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// newIdleConn builds a real *grpc.ClientConn object without dialing (lazy
// connect), suitable for state assertions in tests.
func newIdleConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	cc, err := grpc.NewClient("passthrough:///dummy",
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	return cc
}

// TestReleaseClientConnection_PreservesSharedMeshConn is the regression guard
// for the event-breaker CRIT: a client releasing its cached connection must NOT
// close the process-wide shared mesh connection, because every other service
// client reuses it. Closing it caused cluster-wide "connection is closing"
// churn and event circuit-breaker flapping.
func TestReleaseClientConnection_PreservesSharedMeshConn(t *testing.T) {
	shared := newIdleConn(t)
	defer shared.Close()

	meshMu.Lock()
	prev := meshConn
	meshConn = shared
	meshMu.Unlock()
	// Restore global state after the test.
	defer func() {
		meshMu.Lock()
		meshConn = prev
		meshMu.Unlock()
	}()

	// Releasing the shared mesh conn must be a no-op — it stays usable.
	ReleaseClientConnection(shared)
	if got := shared.GetState(); got == connectivity.Shutdown {
		t.Fatalf("shared mesh conn was closed by ReleaseClientConnection (state=%s); it must be preserved", got)
	}
}

// TestReleaseClientConnection_ClosesDirectConn verifies the other half of the
// ownership contract: a caller-owned direct connection IS closed.
func TestReleaseClientConnection_ClosesDirectConn(t *testing.T) {
	// Ensure the direct conn is not mistaken for the shared one.
	shared := newIdleConn(t)
	defer shared.Close()
	meshMu.Lock()
	prev := meshConn
	meshConn = shared
	meshMu.Unlock()
	defer func() {
		meshMu.Lock()
		meshConn = prev
		meshMu.Unlock()
	}()

	direct := newIdleConn(t)
	ReleaseClientConnection(direct)
	if got := direct.GetState(); got != connectivity.Shutdown {
		t.Fatalf("direct conn should be closed by ReleaseClientConnection, state=%s", got)
	}
}

// TestReleaseClientConnection_NilSafe guards the nil path.
func TestReleaseClientConnection_NilSafe(t *testing.T) {
	ReleaseClientConnection(nil) // must not panic
}
