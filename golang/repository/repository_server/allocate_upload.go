// @awareness namespace=globular.platform
// @awareness component=platform_repository.allocate_upload
// @awareness file_role=exclusive_version_reservation_with_immutability_and_monotonicity_enforcement
// @awareness implements=globular.platform:intent.repository.version_allocation_is_exclusive
// @awareness implements=globular.platform:intent.repository.publish_is_idempotent_by_digest
// @awareness risk=high
package main

// allocate_upload.go — Phase 4: Upload allocation protocol.
//
// AllocateUpload reserves a version and pre-assigns a build_id before the
// client uploads artifact data. The repository is the sole allocator of
// release identity — clients express intent (BUMP_PATCH, BUMP_MINOR, etc.),
// the repository decides the actual version.
//
// Reservations are short-lived (5 min TTL) and keyed on
// (publisher, name, version, platform). Only one reservation per key at a
// time — second caller gets ResourceExhausted.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const reservationTTL = 5 * time.Minute

// reservation tracks an active upload allocation.
type reservation struct {
	ID          string
	Publisher   string
	Name        string
	Version     string
	Platform    string
	BuildID     string
	BuildNumber int64
	Channel     repopb.ArtifactChannel
	ExpiresAt   time.Time
}

// reservationStore manages active reservations in memory.
// For a single-cluster deployment this is sufficient. For multi-instance
// repository deployments, reservations should be stored in ScyllaDB.
type reservationStore struct {
	mu           sync.Mutex
	reservations map[string]*reservation // key: publisher%name%version%platform
}

var reservations = &reservationStore{
	reservations: make(map[string]*reservation),
}

func reservationKey(publisher, name, version, platform string) string {
	return publisher + "%" + name + "%" + version + "%" + platform
}

// allocate creates a new reservation. Returns ResourceExhausted if one exists.
func (rs *reservationStore) allocate(publisher, name, version, platform, buildID string, buildNumber int64, channel repopb.ArtifactChannel) (*reservation, error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	key := reservationKey(publisher, name, version, platform)

	// Check for existing active reservation.
	if existing, ok := rs.reservations[key]; ok {
		if time.Now().Before(existing.ExpiresAt) {
			return nil, fmt.Errorf("reservation already active for %s (expires %s)",
				key, existing.ExpiresAt.Format(time.RFC3339))
		}
		// Expired — clean up.
		delete(rs.reservations, key)
	}

	res := &reservation{
		ID:          "res_" + uuid.Must(uuid.NewV7()).String()[:8],
		Publisher:   publisher,
		Name:        name,
		Version:     version,
		Platform:    platform,
		BuildID:     buildID,
		BuildNumber: buildNumber,
		Channel:     channel,
		ExpiresAt:   time.Now().Add(reservationTTL),
	}
	rs.reservations[key] = res
	return res, nil
}

// consume removes a reservation by ID, returning it if found and not expired.
func (rs *reservationStore) consume(reservationID string) *reservation {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	for key, res := range rs.reservations {
		if res.ID == reservationID {
			delete(rs.reservations, key)
			if time.Now().After(res.ExpiresAt) {
				return nil // expired
			}
			return res
		}
	}
	return nil
}

// cleanup removes expired reservations. Called periodically.
func (rs *reservationStore) cleanup() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	now := time.Now()
	for key, res := range rs.reservations {
		if now.After(res.ExpiresAt) {
			delete(rs.reservations, key)
		}
	}
}

// ── RPC Handler ─────────────────────────────────────────────────────────

