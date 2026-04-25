package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/globulario/services/golang/internal/depcache"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// stubLedger is a minimal manifestLedger whose ListManifests is controlled by
// the test. All other methods are no-ops so that the full interface is satisfied.
type stubLedger struct {
	listFn func(ctx context.Context) ([]manifestRow, error)
}

func (s *stubLedger) ListManifests(ctx context.Context) ([]manifestRow, error) {
	return s.listFn(ctx)
}
func (s *stubLedger) GetManifest(_ context.Context, _ string) (*manifestRow, error) {
	return nil, nil
}
func (s *stubLedger) PutManifest(_ context.Context, _ manifestRow) error        { return nil }
func (s *stubLedger) UpdatePublishState(_ context.Context, _, _ string) error   { return nil }
func (s *stubLedger) DeleteManifest(_ context.Context, _ string) error          { return nil }
func (s *stubLedger) FindByEntrypointChecksum(_ context.Context, _ string) ([]manifestRow, error) {
	return nil, nil
}

// minimalManifestJSON returns just enough proto-JSON for unmarshalManifestWithState
// to produce a valid manifest with the given publishState string.
func minimalManifestJSON(publisherID, name, version, platform string, buildNumber int64, state string) []byte {
	return []byte(`{
		"ref": {
			"publisherId": "` + publisherID + `",
			"name": "` + name + `",
			"version": "` + version + `",
			"platform": "` + platform + `",
			"kind": "SERVICE"
		},
		"buildNumber": ` + fmt.Sprintf("%d", buildNumber) + `,
		"publishState": "` + state + `"
	}`)
}

// newListCache creates a fresh listCache backed by the given ledger.
// Use this in any test that sets srv.scylla after construction (so Init never
// ran and srv.listCache was never set).
func newListCache(ledger manifestLedger) *depcache.Cache[string, []manifestRow] {
	return depcache.New(
		depcache.PolicyRepositoryListView,
		func(ctx context.Context, _ string) ([]manifestRow, error) {
			return ledger.ListManifests(ctx)
		},
		func(rows []manifestRow) []manifestRow {
			out := make([]manifestRow, len(rows))
			copy(out, rows)
			return out
		},
	)
}

// newScyllaServer builds a minimal *server with a mock Scylla ledger and a
// fresh listCache using PolicyRepositoryListView. depHealth is nil so
// requireHealthy() always returns nil.
func newScyllaServer(ledger *stubLedger) *server {
	srv := &server{scylla: ledger}
	srv.listCache = depcache.New(
		depcache.PolicyRepositoryListView,
		func(ctx context.Context, _ string) ([]manifestRow, error) {
			return ledger.listFn(ctx)
		},
		func(rows []manifestRow) []manifestRow {
			out := make([]manifestRow, len(rows))
			copy(out, rows)
			return out
		},
	)
	return srv
}

// TestListArtifactsCacheHit verifies that repeated calls to ListArtifacts
// within the TTL window are served from listCache without hitting Scylla again.
func TestListArtifactsCacheHit(t *testing.T) {
	var calls atomic.Int32
	row := manifestRow{
		ArtifactKey:  "pub%svc%1.0.0%linux%1",
		PublishState: repopb.PublishState_PUBLISHED.String(),
		PublisherID:  "pub",
		Name:         "svc",
		Version:      "1.0.0",
		Platform:     "linux",
		BuildNumber:  1,
		ManifestJSON: minimalManifestJSON("pub", "svc", "1.0.0", "linux", 1, "PUBLISHED"),
	}
	srv := newScyllaServer(&stubLedger{
		listFn: func(_ context.Context) ([]manifestRow, error) {
			calls.Add(1)
			return []manifestRow{row}, nil
		},
	})

	r1, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(r1.GetArtifacts()) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(r1.GetArtifacts()))
	}

	r2, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(r2.GetArtifacts()) != 1 {
		t.Fatalf("expected 1 artifact on second call, got %d", len(r2.GetArtifacts()))
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 Scylla call within TTL, got %d", calls.Load())
	}
}

// TestResolverNoStaleYanked verifies that resolveLatestBuildNumber always sees
// the current Scylla state. When an artifact transitions PUBLISHED → YANKED,
// the very next call must return 0 (no stale PUBLISHED result).
//
// The install path (resolveLatestBuildNumber) calls scylla.ListManifests
// directly — never through listCache — so it cannot serve stale lifecycle state
// even when the display-path cache is still warm.
func TestResolverNoStaleYanked(t *testing.T) {
	const (
		pub      = "globulario"
		name     = "echo"
		version  = "1.0.0"
		platform = "linux"
	)
	key := pub + "%" + name + "%" + version + "%" + platform + "%7"

	var state atomic.Value
	state.Store(repopb.PublishState_PUBLISHED.String())

	srv := newScyllaServer(&stubLedger{
		listFn: func(_ context.Context) ([]manifestRow, error) {
			st := state.Load().(string)
			return []manifestRow{{
				ArtifactKey:  key,
				PublisherID:  pub,
				Name:         name,
				Version:      version,
				Platform:     platform,
				BuildNumber:  7,
				PublishState: st,
				ManifestJSON: minimalManifestJSON(pub, name, version, platform, 7, st),
			}}, nil
		},
	})

	ref := &repopb.ArtifactRef{
		PublisherId: pub,
		Name:        name,
		Version:     version,
		Platform:    platform,
	}

	// Warm the display-path listCache (irrelevant to install path, but ensures
	// listCache holds stale PUBLISHED data when we yank below).
	if _, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{}); err != nil {
		t.Fatalf("warm list: %v", err)
	}

	// Install path: must see PUBLISHED → return build 7.
	if bn := srv.resolveLatestBuildNumber(context.Background(), ref); bn != 7 {
		t.Fatalf("expected build 7 before yank, got %d", bn)
	}

	// Transition to YANKED in Scylla (also invalidate display-path cache as the
	// real write path does via syncStateToScylla).
	state.Store(repopb.PublishState_YANKED.String())
	srv.listCache.InvalidateAll()

	// Install path must return 0 immediately — no stale PUBLISHED allowed.
	if bn := srv.resolveLatestBuildNumber(context.Background(), ref); bn != 0 {
		t.Errorf("expected 0 after yank (install path must not cache), got %d", bn)
	}

	// Display-path listCache was invalidated — must also show the updated state.
	r, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("list after yank: %v", err)
	}
	// YANKED is discovery-hidden for non-admin; no auth context → should be filtered.
	for _, a := range r.GetArtifacts() {
		if a.GetPublishState() == repopb.PublishState_PUBLISHED {
			t.Errorf("stale PUBLISHED artifact visible after yank: %v", a.GetRef())
		}
	}
}
