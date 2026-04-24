package main

// artifact_handlers.go — repository RPC handlers for artifact management.
//
// Implements: ListArtifacts, GetArtifactManifest, UploadArtifact, DownloadArtifact,
// SearchArtifacts, GetArtifactVersions, DeleteArtifact.
//
// Storage layout (relative to the configured backend root):
//
//	artifacts/{publisherID}%{name}%{version}%{platform}%{buildNumber}.manifest.json
//	artifacts/{publisherID}%{name}%{version}%{platform}%{buildNumber}.bin
//
// Legacy artifacts (build_number=0) may still exist under the old 4-field key
// format without the trailing %0 segment. The read path tries both.

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gocql/gocql"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/versionutil"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// syncManifestToScylla writes manifest metadata to ScyllaDB for distributed
// consistency. Best-effort: failure is logged but does not block the MinIO write.
//
// State authority rule:
//   - manifest_json column: stores the manifest at upload time (state = VERIFIED).
//     It is immutable historical metadata. Do NOT read it for current lifecycle state.
//   - publish_state column: the SOLE authority for current lifecycle state.
//     Updated by syncStateToScylla / UpdatePublishState on every transition.
//   - Readers MUST use publish_state column. Never fall back to manifest_json.state.
func (srv *server) syncManifestToScylla(ctx context.Context, key string, manifest *repopb.ArtifactManifest, state repopb.PublishState, mjson []byte) {
	if srv.scylla == nil {
		return
	}
	ref := manifest.GetRef()
	row := manifestRow{
		ArtifactKey:        key,
		ManifestJSON:       mjson,
		PublishState:       state.String(),
		PublisherID:        ref.GetPublisherId(),
		Name:               ref.GetName(),
		Version:            ref.GetVersion(),
		Platform:           ref.GetPlatform(),
		BuildNumber:        manifest.GetBuildNumber(),
		Checksum:           manifest.GetChecksum(),
		EntrypointChecksum: manifest.GetEntrypointChecksum(),
		SizeBytes:          manifest.GetSizeBytes(),
		ModifiedUnix:       manifest.GetModifiedUnix(),
		Kind:               ref.GetKind().String(),
		Channel:            effectiveChannel(manifest).String(),
		CreatedAt:          time.Now(),
	}
	if err := srv.scylla.PutManifest(ctx, row); err != nil {
		slog.Warn("scylladb manifest sync failed (non-fatal)", "key", key, "err", err)
	}
}

// syncStateToScylla updates only the publish state in ScyllaDB.
// syncStateToScylla updates the publish_state column in ScyllaDB — the SOLE
// authoritative source for current artifact lifecycle state. This MUST be called
// on every state transition. Readers use this column and must not fall back to
// manifest_json for lifecycle decisions.
func (srv *server) syncStateToScylla(ctx context.Context, key string, state repopb.PublishState) {
	if srv.scylla == nil {
		return
	}
	if err := srv.scylla.UpdatePublishState(ctx, key, state.String()); err != nil {
		slog.Warn("scylladb state sync failed (non-fatal)", "key", key, "state", state, "err", err)
	}
}

// deleteManifestFromScylla removes a manifest from ScyllaDB.
func (srv *server) deleteManifestFromScylla(ctx context.Context, key string) {
	if srv.scylla == nil {
		return
	}
	if err := srv.scylla.DeleteManifest(ctx, key); err != nil {
		slog.Warn("scylladb manifest delete failed (non-fatal)", "key", key, "err", err)
	}
}

const artifactsDir = "artifacts"

// channelFromString parses a channel name from package.json into the proto enum.
// Unrecognised strings default to STABLE so old packages are treated correctly.
func channelFromString(s string) repopb.ArtifactChannel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stable", "":
		return repopb.ArtifactChannel_STABLE
	case "candidate":
		return repopb.ArtifactChannel_CANDIDATE
	case "canary":
		return repopb.ArtifactChannel_CANARY
	case "dev":
		return repopb.ArtifactChannel_DEV
	case "bootstrap":
		return repopb.ArtifactChannel_BOOTSTRAP
	default:
		return repopb.ArtifactChannel_STABLE
	}
}

// effectiveChannel returns the manifest's channel, treating CHANNEL_UNSET as STABLE.
func effectiveChannel(m *repopb.ArtifactManifest) repopb.ArtifactChannel {
	if ch := m.GetChannel(); ch != repopb.ArtifactChannel_CHANNEL_UNSET {
		return ch
	}
	return repopb.ArtifactChannel_STABLE
}

// isReconcilerSafeChannel returns true if the channel may be returned to the
// cluster reconciler without explicit override. DEV and CANDIDATE are not safe.
func isReconcilerSafeChannel(ch repopb.ArtifactChannel) bool {
	switch ch {
	case repopb.ArtifactChannel_STABLE,
		repopb.ArtifactChannel_BOOTSTRAP,
		repopb.ArtifactChannel_CHANNEL_UNSET:
		return true
	default:
		return false
	}
}

// ── version helpers ───────────────────────────────────────────────────────────

// canonicalizeRefVersion normalizes ref.Version in-place using versionutil.Canonical.
// If the version is empty or not valid semver, it is left unchanged.
func canonicalizeRefVersion(ref *repopb.ArtifactRef) {
	if ref == nil {
		return
	}
	if cv, err := versionutil.Canonical(ref.GetVersion()); err == nil {
		ref.Version = cv
	}
}

// ── storage key helpers ───────────────────────────────────────────────────────

// artifactKeyWithBuild returns a flat, filesystem-safe key including build_number.
// Format: {publisherID}%{name}%{version}%{platform}%{buildNumber}
func artifactKeyWithBuild(ref *repopb.ArtifactRef, buildNumber int64) string {
	return ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform() + "%" + strconv.FormatInt(buildNumber, 10)
}

// artifactKeyLegacy returns the old 4-field key without build_number.
// Used for backward-compat reads of pre-build-number artifacts.
func artifactKeyLegacy(ref *repopb.ArtifactRef) string {
	return ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform()
}

func manifestStorageKey(key string) string { return artifactsDir + "/" + key + ".manifest.json" }
func binaryStorageKey(key string) string   { return artifactsDir + "/" + key + ".bin" }
func publishStateKey(key string) string    { return artifactsDir + "/" + key + ".publish_state" }

// ── publish state helpers ─────────────────────────────────────────────────

// marshalManifestWithState marshals the manifest via protojson and injects the
// publish_state field. This is needed until ./generateCode.sh regenerates the
// pb.go with the publish_state field natively in the proto descriptor.
func marshalManifestWithState(m *repopb.ArtifactManifest, state repopb.PublishState) ([]byte, error) {
	mjson, err := protojson.Marshal(m)
	if err != nil {
		return nil, err
	}
	if state == repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
		return mjson, nil
	}
	// Inject publish_state into the JSON object.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(mjson, &raw); err != nil {
		return nil, err
	}
	stateJSON, _ := json.Marshal(state.String())
	raw["publishState"] = stateJSON
	return json.Marshal(raw)
}

// unmarshalManifestWithState reads a manifest JSON and extracts the publish_state.
// protojson.Unmarshal ignores the publishState key (not in descriptor), so we
// extract it separately.
func unmarshalManifestWithState(data []byte) (*repopb.ArtifactManifest, repopb.PublishState, error) {
	m := &repopb.ArtifactManifest{}
	uopts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := uopts.Unmarshal(data, m); err != nil {
		return nil, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, err
	}
	// Extract publishState from JSON.
	var raw map[string]json.RawMessage
	state := repopb.PublishState_PUBLISH_STATE_UNSPECIFIED
	if err := json.Unmarshal(data, &raw); err == nil {
		if stateJSON, ok := raw["publishState"]; ok {
			var stateStr string
			if err := json.Unmarshal(stateJSON, &stateStr); err == nil {
				if v, ok := repopb.PublishState_value[stateStr]; ok {
					state = repopb.PublishState(v)
				}
			}
		}
	}
	return m, state, nil
}

// isTerminalState returns true if the artifact has reached a state where
// its content is sealed and must not be overwritten. This includes PUBLISHED
// and all post-PUBLISHED lifecycle states where the artifact was successfully
// delivered. FAILED and ORPHANED are NOT terminal — they represent broken
// publish attempts where overwrite is allowed for recovery.
func isTerminalState(s repopb.PublishState) bool {
	switch s {
	case repopb.PublishState_PUBLISHED,
		repopb.PublishState_DEPRECATED,
		repopb.PublishState_YANKED,
		repopb.PublishState_QUARANTINED,
		repopb.PublishState_REVOKED:
		return true
	default:
		return false
	}
}

// ── manifest helpers ──────────────────────────────────────────────────────

// readManifestByKey reads and unmarshals a single manifest JSON from storage.
func (srv *server) readManifestByKey(ctx context.Context, key string) (*repopb.ArtifactManifest, error) {
	_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	_ = state // callers that don't need state use this method
	return m, nil
}

