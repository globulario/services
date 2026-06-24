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
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/digest"
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

// directPublishChannelGate is the pure release-authority decision for the
// direct publish path (UploadArtifact), the P3 parity for AllocateUpload. Given
// the artifact's resolved channel, its publisher, and whether the caller holds
// release authority on the namespace, it returns the final channel and whether
// the caller must be rejected.
//
// Rules (only STABLE — the convergeable channel — is gated):
//   - authorized, or channel ≠ STABLE → unchanged (pass through).
//   - unauthorized STABLE, non-official publisher → forced to DEV. This is what
//     makes agent / `globular pkg publish` / MCP builds DEV by construction: a
//     caller with write access but no release.allocate cannot land STABLE.
//   - unauthorized STABLE, official publisher → reject. The official namespace is
//     sealed and cannot be DEV (identity-lane Rule 2), so there is no safe
//     downgrade — release authority is mandatory, not optional.
//
// Pure (no RBAC, no state) so the policy is unit-tested directly; the caller
// supplies `authorized` from authorizeRelease.
func directPublishChannelGate(effective repopb.ArtifactChannel, publisherID string, authorized bool) (final repopb.ArtifactChannel, rejectOfficial bool) {
	if authorized || effective != repopb.ArtifactChannel_STABLE {
		return effective, false
	}
	if isOfficialPublisher(publisherID) {
		return effective, true // sealed official namespace: no downgrade, reject
	}
	return repopb.ArtifactChannel_DEV, false
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
//   - use a local identity lane (different publisher + local version suffix), OR
//   - (the proven-phantom escape hatch) present a RepairAuthorization via gRPC
//     metadata `x-repair-unseal-official: true` + `x-repair-reason: <text>` +
//     `x-repair-prior-digest: <expected sealed digest>`. The repair path is
//     audited verbatim and rejects malformed authorizations. See
//     repair_authorization.go for the contract.
//
// This check is a no-op for non-official publishers and non-STABLE channels.
//
// `repair` is the parsed repair authorization from the request context. A nil
// value means no repair was requested — the seal is enforced absolutely.
// A non-nil value triggers the repair-path checks: prior digest match,
// non-empty reason. Authorization-flag-set-but-mismatch returns
// PermissionDenied / InvalidArgument / FailedPrecondition with a precise
// reason so the operator can fix exactly what's wrong rather than guessing.
func (srv *server) enforceOfficialNamespaceSeal(ctx context.Context, publisherID, name, version, platform, incomingDigest string, ch repopb.ArtifactChannel, repair *RepairAuthorization) error {
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
	// If digests differ: this is identity fraud — reject with PERMISSION_DENIED
	// UNLESS the caller has presented a valid RepairAuthorization (proven-phantom
	// escape hatch).
	existingDigest := srv.getPublishedDigest(ctx, publisherID, name, version, platform)
	if existingDigest == "" {
		// Ledger has the version but we can't verify the digest — treat as
		// "already published, immutable" rather than silently accepting.
		// Repair path also requires a known prior digest to confirm intent,
		// so this branch is unreachable via repair anyway.
		return status.Errorf(codes.AlreadyExists,
			"official namespace sealed: %s@%s is already published for %s on %s — "+
				"use 'globular pkg publish --bump' to create a new version",
			name, version, publisherID, platform)
	}

	if digestsMatch(existingDigest, incomingDigest) {
		return nil
	}

	// Digests differ. Default policy: reject. Repair path: validate the
	// authorization and let the caller proceed if and only if every gate
	// passes. Each rejection branch returns a precise reason so the operator
	// can fix exactly what's wrong rather than guessing at the contract.
	if repair != nil && repair.Requested {
		if strings.TrimSpace(repair.Reason) == "" {
			return status.Errorf(codes.InvalidArgument,
				"repair-unseal rejected: empty reason — provide --reason \"<why>\" describing why the sealed %s/%s@%s is being repaired",
				publisherID, name, version)
		}
		if !digestsMatch(repair.PriorDigest, existingDigest) {
			return status.Errorf(codes.FailedPrecondition,
				"repair-unseal rejected: prior-digest mismatch — caller asserted prior=%s but actual sealed digest is %s. "+
					"Inspect via 'globular pkg describe' or repository_explain_artifact and re-issue the repair with the correct --prior-digest.",
				shortDigest(repair.PriorDigest), shortDigest(existingDigest))
		}
		// Authorization passed all gates. Mark the repair as consumed so the
		// post-success audit in UploadArtifact can record the full prior-vs-
		// new identity. Log at decision time for live debugging; the
		// authoritative audit event fires only AFTER completePublish
		// succeeds — that way a rejected upload doesn't leave a misleading
		// "unseal authorized" trail.
		repair.Used = true
		if repair.PriorBuildID == "" {
			repair.PriorBuildID = existingBuildID
		}
		slog.Warn("seal repair authorized — bypassing official-namespace seal",
			"publisher", publisherID, "name", name, "version", version, "platform", platform,
			"prior_digest", existingDigest, "new_digest", incomingDigest,
			"prior_build_id", existingBuildID, "reason", repair.Reason,
		)
		return nil
	}

	return status.Errorf(codes.PermissionDenied,
		"official identity conflict: %s/%s@%s on %s is SEALED (digest=%s) — "+
			"incoming artifact has a different digest (%s). "+
			"Official stable artifacts are immutable. "+
			"To test a local fix: use a local publisher (e.g. local@<cluster-id>) "+
			"and a local version suffix (e.g. %s+local.<cluster>.1) with --channel dev. "+
			"To repair a proven-phantom sealed artifact, re-issue with "+
			"--unseal-official --reason \"<why>\" --prior-digest %s",
		publisherID, name, version, platform,
		shortDigest(existingDigest),
		shortDigest(incomingDigest),
		version, shortDigest(existingDigest))
}

// shortDigest returns a log-friendly truncated form of a sha256 digest
// (sha256: prefix stripped, first 16 hex chars). Sha256 prefixes are
// effectively unique at 8+ chars, so 16 is a comfortable display length.
// Use this in error messages and audit logs; do NOT use for equality
// (use digestsMatch which normalizes both inputs).
func shortDigest(d string) string {
	s := strings.TrimPrefix(strings.TrimSpace(d), "sha256:")
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

// digestsMatch returns true if two digest strings represent the same content.
// Handles both bare hex and "sha256:"-prefixed forms.
func digestsMatch(a, b string) bool {
	return digest.CanonicalSHA256(a) == digest.CanonicalSHA256(b)
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

// devLaneVersion returns a lane-safe DEV version that never advances the release
// stream (P5). An already-suffixed version is kept as-is; otherwise the base is
// pinned to the latest published release (or the resolved version when there is
// no release yet) and a `-dev` pre-release suffix is applied — so the DEV build
// semver-orders BELOW the release and cannot squat a clean release version.
// build_number then iterates within that DEV version.
//
// Pure given (latestRelease, resolvedVersion); the caller supplies the latest
// release, so the policy is unit-tested without a ledger.
func devLaneVersion(latestRelease, resolvedVersion string) string {
	if hasLocalVersionSuffix(resolvedVersion) {
		return resolvedVersion // already lane-safe (dev/local/hotfix)
	}
	base := strings.TrimSpace(latestRelease)
	if base == "" {
		base = resolvedVersion // no release to pin to; suffix the resolved version
	}
	return FormatLocalVersion(base, "dev", "", 0) // base + "-dev.1"
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