// AllocateUpload implements the Phase 4 allocation protocol.
func (srv *server) AllocateUpload(ctx context.Context, req *repopb.AllocateUploadRequest) (*repopb.AllocateUploadResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}

	publisher := strings.TrimSpace(req.GetPublisherId())
	name := strings.TrimSpace(req.GetName())
	platform := strings.TrimSpace(req.GetPlatform())

	if publisher == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id is required")
	}
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if platform == "" {
		platform = "linux_amd64"
	}

	// Resolve channel — default to STABLE.
	ch := req.GetChannel()
	if ch == repopb.ArtifactChannel_CHANNEL_UNSET {
		ch = repopb.ArtifactChannel_STABLE
	}
	if ch != repopb.ArtifactChannel_STABLE && req.GetIntent() != repopb.VersionIntent_EXACT {
		return nil, status.Errorf(codes.InvalidArgument,
			"non-STABLE uploads must use intent=EXACT with the platform version; repository allocates build_number under that version")
	}

	// Resolve version from intent. AllocateUpload is the reservation flow
	// used for repository-owned version/build allocation. Repair is not
	// meaningful here; direct UploadArtifact threads repair authorization
	// through artifact_handlers.go.
	version, err := srv.resolveVersionIntent(ctx, publisher, name, platform, req.GetIntent(), req.GetExactVersion(), ch, nil)
	if err != nil {
		return nil, err
	}

	// Generate build_id. build_number is computed after the channel and version
	// are final.
	buildID := uuid.Must(uuid.NewV7()).String()

	// RBAC-native release gate (docs/design/package-lifecycle.md §3.4).
	// Targeting STABLE requires the federated subject to hold release.allocate on
	// the publisher namespace. Federation happened in the auth interceptor
	// (step 1, resolveForgeIdentity); here we AUTHORIZE only (step 2). A subject
	// without the permission is forced to DEV (never granted STABLE); an
	// authenticated caller with no subject is denied entirely (fail closed).
	// CI holds no implicit privilege — it must resolve to a subject and pass this
	// check like any other caller (invariant package.ci_is_not_release_authority).
	if ch == repopb.ArtifactChannel_STABLE {
		id := srv.resolveForgeIdentity(ctx)
		allow, aerr := srv.authorizeRelease(ctx, id, publisher)
		if aerr != nil {
			return nil, aerr
		}
		// Shared decision with the direct-publish gate (releaseChannelDecision):
		// unauthorized STABLE → DEV for a normal namespace; the sealed official
		// namespace cannot be DEV (identity-lane Rule 2), so it is rejected here
		// rather than handed a doomed official-DEV reservation that UploadArtifact
		// would reject at upload. One rule, both gates.
		final, rejectOfficial := releaseChannelDecision(ch, publisher, allow)
		if rejectOfficial {
			return nil, status.Errorf(codes.PermissionDenied,
				"publishing %q to STABLE requires release.allocate on the namespace; "+
					"local/agent builds must use a non-official publisher (DEV lane)", publisher)
		}
		if final != ch {
			slog.Warn("release-authority: subject lacks release.allocate, forcing channel DEV",
				"publisher", publisher, "name", name, "subject", id.Subject)
			ch = final
		}
	}

	// build_number is counted against the repository-resolved version.
	buildNumber := srv.resolveLatestBuildNumber(ctx, &repopb.ArtifactRef{
		PublisherId: publisher, Name: name, Version: version, Platform: platform,
	}) + 1

	// Create reservation.
	res, err := reservations.allocate(publisher, name, version, platform, buildID, buildNumber, ch)
	if err != nil {
		return nil, status.Errorf(codes.ResourceExhausted,
			"version %s already reserved for %s/%s: %v", version, publisher, name, err)
	}

	slog.Info("upload allocated",
		"publisher", publisher, "name", name, "version", version,
		"build_id", buildID, "build_number", buildNumber,
		"reservation_id", res.ID, "expires", res.ExpiresAt.Format(time.RFC3339))

	return &repopb.AllocateUploadResponse{
		Version:       version,
		ReservationId: res.ID,
		BuildId:       buildID,
		BuildNumber:   buildNumber,
	}, nil
}