// readManifestAndStateByKey reads a manifest and its publish state from storage.
// It also corrects the kind for manifests published before proper kind assignment.
// Results are cached in-memory to reduce storage backend reads from the
// reconcile loop (~6 req/s across the cluster).
//
// Phase 7 — Ledger-first read rule:
// When ScyllaDB is available it is the authoritative ledger for artifact existence.
//
//   - Scylla miss  → codes.NotFound   (artifact absent from ledger — never published or GC'd)
//   - Scylla hit   → manifest served from Scylla JSON (no MinIO round-trip for metadata)
//   - Scylla down  → degraded fallback to MinIO-direct read (logged as WARN)
//
// This distinction matters: MinIO being unreachable does NOT mean an artifact is
// absent. Only a Scylla miss is authoritative NotFound. MinIO failure surfaces as
// codes.Unavailable at the binary-download layer, not codes.NotFound at manifest lookup.
func (srv *server) readManifestAndStateByKey(ctx context.Context, key string) (string, repopb.PublishState, *repopb.ArtifactManifest, error) {
	sKey := manifestStorageKey(key)

	// Check cache first.
	if srv.cache != nil {
		if cKey, cState, cManifest, ok := srv.cache.getManifest(sKey); ok {
			return cKey, cState, cManifest, nil
		}
	}

	// Phase 7: Ledger-first read. Use Scylla as authoritative when available.
	if srv.scylla != nil {
		row, scyllaErr := srv.scylla.GetManifest(ctx, key)
		switch {
		case scyllaErr == nil:
			// Scylla hit — parse manifest from ledger JSON.
			m, state, err := unmarshalManifestWithState(row.ManifestJSON)
			if err != nil {
				return key, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil,
					fmt.Errorf("parse manifest %q from ledger: %w", key, err)
			}
			// publish_state column is the SOLE authority for current lifecycle state.
			// UpdatePublishState() updates only the column without rewriting manifest_json,
			// so the column may be ahead of (or differ from) the JSON-embedded state.
			// manifest_json is immutable historical metadata; never trust its embedded
			// state for current lifecycle decisions.
			if row.PublishState != "" {
				if v, ok := repopb.PublishState_value[row.PublishState]; ok {
					state = repopb.PublishState(v)
				}
			}
			// Stamp authoritative state onto manifest proto so callers that read
			// m.GetPublishState() (e.g. the controller release resolver) see the
			// correct current state without re-querying.
			m.PublishState = state
			if ref := m.GetRef(); ref != nil {
				if corrected := inferCorrectKind(ref.GetName(), ref.GetKind()); corrected != ref.GetKind() {
					ref.Kind = corrected
				}
			}
			if srv.cache != nil {
				srv.cache.putManifest(sKey, key, state, m)
			}
			return key, state, m, nil

		case errors.Is(scyllaErr, gocql.ErrNotFound):
			// Authoritative miss — artifact is not in the ledger.
			return key, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil,
				status.Errorf(codes.NotFound, "artifact %q not found", key)

		default:
			// Scylla temporarily unavailable — fall through to MinIO as degraded path.
			// Log at WARN so operators know the service is running in degraded mode.
			slog.Warn("ledger read failed, falling back to minio (degraded mode)",
				"key", key, "err", scyllaErr)
		}
	}

	// Fallback: MinIO-direct read (single-node / Scylla temporarily down).
	data, err := srv.Storage().ReadFile(ctx, sKey)
	if err != nil {
		return key, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil, err
	}
	m, state, err := unmarshalManifestWithState(data)
	if err != nil {
		return key, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, nil, fmt.Errorf("parse manifest %q: %w", key, err)
	}
	// MinIO fallback path: state comes from manifest_json (no Scylla available).
	// Stamp it onto the manifest proto so callers see the correct state via m.GetPublishState().
	m.PublishState = state
	// Correct kind for legacy manifests (published before COMMAND/INFRASTRUCTURE).
	if ref := m.GetRef(); ref != nil {
		if corrected := inferCorrectKind(ref.GetName(), ref.GetKind()); corrected != ref.GetKind() {
			ref.Kind = corrected
		}
	}

	// Populate cache.
	if srv.cache != nil {
		srv.cache.putManifest(sKey, key, state, m)
	}

	return key, state, m, nil
}

// readManifestWithFallback tries to find the best manifest for the given ref.
// When buildNumber=0 (unspecified), it resolves to the latest published build
// first, then falls back to the literal %0 key and legacy 4-field key.
// Non-zero build numbers must match exactly — no silent collapse.
func (srv *server) readManifestWithFallback(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) (*repopb.ArtifactManifest, error) {
	if buildNumber == 0 {
		// Resolve to the latest (highest) PUBLISHED build number first.
		if latest := srv.resolveLatestBuildNumber(ctx, ref); latest > 0 {
			latestKey := artifactKeyWithBuild(ref, latest)
			if lm, lerr := srv.readManifestByKey(ctx, latestKey); lerr == nil {
				return lm, nil
			}
		}
		// Fall back to literal %0 key (artifact actually uploaded as build 0).
		key := artifactKeyWithBuild(ref, 0)
		if m, err := srv.readManifestByKey(ctx, key); err == nil {
			return m, nil
		}
		// Legacy fallback for pre-build-number artifacts (4-field key).
		legacyKey := artifactKeyLegacy(ref)
		if lm, lerr := srv.readManifestByKey(ctx, legacyKey); lerr == nil {
			return lm, nil
		}
		return nil, fmt.Errorf("artifact %s not found (tried latest, build-0, legacy)", artifactKeyWithBuild(ref, 0))
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	m, err := srv.readManifestByKey(ctx, key)
	return m, err
}

// readBinaryWithFallback resolves the binary for the given ref.
// When buildNumber=0, resolves latest published build first, then literal %0, then legacy key.
func (srv *server) readBinaryWithFallback(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) ([]byte, error) {
	if buildNumber == 0 {
		if latest := srv.resolveLatestBuildNumber(ctx, ref); latest > 0 {
			latestKey := artifactKeyWithBuild(ref, latest)
			if ld, lerr := srv.Storage().ReadFile(ctx, binaryStorageKey(latestKey)); lerr == nil {
				return ld, nil
			}
		}
		key := artifactKeyWithBuild(ref, 0)
		if data, err := srv.Storage().ReadFile(ctx, binaryStorageKey(key)); err == nil {
			return data, nil
		}
		legacyKey := artifactKeyLegacy(ref)
		if ld, lerr := srv.Storage().ReadFile(ctx, binaryStorageKey(legacyKey)); lerr == nil {
			return ld, nil
		}
		return nil, fmt.Errorf("binary not found for %s", artifactKeyWithBuild(ref, 0))
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	return srv.Storage().ReadFile(ctx, binaryStorageKey(key))
}

// openBinaryWithFallback returns a streaming reader for the artifact binary.
// When buildNumber=0, resolves latest published build first, then literal %0, then legacy key.
func (srv *server) openBinaryWithFallback(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64) (io.ReadCloser, error) {
	if buildNumber == 0 {
		if latest := srv.resolveLatestBuildNumber(ctx, ref); latest > 0 {
			latestKey := artifactKeyWithBuild(ref, latest)
			if lrc, lerr := srv.Storage().Open(ctx, binaryStorageKey(latestKey)); lerr == nil {
				return lrc, nil
			}
		}
		key := artifactKeyWithBuild(ref, 0)
		if rc, err := srv.Storage().Open(ctx, binaryStorageKey(key)); err == nil {
			return rc, nil
		}
		legacyKey := artifactKeyLegacy(ref)
		if lrc, lerr := srv.Storage().Open(ctx, binaryStorageKey(legacyKey)); lerr == nil {
			return lrc, nil
		}
		return nil, fmt.Errorf("binary not found for %s", artifactKeyWithBuild(ref, 0))
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	return srv.Storage().Open(ctx, binaryStorageKey(key))
}

// ── sorting helpers ──────────────────────────────────────────────────────

// sortManifestsByVersionDesc sorts manifests by semver descending, then
// build_number descending within the same semver.
func sortManifestsByVersionDesc(ms []*repopb.ArtifactManifest) {
	sort.Slice(ms, func(i, j int) bool {
		cmp, err := versionutil.Compare(
			ms[i].GetRef().GetVersion(),
			ms[j].GetRef().GetVersion(),
		)
		if err != nil {
			// Fallback: lexicographic.
			return ms[i].GetRef().GetVersion() > ms[j].GetRef().GetVersion()
		}
		if cmp != 0 {
			return cmp > 0
		}
		// Same semver → higher build_number first.
		return ms[i].GetBuildNumber() > ms[j].GetBuildNumber()
	})
}

// ── build resolution helpers ─────────────────────────────────────────────

// resolveLatestBuildNumber scans the artifacts directory for all builds of the
// given artifact (publisher, name, version, platform) and returns the highest
// build_number found. Returns 0 if no builds are found.
//
// Directory listings are cached (30s TTL) to avoid repeated MinIO ListObjects
// calls from the reconcile loop.
func (srv *server) resolveLatestBuildNumber(ctx context.Context, ref *repopb.ArtifactRef) int64 {
	names := srv.cachedDirNames(ctx)
	if names == nil {
		return 0
	}

	// Build the prefix that all builds of this artifact share:
	// {publisherID}%{name}%{version}%{platform}%
	prefix := ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform() + "%"
	suffix := ".manifest.json"

	var best int64
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		// Extract build number from between prefix and suffix.
		numStr := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
		bn, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil || bn <= 0 {
			continue
		}
		// Only consider PUBLISHED artifacts for latest-build resolution.
		key := strings.TrimSuffix(name, suffix)
		if _, state, _, readErr := srv.readManifestAndStateByKey(ctx, key); readErr == nil {
			if state != repopb.PublishState_PUBLISHED {
				continue
			}
		}
		if bn > best {
			best = bn
		}
	}
	return best
}

// findExistingArtifactByDigest returns an existing artifact with the same
// package identity and content digest. Build numbers are display metadata; the
// repository must not create a second build row for identical bytes just
// because two import paths disagree on a CI/build counter.
func (srv *server) findExistingArtifactByDigest(ctx context.Context, ref *repopb.ArtifactRef, checksum string) (*repopb.ArtifactManifest, repopb.PublishState, string, bool) {
	if ref == nil || checksum == "" {
		return nil, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, "", false
	}
	names := srv.cachedDirNames(ctx)
	if names == nil {
		return nil, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, "", false
	}

	prefix := ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetVersion() + "%" + ref.GetPlatform() + "%"
	suffix := ".manifest.json"

	var best *repopb.ArtifactManifest
	var bestState repopb.PublishState
	var bestKey string
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
			continue
		}
		key := strings.TrimSuffix(name, suffix)
		_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil || m == nil || m.GetChecksum() != checksum {
			continue
		}
		if best == nil || m.GetBuildNumber() > best.GetBuildNumber() {
			best = m
			bestState = state
			bestKey = key
		}
	}
	if best == nil {
		return nil, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, "", false
	}
	return best, bestState, bestKey, true
}

