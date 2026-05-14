package main

// migration.go — migrates existing artifacts into the trust model.
//
// Called on startup (idempotent). For each existing artifact:
//   1. Creates namespace at /namespaces/{publisherID} with "sa" as owner (if missing)
//   2. Creates synthetic provenance for manifests without provenance
//   3. Ensures explicit publish_state is set (defaults to PUBLISHED for legacy)
//   4. Writes marker file artifacts/.trust-migration-complete for idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
)

// syntheticBuildIDNamespace is the fixed UUIDv5 namespace for generating
// deterministic build_id values for pre-Phase-2 artifacts. Hardcoded constant,
// identical across all nodes, never changed.
var syntheticBuildIDNamespace = uuid.MustParse("d4e5f6a7-b8c9-0d1e-2f3a-4b5c6d7e8f90")

const trustMigrationMarker = "artifacts/.trust-migration-complete"
const unclaimedNamespacesFile = "artifacts/.unclaimed-namespaces.json"

// MigrateToTrustModel migrates all existing artifacts into the trust model.
// Idempotent — skips if marker file exists.
func (srv *server) MigrateToTrustModel(ctx context.Context) {
	// Check for marker file.
	if _, err := srv.Storage().ReadFile(ctx, trustMigrationMarker); err == nil {
		slog.Debug("trust model migration already complete")
		return
	}

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		slog.Debug("no artifacts directory, skipping trust migration")
		return
	}

	// Use a service account context for migration operations.
	saCtx := &security.AuthContext{
		Subject:       "sa",
		PrincipalType: "user",
		AuthMethod:    "internal",
	}
	migrationCtx := saCtx.ToContext(ctx)

	publishersSeen := make(map[string]bool)
	var migrated, skipped int

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")

		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil {
			slog.Warn("migration: skip unreadable manifest", "key", key, "err", readErr)
			skipped++
			continue
		}

		changed := false

		// 1. Ensure namespace exists for this publisher.
		pubID := m.GetRef().GetPublisherId()
		if pubID != "" && !publishersSeen[pubID] {
			publishersSeen[pubID] = true
			if err := srv.ensureNamespaceExists(migrationCtx, pubID, "sa", ""); err != nil {
				slog.Warn("migration: namespace creation failed", "publisher", pubID, "err", err)
			}
		}

		// 2. Create synthetic provenance if missing.
		if srv.readProvenance(ctx, key) == nil {
			prov := &repopb.ProvenanceRecord{
				Subject:       "migration",
				PrincipalType: "system",
				AuthMethod:    "none",
				TimestampUnix: time.Now().Unix(),
				BuildCommit:   m.GetBuildCommit(),
				BuildSource:   m.GetBuildSource(),
			}
			if _, err := srv.writeProvenance(ctx, key, prov); err != nil {
				slog.Warn("migration: provenance write failed", "key", key, "err", err)
			}
		}

		// 3. Ensure explicit publish state.
		if state == repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
			mjson, err := marshalManifestWithState(m, repopb.PublishState_PUBLISHED)
			if err == nil {
				if werr := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); werr == nil {
					changed = true
				}
			}
		}

		if changed {
			migrated++
		}
	}

	// Write unclaimed namespaces file — all migrated namespaces are initially unclaimed
	// until a real user claims them via ensureNamespaceExists.
	unclaimedList := make([]string, 0, len(publishersSeen))
	for ns := range publishersSeen {
		unclaimedList = append(unclaimedList, ns)
	}
	if len(unclaimedList) > 0 {
		unclaimedData, _ := json.MarshalIndent(map[string]any{
			"unclaimed_namespaces": unclaimedList,
			"migrated_at":         time.Now().UTC().Format(time.RFC3339),
		}, "", "  ")
		if err := srv.Storage().WriteFile(ctx, unclaimedNamespacesFile, unclaimedData, 0o644); err != nil {
			slog.Warn("migration: unclaimed namespaces file write failed", "err", err)
		}
	}

	// Write marker file.
	marker := map[string]any{
		"migrated_at":          time.Now().UTC().Format(time.RFC3339),
		"publishers":           len(publishersSeen),
		"migrated":             migrated,
		"skipped":              skipped,
		"unclaimed_namespaces": unclaimedList,
	}
	data, _ := json.MarshalIndent(marker, "", "  ")
	if err := srv.Storage().WriteFile(ctx, trustMigrationMarker, data, 0o644); err != nil {
		slog.Warn("migration: marker write failed", "err", err)
	}

	slog.Info("trust model migration complete",
		"publishers", len(publishersSeen),
		"migrated", migrated,
		"skipped", skipped,
		"unclaimed", len(unclaimedList),
	)
}

// ── Phase 2: build_id backfill ──────────────────────────────────────────────

const buildIDMigrationMarker = "artifacts/.build-id-migration-complete"
const buildIDMigrationVersion = "repository-identity-v1"

func buildIDMigrationProvenanceKey(artifactKey string) string {
	return artifactsDir + "/" + artifactKey + ".migration.json"
}

