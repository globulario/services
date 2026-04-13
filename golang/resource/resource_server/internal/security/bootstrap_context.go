package security

import (
	"context"

	"google.golang.org/grpc/metadata"
)

const (
	// InternalBootstrapToken is a special marker token used during service startup.
	// This is NOT a real JWT token and should never be accepted from external clients.
	InternalBootstrapToken = "internal-bootstrap"

	// InternalBootstrapClientID is the identity used during bootstrap operations.
	InternalBootstrapClientID = "system@localhost"
)

// BootstrapContext creates a context with internal bootstrap identity metadata.
// This context should ONLY be used for internal service startup operations,
// never for handling external RPC requests.
//
// The metadata added here allows security.GetClientId to recognize and accept
// internal bootstrap operations without requiring a real user token.
func BootstrapContext() context.Context {
	md := metadata.Pairs(
		"token", InternalBootstrapToken,
		"x-globular-internal", "true",
		"client-id", InternalBootstrapClientID,
	)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// IsBootstrapContext checks if a context contains the internal bootstrap marker.
// This should be used by security functions to identify legitimate internal operations.
func IsBootstrapContext(ctx context.Context) bool {
	// Check outgoing context (internal calls)
	if md, ok := metadata.FromOutgoingContext(ctx); ok {
		if vals := md.Get("x-globular-internal"); len(vals) > 0 && vals[0] == "true" {
			if tokens := md.Get("token"); len(tokens) > 0 && tokens[0] == InternalBootstrapToken {
				return true
			}
		}
	}
	return false
}