// cachedDirNames returns the artifact directory entry names, using the cache
// when available.
func (srv *server) cachedDirNames(ctx context.Context) []string {
	if srv.cache != nil {
		if names, ok := srv.cache.getDir(artifactsDir); ok {
			return names
		}
	}
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	if srv.cache != nil {
		srv.cache.putDir(artifactsDir, names)
	}
	return names
}

// loadPublishedCatalog returns all currently-PUBLISHED manifests.
// Used by the law validator for cross-artifact rules. Errors are swallowed —
// a partial or empty catalog degrades validation gracefully (single-artifact
// rules still run; cross-artifact rules become best-effort).
func (srv *server) loadPublishedCatalog(ctx context.Context) []*repopb.ArtifactManifest {
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return nil
	}
	var out []*repopb.ArtifactManifest
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil || state != repopb.PublishState_PUBLISHED {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ── package.json extraction ───────────────────────────────────────────────

// packageManifest mirrors the pkgpack.Manifest fields relevant to catalog metadata.
// Defined here to avoid a build-time dependency on the CLI package.
type packageManifest struct {
	Type                 string   `json:"type"`
	Name                 string   `json:"name"`
	Profiles             []string `json:"profiles,omitempty"`
	Priority             int      `json:"priority,omitempty"`
	InstallMode          string   `json:"install_mode,omitempty"`
	ManagedUnit          bool     `json:"managed_unit,omitempty"`
	SystemdUnit          string   `json:"systemd_unit,omitempty"`
	ProvidesCapabilities []string `json:"provides_capabilities,omitempty"`
	HealthCheckUnit      string   `json:"health_check_unit,omitempty"`
	HealthCheckPort      int      `json:"health_check_port,omitempty"`
	EntrypointChecksum   string   `json:"entrypoint_checksum,omitempty"`
	Description          string   `json:"description,omitempty"`
	Keywords             []string `json:"keywords,omitempty"`
	License              string   `json:"license,omitempty"`
	Channel              string   `json:"channel,omitempty"`

	// Typed dependency declarations (PR 2).
	HardDeps    []string `json:"hard_deps,omitempty"`
	RuntimeUses []string `json:"runtime_uses,omitempty"`

	// Deprecated: kept for reading legacy packages. Migrated to HardDeps below.
	InstallDependencies      []string `json:"install_dependencies,omitempty"`
	RuntimeLocalDependencies []string `json:"runtime_local_dependencies,omitempty"`
}

// unionStrings returns the union of two string slices with deduplication, order preserved.
func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, s := range append(a, b...) {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// extractPackageManifest reads package.json from a .tgz archive.
// Returns nil (no error) if the archive is not a valid tgz or has no package.json.
func extractPackageManifest(data []byte) *packageManifest {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return nil
		}
		name := path.Clean(hdr.Name)
		if name == "package.json" {
			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil
			}
			var m packageManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil
			}
			return &m
		}
	}
}

// enrichManifestFromPackageJSON populates ArtifactManifest catalog fields from
// the extracted package.json. Only non-empty fields are applied.
func enrichManifestFromPackageJSON(manifest *repopb.ArtifactManifest, pkg *packageManifest) {
	if pkg == nil || manifest == nil {
		return
	}
	if len(pkg.Profiles) > 0 {
		manifest.Profiles = pkg.Profiles
	}
	if pkg.Priority > 0 {
		manifest.Priority = int32(pkg.Priority)
	}
	if pkg.InstallMode != "" {
		manifest.InstallMode = pkg.InstallMode
	}
	manifest.ManagedUnit = pkg.ManagedUnit
	if pkg.SystemdUnit != "" {
		manifest.SystemdUnit = pkg.SystemdUnit
	}
	if len(pkg.ProvidesCapabilities) > 0 {
		manifest.Provides = pkg.ProvidesCapabilities
	}

	// Typed dependency declarations (PR 2).
	// Prefer hard_deps from package.json; fall back to legacy fields.
	hardDeps := pkg.HardDeps
	if len(hardDeps) == 0 {
		hardDeps = unionStrings(pkg.InstallDependencies, pkg.RuntimeLocalDependencies)
	}
	for _, name := range hardDeps {
		manifest.HardDeps = append(manifest.HardDeps, &repopb.ArtifactDependencyRef{Name: name})
	}
	if len(pkg.RuntimeUses) > 0 {
		manifest.RuntimeUses = pkg.RuntimeUses
	}

	// Keep deprecated fields populated for backward-compat readers.
	if len(pkg.InstallDependencies) > 0 {
		manifest.InstallDependencies = pkg.InstallDependencies
	}
	if len(pkg.RuntimeLocalDependencies) > 0 {
		manifest.RuntimeLocalDependencies = pkg.RuntimeLocalDependencies
	}

	if pkg.HealthCheckUnit != "" {
		manifest.HealthCheckUnit = pkg.HealthCheckUnit
	}
	if pkg.HealthCheckPort > 0 {
		manifest.HealthCheckPort = int32(pkg.HealthCheckPort)
	}
	if pkg.EntrypointChecksum != "" {
		manifest.EntrypointChecksum = pkg.EntrypointChecksum
	}
	if pkg.Description != "" && manifest.Description == "" {
		manifest.Description = pkg.Description
	}
	if len(pkg.Keywords) > 0 && len(manifest.Keywords) == 0 {
		manifest.Keywords = pkg.Keywords
	}
	if pkg.License != "" && manifest.License == "" {
		manifest.License = pkg.License
	}
	if pkg.Channel != "" {
		manifest.Channel = channelFromString(pkg.Channel)
	}
}

// ── stream helpers ────────────────────────────────────────────────────────

// recvArtifactStream accumulates all chunks from an UploadArtifact stream.
// The ArtifactRef is taken from the first message that carries a non-nil ref.
// Returns the ref, aggregated data, build_number, and reservation_id from the first message.
func recvArtifactStream(stream repopb.PackageRepository_UploadArtifactServer) (*repopb.ArtifactRef, []byte, int64, string, error) {
	var ref *repopb.ArtifactRef
	var data []byte
	var buildNumber int64
	var reservationID string
	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return ref, data, buildNumber, reservationID, nil
		}
		if err != nil {
			return nil, nil, 0, "", fmt.Errorf("recv artifact: %w", err)
		}
		if ref == nil && msg.GetRef() != nil {
			ref = msg.GetRef()
			buildNumber = msg.GetBuildNumber()
			reservationID = msg.GetReservationId()
		}
		data = append(data, msg.GetData()...)
	}
}

// ── public handlers ───────────────────────────────────────────────────────

// ListArtifacts returns all manifests stored in the repository.
// If the artifacts directory does not yet exist, an empty list is returned.
func (srv *server) ListArtifacts(ctx context.Context, _ *repopb.ListArtifactsRequest) (*repopb.ListArtifactsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		// Directory not yet created → empty catalog (not an error for the caller).
		slog.Debug("artifacts directory not found, returning empty catalog", "err", err)
		return &repopb.ListArtifactsResponse{}, nil
	}

	// Determine if caller is admin/owner for visibility of hidden artifacts.
	authCtx := security.FromContext(ctx)
	isAdmin := authCtx != nil && authCtx.Subject == "sa"

	var manifests []*repopb.ArtifactManifest
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")
		_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil {
			slog.Warn("skipping unreadable manifest", "key", key, "err", err)
			continue
		}

		// Law 4: Hide VERIFIED/YANKED/QUARANTINED/etc. from non-admin catalog views.
		if repopb.IsDiscoveryHidden(state) && !isAdmin {
			if authCtx == nil || !srv.isNamespaceOwner(ctx, m.GetRef().GetPublisherId(), authCtx.Subject) {
				continue
			}
		}

		manifests = append(manifests, m)
	}

	sortManifestsByVersionDesc(manifests)
	return &repopb.ListArtifactsResponse{Artifacts: manifests}, nil
}