// MigrateBuildIDs ensures every artifact in the repository has a build_id.
// New artifacts (uploaded after Phase 2 Step 2) already have a UUIDv7 build_id.
// Old artifacts receive a deterministic synthetic build_id (UUIDv5) so that
// the entire catalog carries exact identity.
//
// Idempotent — skips if marker file exists. Re-running is safe: existing
// build_id values (UUIDv7 or synthetic) are never overwritten.
func (srv *server) MigrateBuildIDs(ctx context.Context) {
	if _, err := srv.Storage().ReadFile(ctx, buildIDMigrationMarker); err == nil {
		slog.Debug("build_id migration already complete")
		return
	}

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		slog.Debug("no artifacts directory, skipping build_id migration")
		return
	}

	var backfilled, skipped, alreadySet int

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")

		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil {
			slog.Warn("build_id migration: skip unreadable manifest", "key", key, "err", readErr)
			skipped++
			continue
		}

		// Already has build_id — leave it alone.
		if m.GetBuildId() != "" {
			alreadySet++
			continue
		}

		// Generate deterministic synthetic build_id from artifact identity.
		ref := m.GetRef()
		input := fmt.Sprintf("%s/%s/%s/%s/%d",
			ref.GetPublisherId(),
			ref.GetName(),
			ref.GetVersion(),
			ref.GetPlatform(),
			m.GetBuildNumber(),
		)
		syntheticID := uuid.NewSHA1(syntheticBuildIDNamespace, []byte(input)).String()
		m.BuildId = syntheticID

		// Rewrite manifest with build_id populated.
		mjson, err := marshalManifestWithState(m, state)
		if err != nil {
			slog.Warn("build_id migration: marshal failed", "key", key, "err", err)
			skipped++
			continue
		}
		if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
			slog.Warn("build_id migration: write failed", "key", key, "err", err)
			skipped++
			continue
		}

		// Sync to ScyllaDB.
		srv.syncManifestToScylla(ctx, key, m, state, mjson)

		// Write non-destructive migration provenance sidecar.
		provenance := map[string]any{
			"migrated_from_legacy": true,
			"legacy_path":          manifestStorageKey(key),
			"migrated_at":          time.Now().UTC().Format(time.RFC3339),
			"migration_version":    buildIDMigrationVersion,
			"build_id":             syntheticID,
		}
		if p, perr := json.MarshalIndent(provenance, "", "  "); perr == nil {
			if werr := srv.Storage().WriteFile(ctx, buildIDMigrationProvenanceKey(key), p, 0o644); werr != nil {
				slog.Warn("build_id migration: provenance write failed", "key", key, "err", werr)
			}
		}

		// Invalidate cache for this manifest.
		if srv.cache != nil {
			srv.cache.invalidateManifest(manifestStorageKey(key))
		}

		backfilled++
	}

	// Write marker file.
	marker := map[string]any{
		"migrated_at":       time.Now().UTC().Format(time.RFC3339),
		"backfilled":        backfilled,
		"already_set":       alreadySet,
		"skipped":           skipped,
		"migration_version": buildIDMigrationVersion,
	}
	data, _ := json.MarshalIndent(marker, "", "  ")
	if err := srv.Storage().WriteFile(ctx, buildIDMigrationMarker, data, 0o644); err != nil {
		slog.Warn("build_id migration: marker write failed", "err", err)
	}

	slog.Info("build_id migration complete",
		"backfilled", backfilled,
		"already_set", alreadySet,
		"skipped", skipped,
	)
}

// ── Phase 6: legacy → canonical alias backfill ──────────────────────────────

const buildIDAliasMigrationMarker = "artifacts/.build-id-alias-migration-complete"
const buildIDAliasMigrationVersion = "repository-identity-alias-v1"

func buildIDAliasMigrationProvenanceKey(artifactKey string) string {
	return artifactsDir + "/" + artifactKey + ".alias-migration.json"
}

