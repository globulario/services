package process

import (
	"testing"

	"github.com/globulario/services/golang/config"
)

func TestConfigureEtcdNodeURLs_UsesRoutableAddressesAndFixedToken(t *testing.T) {
	nodeCfg := map[string]interface{}{}

	err := configureEtcdNodeURLs(nodeCfg, "globule-nuc", "/var/lib/globular/etcd", "https", "10.0.0.63", "globule-nuc.globular.internal")
	if err != nil {
		t.Fatalf("configureEtcdNodeURLs returned error: %v", err)
	}

	if got := nodeCfg["listen-client-urls"]; got != "https://10.0.0.63:2379" {
		t.Fatalf("listen-client-urls = %v, want %q", got, "https://10.0.0.63:2379")
	}
	if got := nodeCfg["listen-peer-urls"]; got != "https://10.0.0.63:2380" {
		t.Fatalf("listen-peer-urls = %v, want %q", got, "https://10.0.0.63:2380")
	}
	if got := nodeCfg["advertise-client-urls"]; got != "https://globule-nuc.globular.internal:2379" {
		t.Fatalf("advertise-client-urls = %v, want %q", got, "https://globule-nuc.globular.internal:2379")
	}
	if got := nodeCfg["initial-advertise-peer-urls"]; got != "https://globule-nuc.globular.internal:2380" {
		t.Fatalf("initial-advertise-peer-urls = %v, want %q", got, "https://globule-nuc.globular.internal:2380")
	}
	if got := nodeCfg["initial-cluster-token"]; got != config.EtcdClusterToken {
		t.Fatalf("initial-cluster-token = %v, want %q", got, config.EtcdClusterToken)
	}
}

func TestConfigureEtcdNodeURLs_RejectsNonRoutableHosts(t *testing.T) {
	tests := []struct {
		name          string
		listenAddress string
		advertiseHost string
	}{
		{
			name:          "unspecified listen address",
			listenAddress: "0.0.0.0",
			advertiseHost: "globule-nuc.globular.internal",
		},
		{
			name:          "loopback advertise host",
			listenAddress: "10.0.0.63",
			advertiseHost: "127.0.0.1",
		},
		{
			name:          "localhost advertise host",
			listenAddress: "10.0.0.63",
			advertiseHost: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := configureEtcdNodeURLs(map[string]interface{}{}, "globule-nuc", "/var/lib/globular/etcd", "https", tt.listenAddress, tt.advertiseHost)
			if err == nil {
				t.Fatalf("expected error for listen=%q advertise=%q", tt.listenAddress, tt.advertiseHost)
			}
		})
	}
}