// GetArtifactManifest returns metadata for a specific artifact reference.
// The build_number is read from the manifest's build_number field in the request.
// When build_number is 0, also tries legacy 4-field key for backward compat.
func (srv *server) GetArtifactManifest(ctx context.Context, req *repopb.GetArtifactManifestRequest) (*repopb.GetArtifactManifestResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Normalize version to canonical semver for key lookup.
	canonVer := ref.GetVersion()
	if cv, err := versionutil.Canonical(canonVer); err == nil {
		canonVer = cv
		ref.Version = cv
	}

	// Default platform to linux_amd64 when unspecified — artifacts are always
	// published with a platform, so an empty platform produces a key mismatch.
	if strings.TrimSpace(ref.GetPlatform()) == "" {
		ref.Platform = "linux_amd64"
	}

	// Build number from the request; 0 means legacy/unspecified.
	buildNumber := req.GetBuildNumber()

	m, err := srv.readManifestWithFallback(ctx, ref, buildNumber)
	if err != nil {
		// Fallback: try v-prefixed key for backward compat with existing storage.
		ref.Version = "v" + canonVer
		if fm, ferr := srv.readManifestWithFallback(ctx, ref, buildNumber); ferr == nil {
			// Attach provenance if available.
			vKey := artifactKeyWithBuild(ref, buildNumber)
			if prov := srv.readProvenance(ctx, vKey); prov != nil {
				fm.Provenance = prov
			}
			return &repopb.GetArtifactManifestResponse{Manifest: fm}, nil
		}
		key := artifactKeyWithBuild(ref, buildNumber)
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}

	// Attach provenance if available.
	key := artifactKeyWithBuild(ref, buildNumber)
	if prov := srv.readProvenance(ctx, key); prov != nil {
		m.Provenance = prov
	}

	return &repopb.GetArtifactManifestResponse{Manifest: m}, nil
}

// UploadArtifact receives a (possibly multi-chunk) artifact binary stream,
// stores the binary and a derived manifest.
//
// Build-number uniqueness: if a manifest already exists for the same
// (publisher, name, version, platform, build_number) and the checksum matches,
// the upload is treated as idempotent (success, no overwrite). If the checksum
// differs, the upload is rejected with AlreadyExists.
func (srv *server) UploadArtifact(stream repopb.PackageRepository_UploadArtifactServer) error {
	if err := srv.requireHealthy(); err != nil {
		return err
	}
	ref, data, buildNumber, reservationID, err := recvArtifactStream(stream)
	if err != nil {
		return status.Errorf(codes.Internal, "receive stream: %v", err)
	}
	if ref == nil {
		return status.Error(codes.InvalidArgument, "no ArtifactRef received in stream")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return status.Error(codes.InvalidArgument, "ref.name is required")
	}
	ctx := stream.Context()

	// ── Version resolution ───────────────────────────────────────────────
	// If the client supplies a version (e.g. from a release pipeline that
	// stamps 1.0.26 into package.json), use it exactly so the repository
	// record matches what the binary self-reports at runtime. This keeps
	// drift detection accurate.
	// If no version is provided, fall back to auto-bumping the patch number.
	var resolvedVer string
	var verErr error
	if clientVer := ref.GetVersion(); clientVer != "" {
		resolvedVer, verErr = srv.resolveVersionIntent(ctx,
			ref.GetPublisherId(), ref.GetName(), ref.GetPlatform(),
			repopb.VersionIntent_EXACT, clientVer)
	} else {
		resolvedVer, verErr = srv.resolveVersionIntent(ctx,
			ref.GetPublisherId(), ref.GetName(), ref.GetPlatform(),
			repopb.VersionIntent_BUMP_PATCH, "")
	}
	if verErr != nil {
		return status.Errorf(codes.Internal, "version resolution failed: %v", verErr)
	}
	ref.Version = resolvedVer

	// ── Publisher namespace + package-level validation ────────────────────
	publisherID := ref.GetPublisherId()
	if err := srv.validatePackageAccess(ctx, publisherID, ref.GetName()); err != nil {
		return err
	}

	newChecksum := checksumBytes(data)

	if existing, state, key, ok := srv.findExistingArtifactByDigest(ctx, ref, newChecksum); ok {
		slog.Info("artifact upload idempotent: identical artifact already exists",
			"key", key,
			"build_id", existing.GetBuildId(),
			"build", existing.GetBuildNumber(),
			"publish_state", state.String(),
		)
		return stream.SendAndClose(&repopb.UploadArtifactResponse{Result: true, BuildId: existing.GetBuildId()})
	}

	// buildNumber is auto-assigned by the repository to the next available
	// number for this version, ensuring uniqueness.
	buildNumber = srv.resolveLatestBuildNumber(ctx, ref) + 1

	key := artifactKeyWithBuild(ref, buildNumber)

	// Ensure artifacts directory exists.
	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "create artifacts dir: %v", err)
	}

	// Persist binary.
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), data, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write artifact binary: %v", err)
	}

	// Build and persist manifest with VERIFIED state.
	// The artifact is uploaded and checksum-verified but not yet discoverable
	// (the caller must PromoteArtifact to PUBLISHED after descriptor registration).
	//
	// Phase 2: allocate a repository-owned build_id (UUIDv7). This is the sole
	// authoritative artifact identity — clients cannot provide or override it.
	buildID := uuid.Must(uuid.NewV7()).String()

	manifest := &repopb.ArtifactManifest{
		Ref:          ref,
		BuildNumber:  buildNumber,
		BuildId:      buildID,
		Checksum:     newChecksum,
		SizeBytes:    int64(len(data)),
		ModifiedUnix: time.Now().Unix(),
	}

	// Enrich manifest with catalog metadata from package.json inside the tgz.
	if pkg := extractPackageManifest(data); pkg != nil {
		enrichManifestFromPackageJSON(manifest, pkg)
	}

	// If a reservation was provided, consume it and apply its channel.
	// The reservation channel overrides whatever was in package.json —
	// publish-time channel wins over build-time default.
	if reservationID != "" {
		if res := reservations.consume(reservationID); res != nil {
			if res.Channel != repopb.ArtifactChannel_CHANNEL_UNSET {
				manifest.Channel = res.Channel
			}
		}
	}
	mjson, err := marshalManifestWithState(manifest, repopb.PublishState_VERIFIED)
	if err != nil {
		return status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	// Sync manifest metadata to ScyllaDB (distributed consistency).
	srv.syncManifestToScylla(ctx, key, manifest, repopb.PublishState_VERIFIED, mjson)

	// Invalidate cache for this artifact and its directory listing.
	if srv.cache != nil {
		srv.cache.invalidatePrefix(artifactsDir + "/" + ref.GetPublisherId() + "%" + ref.GetName() + "%")
	}

	// Write provenance record and digest sidecar.
	prov := buildProvenanceRecord(ctx, manifest)
	provDigest, provErr := srv.writeProvenance(ctx, key, prov)
	if provErr != nil {
		slog.Warn("provenance write failed (non-fatal)", "key", key, "err", provErr)
	}
	_ = provDigest // digest stored as sidecar file for integrity verification

	// Ensure package-level RBAC ownership on first publish.
	srv.ensurePackageOwnership(ctx, publisherID, ref.GetName(), prov.GetSubject(), "")

	// Classify publish mode for audit/trust labels.
	publishMode := srv.classifyPublishMode(ctx, publisherID, ref.GetName())

	slog.Info("artifact uploaded",
		"key", key,
		"build_id", buildID,
		"kind", ref.GetKind(),
		"build", buildNumber,
		"size", len(data),
		"publish_state", repopb.PublishState_VERIFIED.String(),
		"subject", prov.GetSubject(),
		"publish_mode", publishMode,
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.uploaded", map[string]any{
		"key":          key,
		"publisher":    ref.GetPublisherId(),
		"name":         ref.GetName(),
		"version":      ref.GetVersion(),
		"build":        buildNumber,
		"checksum":     newChecksum,
		"publish_mode": publishMode,
	})

	// ── Unified publish: register descriptor + promote to PUBLISHED ──────
	// This replaces the separate Discovery call that the CLI used to make.
	// The artifact was stored and verified above; now complete the pipeline.
	// On failure, the artifact stays in VERIFIED state (not published) so the
	// caller can fix the issue and retry. The workflow run captures the error.
	if err := srv.completePublish(ctx, manifest, key, prov); err != nil {
		slog.Warn("auto-publish failed — artifact stored as VERIFIED, retry with 'globular pkg promote'",
			"key", key, "err", err)
		// Return success for the upload itself — the binary is safely stored.
		// The publish error is recorded in the workflow run for observability.
	}

	return stream.SendAndClose(&repopb.UploadArtifactResponse{Result: true, BuildId: buildID})
}

