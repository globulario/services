package main

import (
	"context"
	"testing"
)

// configurableLedger is a stub manifestLedger whose GetManifest and
// GetArtifactState answers are controlled per-key by the test. All other
// methods are no-ops to satisfy the interface.
type configurableLedger struct {
	manifests map[string]*manifestRow
	states    map[string]string
}

func newConfigurableLedger() *configurableLedger {
	return &configurableLedger{
		manifests: map[string]*manifestRow{},
		states:    map[string]string{},
	}
}

func (l *configurableLedger) GetManifest(_ context.Context, key string) (*manifestRow, error) {
	row, ok := l.manifests[key]
	if !ok {
		return nil, nil
	}
	return row, nil
}
func (l *configurableLedger) GetArtifactState(_ context.Context, key string) (string, error) {
	return l.states[key], nil
}
func (l *configurableLedger) ListManifests(_ context.Context) ([]manifestRow, error) { return nil, nil }
func (l *configurableLedger) PutManifest(_ context.Context, _ manifestRow) error     { return nil }
func (l *configurableLedger) UpdatePublishState(_ context.Context, _, _ string) error {
	return nil
}
func (l *configurableLedger) DeleteManifest(_ context.Context, _ string) error { return nil }
func (l *configurableLedger) FindByEntrypointChecksum(_ context.Context, _ string) ([]manifestRow, error) {
	return nil, nil
}
func (l *configurableLedger) UpdateArtifactState(_ context.Context, _ string, _ scyllaArtifactState) error {
	return nil
}

// TestCanSkipDueToExistingState_NullManifestJSONRefusesSkip is the regression
// for the v1.2.113 sync corruption incident. A sync that was interrupted after
// UpdateArtifactState but before PutManifest left "skeleton" rows in the
// ScyllaDB manifests table: artifact_state=PUBLISHED but manifest_json=NULL.
// On retry, sync read artifact_state=PUBLISHED, called canSkipDueToExistingState
// which returned true, and stamped PUBLISHED again — silently keeping the
// row broken. Every downstream consumer (controller release_resolver,
// repair_artifact, explain_artifact) then failed with
// "parse manifest ... from ledger: proto: syntax error (line 1:1): unexpected token".
//
// Fix: canSkipDueToExistingState must verify the manifest_json column is
// non-empty. Skip is only legal when the manifest is actually parseable.
func TestCanSkipDueToExistingState_NullManifestJSONRefusesSkip(t *testing.T) {
	const key = "core@globular.io%workflow%1.2.113%linux_amd64%364"
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*configurableLedger)
		wantSkip  bool
		wantState ArtifactPipelineState
	}{
		{
			name: "skeleton_row_published_state_null_manifest",
			setup: func(l *configurableLedger) {
				// The exact shape of the bug: artifact_state=PUBLISHED but
				// manifest_json is NULL (len=0). Pre-fix code returned skip=true.
				l.states[key] = string(PipelinePublished)
				l.manifests[key] = &manifestRow{ArtifactKey: key, ManifestJSON: nil}
			},
			wantSkip:  false,
			wantState: PipelinePublished,
		},
		{
			name: "skeleton_row_unspecified_state_null_manifest",
			setup: func(l *configurableLedger) {
				// Same shape but artifact_state is empty — pre-fix code also
				// returned skip=true (treating as "legacy row" for backfill).
				l.manifests[key] = &manifestRow{ArtifactKey: key, ManifestJSON: nil}
			},
			wantSkip:  false,
			wantState: PipelineUnspecified,
		},
		{
			name: "healthy_published_with_manifest_allows_skip",
			setup: func(l *configurableLedger) {
				l.states[key] = string(PipelinePublished)
				l.manifests[key] = &manifestRow{
					ArtifactKey:  key,
					ManifestJSON: []byte(`{"ref":{"name":"workflow"}}`),
				}
			},
			wantSkip:  true,
			wantState: PipelinePublished,
		},
		{
			name: "legacy_unspecified_with_manifest_allows_skip",
			setup: func(l *configurableLedger) {
				l.manifests[key] = &manifestRow{
					ArtifactKey:  key,
					ManifestJSON: []byte(`{"ref":{"name":"workflow"}}`),
				}
			},
			wantSkip:  true,
			wantState: PipelineUnspecified,
		},
		{
			name: "row_entirely_missing_does_not_skip",
			setup: func(l *configurableLedger) {
				// GetManifest returns nil,nil — no row at all.
			},
			wantSkip:  false,
			wantState: PipelineUnspecified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := newConfigurableLedger()
			tt.setup(ledger)
			srv := &server{scylla: ledger}

			gotSkip, gotState := srv.canSkipDueToExistingState(ctx, key)
			if gotSkip != tt.wantSkip {
				t.Errorf("canSkipDueToExistingState skip = %v, want %v (state=%s)",
					gotSkip, tt.wantSkip, gotState)
			}
			if gotState != tt.wantState {
				t.Errorf("state = %q, want %q", gotState, tt.wantState)
			}
		})
	}
}

// TestManifestJSONPresent_DirectChecks covers the helper in isolation.
func TestManifestJSONPresent_DirectChecks(t *testing.T) {
	const key = "core@globular.io%test%1.0.0%linux_amd64%1"
	ctx := context.Background()

	t.Run("nil_scylla_assumes_present", func(t *testing.T) {
		srv := &server{}
		if !srv.manifestJSONPresent(ctx, key) {
			t.Errorf("expected true when scylla is nil (legacy fallback)")
		}
	})

	t.Run("row_missing_returns_false", func(t *testing.T) {
		srv := &server{scylla: newConfigurableLedger()}
		if srv.manifestJSONPresent(ctx, key) {
			t.Errorf("expected false when row is missing")
		}
	})

	t.Run("null_manifest_json_returns_false", func(t *testing.T) {
		ledger := newConfigurableLedger()
		ledger.manifests[key] = &manifestRow{ArtifactKey: key, ManifestJSON: nil}
		srv := &server{scylla: ledger}
		if srv.manifestJSONPresent(ctx, key) {
			t.Errorf("expected false when manifest_json is nil")
		}
	})

	t.Run("empty_manifest_json_returns_false", func(t *testing.T) {
		ledger := newConfigurableLedger()
		ledger.manifests[key] = &manifestRow{ArtifactKey: key, ManifestJSON: []byte{}}
		srv := &server{scylla: ledger}
		if srv.manifestJSONPresent(ctx, key) {
			t.Errorf("expected false when manifest_json is empty bytes")
		}
	})

	t.Run("non_empty_manifest_json_returns_true", func(t *testing.T) {
		ledger := newConfigurableLedger()
		ledger.manifests[key] = &manifestRow{
			ArtifactKey:  key,
			ManifestJSON: []byte(`{"ref":{"name":"workflow"}}`),
		}
		srv := &server{scylla: ledger}
		if !srv.manifestJSONPresent(ctx, key) {
			t.Errorf("expected true when manifest_json has content")
		}
	})
}