// MigrateBuildIDAliases writes a release/build alias for every legacy artifact
// whose manifest carries an UpstreamImportRecord with release_tag + build_number
// but has no on-disk alias mapping (release_tag, build_number) → canonical
// build_id. Closes Phase 6: legacy upstream references stay resolvable after
// canonical identity is build_id.
//
// Runs after MigrateBuildIDs so every manifest already has a build_id. The
// upstream import record is the only legitimate source of release_tag /
// build_number for an artifact that pre-dates the alias model; manifests
// without an UpstreamImportRecord are skipped (no release context to alias).
//
// Idempotent: ensureReleaseBuildAlias compares canonical_build_id when an
// alias file already exists and either dedupes (same id) or refuses to
// overwrite (conflict). The marker file short-circuits subsequent runs.
func (srv *server) MigrateBuildIDAliases(ctx context.Context) {
	if _, err := srv.Storage().ReadFile(ctx, buildIDAliasMigrationMarker); err == nil {
		slog.Debug("build_id alias migration already complete")
		return
	}

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		slog.Debug("no artifacts directory, skipping build_id alias migration")
		return
	}

	var aliased, skippedNoImport, skippedNoBuildID, conflicts, errors int

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(name, ".manifest.json")

		_, _, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil {
			slog.Warn("alias migration: skip unreadable manifest", "key", key, "err", readErr)
			errors++
			continue
		}

		canonicalBuildID := strings.TrimSpace(m.GetBuildId())
		if canonicalBuildID == "" {
			// Should not happen if MigrateBuildIDs ran first; skip defensively.
			skippedNoBuildID++
			continue
		}

		imp := m.GetUpstreamImport()
		releaseTag := strings.TrimSpace(imp.GetReleaseTag())
		buildNumber := imp.GetBuildNumber()
		if releaseTag == "" || buildNumber <= 0 {
			skippedNoImport++
			continue
		}

		err := srv.ensureReleaseBuildAlias(
			ctx,
			m.GetRef(),
			releaseTag,
			buildNumber,
			"", // upstreamBuildID — legacy manifests don't carry it
			canonicalBuildID,
			m.GetChecksum(),
			imp.GetOriginRelease(),
			imp.GetSourceName(),
		)
		if err != nil {
			if isAliasConflictError(err) {
				slog.Warn("alias migration: conflict — leaving existing alias",
					"key", key, "release_tag", releaseTag,
					"build_number", buildNumber, "err", err)
				conflicts++
				continue
			}
			slog.Warn("alias migration: ensureReleaseBuildAlias failed",
				"key", key, "release_tag", releaseTag,
				"build_number", buildNumber, "err", err)
			errors++
			continue
		}

		provenance := map[string]any{
			"migrated_from_legacy": true,
			"legacy_path":          manifestStorageKey(key),
			"migrated_at":          time.Now().UTC().Format(time.RFC3339),
			"migration_version":    buildIDAliasMigrationVersion,
			"build_id":             canonicalBuildID,
			"release_tag":          releaseTag,
			"build_number":         buildNumber,
			"source":               imp.GetSourceName(),
		}
		if p, perr := json.MarshalIndent(provenance, "", "  "); perr == nil {
			if werr := srv.Storage().WriteFile(ctx, buildIDAliasMigrationProvenanceKey(key), p, 0o644); werr != nil {
				slog.Warn("alias migration: provenance write failed", "key", key, "err", werr)
			}
		}

		aliased++
	}

	marker := map[string]any{
		"migrated_at":         time.Now().UTC().Format(time.RFC3339),
		"aliased":             aliased,
		"skipped_no_import":   skippedNoImport,
		"skipped_no_build_id": skippedNoBuildID,
		"conflicts":           conflicts,
		"errors":              errors,
		"migration_version":   buildIDAliasMigrationVersion,
	}
	data, _ := json.MarshalIndent(marker, "", "  ")
	if err := srv.Storage().WriteFile(ctx, buildIDAliasMigrationMarker, data, 0o644); err != nil {
		slog.Warn("alias migration: marker write failed", "err", err)
	}

	slog.Info("build_id alias migration complete",
		"aliased", aliased,
		"skipped_no_import", skippedNoImport,
		"skipped_no_build_id", skippedNoBuildID,
		"conflicts", conflicts,
		"errors", errors,
	)
}

// readUnclaimedNamespaces returns the list of namespaces that were migrated but not yet
// claimed by a real user. Returns nil if the file does not exist.
func (srv *server) readUnclaimedNamespaces(ctx context.Context) []string {
	data, err := srv.Storage().ReadFile(ctx, unclaimedNamespacesFile)
	if err != nil {
		return nil
	}
	var doc struct {
		UnclaimedNamespaces []string `json:"unclaimed_namespaces"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	return doc.UnclaimedNamespaces
}

// removeUnclaimedNamespace removes a namespace from the unclaimed list when a real user
// claims it. This is called from ensureNamespaceExists when the owner is not "sa".
func (srv *server) removeUnclaimedNamespace(ctx context.Context, namespace string) {
	data, err := srv.Storage().ReadFile(ctx, unclaimedNamespacesFile)
	if err != nil {
		return // file doesn't exist, nothing to do
	}

	var doc struct {
		UnclaimedNamespaces []string `json:"unclaimed_namespaces"`
		MigratedAt          string   `json:"migrated_at"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return
	}

	// Filter out the claimed namespace.
	var updated []string
	found := false
	for _, ns := range doc.UnclaimedNamespaces {
		if ns == namespace {
			found = true
			continue
		}
		updated = append(updated, ns)
	}
	if !found {
		return
	}

	doc.UnclaimedNamespaces = updated
	newData, _ := json.MarshalIndent(map[string]any{
		"unclaimed_namespaces": updated,
		"migrated_at":          doc.MigratedAt,
		"last_claim_at":        time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err := srv.Storage().WriteFile(ctx, unclaimedNamespacesFile, newData, 0o644); err != nil {
		slog.Warn("failed to update unclaimed namespaces file", "err", err)
	}
	slog.Info("namespace claimed, removed from unclaimed list", "namespace", namespace)
}
