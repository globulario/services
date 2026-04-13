package security

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestBootstrapContext(t *testing.T) {
	ctx := BootstrapContext()
	if ctx == nil {
		t.Fatal("BootstrapContext() returned nil")
	}

	// Verify metadata is present in outgoing context
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("BootstrapContext() did not create outgoing context with metadata")
	}

	// Check token
	tokens := md.Get("token")
	if len(tokens) == 0 {
		t.Fatal("BootstrapContext() missing token metadata")
	}
	if tokens[0] != InternalBootstrapToken {
		t.Errorf("token = %q, want %q", tokens[0], InternalBootstrapToken)
	}

	// Check internal marker
	internals := md.Get("x-globular-internal")
	if len(internals) == 0 {
		t.Fatal("BootstrapContext() missing x-globular-internal metadata")
	}
	if internals[0] != "true" {
		t.Errorf("x-globular-internal = %q, want \"true\"", internals[0])
	}

	// Check client ID
	clientIDs := md.Get("client-id")
	if len(clientIDs) == 0 {
		t.Fatal("BootstrapContext() missing client-id metadata")
	}
	if clientIDs[0] != InternalBootstrapClientID {
		t.Errorf("client-id = %q, want %q", clientIDs[0], InternalBootstrapClientID)
	}
}

func TestIsBootstrapContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want bool
	}{
		{
			name: "valid bootstrap context",
			ctx:  BootstrapContext(),
			want: true,
		},
		{
			name: "empty context",
			ctx:  context.Background(),
			want: false,
		},
		{
			name: "context with wrong token",
			ctx: metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
				"token", "wrong-token",
				"x-globular-internal", "true",
			)),
			want: false,
		},
		{
			name: "context without internal marker",
			ctx: metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
				"token", InternalBootstrapToken,
			)),
			want: false,
		},
		{
			name: "incoming context with bootstrap token (should not match)",
			ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(
				"token", InternalBootstrapToken,
				"x-globular-internal", "true",
			)),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBootstrapContext(tt.ctx)
			if got != tt.want {
				t.Errorf("IsBootstrapContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBootstrapContextNotAcceptedFromIncoming(t *testing.T) {
	// Create a context that looks like bootstrap but uses incoming metadata
	// This simulates an external client trying to use the internal token
	md := metadata.Pairs(
		"token", InternalBootstrapToken,
		"x-globular-internal", "true",
		"client-id", InternalBootstrapClientID,
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// IsBootstrapContext should reject this
	if IsBootstrapContext(ctx) {
		t.Error("IsBootstrapContext() should reject incoming context with bootstrap token")
	}
}
