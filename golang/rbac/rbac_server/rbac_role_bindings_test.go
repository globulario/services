// rbac_role_bindings_test.go: unit tests for SetRoleBinding / GetRoleBinding / ListRoleBindings.
//
// Run with: go test ./golang/rbac/rbac_server -run "TestSetGetRoleBinding|TestListRoleBindings|TestGetRoleBinding"

package main

import (
	"context"
	"sort"
	"testing"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/storage/storage_store"
	"google.golang.org/grpc/metadata"
)

// newTestServer creates a minimal in-memory server for role-binding tests.
func newTestServer(t *testing.T) *server {
	t.Helper()

	cache := storage_store.NewBigCache_store()
	if err := cache.Open(`{"max_size": 50}`); err != nil {
		t.Fatalf("open cache: %v", err)
	}

	store := storage_store.NewLevelDB_store()
	tmpDir := t.TempDir()
	if err := store.Open(`{"path":"` + tmpDir + `","name":"rbactest"}`); err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() {
		_ = cache.Close()
		_ = store.Close()
	})

	return &server{
		cache:       cache,
		permissions: store,
	}
}

// --- TestSetGetRoleBinding_RoundTrip ----------------------------------------

func TestSetGetRoleBinding_RoundTrip(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	subject := "alice@example.com"
	roles := []string{"globular-operator", "globular-publisher"}

	_, err := srv.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: subject, Roles: roles},
	})
	if err != nil {
		t.Fatalf("SetRoleBinding: %v", err)
	}

	resp, err := srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: subject})
	if err != nil {
		t.Fatalf("GetRoleBinding: %v", err)
	}

	got := resp.GetBinding()
	if got.GetSubject() != subject {
		t.Errorf("subject: got %q, want %q", got.GetSubject(), subject)
	}
	gotRoles := got.GetRoles()
	sort.Strings(gotRoles)
	wantRoles := make([]string, len(roles))
	copy(wantRoles, roles)
	sort.Strings(wantRoles)

	if len(gotRoles) != len(wantRoles) {
		t.Errorf("roles len: got %d, want %d", len(gotRoles), len(wantRoles))
	}
	for i := range wantRoles {
		if i < len(gotRoles) && gotRoles[i] != wantRoles[i] {
			t.Errorf("roles[%d]: got %q, want %q", i, gotRoles[i], wantRoles[i])
		}
	}
	t.Logf("✓ round-trip: %q → %v", subject, gotRoles)
}

// --- TestGetRoleBinding_NotFound_ReturnsEmpty --------------------------------

func TestGetRoleBinding_NotFound_ReturnsEmpty(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	resp, err := srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: "nobody@example.com"})
	if err != nil {
		t.Fatalf("GetRoleBinding for missing subject must not error: %v", err)
	}

	b := resp.GetBinding()
	if b == nil {
		t.Fatal("binding must not be nil for missing subject")
	}
	if len(b.GetRoles()) != 0 {
		t.Errorf("roles must be empty for missing subject, got %v", b.GetRoles())
	}
	t.Log("✓ missing subject returns empty binding, not error")
}

// --- TestListRoleBindings_ReturnsAll ----------------------------------------

// listBindingsStream is a minimal implementation of grpc.ServerStreamingServer[rbacpb.ListRoleBindingsRsp]
// that collects streamed bindings for test assertions.
type listBindingsStream struct {
	items []*rbacpb.RoleBinding
}

func (s *listBindingsStream) Send(rsp *rbacpb.ListRoleBindingsRsp) error {
	s.items = append(s.items, rsp.GetBinding())
	return nil
}
func (s *listBindingsStream) SetHeader(metadata.MD) error  { return nil }
func (s *listBindingsStream) SendHeader(metadata.MD) error { return nil }
func (s *listBindingsStream) SetTrailer(metadata.MD)       {}
func (s *listBindingsStream) Context() context.Context     { return context.Background() }
func (s *listBindingsStream) SendMsg(m any) error          { return nil }
func (s *listBindingsStream) RecvMsg(m any) error          { return nil }

func TestListRoleBindings_ReturnsAll(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	seeds := map[string][]string{
		"controller-sa": {"globular-controller-sa"},
		"node-agent-sa": {"globular-node-agent-sa"},
		"gateway-sa":    {"globular-admin"},
	}

	for subj, roles := range seeds {
		if _, err := srv.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
			Binding: &rbacpb.RoleBinding{Subject: subj, Roles: roles},
		}); err != nil {
			t.Fatalf("seed SetRoleBinding %q: %v", subj, err)
		}
	}

	stream := &listBindingsStream{}
	if err := srv.ListRoleBindings(&rbacpb.ListRoleBindingsRqst{}, stream); err != nil {
		t.Fatalf("ListRoleBindings: %v", err)
	}

	if len(stream.items) != len(seeds) {
		t.Errorf("expected %d bindings, got %d", len(seeds), len(stream.items))
	}

	got := make(map[string][]string, len(stream.items))
	for _, b := range stream.items {
		got[b.GetSubject()] = b.GetRoles()
	}
	for subj, wantRoles := range seeds {
		gotRoles, ok := got[subj]
		if !ok {
			t.Errorf("subject %q missing from list", subj)
			continue
		}
		if len(gotRoles) != len(wantRoles) || gotRoles[0] != wantRoles[0] {
			t.Errorf("subject %q roles: got %v, want %v", subj, gotRoles, wantRoles)
		}
	}
	t.Logf("✓ ListRoleBindings returned %d bindings", len(stream.items))
}
