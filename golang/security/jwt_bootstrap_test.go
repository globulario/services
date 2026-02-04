package security

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/metadata"
)

// TestGetClientIdWithBootstrapContext verifies that GetClientId accepts internal bootstrap context.
func TestGetClientIdWithBootstrapContext(t *testing.T) {
	// Create internal bootstrap context (outgoing)
	md := metadata.Pairs(
		"token", "internal-bootstrap",
		"x-globular-internal", "true",
		"client-id", "system@localhost",
	)
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	clientID, token, err := GetClientId(ctx)
	if err != nil {
		t.Fatalf("GetClientId() error = %v, want nil", err)
	}

	if clientID != "system@localhost" {
		t.Errorf("clientID = %q, want \"system@localhost\"", clientID)
	}

	if token != "internal-bootstrap" {
		t.Errorf("token = %q, want \"internal-bootstrap\"", token)
	}
}

// TestGetClientIdRejectsExternalBootstrapToken verifies that external clients
// cannot use the internal bootstrap token.
func TestGetClientIdRejectsExternalBootstrapToken(t *testing.T) {
	// Simulate external client trying to use internal bootstrap token via incoming context
	md := metadata.Pairs(
		"token", "internal-bootstrap",
		"x-globular-internal", "true",
		"client-id", "system@localhost",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, _, err := GetClientId(ctx)
	if err == nil {
		t.Fatal("GetClientId() should reject internal bootstrap token from incoming context")
	}

	if !strings.Contains(err.Error(), "internal bootstrap token not allowed from external calls") {
		t.Errorf("expected rejection message, got: %v", err)
	}
}

// TestGetClientIdWithEmptyContext verifies the original error message for empty context.
func TestGetClientIdWithEmptyContext(t *testing.T) {
	ctx := context.Background()

	_, _, err := GetClientId(ctx)
	if err == nil {
		t.Fatal("GetClientId() should fail with empty context")
	}

	if !strings.Contains(err.Error(), "no token found in context metadata") {
		t.Errorf("expected 'no token found' error, got: %v", err)
	}
}

// TestGetClientIdWithIncompleteBootstrapContext verifies that incomplete bootstrap
// contexts are rejected.
func TestGetClientIdWithIncompleteBootstrapContext(t *testing.T) {
	tests := []struct {
		name string
		md   metadata.MD
	}{
		{
			name: "missing internal marker",
			md: metadata.Pairs(
				"token", "internal-bootstrap",
				"client-id", "system@localhost",
			),
		},
		{
			name: "missing client-id",
			md: metadata.Pairs(
				"token", "internal-bootstrap",
				"x-globular-internal", "true",
			),
		},
		{
			name: "wrong internal marker value",
			md: metadata.Pairs(
				"token", "internal-bootstrap",
				"x-globular-internal", "false",
				"client-id", "system@localhost",
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewOutgoingContext(context.Background(), tt.md)
			_, _, err := GetClientId(ctx)
			if err == nil {
				t.Error("GetClientId() should reject incomplete bootstrap context")
			}
		})
	}
}
