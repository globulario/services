// rbac_role_bindings_test.go: unit tests for SetRoleBinding / GetRoleBinding / ListRoleBindings.
//
// Run with: go test ./golang/rbac/rbac_server -run "TestSetGetRoleBinding|TestListRoleBindings|TestGetRoleBinding|TestMgmtProtection"

package main

import (
	"context"
	"sort"
	"testing"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

// bootstrapCtx returns a context that simulates Day-0 bootstrap mode.
// The bootstrap gate has already validated loopback + time window + allowlist,
// so no role check is performed.
func bootstrapCtx() context.Context {
	authCtx := &security.AuthContext{IsBootstrap: true, Subject: "bootstrap"}
	return authCtx.ToContext(context.Background())
}

// callerCtx returns a context for an authenticated (non-bootstrap) subject.
func callerCtx(subject string) context.Context {
	authCtx := &security.AuthContext{Subject: subject, IsBootstrap: false}
	return authCtx.ToContext(context.Background())
}

// listBindingsStream is a minimal implementation of grpc.ServerStreamingServer[rbacpb.ListRoleBindingsRsp]
// that collects streamed bindings for test assertions.
type listBindingsStream struct {
	ctx   context.Context
	items []*rbacpb.RoleBinding
}

func (s *listBindingsStream) Send(rsp *rbacpb.ListRoleBindingsRsp) error {
	s.items = append(s.items, rsp.GetBinding())
	return nil
}
func (s *listBindingsStream) SetHeader(metadata.MD) error  { return nil }
func (s *listBindingsStream) SendHeader(metadata.MD) error { return nil }
func (s *listBindingsStream) SetTrailer(metadata.MD)       {}
func (s *listBindingsStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *listBindingsStream) SendMsg(m any) error { return nil }
func (s *listBindingsStream) RecvMsg(m any) error { return nil }

// --- TestSetGetRoleBinding_RoundTrip ----------------------------------------

func TestSetGetRoleBinding_RoundTrip(t *testing.T) {
	srv := newTestServer(t)
	ctx := bootstrapCtx()

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
	// Self-read: subject in AuthContext matches requested subject.
	ctx := callerCtx("nobody@example.com")

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

func TestListRoleBindings_ReturnsAll(t *testing.T) {
	srv := newTestServer(t)
	bctx := bootstrapCtx()

	seeds := map[string][]string{
		"controller-sa": {"globular-controller-sa"},
		"node-agent-sa": {"globular-node-agent-sa"},
		"gateway-sa":    {"globular-admin"},
	}

	for subj, roles := range seeds {
		if _, err := srv.SetRoleBinding(bctx, &rbacpb.SetRoleBindingRqst{
			Binding: &rbacpb.RoleBinding{Subject: subj, Roles: roles},
		}); err != nil {
			t.Fatalf("seed SetRoleBinding %q: %v", subj, err)
		}
	}

	stream := &listBindingsStream{ctx: bctx}
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

// --- Management protection tests --------------------------------------------

// TestMgmtProtection_UnauthenticatedDenied verifies that unauthenticated callers
// (no AuthContext in context) are rejected with Unauthenticated.
func TestMgmtProtection_UnauthenticatedDenied(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background() // no AuthContext

	_, err := srv.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "victim", Roles: []string{"globular-admin"}},
	})
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("SetRoleBinding without auth: want Unauthenticated, got %v (%v)", code, err)
	}

	_, err = srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: "other"})
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("GetRoleBinding without auth: want Unauthenticated, got %v (%v)", code, err)
	}

	stream := &listBindingsStream{ctx: ctx}
	err = srv.ListRoleBindings(&rbacpb.ListRoleBindingsRqst{}, stream)
	if code := status.Code(err); code != codes.Unauthenticated {
		t.Errorf("ListRoleBindings without auth: want Unauthenticated, got %v (%v)", code, err)
	}
	t.Log("✓ unauthenticated callers denied with Unauthenticated")
}