// resolveVersionIntent computes the actual version from the client's intent.
//
// `repair` is the Phase 31/32 repair authorization parsed from gRPC metadata.
// A nil value means no repair was requested — version immutability is
// enforced absolutely (default and correct behaviour for normal publishes).
// A non-nil + valid repair authorization (prior-digest match + non-empty
// reason) is the ONLY way to legitimately re-publish a version already in
// the PUBLISHED ledger; this is the second immutability gate Phase 32
// extends after Phase 31 covered the first (enforceOfficialNamespaceSeal).
// See repair_authorization.go for the contract.
func (srv *server) resolveVersionIntent(ctx context.Context, publisher, name, platform string, intent repopb.VersionIntent, exactVersion string, channel repopb.ArtifactChannel, repair *RepairAuthorization) (string, error) {
	switch intent {
	case repopb.VersionIntent_EXACT:
		if exactVersion == "" {
			return "", status.Error(codes.InvalidArgument, "exact_version is required when intent=EXACT")
		}
		cv, err := versionutil.NormalizeExact(exactVersion)
		if err != nil {
			return "", status.Errorf(codes.InvalidArgument, "invalid version %q: %v", exactVersion, err)
		}
		// Version immutability: if (name, version, platform) is already in the
		// PUBLISHED ledger, reject the allocation. Re-publishing the same version
		// generates a new build_id for an identical artifact — the old build_id
		// stays installed on nodes, the new one enters desired state, and every
		// node in the cluster shows "build drift" forever.
		//
		// Repair escape hatch (Phase 32): if the caller presented a valid
		// RepairAuthorization, the version-immutability gate accepts the
		// re-publish iff prior-digest matches the actually-published digest
		// AND reason is non-empty. Same four gates as the seal check; same
		// audit event timing (post-success in UploadArtifact).
		// STABLE exact versions are immutable. Non-STABLE/local channels may
		// reuse the platform version; build_number/build_id carry the local
		// iteration identity and are allocated by this repository reservation.
		if existingBuildID := srv.getExactRelease(ctx, publisher, name, cv, platform); existingBuildID != "" && channel == repopb.ArtifactChannel_STABLE {
			if repair == nil || !repair.Requested {
				return "", status.Errorf(codes.AlreadyExists,
					"version %s is already published for %s/%s on %s (build_id=%.8s) — published versions are immutable; bump the version to release a new build, "+
						"or pass --unseal-official --reason \"<why>\" --prior-digest <sha256...> to repair a proven phantom",
					cv, publisher, name, platform, existingBuildID)
			}
			if strings.TrimSpace(repair.Reason) == "" {
				return "", status.Errorf(codes.InvalidArgument,
					"repair-unseal rejected at version-immutability gate: empty reason — provide --reason \"<why>\" describing why the published %s/%s@%s is being repaired",
					publisher, name, cv)
			}
			existingDigest := srv.getPublishedDigest(ctx, publisher, name, cv, platform)
			if existingDigest == "" {
				// Ledger has the version but we can't look up the digest. Treat
				// as "already published, immutable" — repair requires a known
				// prior-digest to confirm intent.
				return "", status.Errorf(codes.AlreadyExists,
					"version %s is already published for %s/%s on %s but its digest is not resolvable — refusing repair on weak evidence",
					cv, publisher, name, platform)
			}
			if !digestsMatch(repair.PriorDigest, existingDigest) {
				return "", status.Errorf(codes.FailedPrecondition,
					"repair-unseal rejected at version-immutability gate: prior-digest mismatch — caller asserted prior=%s but actual published digest is %s. "+
						"Inspect via 'globular pkg describe' or repository_explain_artifact and re-issue with the correct --prior-digest.",
					shortDigest(repair.PriorDigest), shortDigest(existingDigest))
			}
			// All gates passed. Mark the repair as consumed and capture prior
			// build_id (if not already set by an upstream gate, e.g. the seal
			// check). Allow the version to be re-used; the upload caller
			// will allocate a NEW build_number so the phantom row stays
			// queryable for forensics.
			repair.Used = true
			if repair.PriorBuildID == "" {
				repair.PriorBuildID = existingBuildID
			}
			slog.Warn("version-immutability gate repair authorized",
				"publisher", publisher, "name", name, "version", cv, "platform", platform,
				"prior_build_id", existingBuildID, "prior_digest", existingDigest,
				"reason", repair.Reason,
			)
			return cv, nil
		}
		// Validate monotonicity only when both versions are SemVer. Exact
		// upstream-native tags are identities, not ordered release streams.
		latestVer, _ := srv.getLatestRelease(ctx, publisher, name, platform)
		if latestVer != "" && versionutil.IsSemver(cv) && versionutil.IsSemver(latestVer) {
			cmp, cmpErr := versionutil.Compare(cv, latestVer)
			if cmpErr == nil && cmp < 0 {
				return "", status.Errorf(codes.FailedPrecondition,
					"version %s < latest PUBLISHED %s — versions must be monotonically increasing", cv, latestVer)
			}
		}
		return cv, nil

	case repopb.VersionIntent_BUMP_PATCH, repopb.VersionIntent_BUMP_MINOR, repopb.VersionIntent_BUMP_MAJOR:
		latestVer, _ := srv.getLatestRelease(ctx, publisher, name, platform)
		if latestVer == "" {
			latestVer = "0.0.0"
		}
		bumped, err := bumpVersion(latestVer, intent)
		if err != nil {
			return "", status.Errorf(codes.Internal, "version bump failed: %v", err)
		}
		return bumped, nil

	default:
		// Unspecified intent — default to BUMP_PATCH.
		latestVer, _ := srv.getLatestRelease(ctx, publisher, name, platform)
		if latestVer == "" {
			latestVer = "0.0.0"
		}
		bumped, err := bumpVersion(latestVer, repopb.VersionIntent_BUMP_PATCH)
		if err != nil {
			return "", status.Errorf(codes.Internal, "version bump failed: %v", err)
		}
		return bumped, nil
	}
}

// bumpVersion increments a semver version according to the intent.
func bumpVersion(current string, intent repopb.VersionIntent) (string, error) {
	// Parse major.minor.patch
	current = strings.TrimPrefix(current, "v")
	parts := strings.SplitN(current, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid semver: %s", current)
	}
	var major, minor, patch int
	fmt.Sscanf(parts[0], "%d", &major)
	fmt.Sscanf(parts[1], "%d", &minor)
	fmt.Sscanf(parts[2], "%d", &patch)

	switch intent {
	case repopb.VersionIntent_BUMP_PATCH:
		patch++
	case repopb.VersionIntent_BUMP_MINOR:
		minor++
		patch = 0
	case repopb.VersionIntent_BUMP_MAJOR:
		major++
		minor = 0
		patch = 0
	}

	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}

// startReservationCleanup runs a background goroutine to expire stale reservations.
func startReservationCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				reservations.cleanup()
			}
		}
	}()
}
