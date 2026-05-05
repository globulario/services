package main

import (
	"context"
	"os"
	"testing"
)

func TestParseEtcdEndpointsPayload_JSONArray(t *testing.T) {
	raw := `["https://10.0.0.8:2379","https://10.0.0.20:2379"]`
	got := parseEtcdEndpointsPayload(raw)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0] != "https://10.0.0.8:2379" || got[1] != "https://10.0.0.20:2379" {
		t.Fatalf("unexpected parsed endpoints: %v", got)
	}
}

func TestRefreshEtcdEndpointsFromSystemKey_WritesFileAndResetsClient(t *testing.T) {
	origFetch := fetchSystemEtcdEndpoints
	origWrite := writeEtcdEndpointsFile
	origReset := resetSharedEtcdClient
	t.Cleanup(func() {
		fetchSystemEtcdEndpoints = origFetch
		writeEtcdEndpointsFile = origWrite
		resetSharedEtcdClient = origReset
	})
	fetchSystemEtcdEndpoints = func(ctx context.Context) (string, error) {
		return `["https://10.0.0.8:2379","https://10.0.0.20:2379"]`, nil
	}
	wrote := ""
	writeEtcdEndpointsFile = func(path string, data []byte, perm os.FileMode) error {
		wrote = string(data)
		return nil
	}
	resetCalled := false
	resetSharedEtcdClient = func() { resetCalled = true }

	srv := &NodeAgentServer{}
	srv.refreshEtcdEndpointsFromSystemKey(context.Background())

	if wrote == "" {
		t.Fatal("expected endpoint file write")
	}
	if wrote != "https://10.0.0.8:2379\nhttps://10.0.0.20:2379\n" {
		t.Fatalf("written content mismatch: %q", wrote)
	}
	if !resetCalled {
		t.Fatal("expected shared etcd client reset")
	}
}