// TestMgmtProtection_NonAdminDenied verifies that authenticated but non-admin
// callers are rejected with PermissionDenied.
func TestMgmtProtection_NonAdminDenied(t *testing.T) {
	srv := newTestServer(t)
	bctx := bootstrapCtx()

	// Seed a non-admin subject.
	_, err := srv.SetRoleBinding(bctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "operator", Roles: []string{"globular-operator"}},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	ctx := callerCtx("operator")

	_, err = srv.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "victim", Roles: []string{"globular-admin"}},
	})
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("SetRoleBinding as non-admin: want PermissionDenied, got %v (%v)", code, err)
	}

	// Non-admin reading someone else's binding — denied.
	_, err = srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: "other-subject"})
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("GetRoleBinding other as non-admin: want PermissionDenied, got %v (%v)", code, err)
	}

	stream := &listBindingsStream{ctx: ctx}
	err = srv.ListRoleBindings(&rbacpb.ListRoleBindingsRqst{}, stream)
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("ListRoleBindings as non-admin: want PermissionDenied, got %v (%v)", code, err)
	}
	t.Log("✓ non-admin callers denied with PermissionDenied")
}

// TestMgmtProtection_AdminAllowed verifies that a principal with globular-admin
// can call all management methods.
func TestMgmtProtection_AdminAllowed(t *testing.T) {
	srv := newTestServer(t)
	bctx := bootstrapCtx()

	// Seed the admin binding during bootstrap.
	_, err := srv.SetRoleBinding(bctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "admin-user", Roles: []string{"globular-admin"}},
	})
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	ctx := callerCtx("admin-user")

	// Admin can set a binding.
	_, err = srv.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "new-user", Roles: []string{"globular-operator"}},
	})
	if err != nil {
		t.Errorf("SetRoleBinding as admin: %v", err)
	}

	// Admin can read any binding.
	_, err = srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: "new-user"})
	if err != nil {
		t.Errorf("GetRoleBinding as admin: %v", err)
	}

	// Admin can list all bindings.
	stream := &listBindingsStream{ctx: ctx}
	if err := srv.ListRoleBindings(&rbacpb.ListRoleBindingsRqst{}, stream); err != nil {
		t.Errorf("ListRoleBindings as admin: %v", err)
	}
	t.Log("✓ admin can call all management methods")
}

// TestMgmtProtection_SelfReadAllowed verifies that any authenticated principal
// can read their own binding without needing globular-admin.
func TestMgmtProtection_SelfReadAllowed(t *testing.T) {
	srv := newTestServer(t)
	bctx := bootstrapCtx()

	_, err := srv.SetRoleBinding(bctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "alice", Roles: []string{"globular-publisher"}},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	ctx := callerCtx("alice")

	resp, err := srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: "alice"})
	if err != nil {
		t.Fatalf("GetRoleBinding self-read: %v", err)
	}
	if len(resp.GetBinding().GetRoles()) == 0 {
		t.Error("expected roles in self-read response")
	}
	t.Log("✓ non-admin can read their own binding")
}

// TestMgmtProtection_BootstrapBypass verifies that bootstrap mode allows all
// management operations regardless of caller role.
func TestMgmtProtection_BootstrapBypass(t *testing.T) {
	srv := newTestServer(t)
	ctx := bootstrapCtx()

	// Bootstrap allows SetRoleBinding without any pre-existing role.
	_, err := srv.SetRoleBinding(ctx, &rbacpb.SetRoleBindingRqst{
		Binding: &rbacpb.RoleBinding{Subject: "first-admin", Roles: []string{"globular-admin"}},
	})
	if err != nil {
		t.Fatalf("SetRoleBinding during bootstrap: %v", err)
	}

	// Bootstrap allows GetRoleBinding of any subject.
	_, err = srv.GetRoleBinding(ctx, &rbacpb.GetRoleBindingRqst{Subject: "first-admin"})
	if err != nil {
		t.Fatalf("GetRoleBinding during bootstrap: %v", err)
	}

	// Bootstrap allows ListRoleBindings.
	stream := &listBindingsStream{ctx: ctx}
	if err := srv.ListRoleBindings(&rbacpb.ListRoleBindingsRqst{}, stream); err != nil {
		t.Fatalf("ListRoleBindings during bootstrap: %v", err)
	}
	t.Log("✓ bootstrap mode bypasses role check (gate enforces loopback + time window)")
}