// SearchArtifacts queries the artifact catalog with optional text/filter criteria.
// It scans all manifests and applies in-memory filtering. For the expected catalog
// sizes (hundreds, not millions) this is efficient and avoids a secondary index.
func (srv *server) SearchArtifacts(ctx context.Context, req *repopb.SearchArtifactsRequest) (*repopb.SearchArtifactsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return &repopb.SearchArtifactsResponse{}, nil
	}

	query := strings.ToLower(strings.TrimSpace(req.GetQuery()))
	filterKind := req.GetKind()
	filterPub := strings.TrimSpace(req.GetPublisherId())
	filterPlat := strings.TrimSpace(req.GetPlatform())
	filterChannel := req.GetChannel()
	includeAllChannels := req.GetIncludeAllChannels()

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 {
		pageSize = 50
	}

	// Determine if caller is admin/owner for visibility of hidden artifacts.
	authCtx := security.FromContext(ctx)
	isAdmin := authCtx != nil && authCtx.Subject == "sa"

	var all []*repopb.ArtifactManifest
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")
		_, state, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil {
			continue
		}

		// Hide YANKED/QUARANTINED/REVOKED from non-owners/non-admins.
		if repopb.IsDiscoveryHidden(state) && !isAdmin {
			if authCtx == nil || !srv.isNamespaceOwner(ctx, m.GetRef().GetPublisherId(), authCtx.Subject) {
				continue
			}
		}

		// Channel filtering:
		//   - include_all_channels: skip channel check (admin/operator UX only)
		//   - filterChannel set: must match exactly
		//   - default (no filter): only return reconciler-safe channels (STABLE, BOOTSTRAP)
		//     DEV and CANDIDATE are never returned unless explicitly requested.
		if !includeAllChannels {
			ch := effectiveChannel(m)
			if filterChannel != repopb.ArtifactChannel_CHANNEL_UNSET {
				if ch != filterChannel {
					continue
				}
			} else if !isReconcilerSafeChannel(ch) {
				continue
			}
		}

		// Filter by kind.
		if filterKind != repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED && m.GetRef().GetKind() != filterKind {
			continue
		}
		// Filter by publisher.
		if filterPub != "" && !strings.EqualFold(m.GetRef().GetPublisherId(), filterPub) {
			continue
		}
		// Filter by platform.
		if filterPlat != "" && !strings.EqualFold(m.GetRef().GetPlatform(), filterPlat) {
			continue
		}
		// Free-text search across name, description, keywords.
		if query != "" && !matchesQuery(m, query) {
			continue
		}
		all = append(all, m)
	}

	// Sort by semver desc, build_number desc, then name asc for ties.
	sortManifestsByVersionDesc(all)

	totalCount := int32(len(all))

	// Pagination via page_token (token = index offset as string).
	startIdx := 0
	if tok := req.GetPageToken(); tok != "" {
		if idx, err := parseInt32(tok); err == nil && int(idx) < len(all) {
			startIdx = int(idx)
		}
	}

	end := startIdx + pageSize
	if end > len(all) {
		end = len(all)
	}
	page := all[startIdx:end]

	var nextToken string
	if end < len(all) {
		nextToken = fmt.Sprintf("%d", end)
	}

	return &repopb.SearchArtifactsResponse{
		Artifacts:     page,
		NextPageToken: nextToken,
		TotalCount:    totalCount,
	}, nil
}

// GetArtifactVersions returns all versions of a given package (publisher + name),
// optionally filtered by platform. Results are sorted by semver desc then
// build_number desc.
func (srv *server) GetArtifactVersions(ctx context.Context, req *repopb.GetArtifactVersionsRequest) (*repopb.GetArtifactVersionsResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	pub := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	filterPlat := strings.TrimSpace(req.GetPlatform())

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		return &repopb.GetArtifactVersionsResponse{}, nil
	}

	var versions []*repopb.ArtifactManifest
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		m, err := srv.readManifestByKey(ctx, key)
		if err != nil {
			continue
		}
		ref := m.GetRef()
		if !strings.EqualFold(ref.GetName(), name) {
			continue
		}
		if pub != "" && !strings.EqualFold(ref.GetPublisherId(), pub) {
			continue
		}
		if filterPlat != "" && !strings.EqualFold(ref.GetPlatform(), filterPlat) {
			continue
		}
		versions = append(versions, m)
	}

	sortManifestsByVersionDesc(versions)
	return &repopb.GetArtifactVersionsResponse{Versions: versions}, nil
}

// DeleteArtifact removes a specific artifact version (manifest + binary) from the repository.
// This is a repository/catalog operation only — it never uninstalls from nodes.
// When force is false (default), deletion is rejected if any node still has
// this artifact installed. Set force=true to remove repository availability
// while leaving installed instances in place.
func (srv *server) DeleteArtifact(ctx context.Context, req *repopb.DeleteArtifactRequest) (*repopb.DeleteArtifactResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}
	if strings.TrimSpace(ref.GetVersion()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.version is required")
	}

	// Validate namespace + package-level access.
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return nil, err
	}

	// Normalize version.
	canonicalizeRefVersion(ref)

	// For delete, use build_number=0 and try legacy fallback.
	buildNumber := int64(0)
	key := artifactKeyWithBuild(ref, buildNumber)
	mKey := manifestStorageKey(key)
	bKey := binaryStorageKey(key)

	// Locate manifest — try new 5-field key first, then legacy 4-field key.
	var targetManifest *repopb.ArtifactManifest
	if data, readErr := srv.Storage().ReadFile(ctx, mKey); readErr != nil {
		// Try legacy key.
		legacyKey := artifactKeyLegacy(ref)
		legacyMKey := manifestStorageKey(legacyKey)
		if _, lerr := srv.Storage().ReadFile(ctx, legacyMKey); lerr != nil {
			return nil, status.Errorf(codes.NotFound, "artifact %q not found", key)
		}
		// Found under legacy key — use those paths for deletion.
		mKey = legacyMKey
		bKey = binaryStorageKey(legacyKey)
		key = legacyKey
	} else {
		// Parse manifest to obtain build_id for the reachability safety check.
		if m, _, parseErr := unmarshalManifestWithState(data); parseErr == nil {
			targetManifest = m
		}
	}

	// Reachability safety check: block deletion if the artifact is reachable
	// (within the retention window or actively deployed on cluster nodes).
	// force=true bypasses both conditions.
	if targetManifest != nil && !req.GetForce() {
		catalog := srv.loadAllManifests(ctx)
		if safe, reason := srv.checkDeletionSafety(ctx, targetManifest, catalog); !safe {
			return &repopb.DeleteArtifactResponse{Result: false, Message: reason}, nil
		}
	}

	// Remove manifest and binary (best-effort for binary — it might not exist).
	if err := srv.Storage().Remove(ctx, mKey); err != nil {
		return nil, status.Errorf(codes.Internal, "delete manifest %q: %v", key, err)
	}
	_ = srv.Storage().Remove(ctx, bKey)

	// Remove from ScyllaDB.
	srv.deleteManifestFromScylla(ctx, key)

	// Invalidate cache.
	if srv.cache != nil {
		srv.cache.invalidateManifest(mKey)
		srv.cache.invalidateDir(artifactsDir)
	}

	msg := fmt.Sprintf("artifact %s@%s deleted from repository", ref.GetName(), ref.GetVersion())
	if req.GetForce() {
		msg += " (force=true: reachability check was bypassed — installed nodes are unaffected)"
	}

	slog.Info("artifact deleted", "key", key, "force", req.GetForce())

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.deleted", map[string]any{
		"key":       key,
		"publisher": ref.GetPublisherId(),
		"name":      ref.GetName(),
		"version":   ref.GetVersion(),
		"force":     req.GetForce(),
	})

	return &repopb.DeleteArtifactResponse{Result: true, Message: msg}, nil
}

// ── kind inference ───────────────────────────────────────────────────────────

// inferCorrectKind returns the correct ArtifactKind for a package name.
// Manifests published before the COMMAND kind was added have kind=SERVICE
// for everything. This function corrects the kind based on naming conventions:
//   - Names ending in "-cmd" → COMMAND (CLI tools)
//   - Known infrastructure daemons → INFRASTRUCTURE
//   - Everything else → unchanged
func inferCorrectKind(name string, current repopb.ArtifactKind) repopb.ArtifactKind {
	lower := strings.ToLower(name)

	// CLI tools: names ending in -cmd
	if strings.HasSuffix(lower, "-cmd") {
		return repopb.ArtifactKind_COMMAND
	}

	// Infrastructure daemons (not Go gRPC services)
	infraNames := map[string]bool{
		"etcd": true, "minio": true, "envoy": true, "xds": true,
		"gateway": true, "prometheus": true, "node-exporter": true,
		"alertmanager": true,
		"scylladb":     true, "scylla-manager": true, "scylla-manager-agent": true,
		"keepalived": true, "sidekick": true,
	}
	if infraNames[lower] {
		return repopb.ArtifactKind_INFRASTRUCTURE
	}

	return current
}

// ── search helpers ───────────────────────────────────────────────────────────

// matchesQuery returns true if the manifest matches a free-text query.
func matchesQuery(m *repopb.ArtifactManifest, query string) bool {
	if strings.Contains(strings.ToLower(m.GetRef().GetName()), query) {
		return true
	}
	if strings.Contains(strings.ToLower(m.GetDescription()), query) {
		return true
	}
	if strings.Contains(strings.ToLower(m.GetAlias()), query) {
		return true
	}
	for _, kw := range m.GetKeywords() {
		if strings.Contains(strings.ToLower(kw), query) {
			return true
		}
	}
	return false
}

// parseInt32 parses a string as an int32.
func parseInt32(s string) (int32, error) {
	var v int32
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}

// ── promote artifact ──────────────────────────────────────────────────────

