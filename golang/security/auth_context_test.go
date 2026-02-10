package security

import (
	"context"
	"net"
	"os"
	"testing"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// TestNewAuthContext_Anonymous verifies anonymous context creation
func TestNewAuthContext_Anonymous(t *testing.T) {
	ctx := context.Background()
	authCtx, err := NewAuthContext(ctx, "/test.TestService/TestMethod")

	if err != nil {
		t.Fatalf("NewAuthContext() error = %v, want nil", err)
	}

	if authCtx.Subject != "" {
		t.Errorf("Subject = %q, want empty for anonymous", authCtx.Subject)
	}

	if authCtx.PrincipalType != "anonymous" {
		t.Errorf("PrincipalType = %q, want \"anonymous\"", authCtx.PrincipalType)
	}

	if authCtx.AuthMethod != "none" {
		t.Errorf("AuthMethod = %q, want \"none\"", authCtx.AuthMethod)
	}

	if authCtx.GRPCMethod != "/test.TestService/TestMethod" {
		t.Errorf("GRPCMethod = %q, want \"/test.TestService/TestMethod\"", authCtx.GRPCMethod)
	}
}

// TestNewAuthContext_Bootstrap verifies bootstrap mode detection
func TestNewAuthContext_Bootstrap(t *testing.T) {
	// Set bootstrap mode
	os.Setenv("GLOBULAR_BOOTSTRAP", "1")
	defer os.Unsetenv("GLOBULAR_BOOTSTRAP")

	ctx := context.Background()
	authCtx, err := NewAuthContext(ctx, "/test.TestService/TestMethod")

	if err != nil {
		t.Fatalf("NewAuthContext() error = %v, want nil", err)
	}

	if !authCtx.IsBootstrap {
		t.Error("IsBootstrap = false, want true")
	}
}

// TestNewAuthContext_Loopback verifies loopback detection
func TestNewAuthContext_Loopback(t *testing.T) {
	tests := []struct {
		name       string
		peerAddr   string
		wantLoopback bool
	}{
		{
			name:       "IPv4 loopback",
			peerAddr:   "127.0.0.1:12345",
			wantLoopback: true,
		},
		{
			name:       "IPv6 loopback",
			peerAddr:   "[::1]:12345",
			wantLoopback: true,
		},
		{
			name:       "localhost hostname",
			peerAddr:   "localhost:12345",
			wantLoopback: true,
		},
		{
			name:       "remote IP",
			peerAddr:   "192.168.1.1:12345",
			wantLoopback: false,
		},
		{
			name:       "no peer info",
			peerAddr:   "",
			wantLoopback: true, // Conservative: treat as loopback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.peerAddr == "" {
				ctx = context.Background()
			} else {
				addr, err := net.ResolveTCPAddr("tcp", tt.peerAddr)
				if err != nil {
					t.Fatalf("failed to resolve address: %v", err)
				}
				ctx = peer.NewContext(context.Background(), &peer.Peer{
					Addr: addr,
				})
			}

			authCtx, err := NewAuthContext(ctx, "/test.TestService/TestMethod")
			if err != nil {
				t.Fatalf("NewAuthContext() error = %v, want nil", err)
			}

			if authCtx.IsLoopback != tt.wantLoopback {
				t.Errorf("IsLoopback = %v, want %v", authCtx.IsLoopback, tt.wantLoopback)
			}
		})
	}
}

// TestAuthContext_ContextStorage verifies ToContext/FromContext roundtrip
func TestAuthContext_ContextStorage(t *testing.T) {
	ctx := context.Background()
	authCtx, err := NewAuthContext(ctx, "/test.TestService/TestMethod")
	if err != nil {
		t.Fatalf("NewAuthContext() error = %v, want nil", err)
	}

	// Store in context
	ctx = authCtx.ToContext(ctx)

	// Retrieve from context
	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("FromContext() returned nil, want AuthContext")
	}

	if retrieved.GRPCMethod != authCtx.GRPCMethod {
		t.Errorf("Retrieved GRPCMethod = %q, want %q", retrieved.GRPCMethod, authCtx.GRPCMethod)
	}
}

// TestAuthContext_String verifies string representation
func TestAuthContext_String(t *testing.T) {
	ctx := context.Background()
	authCtx, err := NewAuthContext(ctx, "/test.TestService/TestMethod")
	if err != nil {
		t.Fatalf("NewAuthContext() error = %v, want nil", err)
	}

	str := authCtx.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Should contain key fields
	if !contains(str, "AuthContext") {
		t.Error("String() should contain 'AuthContext'")
	}
	if !contains(str, authCtx.GRPCMethod) {
		t.Errorf("String() should contain method %q", authCtx.GRPCMethod)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || len(s) > len(substr)+1 && contains(s[1:], substr)))
}

// TestNewAuthContext_InvalidToken verifies graceful handling of invalid tokens
func TestNewAuthContext_InvalidToken(t *testing.T) {
	// Create context with invalid token
	md := metadata.Pairs("token", "invalid-token-xxx")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	authCtx, err := NewAuthContext(ctx, "/test.TestService/TestMethod")

	// Should not error - should treat as anonymous
	if err != nil {
		t.Fatalf("NewAuthContext() error = %v, want nil (graceful handling)", err)
	}

	// Should fall back to anonymous
	if authCtx.PrincipalType != "anonymous" {
		t.Errorf("PrincipalType = %q, want \"anonymous\" for invalid token", authCtx.PrincipalType)
	}
}
