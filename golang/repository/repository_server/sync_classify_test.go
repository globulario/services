package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/repository/upstream"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// A provider that reports a definitive absence (ErrNotFound) must classify as
// codes.NotFound, not codes.Unavailable. Conflating the two made Day-0 bootstrap
// retry a guaranteed-404 three times for a locally-built tag never published
// upstream, and surfaced a transient-looking error for a permanent condition.
func TestClassifyUpstreamFetchError_NotFound(t *testing.T) {
	// Wrapped, as providers return it (e.g. "GET ... returned 404: <ErrNotFound>").
	wrapped := fmt.Errorf("GET https://upstream/v9.9.9/release-index.json returned 404: %w", upstream.ErrNotFound)
	err := classifyUpstreamFetchError(wrapped, "globulario-github", "v9.9.9")
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("ErrNotFound must map to codes.NotFound, got %s: %v", got, err)
	}
}

func TestClassifyUpstreamFetchError_TransientStaysUnavailable(t *testing.T) {
	transient := errors.New("GET https://upstream/v1.0.0/release-index.json: connection refused")
	err := classifyUpstreamFetchError(transient, "globulario-github", "v1.0.0")
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("transient fetch error must map to codes.Unavailable, got %s: %v", got, err)
	}
}

// Guard the exact pairing the bootstrap script greps for: a local-only tag 404
// from the LOCAL_DIR provider path also resolves to NotFound (provider-neutral).
func TestClassifyUpstreamFetchError_LocalDirNotFound(t *testing.T) {
	localMiss := fmt.Errorf("LOCAL_DIR: release-index.json for tag %q not found: %w", "v9.9.9", upstream.ErrNotFound)
	err := classifyUpstreamFetchError(localMiss, "local-bundle", "v9.9.9")
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("LOCAL_DIR absence must map to codes.NotFound, got %s: %v", got, err)
	}
}