// PromoteArtifact implements the gRPC PromoteArtifact RPC.
func (srv *server) PromoteArtifact(ctx context.Context, req *repopb.PromoteArtifactRequest) (*repopb.PromoteArtifactResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	return srv.promoteArtifactInternal(ctx, req.GetRef(), req.GetBuildNumber(), req.GetTargetState())
}

// promoteArtifactInternal transitions an artifact's publish state.
// Used by the gRPC handler and internal callers (tests, CLI client).
func (srv *server) promoteArtifactInternal(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, targetState repopb.PublishState) (*repopb.PromoteArtifactResponse, error) {
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Validate namespace + package-level access.
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return nil, err
	}

	// Normalize version.
	canonicalizeRefVersion(ref)

	key := artifactKeyWithBuild(ref, buildNumber)

	// Read existing manifest and authoritative state via the ledger-first path.
	// Using readManifestAndStateByKey ensures the current state comes from the
	// publish_state column (not from stale manifest_json), which prevents incorrect
	// transition validation when the reconciler has already promoted the artifact.
	_, currentState, m, err := srv.readManifestAndStateByKey(ctx, key)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}

	// Validate transition.
	if !repopb.ValidPromoteTransition(currentState, targetState) {
		return nil, status.Errorf(codes.FailedPrecondition,
			"invalid state transition %s → %s for artifact %q",
			currentState, targetState, key)
	}

	// Enforce artifact laws before promoting to PUBLISHED.
	if targetState == repopb.PublishState_PUBLISHED {
		catalog := srv.loadPublishedCatalog(ctx)
		if violations := NewArtifactLawValidator(m, catalog).Validate(); len(violations) > 0 {
			details := make([]string, 0, len(violations))
			for _, v := range violations {
				details = append(details, v.Error())
			}
			return nil, status.Errorf(codes.FailedPrecondition,
				"artifact law violations prevent promotion: %s", details[0])
		}
	}

	// Write updated manifest with new state.
	mjson, err := marshalManifestWithState(m, targetState)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return nil, status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	// Sync state change to ScyllaDB.
	srv.syncStateToScylla(ctx, key, targetState)

	// Invalidate cached manifest (state changed).
	if srv.cache != nil {
		srv.cache.invalidateManifest(manifestStorageKey(key))
	}

	slog.Info("artifact promoted",
		"key", key,
		"from", currentState.String(),
		"to", targetState.String(),
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.promoted", map[string]any{
		"key":            key,
		"publisher":      ref.GetPublisherId(),
		"name":           ref.GetName(),
		"previous_state": currentState.String(),
		"current_state":  targetState.String(),
	})

	return &repopb.PromoteArtifactResponse{
		Result:        true,
		PreviousState: currentState,
		CurrentState:  targetState,
		Message:       fmt.Sprintf("artifact %q promoted from %s to %s", key, currentState, targetState),
	}, nil
}

// SetArtifactState transitions an artifact's lifecycle state (deprecate, yank, quarantine, revoke).
func (srv *server) SetArtifactState(ctx context.Context, req *repopb.SetArtifactStateRequest) (*repopb.SetArtifactStateResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "ref.name is required")
	}

	// Validate namespace + package-level access.
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return nil, err
	}

	// Normalize version.
	canonicalizeRefVersion(ref)

	targetState := req.GetTargetState()
	authCtx := security.FromContext(ctx)
	buildNumber := req.GetBuildNumber()
	key := artifactKeyWithBuild(ref, buildNumber)

	// Read existing manifest and authoritative state via the ledger-first path.
	// publish_state column is the authority; manifest_json state is not trusted.
	_, currentState, m, err := srv.readManifestAndStateByKey(ctx, key)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}

	// ── Authority boundaries for lifecycle transitions ──
	var authorityMode string
	switch targetState {
	case repopb.PublishState_QUARANTINED:
		// QUARANTINE is a moderation action — admin/superuser only.
		if authCtx == nil || authCtx.Subject != "sa" {
			return nil, status.Error(codes.PermissionDenied,
				"quarantine is a moderation action restricted to administrators")
		}
		authorityMode = "admin"

	case repopb.PublishState_REVOKED:
		// REVOKE can be performed by admin OR namespace/package owner (self-revoke).
		if authCtx == nil || authCtx.Subject != "sa" {
			if authCtx == nil || !srv.isNamespaceOwner(ctx, ref.GetPublisherId(), authCtx.Subject) {
				return nil, status.Error(codes.PermissionDenied,
					"revoke requires administrator or namespace owner (self-revoke)")
			}
			authorityMode = "owner_self_revoke"
		} else {
			authorityMode = "admin"
		}

	default:
		authorityMode = "publisher"
	}

	// Transitions FROM quarantined states also require admin.
	if currentState == repopb.PublishState_QUARANTINED {
		if authCtx == nil || authCtx.Subject != "sa" {
			return nil, status.Error(codes.PermissionDenied,
				"un-quarantine is a moderation action restricted to administrators")
		}
		authorityMode = "admin"
	}

	// Validate transition using the extended state machine.
	if !repopb.ValidStateTransition(currentState, targetState) {
		return nil, status.Errorf(codes.FailedPrecondition,
			"invalid state transition %s → %s for artifact %q",
			currentState, targetState, key)
	}

	// Revoke safety: block if the artifact is actively deployed on cluster nodes.
	// Admin callers (sa) may bypass for security incident response.
	// Retention-window-only artifacts may be revoked freely.
	if targetState == repopb.PublishState_REVOKED {
		isAdmin := authCtx != nil && authCtx.Subject == "sa"
		if blocked, reason := srv.checkRevokeSafety(ctx, m, isAdmin); blocked {
			return nil, status.Errorf(codes.FailedPrecondition, "revoke safety: %s", reason)
		}
	}

	// Write updated manifest with new state.
	mjson, err := marshalManifestWithState(m, targetState)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		return nil, status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	// Sync state change to ScyllaDB.
	srv.syncStateToScylla(ctx, key, targetState)

	// Invalidate cached manifest (state changed).
	if srv.cache != nil {
		srv.cache.invalidateManifest(manifestStorageKey(key))
	}

	slog.Info("artifact state changed",
		"key", key,
		"from", currentState.String(),
		"to", targetState.String(),
		"reason", req.GetReason(),
		"authority", authorityMode,
	)

	// Audit event.
	srv.publishAuditEvent(ctx, "artifact.state_changed", map[string]any{
		"key":            key,
		"publisher":      ref.GetPublisherId(),
		"name":           ref.GetName(),
		"version":        ref.GetVersion(),
		"previous_state": currentState.String(),
		"current_state":  targetState.String(),
		"reason":         req.GetReason(),
		"authority":      authorityMode,
	})

	return &repopb.SetArtifactStateResponse{
		PreviousState: currentState,
		CurrentState:  targetState,
	}, nil
}

// DownloadArtifact streams the binary for a stored artifact.
func (srv *server) DownloadArtifact(req *repopb.DownloadArtifactRequest, stream repopb.PackageRepository_DownloadArtifactServer) error {
	if err := srv.requireHealthy(); err != nil {
		return err
	}
	ref := req.GetRef()
	if ref == nil {
		return status.Error(codes.InvalidArgument, "ref is required")
	}

	// Normalize version to canonical semver for key lookup.
	canonicalizeRefVersion(ref)

	// Check publish state — block downloads of YANKED/QUARANTINED/REVOKED
	// unless the caller is the namespace owner or admin.
	canonVer := ref.GetVersion()
	buildNumber := req.GetBuildNumber()

	// When build_number=0 (unspecified), resolve to the latest published build.
	if buildNumber == 0 {
		if latest := srv.resolveLatestBuildNumber(stream.Context(), ref); latest > 0 {
			buildNumber = latest
		}
	}

	key := artifactKeyWithBuild(ref, buildNumber)
	if _, state, _, readErr := srv.readManifestAndStateByKey(stream.Context(), key); readErr == nil {
		if repopb.IsDownloadBlocked(state) {
			// Check if caller is namespace owner (allowed to download their own blocked artifacts).
			authCtx := security.FromContext(stream.Context())
			if authCtx == nil || !srv.isNamespaceOwner(stream.Context(), ref.GetPublisherId(), authCtx.Subject) {
				return status.Errorf(codes.PermissionDenied,
					"artifact %q is %s — download blocked", ref.GetName(), state)
			}
		}
	}

	// Try new 5-field key, then legacy 4-field key.
	// Stream directly from storage to avoid buffering the entire artifact in memory.
	reader, err := srv.openBinaryWithFallback(stream.Context(), ref, buildNumber)
	if err != nil {
		// Fallback: try v-prefixed key for backward compat with existing storage.
		ref.Version = "v" + canonVer
		if fr, ferr := srv.openBinaryWithFallback(stream.Context(), ref, buildNumber); ferr == nil {
			reader = fr
			err = nil
		}
		ref.Version = canonVer // restore
	}
	if err != nil {
		key := artifactKeyWithBuild(ref, buildNumber)
		return status.Errorf(codes.NotFound, "artifact %q not found: %v", key, err)
	}
	defer reader.Close()

	buf := make([]byte, 256*1024) // 256KB chunks (larger = fewer round-trips)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&repopb.DownloadArtifactResponse{Data: buf[:n]}); err != nil {
				return fmt.Errorf("send artifact chunk: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read artifact: %w", readErr)
		}
	}
	return nil
}

