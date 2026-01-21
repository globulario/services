package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/config"
	certpkg "github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/certs"
)

type fakeCertKV struct {
	bundle certpkg.CertBundle
	gen    uint64
}

func (f *fakeCertKV) AcquireCertIssuerLock(ctx context.Context, domain, nodeID string, ttl time.Duration) (bool, func(), error) {
	return true, func() {}, nil
}

func (f *fakeCertKV) PutBundle(ctx context.Context, domain string, bundle certpkg.CertBundle) error {
	f.bundle = bundle
	f.gen = bundle.Generation
	return nil
}

func (f *fakeCertKV) GetBundle(ctx context.Context, domain string) (certpkg.CertBundle, error) {
	return f.bundle, nil
}

func (f *fakeCertKV) WaitForBundle(ctx context.Context, domain string, timeout time.Duration) (certpkg.CertBundle, error) {
	return f.bundle, nil
}

func (f *fakeCertKV) GetBundleGeneration(ctx context.Context, domain string) (uint64, error) {
	return f.gen, nil
}

func setupStateDirs(t *testing.T) string {
	t.Helper()
	stateDir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	t.Setenv("GLOBULAR_STATE_DIR", stateDir)
	t.Setenv("GLOBULAR_CONFIG_DIR", filepath.Join(stateDir, "etc"))
	return stateDir
}

func TestCertWatcher_RestartsOnGenerationIncrease(t *testing.T) {
	setupStateDirs(t)
	_, fullchain, key, ca := config.CanonicalTLSPaths(config.GetRuntimeConfigDir())
	kv := &fakeCertKV{
		gen: 2,
		bundle: certpkg.CertBundle{
			Key:        []byte("k"),
			Fullchain:  []byte("f"),
			CA:         []byte("c"),
			Generation: 2,
		},
	}
	var restarted [][]string
	convergenceCalled := false
	srv := &NodeAgentServer{
		state: &nodeAgentState{
			Protocol:       "https",
			ClusterDomain:  "example.com",
			CertGeneration: 1,
		},
		certKV: kv,
		restartHook: func(units []string, _ *operation) error {
			restarted = append(restarted, append([]string{}, units...))
			return nil
		},
		healthCheckHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			convergenceCalled = true
			if len(restarted) == 0 {
				t.Fatalf("convergence ran before restart")
			}
			return nil
		},
		lastSpec:  &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "https"},
		statePath: filepath.Join(config.GetRuntimeConfigDir(), "state.json"),
	}

	srv.pollCertGeneration(context.Background())

	if len(restarted) != 1 {
		t.Fatalf("expected 1 restart, got %d", len(restarted))
	}
	expectOrder := []string{"globular-xds.service", "globular-envoy.service", "globular-gateway.service"}
	if !reflect.DeepEqual(restarted[0], expectOrder) {
		t.Fatalf("unexpected restart order %v", restarted[0])
	}
	if !convergenceCalled {
		t.Fatalf("expected convergence checks to run after restart")
	}
	// TLS files should be written
	for _, p := range []string{fullchain, key} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected tls file at %s: %v", p, err)
		}
	}
	if srv.state.CertGeneration != 2 {
		t.Fatalf("state generation not updated, got %d", srv.state.CertGeneration)
	}
	_ = ca // ca may be empty bundle; just ensure path exists if present
}

func TestCertWatcher_NoRestartIfGenerationUnchanged(t *testing.T) {
	setupStateDirs(t)
	kv := &fakeCertKV{
		gen: 1,
		bundle: certpkg.CertBundle{
			Key:        []byte("k"),
			Fullchain:  []byte("f"),
			Generation: 1,
		},
	}
	called := false
	convergence := false
	srv := &NodeAgentServer{
		state: &nodeAgentState{
			Protocol:       "https",
			ClusterDomain:  "example.com",
			CertGeneration: 1,
		},
		certKV: kv,
		restartHook: func(units []string, _ *operation) error {
			called = true
			return nil
		},
		healthCheckHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
			convergence = true
			return nil
		},
		lastSpec:  &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "https"},
		statePath: filepath.Join(config.GetRuntimeConfigDir(), "state.json"),
	}
	srv.pollCertGeneration(context.Background())
	if called {
		t.Fatalf("restart should not have been called when generation unchanged")
	}
	if convergence {
		t.Fatalf("convergence should not run when restart not triggered")
	}
}

func TestCertWatcher_DedupAndDebounce(t *testing.T) {
	setupStateDirs(t)
	kv := &fakeCertKV{
		gen: 2,
		bundle: certpkg.CertBundle{
			Key:        []byte("k"),
			Fullchain:  []byte("f"),
			Generation: 2,
		},
	}
	var count int
	srv := &NodeAgentServer{
		state: &nodeAgentState{
			Protocol:       "https",
			ClusterDomain:  "example.com",
			CertGeneration: 1,
		},
		certKV: kv,
		restartHook: func(units []string, _ *operation) error {
			count++
			return nil
		},
		healthCheckHook: func(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error { return nil },
		lastSpec:        &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com", Protocol: "https"},
		statePath:       filepath.Join(config.GetRuntimeConfigDir(), "state.json"),
	}

	srv.pollCertGeneration(context.Background()) // gen 2
	srv.pollCertGeneration(context.Background()) // duplicate gen should not restart
	kv.gen = 3
	kv.bundle.Generation = 3
	srv.lastCertRestart = time.Now().Add(-11 * time.Second) // defeat debounce
	srv.pollCertGeneration(context.Background())            // gen 3

	if count != 2 {
		t.Fatalf("expected 2 restarts, got %d", count)
	}
}
