// @awareness namespace=globular.platform
// @awareness component=repository.local_publish_guard
// @awareness file_role=identity_lane_enforcer
// @awareness enforces=globular.platform:invariant.package.official_identity_immutable
// @awareness enforces=globular.platform:invariant.package.local_publish_requires_local_identity
// @awareness enforces=globular.platform:invariant.package.promotion_must_use_official_release_pipeline
// @awareness risk=high
package main

// local_publish_guard.go — Identity lane enforcement for local/dev/hotfix builds.
//
// The official stable namespace (publisher=core@globular.io, channel=STABLE) is
// SEALED: once a (version, platform) is published with a given digest, no other
// byte stream may claim that identity. A local fix must use a distinct identity.
//
// Identity lanes:
//
//   stable  → publisher=core@globular.io  channel=STABLE     version=semver (no suffix)
//   dev     → publisher=local@<id>         channel=DEV        version with +local./-dev. suffix
//   hotfix  → publisher=<any>              channel=CANDIDATE  version with -hotfix. suffix
//
// Enforcement entry points:
//   - validateLocalIdentityRules()  called from UploadArtifact before any writes
//   - enforceOfficialNamespaceSeal() called from UploadArtifact to block digest fraud
//
// Neither function modifies state — they only return errors.

import (
	"context"
	"fmt"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// officialPublisher is the sole publisher allowed to produce STABLE artifacts
// in the canonical namespace.
const officialPublisher = "core@globular.io"

// isOfficialPublisher returns true for the canonical stable publisher.
func isOfficialPublisher(publisherID string) bool {
	return strings.EqualFold(strings.TrimSpace(publisherID), officialPublisher)
}

// isLocalChannel returns true for channels that mark non-stable local/dev/hotfix builds.
func isLocalChannel(ch repopb.ArtifactChannel) bool {
	switch ch {
	case repopb.ArtifactChannel_DEV,
		repopb.ArtifactChannel_CANDIDATE,
		repopb.ArtifactChannel_CANARY:
		return true
	}
	return false
}

// hasLocalVersionSuffix returns true if the version string carries a local/dev/hotfix
// suffix, indicating the artifact is not a clean official release.
//
//	+local.<cluster>.<n>   e.g. 1.2.43+local.ryzen.1
//	-dev.<branch>.<sha>    e.g. 1.2.43-dev.fix-retry.a1b2c3
//	-hotfix.<n>            e.g. 1.2.43-hotfix.1
func hasLocalVersionSuffix(version string) bool {
	lower := strings.ToLower(version)
	return strings.Contains(lower, "+local.") ||
		strings.Contains(lower, "-dev.") ||
		strings.Contains(lower, "-hotfix.") ||
		strings.Contains(lower, "+dev.") ||
		strings.Contains(lower, "+hotfix.")
}

// validateLocalIdentityRules enforces that local/dev/hotfix artifacts cannot
// masquerade as official stable artifacts.
//
// Rules:
//  1. If channel=STABLE (or unset) AND publisher=official → version must NOT have
//     a local suffix. Official stable artifacts must be clean semver only.
//  2. If channel=DEV → publisher must NOT be official. Local test builds must use
//     a non-official publisher (e.g. local@<cluster-id>, org@<domain>).
//  3. If version has a local suffix → channel must NOT be STABLE with official publisher.
//     Mixing official stable identity with a modified version is forbidden.
func validateLocalIdentityRules(publisherID string, ch repopb.ArtifactChannel, version string) error {
	effective := ch
	if effective == repopb.ArtifactChannel_CHANNEL_UNSET {
		effective = repopb.ArtifactChannel_STABLE
	}

	localSuffix := hasLocalVersionSuffix(version)
	official := isOfficialPublisher(publisherID)

	// Rule 1: official stable artifacts must use clean semver — no local suffixes.
	if official && effective == repopb.ArtifactChannel_STABLE && localSuffix {
		return status.Errorf(codes.InvalidArgument,
			"identity lane violation: publisher %q with channel STABLE may not use a local version suffix %q — "+
				"use channel=dev (DEV) or channel=hotfix (CANDIDATE) and a non-official publisher for local test builds",
			publisherID, version)
	}

	// Rule 2: DEV channel artifacts must not use the official publisher.
	// Official publisher + DEV channel would allow a local build to pollute the
	// official namespace in resolution — only non-official publishers may publish DEV.
	if official && effective == repopb.ArtifactChannel_DEV {
		return status.Errorf(codes.InvalidArgument,
			"identity lane violation: publisher %q may not publish to DEV channel — "+
				"use a local publisher (e.g. local@<cluster-id>) for local test builds",
			publisherID)
	}

	// Rule 3: version with local suffix + official publisher + stable = forbidden.
	// Belt-and-suspenders guard for channels other than DEV where the caller
	// might not set channel correctly but still carries a local version.
	if official && effective == repopb.ArtifactChannel_STABLE && localSuffix {
		return status.Errorf(codes.InvalidArgument,
			"identity lane violation: local version suffix %q cannot be published as official stable — "+
				"bump the version and use the official release pipeline", version)
	}

	return nil
}

// enforceOfficialNamespaceSeal checks that no artifact with different bytes can
// claim an already-published official stable (publisher, name, version, platform)
// identity. This is the strongest protection against silent identity fraud.
//
// If the official namespace already has a published artifact at this
// (publisher, name, version, platform) with a DIFFERENT digest, the upload
// must be rejected immediately (before any writes). The caller is expected to:
//   - bump the version (to create a new official release), OR
//   - use a local identity lane (different publisher + local version suffix)
//
// This check is a no-op for non-official publishers and non-STABLE channels.
func (srv *server) enforceOfficialNamespaceSeal(ctx context.Context, publisherID, name, version, platform, incomingDigest string, ch repopb.ArtifactChannel) error {
	effective := ch
	if effective == repopb.ArtifactChannel_CHANNEL_UNSET {
		effective = repopb.ArtifactChannel_STABLE
	}

	// Seal only applies to the official stable namespace.
	if !isOfficialPublisher(publisherID) || effective != repopb.ArtifactChannel_STABLE {
		return nil
	}

	// No-op when version contains a local suffix — validateLocalIdentityRules
	// already rejected that combination. Don't double-fault here.
	if hasLocalVersionSuffix(version) {
		return nil
	}

	// Look up the published ledger for this (publisher, name, platform).
	existingBuildID := srv.getExactRelease(ctx, publisherID, name, version, platform)
	if existingBuildID == "" {
		return nil // nothing published yet — new version, allow
	}

	// The version is already in the ledger. Check if the digest matches.
	// If it does, UploadArtifact's idempotency check (findExistingArtifactByDigest)
	// will catch it earlier and return the existing build_id as success.
	// If digests differ: this is identity fraud — reject with PERMISSION_DENIED.
	existingDigest := srv.getPublishedDigest(ctx, publisherID, name, version, platform)
	if existingDigest == "" {
		// Ledger has the version but we can't verify the digest — treat as
		// "already published, immutable" rather than silently accepting.
		return status.Errorf(codes.AlreadyExists,
			"official namespace sealed: %s@%s is already published for %s on %s — "+
				"use 'globular pkg publish --bump' to create a new version",
			name, version, publisherID, platform)
	}

	if !digestsMatch(existingDigest, incomingDigest) {
		return status.Errorf(codes.PermissionDenied,
			"official identity conflict: %s/%s@%s on %s is SEALED (digest=%s) — "+
				"incoming artifact has a different digest (%s). "+
				"Official stable artifacts are immutable. "+
				"To test a local fix: use a local publisher (e.g. local@<cluster-id>) "+
				"and a local version suffix (e.g. %s+local.<cluster>.1) with --channel dev",
			publisherID, name, version, platform,
			existingDigest[:min(16, len(existingDigest))],
			incomingDigest[:min(16, len(incomingDigest))],
			version)
	}

	return nil
}

// digestsMatch returns true if two digest strings represent the same content.
// Handles both bare hex and "sha256:"-prefixed forms.
func digestsMatch(a, b string) bool {
	return normalizeDigest(a) == normalizeDigest(b)
}

// getPublishedDigest returns the checksum for an exact (publisher, name, version,
// platform) tuple from the release ledger, or "" if not found.
func (srv *server) getPublishedDigest(ctx context.Context, publisher, name, version, platform string) string {
	ledger := srv.readLedger(ctx, publisher, name)
	if ledger == nil {
		return ""
	}
	for _, r := range ledger.Releases {
		if r.Version == version && (r.Platform == platform || platform == "") {
			return r.Digest
		}
	}
	return ""
}

// FormatLocalVersion formats a local version string from an upstream version
// and a local qualifier. Used by the CLI and tests.
//
//	FormatLocalVersion("1.2.43", "local", "ryzen", 1) → "1.2.43+local.ryzen.1"
//	FormatLocalVersion("1.2.43", "dev",   "fix-retry", 0) → "1.2.43-dev.fix-retry"
//	FormatLocalVersion("1.2.43", "hotfix", "", 2) → "1.2.43-hotfix.2"
func FormatLocalVersion(baseVersion, lane, qualifier string, n int) string {
	v := strings.TrimPrefix(baseVersion, "v")
	switch lane {
	case "hotfix":
		if n > 0 {
			return fmt.Sprintf("%s-hotfix.%d", v, n)
		}
		return v + "-hotfix.1"
	case "dev":
		if qualifier != "" && n > 0 {
			return fmt.Sprintf("%s-dev.%s.%d", v, qualifier, n)
		}
		if qualifier != "" {
			return fmt.Sprintf("%s-dev.%s", v, qualifier)
		}
		return v + "-dev.1"
	default: // local
		if qualifier != "" && n > 0 {
			return fmt.Sprintf("%s+local.%s.%d", v, qualifier, n)
		}
		if qualifier != "" {
			return fmt.Sprintf("%s+local.%s", v, qualifier)
		}
		return v + "+local.1"
	}
}
