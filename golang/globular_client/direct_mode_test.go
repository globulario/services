package globular_client

// Regression for the blob-scatter scar: `pkg publish --repository <node>:10004`
// was silently load-balanced by mesh-first routing, so 96 publishes scattered
// node-local CAS blobs 16/8/19 across three repository instances instead of
// placing them. Direct mode must address exactly one instance.

import "testing"

type fakeDirectClient struct {
	Client
	addr string
	port int
}

func (f *fakeDirectClient) GetAddress() string { return f.addr }
func (f *fakeDirectClient) GetPort() int       { return f.port }

func TestDirectModeIsOptInAndClearable(t *testing.T) {
	c := &fakeDirectClient{addr: "10.0.0.20", port: 10004}

	if IsDirectMode(c) {
		t.Fatal("a fresh client must default to mesh-preferred, not direct")
	}
	SetConnectionMode(c, ConnectionModeDirect)
	if !IsDirectMode(c) {
		t.Fatal("SetConnectionMode(Direct) did not pin the client")
	}
	// Clearing must restore mesh routing for ordinary traffic — direct mode is
	// narrow by design and must not leak into normal application clients.
	SetConnectionMode(c, ConnectionModeMeshPreferred)
	if IsDirectMode(c) {
		t.Fatal("ConnectionModeMeshPreferred did not clear the direct pin")
	}
}

// The supplied host:port must be dialled verbatim. Reinterpreting it — or
// substituting the mesh VIP — is the defect this mode exists to prevent.
func TestDirectTargetHonoursSuppliedEndpointVerbatim(t *testing.T) {
	withPort := &fakeDirectClient{addr: "10.0.0.20:10004", port: 99999}
	if got := DirectTarget(withPort); got != "10.0.0.20:10004" {
		t.Errorf("explicit host:port must be used verbatim: got %q, want 10.0.0.20:10004", got)
	}

	bare := &fakeDirectClient{addr: "10.0.0.8", port: 10004}
	if got := DirectTarget(bare); got != "10.0.0.8:10004" {
		t.Errorf("bare host must combine with the client port: got %q, want 10.0.0.8:10004", got)
	}
}

// Direct mode must never reroute. Two distinct endpoints must stay distinct —
// if these collapsed to one target, blobs would scatter exactly as before.
func TestDirectModeKeepsInstancesDistinct(t *testing.T) {
	a := &fakeDirectClient{addr: "10.0.0.20", port: 10004}
	b := &fakeDirectClient{addr: "10.0.0.8", port: 10004}
	SetConnectionMode(a, ConnectionModeDirect)
	SetConnectionMode(b, ConnectionModeDirect)
	defer SetConnectionMode(a, ConnectionModeMeshPreferred)
	defer SetConnectionMode(b, ConnectionModeMeshPreferred)

	if DirectTarget(a) == DirectTarget(b) {
		t.Fatalf("two direct-mode clients collapsed to one target %q — this is the round-robin bug", DirectTarget(a))
	}
}
