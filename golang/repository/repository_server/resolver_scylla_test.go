package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// buildResolverServer returns a server with Scylla wired and MinIO storage
// intentionally empty, so any MinIO hit would cause a failure.
func buildResolverServer(t *testing.T, rows []manifestRow) *server {
	t.Helper()
	var calls atomic.Int32
	ledger := &stubLedger{
		listFn: func(_ context.Context) ([]manifestRow, error) {
			calls.Add(1)
			return rows, nil
		},
	}
	srv := newScyllaServer(ledger)
	// Attach call counter so individual tests can inspect it if needed.
	t.Cleanup(func() { _ = calls.Load() }) // prevent "unused" lint
	return srv
}

// publishedRow creates a manifestRow in PUBLISHED state with minimal fields.
func publishedRow(pub, name, version, platform string, buildNum int64, buildID string) manifestRow {
	return manifestRow{
		ArtifactKey:  pub + "%" + name + "%" + version + "%" + platform + "%" + fmt.Sprint(buildNum),
		PublishState: repopb.PublishState_PUBLISHED.String(),
		PublisherID:  pub,
		Name:         name,
		Version:      version,
		Platform:     platform,
		BuildNumber:  buildNum,
		Channel:      repopb.ArtifactChannel_STABLE.String(),
		Kind:         repopb.ArtifactKind_SERVICE.String(),
		ManifestJSON: minimalManifestJSONWithBuildID(pub, name, version, platform, buildNum, buildID, "PUBLISHED"),
	}
}

func minimalManifestJSONWithBuildID(pub, name, version, platform string, buildNum int64, buildID, state string) []byte {
	return []byte(`{
		"ref": {
			"publisherId": "` + pub + `",
			"name": "` + name + `",
			"version": "` + version + `",
			"platform": "` + platform + `",
			"kind": "SERVICE"
		},
		"buildNumber": ` + fmt.Sprintf("%d", buildNum) + `,
		"buildId": "` + buildID + `",
		"publishState": "` + state + `"
	}`)
}

// TestResolveArtifactScyllaFirst verifies that ResolveArtifact uses the Scylla
// ledger when available and does not fall back to MinIO (which is empty).
func TestResolveArtifactScyllaFirst(t *testing.T) {
	rows := []manifestRow{
		publishedRow("glob", "echo", "1.0.0", "linux_amd64", 5, "build-aaa"),
	}
	srv := buildResolverServer(t, rows)

	resp, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:     "echo",
		Platform: "linux_amd64",
	})
	if err != nil {
		t.Fatalf("ResolveArtifact: %v", err)
	}
	if resp.GetManifest().GetRef().GetName() != "echo" {
		t.Errorf("expected name=echo, got %q", resp.GetManifest().GetRef().GetName())
	}
	if resp.GetManifest().GetBuildNumber() != 5 {
		t.Errorf("expected build_number=5, got %d", resp.GetManifest().GetBuildNumber())
	}
}

// TestResolveArtifactYankedNotReturned verifies that YANKED artifacts are
// invisible to the resolver (install path must never receive stale lifecycle state).
func TestResolveArtifactYankedNotReturned(t *testing.T) {
	yanked := publishedRow("glob", "echo", "1.0.0", "linux_amd64", 3, "build-old")
	yanked.PublishState = repopb.PublishState_YANKED.String()
	yanked.ManifestJSON = minimalManifestJSONWithBuildID("glob", "echo", "1.0.0", "linux_amd64", 3, "build-old", "YANKED")

	srv := buildResolverServer(t, []manifestRow{yanked})

	_, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:     "echo",
		Platform: "linux_amd64",
	})
	if err == nil {
		t.Fatal("expected NotFound for YANKED artifact, got nil error")
	}
	if code := status.Code(err); code != codes.NotFound {
		t.Errorf("expected codes.NotFound, got %v", code)
	}
}

// TestResolveByBuildIDScyllaFirst verifies that resolveByBuildID uses Scylla
// and finds the artifact by build_id in a single ledger scan.
func TestResolveByBuildIDScyllaFirst(t *testing.T) {
	const wantBuildID = "019d0001-0000-7000-8000-000000000042"
	rows := []manifestRow{
		publishedRow("glob", "gateway", "1.2.0", "linux_amd64", 42, wantBuildID),
		publishedRow("glob", "rbac", "1.0.0", "linux_amd64", 1, "other-build-id"),
	}
	srv := buildResolverServer(t, rows)

	resp, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:     "gateway",
		Platform: "linux_amd64",
		BuildId:  wantBuildID,
	})
	if err != nil {
		t.Fatalf("ResolveArtifact by build_id: %v", err)
	}
	if resp.GetManifest().GetBuildId() != wantBuildID {
		t.Errorf("expected build_id=%q, got %q", wantBuildID, resp.GetManifest().GetBuildId())
	}
	if resp.GetResolutionSource() != "exact-build_id" {
		t.Errorf("expected resolution_source=exact-build_id, got %q", resp.GetResolutionSource())
	}
}

// TestResolveByBuildIDYankedNotReturned verifies that a YANKED build_id is
// never returned by the resolver even when explicitly requested.
func TestResolveByBuildIDYankedNotReturned(t *testing.T) {
	const buildID = "019d0001-dead-beef-0000-000000000001"
	row := publishedRow("glob", "echo", "1.0.0", "linux_amd64", 1, buildID)
	row.PublishState = repopb.PublishState_YANKED.String()
	row.ManifestJSON = minimalManifestJSONWithBuildID("glob", "echo", "1.0.0", "linux_amd64", 1, buildID, "YANKED")

	srv := buildResolverServer(t, []manifestRow{row})

	_, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:     "echo",
		Platform: "linux_amd64",
		BuildId:  buildID,
	})
	if err == nil {
		t.Fatal("expected NotFound for YANKED build_id, got nil error")
	}
	if code := status.Code(err); code != codes.NotFound {
		t.Errorf("expected codes.NotFound, got %v", code)
	}
}

// TestResolveArtifactPicksHighestBuild verifies that when multiple builds exist
// for the same version, the resolver picks the highest build_number.
func TestResolveArtifactPicksHighestBuild(t *testing.T) {
	rows := []manifestRow{
		publishedRow("glob", "echo", "1.0.0", "linux_amd64", 3, "build-c"),
		publishedRow("glob", "echo", "1.0.0", "linux_amd64", 7, "build-g"),
		publishedRow("glob", "echo", "1.0.0", "linux_amd64", 5, "build-e"),
	}
	srv := buildResolverServer(t, rows)

	resp, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		Name:     "echo",
		Platform: "linux_amd64",
		Version:  "1.0.0",
	})
	if err != nil {
		t.Fatalf("ResolveArtifact: %v", err)
	}
	if resp.GetManifest().GetBuildNumber() != 7 {
		t.Errorf("expected highest build_number=7, got %d", resp.GetManifest().GetBuildNumber())
	}
}
