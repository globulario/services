// @awareness namespace=globular.platform
// @awareness component=platform_repository.blob_seed
// @awareness file_role=day1_local_cas_seeder_from_staged_join_packages
// @awareness implements=globular.platform:intent.repository.local_cas_is_installability_authority
// @awareness implements=globular.platform:intent.repository.metadata_is_authority
// @awareness risk=high
package main

// blob_seed.go — Day-1 local CAS seeding from staged join packages.
//
// PROBLEM: on a Day-1 join, the gateway stages every package archive to
// /var/lib/globular/packages/ (the node-agent install cache) BEFORE the
// node-agent starts — but nothing populated the repository service's own
// blob store (/var/lib/globular/repository/artifacts/). A repository
// instance on a freshly joined node therefore reported
// repository.identity.missing_blob_for_published_manifest for every
// PUBLISHED artifact, and any install routed to that instance could not be
// served locally. Observed live on the globule-dell + globule-nuc joins,
// 2026-06-12 (57 findings).
//
// FIX: at startup, once the Scylla manifest authority is reachable, walk
// every PUBLISHED manifest and, for each blob missing from the local CAS,
// look for the staged archive in the join-package directories. The Scylla
// manifest is the authority (intent:repository.metadata_is_authority); the
// staged file is only evidence — it is materialized into the CAS strictly
// when its sha256 digest matches the manifest checksum, via an atomic,
// digest-verified write. Filenames narrow the search but NEVER decide
// identity (rule: do not infer truth from filenames when a manifest exists).
//
// MinIO is deliberately NOT involved anywhere in this path: packages never
// live in MinIO (see initStorage).

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// stagedPackageDirs are the directories the Day-0/Day-1 flows stage package
// archives into on every node, in search order. Mirrors the node-agent's
// localPackageDirs (installer_api.go) — the gateway's /join/packages/
// endpoint writes here before the node-agent starts.
var stagedPackageDirs = []string{
	"/var/lib/globular/packages/pinned",
	"/var/lib/globular/packages",
	"/var/lib/globular/staging/local",
}

// seedPollInterval / seedMaxWait bound the wait for the Scylla manifest
// authority at startup. If Scylla never becomes ready the seeder gives up
// loudly — it never blocks serving (local CAS reads need no Scylla).
const (
	seedPollInterval = 15 * time.Second
	seedMaxWait      = 10 * time.Minute
)

// seedLocalCASFromStagedPackages is the startup entry point. It waits for
// the manifest authority, then runs one seeding pass. Run as a goroutine;
// it logs its outcome and exits.
func (srv *server) seedLocalCASFromStagedPackages(ctx context.Context) {
	deadline := time.Now().Add(seedMaxWait)
	for srv.requireCapability(CapRepoQuery) != nil {
		if time.Now().After(deadline) {
			logger.Warn("blob-seed: Scylla manifest authority not ready within wait window — seeding skipped (will not retry until next restart)",
				"waited", seedMaxWait.String())
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(seedPollInterval):
		}
	}

	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		logger.Warn("blob-seed: ListManifests failed — seeding skipped", "err", err)
		return
	}
	seeded, present, unmatched := srv.seedManifestRowsFromDirs(ctx, rows, stagedPackageDirs)
	logger.Info("blob-seed: local CAS seeding pass complete",
		"published_manifests", len(rows),
		"already_present", present,
		"seeded_from_staged", seeded,
		"no_staged_match", unmatched,
		"scope", "this node's local POSIX CAS only",
	)
}

