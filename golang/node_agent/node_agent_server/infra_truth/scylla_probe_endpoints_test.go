package infra_truth

import "testing"

// TestResolveEndpoints_UsesRenderedBindAddress is the regression for the false
// scylla.probe_required_when_installed ERROR: ScyllaDB binds REST (api_address)
// and CQL (rpc_address) to the cluster-facing node IP, but the probe used to
// dial a hardcoded 127.0.0.1 and got connection-refused — reporting a fully
// healthy node as daemon_starting. The probe must derive its targets from the
// rendered config. Honors infra.runtime_truth_must_be_observed_via_native_api.
func TestResolveEndpoints_UsesRenderedBindAddress(t *testing.T) {
	tests := []struct {
		name         string
		api          string
		rpc          string
		wantRESTBase string
		wantCQLAddr  string
	}{
		{
			// The INC scenario: node-IP bind. Probe MUST follow it, not loopback.
			name:         "node_ip_bind",
			api:          "10.0.0.63",
			rpc:          "10.0.0.63",
			wantRESTBase: "http://10.0.0.63:10000",
			wantCQLAddr:  "10.0.0.63:9042",
		},
		{
			// Loopback api_address (the secure local-admin default) still works.
			name:         "loopback_api",
			api:          "127.0.0.1",
			rpc:          "10.0.0.8",
			wantRESTBase: "http://127.0.0.1:10000",
			wantCQLAddr:  "10.0.0.8:9042",
		},
		{
			// Bind-all / unset → loopback is the safe local target.
			name:         "bind_all_and_empty",
			api:          "0.0.0.0",
			rpc:          "",
			wantRESTBase: "http://127.0.0.1:10000",
			wantCQLAddr:  "127.0.0.1:9042",
		},
		{
			// Quoted values (ScyllaDB yaml often quotes addresses) are stripped.
			name:         "quoted",
			api:          "'10.0.0.20'",
			rpc:          "'10.0.0.20'",
			wantRESTBase: "http://10.0.0.20:10000",
			wantCQLAddr:  "10.0.0.20:9042",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewScyllaProber() // production loopback defaults
			rendered := &ScyllaRenderedConfig{APIAddress: tt.api, RPCAddress: tt.rpc}
			restBase, cqlAddr := p.resolveEndpoints(rendered)
			if restBase != tt.wantRESTBase {
				t.Errorf("restBase = %q, want %q", restBase, tt.wantRESTBase)
			}
			if cqlAddr != tt.wantCQLAddr {
				t.Errorf("cqlAddr = %q, want %q", cqlAddr, tt.wantCQLAddr)
			}
		})
	}
}

// TestResolveEndpoints_HonorsExplicitInjection ensures a test/operator override
// (non-default RESTBase/CQLAddr, e.g. an httptest server) is used verbatim and
// is never clobbered by the rendered config.
func TestResolveEndpoints_HonorsExplicitInjection(t *testing.T) {
	p := NewScyllaProber()
	p.RESTBase = "http://127.0.0.1:18080"
	p.CQLAddr = "127.0.0.1:19042"
	rendered := &ScyllaRenderedConfig{APIAddress: "10.0.0.63", RPCAddress: "10.0.0.63"}
	restBase, cqlAddr := p.resolveEndpoints(rendered)
	if restBase != "http://127.0.0.1:18080" {
		t.Errorf("restBase = %q, want injected http://127.0.0.1:18080", restBase)
	}
	if cqlAddr != "127.0.0.1:19042" {
		t.Errorf("cqlAddr = %q, want injected 127.0.0.1:19042", cqlAddr)
	}
}
