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
	if err := srv.requireHealthy(); err != nil {
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

	// Resolve version from intent.
	version, err := srv.resolveVersionIntent(ctx, publisher, name, platform, req.GetIntent(), req.GetExactVersion())
	if err != nil {
		return nil, err
	}

	// Generate build_id and build_number.
	buildID := uuid.Must(uuid.NewV7()).String()
	buildNumber := srv.resolveLatestBuildNumber(ctx, &repopb.ArtifactRef{
		PublisherId: publisher, Name: name, Version: version, Platform: platform,
	}) + 1

	// Resolve channel — default to STABLE.
	ch := req.GetChannel()
	if ch == repopb.ArtifactChannel_CHANNEL_UNSET {
		ch = repopb.ArtifactChannel_STABLE
	}

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
func (srv *server) resolveVersionIntent(ctx context.Context, publisher, name, platform string, intent repopb.VersionIntent, exactVersion string) (string, error) {
	switch intent {
	case repopb.VersionIntent_EXACT:
		if exactVersion == "" {
			return "", status.Error(codes.InvalidArgument, "exact_version is required when intent=EXACT")
		}
		cv, err := versionutil.NormalizeExact(exactVersion)
		if err != nil {
			return "", status.Errorf(codes.InvalidArgument, "invalid version %q: %v", exactVersion, err)
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