// UpdateArtifactBinary implements the delta-deploy RPC. It receives a new binary
// for an existing artifact, creates a new build entry by copying the latest
// manifest, and promotes to PUBLISHED.
//
// Flow:
//  1. Receive binary stream (header + chunks)
//  2. Verify checksum matches
//  3. Find the latest published build for this (publisher, name, version, platform)
//  4. Copy its manifest, update checksum + size + build_number
//  5. Store new binary + manifest
//  6. Complete publish pipeline (register descriptor + promote)
//  7. Return new build_number
func (srv *server) UpdateArtifactBinary(stream repopb.PackageRepository_UpdateArtifactBinaryServer) error {
	if err := srv.requireHealthy(); err != nil {
		return err
	}
	// ── Receive header ──────────────────────────────────────────────────
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "receive header: %v", err)
	}
	header := firstMsg.GetHeader()
	if header == nil {
		return status.Error(codes.InvalidArgument, "first message must contain header")
	}
	ref := header.GetRef()
	if ref == nil {
		return status.Error(codes.InvalidArgument, "header.ref is required")
	}
	if strings.TrimSpace(ref.GetName()) == "" {
		return status.Error(codes.InvalidArgument, "ref.name is required")
	}
	if strings.TrimSpace(ref.GetVersion()) == "" {
		return status.Error(codes.InvalidArgument, "ref.version is required")
	}
	if canonVer, verr := versionutil.Canonical(ref.GetVersion()); verr == nil {
		ref.Version = canonVer
	}

	ctx := stream.Context()

	// ── Publisher namespace + access validation ─────────────────────────
	if err := srv.validatePackageAccess(ctx, ref.GetPublisherId(), ref.GetName()); err != nil {
		return err
	}

	// ── Receive binary chunks ───────────────────────────────────────────
	var data []byte
	for {
		msg, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			return status.Errorf(codes.Internal, "receive chunk: %v", recvErr)
		}
		data = append(data, msg.GetChunk()...)
	}

	// ── Verify checksum ─────────────────────────────────────────────────
	actualChecksum := checksumBytes(data)
	expectedChecksum := header.GetChecksum()
	if expectedChecksum != "" && actualChecksum != expectedChecksum {
		return status.Errorf(codes.InvalidArgument,
			"checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// ── Find latest published build ─────────────────────────────────────
	latestBuild := srv.resolveLatestBuildNumber(ctx, ref)
	if latestBuild == 0 {
		return status.Error(codes.FailedPrecondition,
			"no existing published build found — use UploadArtifact for first publish")
	}

	latestKey := artifactKeyWithBuild(ref, latestBuild)
	latestManifest, mErr := srv.readManifestByKey(ctx, latestKey)
	if mErr != nil {
		return status.Errorf(codes.Internal, "read latest manifest %q: %v", latestKey, mErr)
	}

	// ── Assign next build number ────────────────────────────────────────
	newBuild := latestBuild + 1
	newKey := artifactKeyWithBuild(ref, newBuild)

	// ── Ensure artifacts directory ──────────────────────────────────────
	if err := srv.Storage().MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		return status.Errorf(codes.Internal, "create artifacts dir: %v", err)
	}

	// ── Store new binary ────────────────────────────────────────────────
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(newKey), data, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write binary: %v", err)
	}

	// ── Create new manifest (clone from latest, update binary fields) ──
	// Phase 2: allocate a new build_id for the delta-deploy artifact.
	newBuildID := uuid.Must(uuid.NewV7()).String()

	newManifest := &repopb.ArtifactManifest{
		Ref:                      ref,
		BuildNumber:              newBuild,
		BuildId:                  newBuildID,
		Checksum:                 actualChecksum,
		SizeBytes:                int64(len(data)),
		ModifiedUnix:             time.Now().Unix(),
		Profiles:                 latestManifest.GetProfiles(),
		Priority:                 latestManifest.GetPriority(),
		InstallMode:              latestManifest.GetInstallMode(),
		ManagedUnit:              latestManifest.GetManagedUnit(),
		SystemdUnit:              latestManifest.GetSystemdUnit(),
		RuntimeLocalDependencies: latestManifest.GetRuntimeLocalDependencies(),
		InstallDependencies:      latestManifest.GetInstallDependencies(),
		HardDeps:                 latestManifest.GetHardDeps(),
		RuntimeUses:              latestManifest.GetRuntimeUses(),
		HealthCheckUnit:          latestManifest.GetHealthCheckUnit(),
		HealthCheckPort:          latestManifest.GetHealthCheckPort(),
		Provides:                 latestManifest.GetProvides(),
		Requires:                 latestManifest.GetRequires(),
		Defaults:                 latestManifest.GetDefaults(),
		Entrypoints:              latestManifest.GetEntrypoints(),
		Description:              latestManifest.GetDescription(),
		Keywords:                 latestManifest.GetKeywords(),
		Icon:                     latestManifest.GetIcon(),
		Alias:                    latestManifest.GetAlias(),
		License:                  latestManifest.GetLicense(),
		MinGlobularVersion:       latestManifest.GetMinGlobularVersion(),
	}

	// Copy type_detail from the latest manifest.
	switch td := latestManifest.GetTypeDetail().(type) {
	case *repopb.ArtifactManifest_ServiceDetail:
		newManifest.TypeDetail = td
	case *repopb.ArtifactManifest_ApplicationDetail:
		newManifest.TypeDetail = td
	case *repopb.ArtifactManifest_InfrastructureDetail:
		newManifest.TypeDetail = td
	}

	// Also try to enrich from the new binary's package.json if it's a .tgz.
	if pkg := extractPackageManifest(data); pkg != nil {
		enrichManifestFromPackageJSON(newManifest, pkg)
	}

	mjson, err := marshalManifestWithState(newManifest, repopb.PublishState_VERIFIED)
	if err != nil {
		return status.Errorf(codes.Internal, "marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(newKey), mjson, 0o644); err != nil {
		return status.Errorf(codes.Internal, "write manifest: %v", err)
	}

	// Sync manifest metadata to ScyllaDB.
	srv.syncManifestToScylla(ctx, newKey, newManifest, repopb.PublishState_VERIFIED, mjson)

	// ── Provenance ──────────────────────────────────────────────────────
	prov := buildProvenanceRecord(ctx, newManifest)
	if _, provErr := srv.writeProvenance(ctx, newKey, prov); provErr != nil {
		slog.Warn("delta-deploy provenance write failed (non-fatal)", "key", newKey, "err", provErr)
	}

	slog.Info("delta-deploy: binary updated",
		"key", newKey,
		"build", newBuild,
		"size", len(data),
		"checksum", actualChecksum,
		"prev_build", latestBuild,
	)

	// ── Complete publish pipeline ───────────────────────────────────────
	if err := srv.completePublish(ctx, newManifest, newKey, prov); err != nil {
		slog.Warn("delta-deploy: auto-publish failed — artifact stored as VERIFIED",
			"key", newKey, "err", err)
	}

	return stream.SendAndClose(&repopb.UpdateArtifactBinaryResponse{
		BuildNumber: newBuild,
		Checksum:    actualChecksum,
		Status:      "published",
	})
}

// ResolveByEntrypointChecksum performs a reverse lookup: given a binary's SHA256
// checksum, find the artifact manifest that produced it. Used by node-agent
// process fingerprinting to resolve "which version is this binary?"
//
// Two-phase approach:
//  1. Fast lookup via ScyllaDB secondary index on entrypoint_checksum.
//  2. Validate from MinIO: crack open the .tgz, verify the binary inside
//     matches the declared checksum. If mismatch → mark CORRUPTED.
//
// Falls back to MinIO full scan if ScyllaDB is unavailable or has no match
// (handles packages published before entrypoint_checksum was indexed).
func (srv *server) ResolveByEntrypointChecksum(ctx context.Context, req *repopb.ResolveByEntrypointChecksumRequest) (*repopb.ResolveByEntrypointChecksumResponse, error) {
	if err := srv.requireHealthy(); err != nil {
		return nil, err
	}
	checksum := strings.TrimSpace(req.GetChecksum())
	if checksum == "" {
		return nil, status.Error(codes.InvalidArgument, "checksum is required")
	}
	platform := strings.TrimSpace(req.GetPlatform())
	if platform == "" {
		platform = "linux_amd64"
	}

	prefixed := "sha256:" + checksum

	// ── Phase 1: ScyllaDB fast lookup ───────────────────────────────────
	if srv.scylla != nil {
		// Try both with and without prefix — callers may use either form.
		for _, query := range []string{prefixed, checksum} {
			rows, err := srv.scylla.FindByEntrypointChecksum(ctx, query)
			if err != nil {
				slog.Warn("scylla entrypoint_checksum lookup failed, falling back to MinIO", "err", err)
				break
			}
			if m := srv.pickBestCandidate(ctx, rows, platform, prefixed, checksum); m != nil {
				return &repopb.ResolveByEntrypointChecksumResponse{Manifest: m}, nil
			}
		}
	}

	// ── Phase 2: MinIO full scan (fallback) ─────────────────────────────
	// Handles packages published before entrypoint_checksum was indexed,
	// or when ScyllaDB is unavailable.
	names := srv.cachedDirNames(ctx)
	if names == nil {
		return nil, status.Error(codes.Internal, "cannot list artifacts directory")
	}

	var bestManifest *repopb.ArtifactManifest
	for _, name := range names {
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")
		_, st, m, err := srv.readManifestAndStateByKey(ctx, key)
		if err != nil || st != repopb.PublishState_PUBLISHED {
			continue
		}
		if ref := m.GetRef(); ref != nil && !strings.EqualFold(ref.GetPlatform(), platform) {
			continue
		}

		// Read .tgz and extract package.json for ground truth.
		binData, binErr := srv.Storage().ReadFile(ctx, binaryStorageKey(key))
		if binErr != nil {
			continue
		}
		pkg := extractPackageManifest(binData)
		if pkg == nil || pkg.EntrypointChecksum == "" {
			continue
		}
		if pkg.EntrypointChecksum != prefixed && pkg.EntrypointChecksum != checksum {
			continue
		}

		// Validate binary integrity.
		if !srv.validateBinaryIntegrity(ctx, key, m, pkg, binData) {
			continue
		}

		// Keep highest version/build_number.
		if bestManifest == nil {
			bestManifest = m
		} else if isBetter(m, bestManifest) {
			bestManifest = m
		}
	}

	if bestManifest == nil {
		return nil, status.Errorf(codes.NotFound,
			"no published artifact with entrypoint_checksum %s found", checksum)
	}

	slog.Debug("resolved entrypoint checksum (MinIO fallback)",
		"checksum", checksum[:12],
		"name", bestManifest.GetRef().GetName(),
		"version", bestManifest.GetRef().GetVersion(),
		"build", bestManifest.GetBuildNumber(),
	)
	return &repopb.ResolveByEntrypointChecksumResponse{Manifest: bestManifest}, nil
}

