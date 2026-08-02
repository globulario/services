package globular_client

// direct_mode.go — explicit single-instance addressing.
//
// GetClientConnection is mesh-first: it routes through local Envoy, which
// load-balances across every instance behind the VIP. That is correct for
// ordinary application traffic and wrong for administrative operations that
// name one instance on purpose.
//
// The scar: `globular pkg publish --repository 10.0.0.20:10004` never reached
// 10.0.0.20. Ninety-six publishes round-robined, scattering node-local CAS
// blobs 16/8/19 across three repository instances while the manifests (which
// are cluster-wide in ScyllaDB) advertised every artifact as PUBLISHED
// everywhere. Downloads then intermittently hit an instance without the blob,
// and platform-upgrade re-dispatched the resulting drift every 30s — 914
// consecutive cycles for globular-cli@1.2.289.
//
// Direct mode is deliberately narrow: it is opt-in per client, it never
// consults mesh discovery, and it never falls back. A caller that names an
// instance gets that instance or an error.

import (
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	Utility "github.com/globulario/utility"
)

// ConnectionMode selects how a client resolves its endpoint.
type ConnectionMode int

const (
	// ConnectionModeMeshPreferred is the default: route via Envoy when
	// available, fall back to a direct dial.
	ConnectionModeMeshPreferred ConnectionMode = iota
	// ConnectionModeDirect dials exactly the address the caller supplied.
	// No mesh, no discovery, no fallback.
	ConnectionModeDirect
)

var (
	directMu   sync.RWMutex
	directMode = map[Client]ConnectionMode{}
)

// SetConnectionMode records the connection mode for a client. Passing
// ConnectionModeMeshPreferred clears any previous direct pin.
func SetConnectionMode(client Client, mode ConnectionMode) {
	directMu.Lock()
	defer directMu.Unlock()
	if mode == ConnectionModeDirect {
		directMode[client] = mode
		return
	}
	delete(directMode, client)
}

// IsDirectMode reports whether this client must bypass the mesh.
func IsDirectMode(client Client) bool {
	directMu.RLock()
	defer directMu.RUnlock()
	return directMode[client] == ConnectionModeDirect
}

// DirectTarget returns the exact host:port a direct-mode client will dial.
// Exposed so callers can show the operator which instance was contacted —
// silent rerouting is the failure this mode exists to prevent.
func DirectTarget(client Client) string {
	address := client.GetAddress()
	if strings.Contains(address, ":") {
		// Address already carries an explicit port; honour it verbatim.
		return address
	}
	return address + ":" + Utility.ToString(client.GetPort())
}

// dialDirect connects to exactly the client's configured endpoint. TLS and the
// client interceptor are applied exactly as in the mesh path, so credentials
// and token propagation are unchanged. Any failure is returned to the caller
// rather than retried against a different instance.
func dialDirect(client Client) (*grpc.ClientConn, error) {
	target := DirectTarget(client)

	if client.HasTLS() {
		tcfg, err := GetClientTlsConfig(client)
		if err != nil {
			return nil, err
		}
		return grpc.Dial(target,
			grpc.WithTransportCredentials(credentials.NewTLS(tcfg)),
			grpc.WithUnaryInterceptor(clientInterceptor(client)),
		)
	}
	return grpc.Dial(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(clientInterceptor(client)),
	)
}
