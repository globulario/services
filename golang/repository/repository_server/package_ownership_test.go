package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/security"
)

// ── validatePublisherAccess ──────────────────────────────────────────────────

func TestValidatePublisherAccess_NilAuthContext_Allowed(t *testing.T) {
	// Internal calls (no AuthContext in context) should pass without auth.
	srv := newTestServer(t)
	ctx := context.Background() // no AuthContext injected

	err := srv.validatePublisherAccess(ctx, "acme")
	if err != nil {
		t.Fatalf("expected nil error for nil AuthContext (internal call), got: %v", err)
	}
}

func TestValidatePublisherAccess_Superuser_Allowed(t *testing.T) {
	// "sa" superuser should always pass.
	srv := newTestServer(t)
	authCtx := &security.AuthContext{
		Subject:       "sa",
		PrincipalType: "user",
		AuthMethod:    "jwt",
	}
	ctx := authCtx.ToContext(context.Background())

	err := srv.validatePublisherAccess(ctx, "any-namespace")
	if err != nil {
		t.Fatalf("expected nil error for superuser, got: %v", err)
	}
}

func TestValidatePublisherAccess_EmptyPublisherID_Rejected(t *testing.T) {
	srv := newTestServer(t)
	err := srv.validatePublisherAccess(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty publisher_id")
	}
}

func TestValidatePublisherAccess_EmptySubject_Rejected(t *testing.T) {
	srv := newTestServer(t)
	authCtx := &security.AuthContext{
		Subject:       "",
		PrincipalType: "user",
		AuthMethod:    "jwt",
	}
	ctx := authCtx.ToContext(context.Background())

	err := srv.validatePublisherAccess(ctx, "acme")
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
}

// ── validatePackageAccess ────────────────────────────────────────────────────

func TestValidatePackageAccess_NilAuthContext_Allowed(t *testing.T) {
	// Internal calls pass without auth at both namespace and package level.
	srv := newTestServer(t)
	ctx := context.Background()

	err := srv.validatePackageAccess(ctx, "acme", "my-service")
	if err != nil {
		t.Fatalf("expected nil error for nil AuthContext, got: %v", err)
	}
}

func TestValidatePackageAccess_Superuser_Allowed(t *testing.T) {
	srv := newTestServer(t)
	authCtx := &security.AuthContext{
		Subject:       "sa",
		PrincipalType: "user",
		AuthMethod:    "jwt",
	}
	ctx := authCtx.ToContext(context.Background())

	err := srv.validatePackageAccess(ctx, "any-namespace", "any-package")
	if err != nil {
		t.Fatalf("expected nil error for superuser, got: %v", err)
	}
}

// ── classifyPublishMode ──────────────────────────────────────────────────────

func TestClassifyPublishMode_Human(t *testing.T) {
	srv := newTestServer(t)
	authCtx := &security.AuthContext{
		Subject:       "dave",
		PrincipalType: "user",
		AuthMethod:    "jwt",
	}
	ctx := authCtx.ToContext(context.Background())

	mode := srv.classifyPublishMode(ctx, "acme", "my-service")
	if mode != "human" {
		t.Errorf("expected 'human', got %q", mode)
	}
}

func TestClassifyPublishMode_Internal(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background() // no AuthContext

	mode := srv.classifyPublishMode(ctx, "acme", "my-service")
	if mode != "internal" {
		t.Errorf("expected 'internal', got %q", mode)
	}
}

func TestClassifyPublishMode_MachinePublisher(t *testing.T) {
	// APPLICATION principal with no trusted publisher relationship.
	srv := newTestServer(t)
	authCtx := &security.AuthContext{
		Subject:       "ci-bot",
		PrincipalType: "application",
		AuthMethod:    "jwt",
	}
	ctx := authCtx.ToContext(context.Background())

	mode := srv.classifyPublishMode(ctx, "acme", "my-service")
	if mode != "machine_publisher" {
		t.Errorf("expected 'machine_publisher', got %q", mode)
	}
}

// ── principalToSubjectType ───────────────────────────────────────────────────

func TestPrincipalToSubjectType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"application", "APPLICATION"},
		{"node", "NODE_IDENTITY"},
		{"user", "ACCOUNT"},
		{"", "ACCOUNT"},
		{"unknown", "ACCOUNT"},
	}
	for _, tt := range tests {
		got := principalToSubjectType(tt.input)
		if got.String() != tt.want {
			t.Errorf("principalToSubjectType(%q) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
