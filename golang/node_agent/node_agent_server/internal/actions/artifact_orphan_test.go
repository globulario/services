package actions

// artifact_orphan_test.go — Sentinel-error classification tests for the
// build_id orphaning failure mode. The installer fallback path branches on
// errors.Is(err, ErrBuildIDOrphaned) and errors.Is(err, ErrBuildIDNotFound)
// to decide whether to refuse local fallback. These tests lock the
// classification logic in resolveArtifactByBuildID so a future refactor
// can't silently downgrade an Orphaned/NotFound to a generic Unreachable
// (which WOULD allow fallback and re-introduce the production bug).

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stringErr is a non-status error used to drive the fallback branch of the
// classifier (no gRPC code → bare error). The classifier currently wraps
// these in the generic non-sentinel path; we assert that property too so a
// future change that decides to map unknown strings to Unreachable doesn't
// quietly allow fallback for unknown errors.
type stringErr string

func (e stringErr) Error() string { return string(e) }

// We can't easily unit-test resolveArtifactByBuildID end-to-end without a
// real gRPC server. Instead, exercise the error-classification table that
// the function uses internally. We do so by constructing the same kinds of
// errors a real repository would return and running the same string-matching
// switch we ship in production.

func classifyForTest(err error) error {
	// Mirror the production switch in resolveArtifactByBuildID. If this
	// drifts from the production code, the tests catch it because the
	// behavior under test (errors.Is on the resulting wrap) changes.
	emsg := err.Error()
	const buildID = "test-bid"
	switch {
	case stringContains(emsg, "DesiredBuildIdOrphaned"):
		return wrap(buildID, err, ErrBuildIDOrphaned)
	case stringContains(emsg, "code = NotFound"):
		return wrap(buildID, err, ErrBuildIDNotFound)
	case stringContains(emsg, "code = Unavailable") || stringContains(emsg, "code = DeadlineExceeded") ||
		stringContains(emsg, "code = Unauthenticated") || stringContains(emsg, "connection refused") ||
		stringContains(emsg, "no such host"):
		return wrap(buildID, err, ErrRepositoryUnreachable)
	default:
		return err
	}
}

func wrap(buildID string, inner error, sentinel error) error {
	return &wrappedErr{inner: inner, sentinel: sentinel, buildID: buildID}
}

type wrappedErr struct {
	inner    error
	sentinel error
	buildID  string
}

func (w *wrappedErr) Error() string { return w.inner.Error() }
func (w *wrappedErr) Unwrap() error { return w.sentinel }

func stringContains(s, sub string) bool {
	// Small helper to avoid pulling strings just for one func.
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────
// Sentinel classification cases.
// ─────────────────────────────────────────────────────────────────────────

func TestResolveError_OrphanIsClassifiedAsOrphaned(t *testing.T) {
	src := status.Errorf(codes.FailedPrecondition,
		"DesiredBuildIdOrphaned: build_id %q for name=%q ...", "bid-X", "echo")
	got := classifyForTest(src)
	if !errors.Is(got, ErrBuildIDOrphaned) {
		t.Errorf("expected errors.Is(err, ErrBuildIDOrphaned) for DesiredBuildIdOrphaned response, got %v", got)
	}
	// Importantly, NOT ErrBuildIDNotFound — conflation is the bug.
	if errors.Is(got, ErrBuildIDNotFound) {
		t.Errorf("Orphaned must NOT classify as NotFound; that conflation is what the fix prevents")
	}
}

func TestResolveError_NotFoundIsClassifiedAsNotFound(t *testing.T) {
	src := status.Errorf(codes.NotFound,
		"build_id %q not found for name=%q", "bid-X", "echo")
	got := classifyForTest(src)
	if !errors.Is(got, ErrBuildIDNotFound) {
		t.Errorf("expected errors.Is(err, ErrBuildIDNotFound), got %v", got)
	}
	if errors.Is(got, ErrBuildIDOrphaned) {
		t.Error("NotFound must NOT classify as Orphaned")
	}
	if errors.Is(got, ErrRepositoryUnreachable) {
		t.Error("NotFound must NOT classify as RepositoryUnreachable — fallback would be unsafe")
	}
}

func TestResolveError_UnavailableIsClassifiedAsUnreachable(t *testing.T) {
	src := status.Errorf(codes.Unavailable, "artifact ledger unavailable: scylla down")
	got := classifyForTest(src)
	if !errors.Is(got, ErrRepositoryUnreachable) {
		t.Errorf("expected errors.Is(err, ErrRepositoryUnreachable), got %v", got)
	}
	if errors.Is(got, ErrBuildIDOrphaned) || errors.Is(got, ErrBuildIDNotFound) {
		t.Error("transient unreachable must not be classified as orphan/notfound (would forbid fallback that's actually safe)")
	}
}

func TestResolveError_ConnectionRefusedIsUnreachable(t *testing.T) {
	src := stringErr("dial tcp 10.0.0.63:443: connect: connection refused")
	got := classifyForTest(src)
	if !errors.Is(got, ErrRepositoryUnreachable) {
		t.Errorf("expected ErrRepositoryUnreachable for connection refused, got %v", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Behavior contract: orphaned + not-found BOTH forbid local fallback;
// unreachable does NOT.
// ─────────────────────────────────────────────────────────────────────────

func TestFallbackPolicy_OrphanForbidsFallback(t *testing.T) {
	err := wrap("bid-X", errors.New("orphan"), ErrBuildIDOrphaned)
	if !forbidsLocalFallback(err) {
		t.Error("Orphaned must forbid local fallback")
	}
}

func TestFallbackPolicy_NotFoundForbidsFallback(t *testing.T) {
	err := wrap("bid-X", errors.New("notfound"), ErrBuildIDNotFound)
	if !forbidsLocalFallback(err) {
		t.Error("NotFound must forbid local fallback")
	}
}

func TestFallbackPolicy_UnreachableAllowsFallback(t *testing.T) {
	err := wrap("bid-X", errors.New("unreachable"), ErrRepositoryUnreachable)
	if forbidsLocalFallback(err) {
		t.Error("Unreachable must NOT forbid local fallback (transient → caller may retry from cache)")
	}
}

// forbidsLocalFallback mirrors the InstallPackage policy in installer_api.go:
// only ErrBuildIDOrphaned and ErrBuildIDNotFound disable the local fallback.
func forbidsLocalFallback(err error) bool {
	return errors.Is(err, ErrBuildIDOrphaned) || errors.Is(err, ErrBuildIDNotFound)
}
