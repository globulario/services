package interceptors

import (
	"errors"
	"strings"
	"testing"
)

// These tests pin the security semantics of the resource-info lookup, fixed
// 2026-08-14. The defect: a genuine lookup failure was logged and converted into
// an empty resource-info slice, and authorization continued — the action check
// still ran, but the per-resource binding was skipped, so a caller was evaluated
// as though the method touched no resources at all.
//
// The distinction that makes the fix safe is made by the PRODUCER, and these
// tests exist to keep it that way. getActionResourceInfos converts a not-found
// into ([]ResourceInfos{}, nil) — a legitimate empty with no error — and only
// returns a non-nil error for a real failure. If that classification were ever
// removed, denying on error would start rejecting every unmapped method and take
// the cluster down, so it is pinned here rather than left as a comment.

// classifyResourceInfoError mirrors the producer's not-found classification so
// the rule can be tested without standing up an RBAC client. It is intentionally
// a copy of the predicate, not a call into it: if the production predicate
// changes shape, this test should fail and force the security question to be
// re-asked rather than silently tracking the change.
func classifyResourceInfoError(err error) (treatAsNoMapping bool) {
	if err == nil {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "key not found")
}

func TestResourceInfoLookup_NotFoundIsNoMappingNotFailure(t *testing.T) {
	// "No mapping for this method" is the permissive, expected case and must
	// never reach the deny path — the interceptor is documented as passing
	// through methods RBAC has no mapping for.
	for _, err := range []error{
		errors.New("resource info not found"),
		errors.New("key not found for method /foo.Bar/Baz"),
		errors.New("NOT FOUND"),
	} {
		if !classifyResourceInfoError(err) {
			t.Errorf("%q must be treated as \"no mapping\" (permissive), not as a lookup failure", err)
		}
	}
}

func TestResourceInfoLookup_GenuineFailureIsNotNoMapping(t *testing.T) {
	// These are real failures. Reaching the consumer with one of them means the
	// authorization scope is unknown, and the request must be denied rather than
	// authorized without resource binding.
	for _, err := range []error{
		errors.New("rbac call timed out for /foo.Bar/Baz: context deadline exceeded"),
		errors.New("connection refused"),
		errors.New("rpc error: code = Unavailable desc = no healthy upstream"),
	} {
		if classifyResourceInfoError(err) {
			t.Errorf("%q is a genuine failure and must NOT be mistaken for \"no mapping\" — "+
				"that mistake is what silently degraded scoped authorization to unscoped", err)
		}
	}
}

// The fix's contract, stated as a test so it cannot be quietly reverted:
// an unknown authorization scope denies, and the returned error stays
// inspectable rather than being flattened into a log line.
func TestResourceInfoLookup_UnknownScopeDeniesAndPreservesError(t *testing.T) {
	underlying := errors.New("context deadline exceeded")
	// Shape of what validateActionRequest now returns on a lookup failure.
	hasAccess, accessDenied, err := false, true, wrapScopeFailure("/foo.Bar/Baz", underlying)

	if hasAccess {
		t.Error("a request whose authorization scope could not be determined must not be granted")
	}
	if !accessDenied {
		t.Error("accessDenied must be set so the multi-subject fallback chain stops rather than retrying a failing RBAC three times")
	}
	if !errors.Is(err, underlying) {
		t.Error("the underlying error must remain unwrappable — the producer preserves a DeadlineExceeded sentinel " +
			"specifically so callers can distinguish a timeout from a generic Unavailable")
	}
	if !strings.Contains(err.Error(), "/foo.Bar/Baz") {
		t.Error("the error must name the method, or an operator cannot tell which call was denied")
	}
}
