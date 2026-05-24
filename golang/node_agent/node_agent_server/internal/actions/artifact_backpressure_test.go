package actions

import (
	"fmt"
	"testing"
)

// TestIsRepositoryBackpressure verifies that ResourceExhausted and server-overload
// errors are classified as transient backpressure, while permanent artifact
// identity errors are not.
func TestIsRepositoryBackpressure(t *testing.T) {
	transient := []struct {
		name string
		err  error
	}{
		{"ResourceExhausted gRPC code", fmt.Errorf("rpc error: code = ResourceExhausted desc = server overloaded: too many concurrent requests")},
		{"server overloaded string", fmt.Errorf("server overloaded: 42 pending")},
		{"too many concurrent requests", fmt.Errorf("too many concurrent requests")},
		{"repository overloaded", fmt.Errorf("repository overloaded")},
		{"repository backpressure", fmt.Errorf("repository backpressure while resolving artifact")},
		{"mixed case resourceexhausted", fmt.Errorf("ResourceExhausted: quota exceeded")},
		{"resource exhausted spaced", fmt.Errorf("resource exhausted for service dns")},
	}
	for _, tc := range transient {
		t.Run(tc.name, func(t *testing.T) {
			if !isRepositoryBackpressure(tc.err) {
				t.Errorf("expected backpressure=true for %q", tc.err)
			}
		})
	}

	permanent := []struct {
		name string
		err  error
	}{
		{"checksum mismatch", fmt.Errorf("checksum mismatch: expected abc got def")},
		{"DesiredBuildIdOrphaned", fmt.Errorf("DesiredBuildIdOrphaned: build was archived")},
		{"NotFound", fmt.Errorf("code = NotFound desc = artifact not in catalog")},
		{"manifest nil", fmt.Errorf("no manifest returned for build_id abc123")},
		{"connection refused", fmt.Errorf("dial tcp 10.0.0.1:443: connect: connection refused")},
		{"nil", nil},
	}
	for _, tc := range permanent {
		t.Run(tc.name, func(t *testing.T) {
			if isRepositoryBackpressure(tc.err) {
				t.Errorf("expected backpressure=false for %v", tc.err)
			}
		})
	}
}

// TestIsRepositoryBackpressure_WrappedError verifies the classifier works
// when errors are wrapped via fmt.Errorf("%w").
func TestIsRepositoryBackpressure_WrappedError(t *testing.T) {
	inner := fmt.Errorf("code = ResourceExhausted desc = server overloaded: too many concurrent requests")
	wrapped := fmt.Errorf("resolve artifact by build_id abc: %w", inner)
	if !isRepositoryBackpressure(wrapped) {
		t.Errorf("expected backpressure=true for wrapped error %q", wrapped)
	}

	permanentInner := fmt.Errorf("checksum mismatch")
	permanentWrapped := fmt.Errorf("verify artifact: %w", permanentInner)
	if isRepositoryBackpressure(permanentWrapped) {
		t.Errorf("expected backpressure=false for wrapped permanent error %q", permanentWrapped)
	}
}
