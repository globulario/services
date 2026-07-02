package main

// dep_health_test.go — T1-T10: capability-aware degraded mode tests.
//
// These tests verify that the repository service correctly gates RPCs by
// capability tier, not by a single all-or-nothing "healthy" flag.
//
// T1:  Scylla down → ServiceMode() = READ_ONLY
// T2:  Scylla down → RequireCapability(Write) returns codes.Unavailable
// T3:  Scylla down → RequireCapability(Read) returns nil
// T4:  Scylla down → DownloadArtifact for local blob must not return codes.Unavailable
// T5:  Scylla down → UploadArtifact returns codes.Unavailable
// T10: GetRepositoryStatus reflects actual watchdog mode
//
// (The repository has no MinIO dependency — packages never live in MinIO —
// so the former MinIO-down tests T6-T9 no longer apply.)

import (
	"bytes"
	"context"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/globulario/services/golang/operational"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/storage_backend"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ── watchdog constructors ─────────────────────────────────────────────────────

// newTestWatchdog builds a watchdog for unit tests with initialized=true,
// bypassing the startup window. Tests that explicitly want pre-init behavior
// can set initialized to a new atomic.Bool{} (false).
func newTestWatchdog(scyllaOK bool) *depHealthWatchdog {
	healthy := &atomic.Bool{}
	healthy.Store(scyllaOK)
	initialized := &atomic.Bool{}
	initialized.Store(true) // tests inject post-check state directly
	// scyllaSub is only used in the background ping loop; it is never called by
	// RequireCapability, ServiceMode, or OperationalStatus.
	return &depHealthWatchdog{
		healthy:     healthy,
		initialized: initialized,
		logger:      slog.Default(),
	}
}

func newWatchdogScyllaDown() *depHealthWatchdog { return newTestWatchdog(false) }
func newWatchdogFull() *depHealthWatchdog       { return newTestWatchdog(true) }

func newTestServerScyllaDown(t *testing.T) *server {
	t.Helper()
	srv := newTestServer(t)
	srv.depHealth = newWatchdogScyllaDown()
	return srv
}

// ── T1 ────────────────────────────────────────────────────────────────────────

func TestT1_ScyllaDown_ServiceModeReadOnly(t *testing.T) {
	w := newWatchdogScyllaDown()
	mode, _ := w.ServiceMode()
	if mode != operational.ModeReadOnly {
		t.Errorf("ServiceMode with Scylla down: got %q, want %q", mode, operational.ModeReadOnly)
	}
}

// ── T2 ────────────────────────────────────────────────────────────────────────

func TestT2_ScyllaDown_RequireWriteBlocked(t *testing.T) {
	w := newWatchdogScyllaDown()
	err := w.RequireCapability(CapRepoWrite)
	if err == nil {
		t.Fatal("RequireCapability(Write) with Scylla down: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unavailable {
		t.Errorf("error code: got %v, want codes.Unavailable", err)
	}
}

// ── T3 ────────────────────────────────────────────────────────────────────────

func TestT3_ScyllaDown_RequireReadAllowed(t *testing.T) {
	w := newWatchdogScyllaDown()
	if err := w.RequireCapability(CapRepoRead); err != nil {
		t.Errorf("RequireCapability(Read) with Scylla down: expected nil, got %v", err)
	}
}

// ── T4 ────────────────────────────────────────────────────────────────────────

func TestT4_ScyllaDown_DownloadArtifactNotBlocked(t *testing.T) {
	srv := newTestServerScyllaDown(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "t4-pkg",
		Version: "1.0.0", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	// Seed manifest + blob via localStorage (no Scylla write path).
	writeManifestLocal(t, srv, ref, 0, "sha256:deadbeef", 11)
	key := artifactKeyWithBuild(ref, 0)
	if err := srv.localStorage.WriteFile(ctx, binaryStorageKey(key), []byte("hello-world"), 0o644); err != nil {
		t.Fatalf("seed blob: %v", err)
	}

	stream := &depTestDownloadStream{ctx: ctx}
	err := srv.DownloadArtifact(&repopb.DownloadArtifactRequest{Ref: ref}, stream)
	if err != nil {
		st, _ := status.FromError(err)
		if st.Code() == codes.Unavailable {
			t.Fatalf("DownloadArtifact with Scylla down returned codes.Unavailable — local reads must never be blocked")
		}
		// Any other error is acceptable in a stripped-down unit test context.
		t.Logf("DownloadArtifact returned non-Unavailable error (acceptable): %v", err)
		return
	}
	if len(stream.chunks) == 0 {
		t.Fatal("expected data chunks from DownloadArtifact, got none")
	}
}

// depTestDownloadStream implements grpc.ServerStreamingServer[DownloadArtifactResponse].
type depTestDownloadStream struct {
	ctx    context.Context
	chunks [][]byte
}

func (s *depTestDownloadStream) Send(r *repopb.DownloadArtifactResponse) error {
	s.chunks = append(s.chunks, r.GetData())
	return nil
}
func (s *depTestDownloadStream) Context() context.Context { return s.ctx }
func (s *depTestDownloadStream) SendMsg(m any) error {
	return s.Send(m.(*repopb.DownloadArtifactResponse))
}
func (s *depTestDownloadStream) RecvMsg(_ any) error            { return nil }
func (s *depTestDownloadStream) SetHeader(_ metadata.MD) error  { return nil }
func (s *depTestDownloadStream) SendHeader(_ metadata.MD) error { return nil }
func (s *depTestDownloadStream) SetTrailer(_ metadata.MD)       {}

// ── T5 ────────────────────────────────────────────────────────────────────────

func TestT5_ScyllaDown_UploadArtifactBlocked(t *testing.T) {
	srv := newTestServerScyllaDown(t)
	ctx := context.Background()

	stream := &depTestUploadStream{
		ctx: ctx,
		req: &repopb.UploadArtifactRequest{
			Ref: &repopb.ArtifactRef{
				PublisherId: "core@globular.io", Name: "t5-pkg",
				Version: "1.0.0", Platform: "linux_amd64",
				Kind: repopb.ArtifactKind_SERVICE,
			},
			Data: []byte("content"),
		},
	}
	err := srv.UploadArtifact(stream)
	if err == nil {
		t.Fatal("UploadArtifact with Scylla down: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Unavailable {
		t.Errorf("UploadArtifact error code: got %v, want codes.Unavailable", err)
	}
}

// depTestUploadStream implements grpc.ClientStreamingServer[UploadArtifactRequest, UploadArtifactResponse].
type depTestUploadStream struct {
	ctx  context.Context
	req  *repopb.UploadArtifactRequest
	sent bool
}

func (s *depTestUploadStream) Recv() (*repopb.UploadArtifactRequest, error) {
	if !s.sent {
		s.sent = true
		return s.req, nil
	}
	return nil, status.Error(codes.Internal, "stub stream end")
}
func (s *depTestUploadStream) SendAndClose(_ *repopb.UploadArtifactResponse) error { return nil }
func (s *depTestUploadStream) Context() context.Context                            { return s.ctx }
func (s *depTestUploadStream) SendMsg(_ any) error                                 { return nil }
func (s *depTestUploadStream) RecvMsg(_ any) error                                 { return nil }
func (s *depTestUploadStream) SetHeader(_ metadata.MD) error                       { return nil }
func (s *depTestUploadStream) SendHeader(_ metadata.MD) error                      { return nil }
func (s *depTestUploadStream) SetTrailer(_ metadata.MD)                            {}

// ── T4b ── stronger: DownloadArtifact returns actual bytes from local CAS ─────
// T4 proves codes.Unavailable is not returned. T4b proves actual bytes flow
// through and GetRepositoryStatus reports READ_ONLY throughout.

func TestT4b_ScyllaDown_DownloadArtifactServesLocalBytes(t *testing.T) {
	srv := newTestServerScyllaDown(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "t4b-pkg",
		Version: "1.0.0", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	const buildNum = int64(1)
	data := []byte("t4b local-only binary payload")
	digest := checksumBytes(data)

	// Seed manifest in localStorage with correct digest.
	writeManifestLocal(t, srv, ref, buildNum, digest, int64(len(data)))

	// Seed verified blob in localStorage.
	key := artifactKeyWithBuild(ref, buildNum)
	if _, err := srv.localStorage.WriteFileAtomic(ctx, binaryStorageKey(key),
		bytes.NewReader(data), digest, int64(len(data))); err != nil {
		t.Fatalf("seed local blob: %v", err)
	}
	// Transition pipeline state to PUBLISHED so the download gate passes.
	_ = srv.transitionArtifactState(ctx, key, PipelinePublished, "seed", "", ArtifactStateFields{
		BlobKey: binaryStorageKey(key), Checksum: digest, SizeBytes: int64(len(data)),
	})

	// GetRepositoryStatus must report READ_ONLY (Scylla down, MinIO healthy).
	statusResp, err := srv.GetRepositoryStatus(ctx, &repopb.GetRepositoryStatusRequest{})
	if err != nil {
		t.Fatalf("GetRepositoryStatus: %v", err)
	}
	if statusResp.GetMode() != string(operational.ModeReadOnly) {
		t.Errorf("mode: got %q, want %q", statusResp.GetMode(), operational.ModeReadOnly)
	}

	// DownloadArtifact must deliver bytes — explicitly provide build_number
	// so resolveLatestBuildNumber (which needs Scylla) is bypassed.
	stream := &depTestDownloadStream{ctx: ctx}
	if err := srv.DownloadArtifact(&repopb.DownloadArtifactRequest{
		Ref: ref, BuildNumber: buildNum,
	}, stream); err != nil {
		t.Fatalf("DownloadArtifact with Scylla down: %v", err)
	}
	var got []byte
	for _, chunk := range stream.chunks {
		got = append(got, chunk...)
	}
	if string(got) != string(data) {
		t.Errorf("payload mismatch: got %q, want %q", got, data)
	}
}

// ── T10 ───────────────────────────────────────────────────────────────────────

func TestT10_GetRepositoryStatus_ReflectsActualMode(t *testing.T) {
	cases := []struct {
		name     string
		watchdog *depHealthWatchdog
		wantMode string
	}{
		{"full", newWatchdogFull(), string(operational.ModeFull)},
		{"scylla-down", newWatchdogScyllaDown(), string(operational.ModeReadOnly)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			local := storage_backend.NewOSStorage(dir)
			srv := &server{Root: dir}
			srv.storage = local
			srv.localStorage = local
			srv.depHealth = tc.watchdog
			srv.ensureSignaturePolicy().SetPolicyForTest(&repopb.SignaturePolicy{
				AllowUnsignedLocalDevelopment: true,
				TrustedCorePublishers:         []string{"core@globular.io"},
			})

			resp, err := srv.GetRepositoryStatus(context.Background(), &repopb.GetRepositoryStatusRequest{})
			if err != nil {
				t.Fatalf("GetRepositoryStatus: %v", err)
			}
			if resp.GetMode() != tc.wantMode {
				t.Errorf("mode: got %q, want %q", resp.GetMode(), tc.wantMode)
			}
			if resp.GetObservedAtUnix() == 0 {
				t.Error("observed_at_unix must be non-zero")
			}
			if len(resp.GetDependencies()) == 0 {
				t.Error("expected at least one dependency in response")
			}
			if len(resp.GetCapabilities()) == 0 {
				t.Error("expected at least one capability in response")
			}
		})
	}
}

// ── initialization guard tests ────────────────────────────────────────────────

func TestGetRepositoryStatus_NilWatchdog_ReturnsDegraded(t *testing.T) {
	dir := t.TempDir()
	local := storage_backend.NewOSStorage(dir)
	srv := &server{Root: dir}
	srv.storage = local
	srv.localStorage = local
	// depHealth intentionally nil — simulates service before watchdog starts.
	srv.ensureSignaturePolicy().SetPolicyForTest(&repopb.SignaturePolicy{
		AllowUnsignedLocalDevelopment: true,
		TrustedCorePublishers:         []string{"core@globular.io"},
	})

	resp, err := srv.GetRepositoryStatus(context.Background(), &repopb.GetRepositoryStatusRequest{})
	if err != nil {
		t.Fatalf("GetRepositoryStatus: %v", err)
	}
	if resp.GetMode() != string(operational.ModeDegraded) {
		t.Errorf("nil watchdog mode: got %q, want %q", resp.GetMode(), operational.ModeDegraded)
	}
	if resp.GetReason() != "dependency_watchdog_not_initialized" {
		t.Errorf("nil watchdog reason: got %q, want dependency_watchdog_not_initialized", resp.GetReason())
	}
	for _, c := range resp.GetCapabilities() {
		if c.GetStatus() != string(operational.CapUnknown) {
			t.Errorf("nil watchdog cap %q: got status %q, want %q", c.GetName(), c.GetStatus(), operational.CapUnknown)
		}
	}
}

func TestServiceMode_PreInit_ReturnsDegraded(t *testing.T) {
	// A watchdog where initialized=false (startup window) must report DEGRADED
	// regardless of the healthy bit.
	healthy := &atomic.Bool{}
	healthy.Store(true)
	initialized := &atomic.Bool{} // false — not yet set
	w := &depHealthWatchdog{
		healthy:     healthy,
		initialized: initialized,
		logger:      slog.Default(),
	}
	mode, reason := w.ServiceMode()
	if mode != operational.ModeDegraded {
		t.Errorf("pre-init ServiceMode: got %q, want %q", mode, operational.ModeDegraded)
	}
	if reason == "" {
		t.Error("pre-init reason must be non-empty")
	}
}
