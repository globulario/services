package main

import (
	"context"
	"testing"
)

func TestEnsureSystemConfigKey_SeedsWhenMissing(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)

	srv.ensureSystemConfigKey(context.Background(), kv)

	resp, err := kv.Get(context.Background(), systemConfigKey)
	if err != nil {
		t.Fatalf("kv get: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatalf("expected %s to be seeded", systemConfigKey)
	}
}

func TestEnsureCriticalPrefixMarkers_SeedsMarkers(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)

	srv.ensureCriticalPrefixMarkers(context.Background(), kv)

	for _, key := range []string{
		resourcesBootstrapMarker,
		nodesBootstrapMarker,
		scyllaBootstrapMarker,
	} {
		resp, err := kv.Get(context.Background(), key)
		if err != nil {
			t.Fatalf("kv get %s: %v", key, err)
		}
		if len(resp.Kvs) == 0 {
			t.Fatalf("expected marker %s to be seeded", key)
		}
	}
}