// pickBestCandidate filters ScyllaDB rows by state/platform, validates against
// MinIO, and returns the best matching manifest.
func (srv *server) pickBestCandidate(ctx context.Context, rows []manifestRow, platform, prefixed, checksum string) *repopb.ArtifactManifest {
	var best *repopb.ArtifactManifest
	for _, row := range rows {
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			continue
		}
		if !strings.EqualFold(row.Platform, platform) {
			continue
		}

		// Read the manifest from the canonical source (MinIO/cache).
		_, st, m, err := srv.readManifestAndStateByKey(ctx, row.ArtifactKey)
		if err != nil || st != repopb.PublishState_PUBLISHED {
			continue
		}

		// Validate from MinIO: crack open .tgz and verify binary integrity.
		binData, binErr := srv.Storage().ReadFile(ctx, binaryStorageKey(row.ArtifactKey))
		if binErr != nil {
			continue
		}
		pkg := extractPackageManifest(binData)
		if pkg == nil || pkg.EntrypointChecksum == "" {
			continue
		}
		if pkg.EntrypointChecksum != prefixed && pkg.EntrypointChecksum != checksum {
			// ScyllaDB had it but the .tgz doesn't match — stale index.
			slog.Warn("scylla/minio entrypoint_checksum mismatch",
				"key", row.ArtifactKey,
				"scylla", row.EntrypointChecksum,
				"tgz", pkg.EntrypointChecksum,
			)
			continue
		}

		if !srv.validateBinaryIntegrity(ctx, row.ArtifactKey, m, pkg, binData) {
			continue
		}

		slog.Debug("resolved entrypoint checksum (ScyllaDB)",
			"checksum", checksum[:12],
			"name", m.GetRef().GetName(),
			"version", m.GetRef().GetVersion(),
			"build", m.GetBuildNumber(),
		)

		if best == nil || isBetter(m, best) {
			best = m
		}
	}
	return best
}

// validateBinaryIntegrity extracts the binary from the archive and verifies
// its checksum matches the declared entrypoint_checksum in package.json.
// Returns false and marks the package CORRUPTED if they don't match.
func (srv *server) validateBinaryIntegrity(ctx context.Context, key string, m *repopb.ArtifactManifest, pkg *packageManifest, binData []byte) bool {
	actualCS := computeBinaryChecksumFromArchive(binData)
	if actualCS == "" {
		return true // can't verify — allow it through
	}
	actualPrefixed := "sha256:" + actualCS
	if pkg.EntrypointChecksum == actualPrefixed || pkg.EntrypointChecksum == actualCS {
		return true
	}

	slog.Warn("integrity check failed — binary doesn't match declared entrypoint_checksum",
		"key", key,
		"declared", pkg.EntrypointChecksum,
		"actual", actualPrefixed,
	)
	srv.markCorrupted(ctx, key, m, pkg.EntrypointChecksum, actualPrefixed)
	return false
}

// isBetter returns true if candidate has a higher version or build_number than current.
func isBetter(candidate, current *repopb.ArtifactManifest) bool {
	cmp, err := versionutil.Compare(
		candidate.GetRef().GetVersion(),
		current.GetRef().GetVersion(),
	)
	if err == nil && cmp > 0 {
		return true
	}
	return cmp == 0 && candidate.GetBuildNumber() > current.GetBuildNumber()
}

// computeBinaryChecksumFromArchive extracts the first regular executable file
// from a .tgz archive and returns its SHA256 hex digest.
// Returns empty string if the archive can't be read or contains no binary.
func computeBinaryChecksumFromArchive(data []byte) string {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return ""
		}
		// Look for the entrypoint binary: files in bin/ that are executable.
		name := path.Clean(hdr.Name)
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasPrefix(name, "bin/") && !strings.Contains(name, "/bin/") {
			continue
		}
		if hdr.Mode&0111 == 0 {
			continue
		}
		// Found the binary — compute its SHA256.
		return checksumReader(tr)
	}
}

// checksumReader computes the SHA256 hex digest of all data from r.
func checksumReader(r io.Reader) string {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// markCorrupted transitions a package to CORRUPTED state and logs an audit event.
func (srv *server) markCorrupted(ctx context.Context, key string, m *repopb.ArtifactManifest, declared, actual string) {
	mjson, err := marshalManifestWithState(m, repopb.PublishState_CORRUPTED)
	if err != nil {
		slog.Warn("failed to marshal corrupted manifest", "key", key, "err", err)
		return
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		slog.Warn("failed to write corrupted manifest", "key", key, "err", err)
		return
	}
	srv.syncStateToScylla(ctx, key, repopb.PublishState_CORRUPTED)
	if srv.cache != nil {
		srv.cache.invalidateManifest(manifestStorageKey(key))
	}

	slog.Error("package marked CORRUPTED — binary integrity check failed",
		"key", key,
		"declared_checksum", declared,
		"actual_checksum", actual,
	)

	srv.publishAuditEvent(ctx, "artifact.corrupted", map[string]any{
		"key":               key,
		"declared_checksum": declared,
		"actual_checksum":   actual,
	})
}

// ── Publish-state backfill ──────────────────────────────────────────────────

// BackfillPublishStateResult reports the outcome of a backfill run.
type BackfillPublishStateResult struct {
	RowsChecked    int
	RowsBackfilled int // publish_state was empty → set from manifest_json
	RowsDrifted    int // publish_state and manifest_json disagree (column wins, drift logged)
	RowsFailed     int
}

// backfillPublishState repairs Scylla rows where the publish_state column is
// empty (legacy rows written before the column was authoritative).
//
// Rules:
//   - publish_state empty → set from manifest_json state (backfill)
//   - publish_state present but differs from manifest_json → column wins, drift logged
//   - Never downgrades PUBLISHED to VERIFIED from manifest_json
//
// Safe to call at startup or on demand. Idempotent.
func (srv *server) backfillPublishState(ctx context.Context) BackfillPublishStateResult {
	if srv.scylla == nil {
		return BackfillPublishStateResult{}
	}
	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		slog.Warn("backfillPublishState: list manifests failed", "err", err)
		return BackfillPublishStateResult{}
	}

	var result BackfillPublishStateResult
	for _, row := range rows {
		result.RowsChecked++

		_, jsonState, parseErr := unmarshalManifestWithState(row.ManifestJSON)
		if parseErr != nil {
			result.RowsFailed++
			continue
		}

		columnState := repopb.PublishState_PUBLISH_STATE_UNSPECIFIED
		if row.PublishState != "" {
			if v, ok := repopb.PublishState_value[row.PublishState]; ok {
				columnState = repopb.PublishState(v)
			}
		}

		if columnState == repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
			// Column is empty — backfill from manifest_json state.
			if jsonState == repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
				continue // nothing to backfill
			}
			if err := srv.scylla.UpdatePublishState(ctx, row.ArtifactKey, jsonState.String()); err != nil {
				slog.Warn("backfillPublishState: update failed", "key", row.ArtifactKey, "err", err)
				result.RowsFailed++
			} else {
				result.RowsBackfilled++
			}
			continue
		}

		// Column is set. Check for drift with manifest_json.
		if columnState != jsonState && jsonState != repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
			slog.Warn("backfillPublishState: state drift detected — column wins",
				"key", row.ArtifactKey,
				"column_state", columnState,
				"json_state", jsonState,
				"authoritative", columnState,
			)
			result.RowsDrifted++
		}
	}

	if result.RowsBackfilled > 0 || result.RowsDrifted > 0 {
		slog.Info("backfillPublishState: complete",
			"checked", result.RowsChecked,
			"backfilled", result.RowsBackfilled,
			"drifted", result.RowsDrifted,
			"failed", result.RowsFailed,
		)
	}
	return result
}