// seedManifestRowsFromDirs is the testable core: for every PUBLISHED manifest
// whose blob is missing from the local CAS, find a digest-matching staged
// archive and materialize it (blob + manifest sidecar) atomically.
//
// Returns (seeded, alreadyPresent, unmatched) counts. Rows that are not
// PUBLISHED are ignored entirely — seeding must never make a non-published
// artifact look present (half-done must not look done).
func (srv *server) seedManifestRowsFromDirs(ctx context.Context, rows []manifestRow, dirs []string) (seeded, alreadyPresent, unmatched int) {
	if srv.localStorage == nil {
		logger.Warn("blob-seed: local storage not initialised — seeding skipped")
		return 0, 0, 0
	}
	for i := range rows {
		row := &rows[i]
		if ctx.Err() != nil {
			return seeded, alreadyPresent, unmatched
		}
		if !strings.EqualFold(strings.TrimSpace(row.PublishState), "published") {
			continue
		}
		blobKey := binaryStorageKey(row.ArtifactKey)
		if _, err := srv.localStorage.Stat(ctx, blobKey); err == nil {
			alreadyPresent++
			continue
		}

		staged := findStagedArchiveByDigest(row, dirs)
		if staged == "" {
			unmatched++
			logger.Info("blob-seed: no digest-matching staged archive on this node — blob stays missing locally (serve via another instance or repair)",
				"artifact", row.ArtifactKey, "checksum", truncDigest(row.Checksum))
			continue
		}

		if err := srv.materializeStagedArchive(ctx, row, staged, blobKey); err != nil {
			unmatched++
			logger.Warn("blob-seed: materialize failed", "artifact", row.ArtifactKey, "staged", staged, "err", err)
			continue
		}
		seeded++
		logger.Info("blob-seed: materialized blob into local CAS from staged join package",
			"artifact", row.ArtifactKey, "staged", staged, "checksum", truncDigest(row.Checksum))
	}
	return seeded, alreadyPresent, unmatched
}

// materializeStagedArchive writes the staged archive into the local CAS via
// the atomic, digest-and-size-verified write, then writes the manifest
// sidecar so the local layout matches a Day-0 publish.
func (srv *server) materializeStagedArchive(ctx context.Context, row *manifestRow, stagedPath, blobKey string) error {
	f, err := os.Open(stagedPath)
	if err != nil {
		return err
	}
	defer f.Close()

	expectedSize := row.SizeBytes // 0 disables the size check in WriteFileAtomic
	if _, err := srv.localStorage.WriteFileAtomic(ctx, blobKey, f, canonicalDigest(row.Checksum), expectedSize); err != nil {
		return err
	}
	if len(row.ManifestJSON) > 0 {
		if err := srv.localStorage.WriteFile(ctx, manifestStorageKey(row.ArtifactKey), row.ManifestJSON, 0o644); err != nil {
			// Blob landed and is digest-verified; a sidecar write failure only
			// degrades local-listing paths. Log loudly, do not undo the blob.
			logger.Warn("blob-seed: blob seeded but manifest sidecar write failed",
				"artifact", row.ArtifactKey, "err", err)
		}
	}
	return nil
}

// findStagedArchiveByDigest searches the staged-package directories for an
// archive whose sha256 matches the manifest checksum. Filename patterns only
// NARROW the candidate set (cheap); the digest match is the sole gate.
func findStagedArchiveByDigest(row *manifestRow, dirs []string) string {
	want := row.Checksum
	if normalizeDigest(want) == "" {
		return "" // no authority digest — never seed unverified bytes
	}
	for _, dir := range dirs {
		for _, pattern := range stagedArchivePatterns(row.Name, row.Version, row.Platform) {
			matches, err := filepath.Glob(filepath.Join(dir, pattern))
			if err != nil {
				continue
			}
			for _, candidate := range matches {
				got, err := checksumLocalFile(candidate)
				if err != nil {
					continue
				}
				if digestEqual(got, want) {
					return candidate
				}
			}
		}
	}
	return ""
}

// stagedArchivePatterns mirrors the node-agent's archive naming forms
// (installer_api.go): exact triple first, then progressively looser. Loose
// patterns are safe because the digest gate decides, never the name.
func stagedArchivePatterns(name, version, platform string) []string {
	return []string{
		name + "_" + version + "_" + platform + ".tgz",
		name + "_" + version + ".tgz",
		name + "_*_" + platform + ".tgz",
		name + ".tgz",
	}
}
