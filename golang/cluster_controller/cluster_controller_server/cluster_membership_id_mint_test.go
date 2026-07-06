package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/netutil"
	"github.com/google/uuid"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// fakeMintKV is a minimal in-memory kvClient for the mint tests.
type fakeMintKV struct {
	store map[string]string
	puts  int
}

func (f *fakeMintKV) Get(_ context.Context, key string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	r := &clientv3.GetResponse{}
	if v, ok := f.store[key]; ok {
		r.Kvs = []*mvccpb.KeyValue{{Key: []byte(key), Value: []byte(v)}}
	}
	return r, nil
}

func (f *fakeMintKV) Put(_ context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	if f.store == nil {
		f.store = map[string]string{}
	}
	f.store[key] = val
	f.puts++
	return &clientv3.PutResponse{}, nil
}

func (f *fakeMintKV) Delete(_ context.Context, _ string, _ ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	return &clientv3.DeleteResponse{}, nil
}

// TestEnsureClusterMembershipID_MintsOnceOpaqueUUID: on an empty store the
// controller mints a valid opaque UUID; the value is NOT the domain.
func TestEnsureClusterMembershipID_MintsOnceOpaqueUUID(t *testing.T) {
	kv := &fakeMintKV{}
	srv := &server{}

	srv.ensureClusterMembershipID(context.Background(), kv)

	if kv.puts != 1 {
		t.Fatalf("expected exactly 1 Put (mint), got %d", kv.puts)
	}
	got := kv.store[config.ClusterMembershipIDKey]
	if _, err := uuid.Parse(got); err != nil {
		t.Errorf("minted value %q is not a valid UUID: %v", got, err)
	}
	// Identity must not be the mutable domain (the whole point of the migration).
	if got == "globular.internal" || got == "" {
		t.Errorf("minted membership id must be an opaque UUID, got %q", got)
	}
}

// TestEnsureClusterMembershipID_Idempotent: re-running the Day-0 seed pass does
// not re-mint (day0_day1_are_repeatable_ceremonies) — mint-once holds.
func TestEnsureClusterMembershipID_Idempotent(t *testing.T) {
	kv := &fakeMintKV{}
	srv := &server{}

	srv.ensureClusterMembershipID(context.Background(), kv)
	first := kv.store[config.ClusterMembershipIDKey]
	srv.ensureClusterMembershipID(context.Background(), kv)
	srv.ensureClusterMembershipID(context.Background(), kv)

	if kv.puts != 1 {
		t.Errorf("mint-once violated: %d Puts across 3 passes, want 1", kv.puts)
	}
	if kv.store[config.ClusterMembershipIDKey] != first {
		t.Errorf("membership id changed across passes: %q → %q", first, kv.store[config.ClusterMembershipIDKey])
	}
}

// TestEnsureClusterMembershipID_PopulatesState proves the A3 core: the minted
// membership UUID is cached into controller state (state.ClusterUID) so identity
// readers use it without a per-read etcd call — and it is NOT the domain.
func TestEnsureClusterMembershipID_PopulatesState(t *testing.T) {
	state := newControllerState()
	srv := newTestServer(t, state)
	kv := &fakeMintKV{store: map[string]string{config.ClusterMembershipIDKey: "aaaaaaaa-1111-2222-3333-444444444444"}}

	srv.ensureClusterMembershipID(context.Background(), kv)

	if srv.state.ClusterUID != "aaaaaaaa-1111-2222-3333-444444444444" {
		t.Errorf("state.ClusterUID = %q, want the minted UUID cached from the authority", srv.state.ClusterUID)
	}
	if srv.state.ClusterUID == srv.state.ClusterId {
		t.Errorf("membership UUID must not equal the namespace ClusterId")
	}
}

// TestEnsureClusterMembershipID_BackfillsUnboundTokens: minting the identity
// binds — atomically with caching state.ClusterUID under one lock hold — any join
// token that was seeded before the identity existed (the config token is seeded
// pre-mint on Day-0). This closes the window that would otherwise let an
// initialized cluster hold an unbound token, which the strict join gate rejects.
// A token already bound (even to a different value) is NEVER silently rebound —
// that would be a cross-cluster hijack.
func TestEnsureClusterMembershipID_BackfillsUnboundTokens(t *testing.T) {
	state := newControllerState()
	state.JoinTokens["cfg-tok"] = &joinTokenRecord{Token: "cfg-tok"}                          // unbound (pre-mint seed)
	state.JoinTokens["foreign"] = &joinTokenRecord{Token: "foreign", ClusterUID: "keep-me-9"} // already bound
	srv := newTestServer(t, state)
	kv := &fakeMintKV{} // empty store → mint fresh

	srv.ensureClusterMembershipID(context.Background(), kv)

	minted := srv.state.ClusterUID
	if minted == "" {
		t.Fatal("expected a minted membership UUID cached into state")
	}
	if got := srv.state.JoinTokens["cfg-tok"].ClusterUID; got != minted {
		t.Errorf("unbound token was not backfilled: ClusterUID=%q, want minted %q", got, minted)
	}
	if got := srv.state.JoinTokens["foreign"].ClusterUID; got != "keep-me-9" {
		t.Errorf("an already-bound token must not be rebound (hijack guard): got %q, want keep-me-9", got)
	}
}

// TestLoadControllerState_DoesNotCoerceOpaqueClusterId is the guard on the
// "dragon's nostril": loadControllerState must NOT rewrite an opaque (UUID)
// ClusterId back to a domain shape. It defaults to the domain only when unset.
func TestLoadControllerState_DoesNotCoerceOpaqueClusterId(t *testing.T) {
	const opaque = "abcd1234-5678-49ab-8cde-0123456789ab" // no dot: would have been coerced before

	dir := t.TempDir()

	// Opaque cluster id survives verbatim.
	p1 := filepath.Join(dir, "state_uuid.json")
	if err := os.WriteFile(p1, []byte(`{"cluster_id":"`+opaque+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := loadControllerState(p1)
	if err != nil {
		t.Fatalf("loadControllerState: %v", err)
	}
	if st.ClusterId != opaque {
		t.Errorf("opaque ClusterId was coerced: got %q, want %q (the isDomainLike rewrite must be gone)", st.ClusterId, opaque)
	}

	// Empty cluster id still defaults to the domain (behavior preserved).
	p2 := filepath.Join(dir, "state_empty.json")
	if err := os.WriteFile(p2, []byte(`{"cluster_id":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st2, err := loadControllerState(p2)
	if err != nil {
		t.Fatalf("loadControllerState: %v", err)
	}
	if st2.ClusterId != netutil.DefaultClusterDomain() {
		t.Errorf("empty ClusterId default changed: got %q, want %q", st2.ClusterId, netutil.DefaultClusterDomain())
	}
}

// TestEnsureClusterMembershipID_NeverOverwrites: an already-established identity
// is immutable — the controller must never overwrite it.
func TestEnsureClusterMembershipID_NeverOverwrites(t *testing.T) {
	const existing = "11111111-2222-3333-4444-555555555555"
	kv := &fakeMintKV{store: map[string]string{config.ClusterMembershipIDKey: existing}}
	srv := &server{}

	srv.ensureClusterMembershipID(context.Background(), kv)

	if kv.puts != 0 {
		t.Errorf("immutability violated: %d Puts over an existing id, want 0", kv.puts)
	}
	if kv.store[config.ClusterMembershipIDKey] != existing {
		t.Errorf("membership id overwritten: %q, want %q", kv.store[config.ClusterMembershipIDKey], existing)
	}
}
