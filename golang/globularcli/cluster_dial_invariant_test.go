package main

import (
	"errors"
	"strings"
	"testing"
)

func TestRewriteDialInvariantError_TLSHandshake(t *testing.T) {
	baseErr := errors.New("transport: authentication handshake failed: tls: first record does not look like a TLS handshake")
	err := rewriteDialInvariantError("10.0.0.63:40377", "10.0.0.63:40377", baseErr)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	got := err.Error()
	if !strings.Contains(got, "endpoint protocol invariant violation") {
		t.Fatalf("expected invariant violation message, got: %s", got)
	}
	if !strings.Contains(got, "use a TLS-routable endpoint") {
		t.Fatalf("expected remediation hint, got: %s", got)
	}
}

func TestRewriteDialInvariantError_Passthrough(t *testing.T) {
	baseErr := errors.New("context deadline exceeded")
	err := rewriteDialInvariantError("globular.internal", "globular.internal:443", baseErr)
	if !errors.Is(err, baseErr) {
		t.Fatalf("expected original error passthrough, got: %v", err)
	}
}
