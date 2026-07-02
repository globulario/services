// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.desired_state_handlers
// @awareness file_role=typed_grpc_mutation_gate_for_desired_state_resources
// @awareness implements=globular.platform:intent.desired_state.is_authority
// @awareness implements=globular.platform:intent.controller.leader_election_gates_all_writes
// @awareness risk=critical
package main

// desired_state_handlers.go — typed gRPC handlers for GetDesiredState,
// UpsertDesiredService, RemoveDesiredService, SeedDesiredState,
// ValidateArtifact, and PreviewDesiredServices.
//
// These replace the JSON-codec ResourcesService hack. They persist desired
// service entries via the same resource store as ApplyServiceDesiredVersion.

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/audittrail"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/repository/repository_client"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func normalizeTargetNodeIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// listAllDesiredServices fetches all ServiceDesiredVersion objects from the
// resource store and converts them into the proto DesiredState.
// Entries are deduplicated by canonical name — if a stale domain-prefixed key
// (e.g. "localhost/authentication") coexists with the canonical key
// ("authentication"), only the canonical one is returned.
func (srv *server) listAllDesiredServices(ctx context.Context) (*cluster_controllerpb.DesiredState, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}

	// Fetch all resource types in parallel to minimize etcd round-trips.
	// Errors are captured — an etcd timeout must not silently produce an
	// empty list (absence_scope_must_be_explicit).
	type listResult struct {
		items []interface{}
		rv    string
		err   error
	}
	var (
		sdvRes, svcRelRes, infraRelRes, appRelRes listResult
		wg                                        sync.WaitGroup
	)
	wg.Add(4)
	go func() {
		defer wg.Done()
		items, rv, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
		sdvRes = listResult{items, rv, err}
	}()
	go func() {
		defer wg.Done()
		items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
		svcRelRes = listResult{items: items, err: err}
	}()
	go func() {
		defer wg.Done()
		items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
		infraRelRes = listResult{items: items, err: err}
	}()
	go func() {
		defer wg.Done()
		items, _, err := srv.resources.List(ctx, "ApplicationRelease", "")
		appRelRes = listResult{items: items, err: err}
	}()
	wg.Wait()

	// If the primary resource (ServiceDesiredVersion) failed, refuse to
	// return an empty list — callers must not interpret an etcd error as
	// "no desired services exist." See meta.absence_scope_must_be_explicit.
	if sdvRes.err != nil {
		return nil, status.Errorf(codes.Unavailable, "list ServiceDesiredVersion: %v", sdvRes.err)
	}

	// Build phase lookup from all release types.
	releasePhases := make(map[string]string)
	for _, obj := range svcRelRes.items {
		if rel, ok := obj.(*cluster_controllerpb.ServiceRelease); ok && rel.Spec != nil && rel.Status != nil {
			releasePhases[canonicalServiceName(rel.Spec.ServiceName)] = rel.Status.Phase
		}
	}
	for _, obj := range infraRelRes.items {
		if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil && rel.Status != nil {
			releasePhases[canonicalServiceName(rel.Spec.Component)] = rel.Status.Phase
		}
	}
	for _, obj := range appRelRes.items {
		if rel, ok := obj.(*cluster_controllerpb.ApplicationRelease); ok && rel.Spec != nil && rel.Status != nil {
			releasePhases[rel.Spec.AppName] = rel.Status.Phase
		}
	}

	ds := &cluster_controllerpb.DesiredState{Revision: sdvRes.rv}
	seen := make(map[string]bool)
	for _, obj := range sdvRes.items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" {
			canon = sdv.Spec.ServiceName
		}
		if seen[canon] {
			continue
		}
		seen[canon] = true
		ds.Services = append(ds.Services, &cluster_controllerpb.DesiredService{
			ServiceId:     canon,
			Version:       sdv.Spec.Version,
			BuildNumber:   sdv.Spec.BuildNumber,
			BuildId:       sdv.Spec.BuildID,
			TargetNodeIds: append([]string(nil), sdv.Spec.TargetNodeIDs...),
			Status:        releasePhases[canon],
		})
	}
	// Merge InfrastructureRelease entries so infrastructure daemons
	// (etcd, minio, envoy, etc.) appear in the desired-state response
	// alongside gRPC services. Without this, the UI shows them as removable.
	{
		for _, obj := range infraRelRes.items {
			rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
			if !ok || rel.Spec == nil {
				continue
			}
			canon := canonicalServiceName(rel.Spec.Component)
			if canon == "" && rel.Meta != nil {
				canon = canonicalServiceName(rel.Meta.Name)
			}
			if canon == "" || seen[canon] {
				continue
			}
			seen[canon] = true
			ds.Services = append(ds.Services, &cluster_controllerpb.DesiredService{
				ServiceId:   canon,
				Version:     rel.Spec.Version,
				BuildNumber: rel.Spec.BuildNumber,
				BuildId:     rel.Spec.BuildID,
				Status:      releasePhases[canon],
			})
		}
	}
	// Merge ApplicationRelease entries similarly.
	{
		for _, obj := range appRelRes.items {
			rel, ok := obj.(*cluster_controllerpb.ApplicationRelease)
			if !ok || rel.Spec == nil {
				continue
			}
			name := rel.Spec.AppName
			if name == "" && rel.Meta != nil {
				name = rel.Meta.Name
			}
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			ds.Services = append(ds.Services, &cluster_controllerpb.DesiredService{
				ServiceId:   name,
				Version:     rel.Spec.Version,
				BuildNumber: rel.Spec.BuildNumber,
				BuildId:     rel.Spec.BuildID,
				Status:      releasePhases[name],
			})
		}
	}
	return ds, nil
}

// cleanupStaleDesiredKeys removes resource store entries whose key contains a
// domain prefix (e.g. "localhost/authentication") that is now redundant because
// canonicalServiceName strips domain prefixes. This is a one-time migration
// step; new entries are always stored under the canonical key.
// cleanupStaleDesiredKeys removes resource store entries whose key doesn't
// match its canonical form. This handles:
//   - Domain-prefixed keys: "localhost/authentication" → canonical "authentication"
//   - Underscore variants: "cluster_controller" → canonical "cluster-controller"
//   - Proto-qualified names: "cluster_doctor.clusterdoctorservice" → canonical "cluster-doctor"
//
// When a stale key's canonical form already exists, the stale entry is simply
// deleted. When no canonical entry exists yet, the stale entry is re-upserted
// under the canonical key before deleting the old one.
func (srv *server) cleanupStaleDesiredKeys(ctx context.Context) int {
	if srv.resources == nil {
		return 0
	}
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return 0
	}

	// First pass: collect what canonical keys already exist and identify stale entries.
	type staleEntry struct {
		storedKey string
		canon     string
		version   string
	}
	canonExists := make(map[string]bool)
	var stale []staleEntry

	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Meta == nil || sdv.Spec == nil {
			continue
		}
		storedKey := sdv.Meta.Name
		canon := canonicalServiceName(storedKey)
		if canon == "" {
			canon = storedKey
		}
		if storedKey == canon {
			canonExists[canon] = true
		} else {
			stale = append(stale, staleEntry{storedKey: storedKey, canon: canon, version: sdv.Spec.Version})
		}
	}

	// Second pass: migrate or delete stale entries.
	removed := 0
	for _, s := range stale {
		// If canonical key doesn't exist yet, re-create it before deleting stale.
		if !canonExists[s.canon] {
			// allowRegression=true: this is a key-relocation (re-create the
			// canonical key from the stale record's own version before the stale
			// key is deleted), not an operator downgrade. A floor-reject here
			// would let the stale Delete below drop desired state on an
			// installed-state observation — exactly what
			// cluster.desired_state_authority_over_installed_state forbids.
			_ = srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{
				ServiceId: s.canon,
				Version:   s.version,
			}, true)
			canonExists[s.canon] = true
		}
		if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", s.storedKey); err == nil {
			removed++
		}
	}
	return removed
}

// upsertOne applies a single DesiredService to the resource store.
// writeDesiredAudit is the seam for desired-state provenance writes. Production
// points it at audittrail.WriteDesiredWriteRecord (etcd-backed); tests swap it to
// observe the dedicated regression-override action without an etcd backend.
var writeDesiredAudit = audittrail.WriteDesiredWriteRecord

// desiredVersionFloor returns the version a desired write must not go below:
// the greater of the current desired version and the highest healthy installed
// version. Either argument may be "" (no floor contributed from that source).
func desiredVersionFloor(currentDesired, installedHigh string) string {
	floor := strings.TrimSpace(currentDesired)
	high := strings.TrimSpace(installedHigh)
	if high == "" {
		return floor
	}
	if floor == "" {
		return high
	}
	if cmp, err := versionutil.Compare(high, floor); err == nil && cmp > 0 {
		return high
	}
	return floor
}

// regressesBelowFloor reports whether requested is strictly below floor. An empty
// floor (nothing to regress against) never regresses; an unparseable comparison
// is treated as non-regressing — the repository artifact check is the backstop.
func regressesBelowFloor(requested, floor string) bool {
	if strings.TrimSpace(floor) == "" {
		return false
	}
	cmp, err := versionutil.Compare(requested, floor)
	return err == nil && cmp < 0
}

// currentDesiredServiceVersion returns the version pinned in the current
// ServiceDesiredVersion for canon, or "" if none exists / is unreadable.
func (srv *server) currentDesiredServiceVersion(ctx context.Context, canon string) string {
	if srv.resources == nil {
		return ""
	}
	obj, _, err := srv.resources.Get(ctx, "ServiceDesiredVersion", canon)
	if err != nil || obj == nil {
		return ""
	}
	sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
	if !ok || sdv.Spec == nil {
		return ""
	}
	return strings.TrimSpace(sdv.Spec.Version)
}

// enforceServiceDesiredFloor applies the D4 no-regression policy to a SERVICE
// desired write. floor = max(current ServiceDesiredVersion, installedHigh). A
// requested version below the floor is rejected with a typed FailedPrecondition
// unless allowRegression is set, in which case the write proceeds and a distinct,
// audited action records the deliberate regression.
// invariant:desired.no_regression_all_paths
func (srv *server) enforceServiceDesiredFloor(ctx context.Context, canon, requested, installedHigh string, allowRegression bool) error {
	floor := desiredVersionFloor(srv.currentDesiredServiceVersion(ctx, canon), installedHigh)
	if !regressesBelowFloor(requested, floor) {
		return nil
	}
	if !allowRegression {
		return status.Errorf(codes.FailedPrecondition,
			"desired-state: refusing to regress %s — desired floor is %s, attempted %s; pass --allow-regression to override (audited)",
			canon, floor, requested)
	}
	_ = writeDesiredAudit(ctx, audittrail.DesiredWriteRecord{
		Service:   canon,
		Actor:     "cluster-controller",
		Source:    "upsertOne",
		Action:    "desired_regression_override",
		Reason:    fmt.Sprintf("explicit allow_regression: %s desired moved backward %s -> %s (floor %s)", canon, floor, requested, floor),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})
	return nil
}

// normalizeDesiredVersion validates and normalizes a desired-state version.
// It canonicalizes real SemVer AND preserves upstream-native (non-SemVer)
// package tags — ffmpeg n8.x, minio RELEASE.x, mc RELEASE.x — so a
// platform-upgrade that touches a native-version package does not fail the
// whole dispatch step.
//
// Do NOT revert this to versionutil.Canonical: Canonical rejects native tags,
// which is exactly failure_mode
// release.platform_upgrade_dispatch_rejects_non_semver_versions —
// platform_upgrade.dispatch upserted a native-version package (ffmpeg), upsertOne
// rejected the version as invalid semver, and the whole platform-upgrade reported
// FAILED even though every SemVer package converged. Native versions cannot be
// ordered, so the no-regression floor below skips them gracefully
// (regressesBelowFloor returns false when versionutil.Compare errors); the
// repository artifact resolver remains the final gate on installability.
func normalizeDesiredVersion(raw string) (string, error) {
	return versionutil.NormalizeExact(strings.TrimSpace(raw))
}

func (srv *server) upsertOne(ctx context.Context, svc *cluster_controllerpb.DesiredService, allowRegression bool) error {
	if svc == nil {
		return fmt.Errorf("nil service")
	}
	canon := canonicalServiceName(svc.ServiceId)
	if canon == "" {
		return fmt.Errorf("invalid service_id %q", svc.ServiceId)
	}
	targetNodeIDs := normalizeTargetNodeIDs(svc.GetTargetNodeIds())
	version := strings.TrimSpace(svc.Version)
	if version == "" {
		return fmt.Errorf("version is required for %q", svc.ServiceId)
	}
	if nv, err := normalizeDesiredVersion(version); err != nil {
		return fmt.Errorf("invalid version %q for %q: %w", svc.Version, svc.ServiceId, err)
	} else {
		version = nv
	}
	// Observability blackout is a hard precondition, independent of the
	// regression policy: with no fresh heartbeat we cannot compute the
	// installed-high floor and must not read "no signal" as "nothing is running"
	// (meta.absence_scope_must_be_explicit). Caller surfaces the FailedPrecondition
	// so the operator can retry once heartbeats recover.
	highVer, observable := srv.highestHealthyInstalledVersion(canon)
	if !observable {
		return status.Errorf(codes.FailedPrecondition,
			"desired-state: cannot validate version %s for %s — no node has a fresh heartbeat (cluster observability blackout); retry when heartbeats recover",
			version, canon)
	}

	// Phase 1: verify the artifact exists in the repository before writing
	// desired state. Fail closed: if repository is unreachable, reject.
	// Phase 2: also resolve build_id from the artifact manifest.
	//
	// D4: the previous behaviour silently auto-corrected a too-low version upward,
	// hiding operator mistakes. Regression is now rejected (unless an explicit
	// audited allow_regression override) — enforced per write path: the SERVICE
	// floor below, and the infrastructure floor inside routeInfrastructureDesired.
	// No more version mutation here, so no auto-correct fallback is needed.
	buildID, err := srv.validateArtifactInRepo(ctx, canon, version, svc.BuildNumber)
	if err != nil {
		return err
	}

	// ── Kind routing (invariant desired.keyed_by_kind_and_name) ───────────
	// Kind is the resource type: ServiceDesiredVersion (SERVICE) vs
	// InfrastructureRelease (INFRASTRUCTURE), each keyed by name. If this name is
	// already managed as INFRASTRUCTURE, route the write to its
	// InfrastructureRelease and NEVER create a SERVICE ServiceDesiredVersion for
	// the same name — that cross-kind ghost is the collision that fired the xds
	// incident. The detection moved ABOVE the ServiceDesiredVersion write so the
	// ghost is never created in the first place (previously it was written, then
	// the infra release was bumped, leaving both records for one name).
	//
	// We keep routing (rather than hard-rejecting) so infra updates that still
	// funnel through UpsertDesiredService keep working; the caller-side hard
	// reject is D2. A name whose INFRASTRUCTURE record cannot be read is refused,
	// not ghosted.
	if handled, rerr := routeInfrastructureDesired(ctx, srv.resources, canon, version, svc.BuildNumber, allowRegression, highVer); handled {
		if rerr != nil {
			return rerr
		}
		_ = audittrail.WriteDesiredWriteRecord(ctx, audittrail.DesiredWriteRecord{
			Service:   canon,
			Actor:     "cluster-controller",
			Source:    "upsertOne",
			Action:    "route_desired_to_infrastructure",
			Reason:    "name managed as INFRASTRUCTURE; cross-kind SERVICE write routed, no ServiceDesiredVersion ghost",
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		})
		return nil // INFRASTRUCTURE owns this name — never write a SERVICE ServiceDesiredVersion
	}

	// ── Same-kind SERVICE write (no InfrastructureRelease for this name) ──
	// D4: enforce the no-regression floor for the SERVICE path here — only after
	// kind routing has confirmed this is a genuine SERVICE write, so the SERVICE
	// floor never interferes with an infrastructure-routed write.
	floor := desiredVersionFloor(srv.currentDesiredServiceVersion(ctx, canon), highVer)
	allowDowngrade := allowRegression && regressesBelowFloor(version, floor)
	if err := srv.enforceServiceDesiredFloor(ctx, canon, version, highVer, allowRegression); err != nil {
		return err
	}
	obj := &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: canon},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName:    canon,
			Version:        version,
			BuildNumber:    svc.BuildNumber,
			BuildID:        buildID,
			AllowDowngrade: allowDowngrade,
			TargetNodeIDs:  targetNodeIDs,
		},
	}
	if _, err = srv.resources.Apply(ctx, "ServiceDesiredVersion", obj); err != nil {
		return err
	}
	_ = audittrail.WriteDesiredWriteRecord(ctx, audittrail.DesiredWriteRecord{
		Service:   canon,
		Actor:     "cluster-controller",
		Source:    "upsertOne",
		Action:    "upsert_desired",
		Reason:    "authoritative desired-state update",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	})

	// Ensure a corresponding ServiceRelease exists so the release reconciler can
	// track per-service lifecycle phases.
	srv.ensureServiceRelease(ctx, canon, "", version, svc.BuildNumber, targetNodeIDs, allowDowngrade)

	return nil
}

// routeInfrastructureDesired enforces the kind half of invariant
// desired.keyed_by_kind_and_name on the SERVICE desired-write path. Kind is the
// resource type: ServiceDesiredVersion (SERVICE) vs InfrastructureRelease
// (INFRASTRUCTURE), each keyed by name. If `canon` is already managed as
// INFRASTRUCTURE the write is routed to its InfrastructureRelease (bumped to
// `version` when it differs) and NO cross-kind SERVICE ServiceDesiredVersion is
// created for the same name — that ghost is the collision that fired the xds
// incident.
//
//	handled=false           → no InfrastructureRelease for this name; the caller
//	                          writes the same-kind ServiceDesiredVersion.
//	handled=true,  err=nil  → routed to INFRASTRUCTURE (bump or already current).
//	handled=true,  err!=nil → the INFRASTRUCTURE record is unreadable; refuse the
//	                          cross-kind write (typed FailedPrecondition) rather
//	                          than ghost it.
func routeInfrastructureDesired(ctx context.Context, resources resourcestore.Store, canon, version string, buildNumber int64, allowRegression bool, installedHigh string) (handled bool, err error) {
	infraObj, _, infraErr := resources.Get(ctx, "InfrastructureRelease", defaultPublisherID()+"/"+canon)
	if infraErr != nil || infraObj == nil {
		return false, nil // no INFRASTRUCTURE record — same-kind SERVICE write
	}
	infraRel, ok := infraObj.(*cluster_controllerpb.InfrastructureRelease)
	if !ok || infraRel.Spec == nil {
		return true, status.Errorf(codes.FailedPrecondition,
			"desired-state: %q is managed as INFRASTRUCTURE but its release record is unreadable — refusing cross-kind SERVICE desired write (desired.keyed_by_kind_and_name)", canon)
	}
	// D4: infrastructure desired version must not regress below the floor =
	// max(current infra desired, installed high) without an explicit audited
	// override (invariant desired.no_regression_all_paths). Same policy and helper
	// as the SERVICE path — governance with a keyhole, not a brick wall.
	floor := desiredVersionFloor(infraRel.Spec.Version, installedHigh)
	if regressesBelowFloor(version, floor) {
		if !allowRegression {
			return true, status.Errorf(codes.FailedPrecondition,
				"desired-state: refusing to regress infrastructure %s — desired floor is %s, attempted %s; pass --allow-regression to override (audited)",
				canon, floor, version)
		}
		_ = writeDesiredAudit(ctx, audittrail.DesiredWriteRecord{
			Service:   canon,
			Actor:     "cluster-controller",
			Source:    "upsertOne",
			Action:    "desired_regression_override",
			Reason:    fmt.Sprintf("explicit allow_regression: infrastructure %s desired moved backward %s -> %s (floor %s)", canon, floor, version, floor),
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		})
	}
	if infraRel.Spec.Version != version {
		infraCopy := *infraRel
		specCopy := *infraRel.Spec
		specCopy.Version = version
		specCopy.BuildNumber = buildNumber
		infraCopy.Spec = &specCopy
		if _, applyErr := resources.Apply(ctx, "InfrastructureRelease", &infraCopy); applyErr != nil {
			log.Printf("upsertOne: failed to bump InfrastructureRelease %s to %s: %v", canon, version, applyErr)
		} else {
			log.Printf("upsertOne: routed SERVICE desired write for %s to InfrastructureRelease (spec.version %s → %s) — no cross-kind ServiceDesiredVersion ghost", canon, infraRel.Spec.Version, version)
		}
	}
	return true, nil
}

// highestHealthyInstalledVersion returns the highest version of a service
// installed on any node with a recent heartbeat (< 10 minutes), AND a
// bool indicating whether the answer was observable.
//
//   - (version, true)  — at least one node had a fresh heartbeat. The
//     version is the highest reported (or "" if no fresh node has the
//     service installed). Caller can safely apply the downgrade guard.
//   - ("", false)      — NO node had a fresh heartbeat. We have zero
//     observability of the current cluster state and MUST NOT distinguish
//     "no node is running this service" from "we cannot see what is
//     running". Caller MUST refuse to make downgrade-vs-not decisions.
//
// Previously this returned "" for both cases. The caller's downgrade
// guard read "" as "nothing installed cluster-wide" and let arbitrary
// downgrades through during a heartbeat blackout — exactly the
// "absent in our observation == absent in the cluster" conflation that
// meta.absence_scope_must_be_explicit and meta.authority_must_express_uncertainty
// forbid.
func (srv *server) highestHealthyInstalledVersion(serviceName string) (string, bool) {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	var best string
	staleThreshold := 10 * time.Minute
	freshNodes := 0

	for _, node := range srv.state.Nodes {
		// Only consider nodes with recent heartbeats.
		if time.Since(node.LastSeen) > staleThreshold {
			continue
		}
		freshNodes++
		if node.InstalledVersions == nil {
			continue
		}
		ver, ok := node.InstalledVersions[serviceName]
		if !ok || ver == "" || ver == "0.0.1" {
			continue
		}
		if best == "" {
			best = ver
			continue
		}
		if cmp, err := versionutil.Compare(ver, best); err == nil && cmp > 0 {
			best = ver
		}
	}
	if freshNodes == 0 {
		return "", false
	}
	return best, true
}

// validateArtifactInRepo verifies that the specified artifact exists in the
// repository and is reachable. This prevents desired state from referencing
// non-existent artifacts. Fails closed: if the repository is unreachable,
// the write is rejected.
// validateArtifactInRepo verifies the artifact exists in the repository and
// returns its build_id. Phase 2: the build_id is persisted into desired-state
// so convergence can use exact identity.
func (srv *server) validateArtifactInRepo(ctx context.Context, serviceName, version string, buildNumber int64) (string, error) {
	// Resolve repository address — same path used by the release resolver.
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		return "", status.Errorf(codes.Unavailable,
			"repository address not configured; cannot validate artifact for %s@%s", serviceName, version)
	}

	// Use the system default publisher — same as ensureServiceRelease,
	// RemoveDesiredService, and the reconciler.
	publisher := defaultPublisherID()

	// Default platform — same as release_resolver.go:120-124.
	platform := "linux_amd64"

	repoClient, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return "", status.Errorf(codes.Unavailable,
			"repository unreachable at %s; cannot validate artifact for %s@%s: %v", addr, serviceName, version, err)
	}
	defer repoClient.Close()

	ref := &repositorypb.ArtifactRef{
		PublisherId: publisher,
		Name:        serviceName,
		Version:     version,
		Platform:    platform,
	}

	manifest, err := repoClient.GetArtifactManifest(ref, buildNumber)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			return "", status.Errorf(codes.NotFound,
				"artifact %s@%s (build %d) not found in repository; "+
					"cannot set desired state for non-existent artifact",
				serviceName, version, buildNumber)
		case codes.Unavailable:
			return "", status.Errorf(codes.Unavailable,
				"repository unreachable at %s; cannot validate artifact for %s@%s: %v",
				addr, serviceName, version, err)
		default:
			return "", status.Errorf(codes.Internal,
				"repository validation failed for %s@%s: %v",
				serviceName, version, err)
		}
	}

	// Boundary (package.release_vs_dev_channel_boundary): a DEV-channel artifact must
	// never become cluster desired-state. The manifest carries the canonical channel
	// (field 45), so reject DEV here — the single desired-state write chokepoint
	// (upsertOne is the only caller) — so no path (operator CLI, agent/MCP, or
	// deploy) can make a dev build a convergence target. CHANNEL_UNSET is STABLE;
	// CANDIDATE/CANARY/BOOTSTRAP are release tiers and stay eligible.
	if manifest != nil && !channelEligibleForDesiredState(manifest.GetChannel()) {
		return "", status.Errorf(codes.FailedPrecondition,
			"artifact %s@%s (build %d) is DEV-channel — a dev build must not become cluster desired-state "+
				"(package.release_vs_dev_channel_boundary); publish to a release channel first",
			serviceName, version, buildNumber)
	}

	// Phase 2: extract build_id from the manifest.
	buildID := ""
	if manifest != nil {
		buildID = manifest.GetBuildId()
	}
	return buildID, nil
}

// channelEligibleForDesiredState reports whether an artifact on the given channel
// may become cluster desired-state. DEV builds (local developer / agent / MCP) are
// dev-lane only and never convergence targets
// (package.release_vs_dev_channel_boundary). All release tiers — STABLE, CANDIDATE,
// CANARY, BOOTSTRAP, and CHANNEL_UNSET (treated as STABLE) — are eligible.
func channelEligibleForDesiredState(ch repositorypb.ArtifactChannel) bool {
	return ch != repositorypb.ArtifactChannel_DEV
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// GetDesiredState returns the current desired-service plan.
func (srv *server) GetDesiredState(ctx context.Context, _ *emptypb.Empty) (*cluster_controllerpb.DesiredState, error) {
	return srv.listAllDesiredServices(ctx)
}

// GetRoutingRefresh returns the controller's current leader-epoch so
// xDS, gateway, and other routing-aware components can detect leader
// changes without watching /globular/routing/refresh-generation
// directly. Anchored by
// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage —
// the cluster_controller OWNS the routing-refresh signal; consumers
// read it through this typed RPC rather than from etcd.
//
// Leader-forwarded so the returned epoch is the authoritative
// leader's value. Followers retain their last-known leaderEpoch,
// which becomes stale after a resign — calling against a follower
// without forwarding would surface that staleness as a missed
// routing change.
//
// The leader_addr field is best-effort: the leader knows its own
// address, but a follower's view may lag the actual leader. Consumers
// must compare epoch (not addr) to decide whether routing changed.
func (srv *server) GetRoutingRefresh(ctx context.Context, req *cluster_controllerpb.GetRoutingRefreshRequest) (*cluster_controllerpb.GetRoutingRefreshResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.GetRoutingRefreshResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/GetRoutingRefresh", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	epoch := srv.leaderEpoch.Load()
	var leaderAddr string
	if srv.cfg != nil {
		if ip := config.GetRoutableIPv4(); ip != "" {
			leaderAddr = fmt.Sprintf("%s:%d", ip, srv.cfg.Port)
		}
	}
	// leaderEpoch is stored as an atomic.Int64 (controller-side
	// invariant: monotonically increasing, never negative); cast to
	// the proto's uint64 carrier.
	var epochU64 uint64
	if epoch > 0 {
		epochU64 = uint64(epoch)
	}
	return &cluster_controllerpb.GetRoutingRefreshResponse{
		Epoch:      epochU64,
		LeaderAddr: leaderAddr,
		Timestamp:  timestamppb.Now(),
	}, nil
}

// ListDesiredBuildIDs returns the canonical reachability set of
// artifact build_ids the controller currently considers actively
// desired. The set is the union of build_ids referenced by every
// active desired-state record:
//
//   - ServiceDesiredVersion.Spec.BuildID
//   - ServiceRelease.Spec.BuildID + Status.ResolvedBuildID
//   - InfrastructureRelease.Spec.BuildID + Status.ResolvedBuildID
//   - ApplicationRelease.Spec.BuildID + Status.ResolvedBuildID
//
// Empty build_ids are skipped; duplicates are deduplicated; order is
// unspecified.
//
// This RPC is the single canonical answer to "which build_ids must I
// keep around?" — repository (purge / GC / revoke guards) and
// cluster_doctor (build-id reachability invariants) MUST call it
// instead of scanning /globular/resources/* etcd prefixes directly.
//
// Anchored by:
//
//	invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
//	invariant:repository.desired_build_id_is_hard_reachability_root
//	invariant:repository.purge_must_not_delete_active_desired_builds
//	failure_mode:repository.desired_build_id_orphaned
func (srv *server) ListDesiredBuildIDs(ctx context.Context, _ *cluster_controllerpb.ListDesiredBuildIDsRequest) (*cluster_controllerpb.ListDesiredBuildIDsResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}

	// Fetch all four kinds via the typed store in parallel — the same
	// pattern listAllDesiredServices uses. Each List call applies the
	// owner's canonicalization and version contracts; a raw etcd Get
	// would skip them and is the very vector this RPC closes.
	//
	// Per-source errors are collected, not discarded. If any kind fetch
	// fails the response is refused with codes.Unavailable — a partial
	// answer here looks identical to "fewer pins" and the repository
	// GC consumer uses the set to gate destructive deletes. Silently
	// shrinking the set would archive build_ids that are actually still
	// referenced (forbidden.silent_drop_on_partial_fetch + the round-2
	// reachability_guard fix where GC now refuses on trusted=false).
	type listResult struct {
		items []interface{}
		rv    string
		err   error
	}
	var (
		sdvRes, svcRelRes, infraRelRes, appRelRes listResult
		wg                                        sync.WaitGroup
	)
	wg.Add(4)
	go func() {
		defer wg.Done()
		items, rv, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
		sdvRes = listResult{items: items, rv: rv, err: err}
	}()
	go func() {
		defer wg.Done()
		items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
		svcRelRes = listResult{items: items, err: err}
	}()
	go func() {
		defer wg.Done()
		items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
		infraRelRes = listResult{items: items, err: err}
	}()
	go func() {
		defer wg.Done()
		items, _, err := srv.resources.List(ctx, "ApplicationRelease", "")
		appRelRes = listResult{items: items, err: err}
	}()
	wg.Wait()

	for kind, res := range map[string]listResult{
		"ServiceDesiredVersion": sdvRes,
		"ServiceRelease":        svcRelRes,
		"InfrastructureRelease": infraRelRes,
		"ApplicationRelease":    appRelRes,
	} {
		if res.err != nil {
			return nil, status.Errorf(codes.Unavailable,
				"ListDesiredBuildIDs: cannot enumerate %s (refusing partial response — repository GC would delete still-referenced build_ids): %v",
				kind, res.err)
		}
	}

	seen := make(map[string]struct{})
	add := func(id string) {
		if id == "" {
			return
		}
		seen[id] = struct{}{}
	}

	for _, obj := range sdvRes.items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		add(sdv.Spec.BuildID)
	}
	for _, obj := range svcRelRes.items {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok {
			continue
		}
		if rel.Spec != nil {
			add(rel.Spec.BuildID)
		}
		if rel.Status != nil {
			add(rel.Status.ResolvedBuildID)
		}
	}
	for _, obj := range infraRelRes.items {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok {
			continue
		}
		if rel.Spec != nil {
			add(rel.Spec.BuildID)
		}
		if rel.Status != nil {
			add(rel.Status.ResolvedBuildID)
		}
	}
	for _, obj := range appRelRes.items {
		rel, ok := obj.(*cluster_controllerpb.ApplicationRelease)
		if !ok {
			continue
		}
		if rel.Spec != nil {
			add(rel.Spec.BuildID)
		}
		if rel.Status != nil {
			add(rel.Status.ResolvedBuildID)
		}
	}

	resp := &cluster_controllerpb.ListDesiredBuildIDsResponse{
		BuildIds: make([]string, 0, len(seen)),
		Revision: sdvRes.rv,
	}
	for id := range seen {
		resp.BuildIds = append(resp.BuildIds, id)
	}
	return resp, nil
}

// UpsertDesiredService creates or updates a single desired-service entry.
func (srv *server) UpsertDesiredService(ctx context.Context, req *cluster_controllerpb.UpsertDesiredServiceRequest) (*cluster_controllerpb.DesiredState, error) {
	if req.GetService() == nil {
		return nil, status.Error(codes.InvalidArgument, "service is required")
	}
	// Cross-kind guard (invariant desired.keyed_by_kind_and_name): a
	// ServiceDesiredVersion is a SERVICE-kind record, so the operator-facing RPC
	// refuses to write one for a package whose canonical kind is infrastructure or
	// command. This closes the `services desired set <infra> --force` cross-kind
	// bypass that wrote a SERVICE record onto xds and produced unconvergeable drift
	// (forbidden_fix:cli_writes_cross_kind_desired_record). Internal orchestration
	// (deploy_control_plane, platform_upgrade, reconciler) calls upsertOne directly
	// and is intentionally unaffected; infrastructure desired state has its own
	// first-class owner path (ApplyInfrastructureRelease). The check runs before the
	// leader-forward because a cross-kind request is invalid on every node.
	if err := rejectCrossKindDesiredWrite(req.Service.GetServiceId()); err != nil {
		return nil, err
	}
	if !srv.isLeader() {
		resp := &cluster_controllerpb.DesiredState{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/UpsertDesiredService", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if err := srv.upsertOne(ctx, req.Service, req.GetAllowRegression()); err != nil {
		// Preserve a typed status (e.g. FailedPrecondition from the regression
		// floor or the observability blackout) so the operator sees the real
		// reason instead of an opaque Internal.
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, err
		}
		return nil, status.Errorf(codes.Internal, "upsert desired service: %v", err)
	}
	return srv.listAllDesiredServices(ctx)
}

// rejectCrossKindDesiredWrite enforces invariant desired.keyed_by_kind_and_name at
// the operator boundary. A ServiceDesiredVersion is a SERVICE-kind record; the
// component catalog (component_catalog.go) is the canonical kind authority. A name
// that is not in the catalog is treated as a service (fail-open) so third-party
// services are unaffected; only a known infrastructure or command package is
// refused.
func rejectCrossKindDesiredWrite(serviceID string) error {
	canon := canonicalServiceName(serviceID)
	if canon == "" {
		return nil // empty/invalid name — upsertOne reports it with a clearer message.
	}
	comp := CatalogByName(canon)
	if comp == nil {
		return nil // not in the catalog → treated as a service.
	}
	switch comp.Kind {
	case KindInfrastructure:
		return status.Errorf(codes.InvalidArgument,
			"desired-state: %q is INFRASTRUCTURE, not a service — writing a ServiceDesiredVersion for it is a cross-kind write (desired.keyed_by_kind_and_name). Use ApplyInfrastructureRelease for infrastructure desired state.",
			canon)
	case KindCommand:
		return status.Errorf(codes.InvalidArgument,
			"desired-state: %q is a COMMAND package, not a service — it has no ServiceDesiredVersion (desired.keyed_by_kind_and_name).",
			canon)
	}
	return nil
}

// RemoveDesiredService deletes the ServiceDesiredVersion and sets the Removing
// flag on the corresponding ServiceRelease, triggering a lifecycle-tracked
// removal workflow (REMOVING → REMOVED).
func (srv *server) RemoveDesiredService(ctx context.Context, req *cluster_controllerpb.RemoveDesiredServiceRequest) (*cluster_controllerpb.DesiredState, error) {
	// Cross-kind guard (invariant desired.keyed_by_kind_and_name): RemoveDesiredService
	// is a SERVICE-kind operation — it deletes a ServiceDesiredVersion and drives the
	// service removal workflow. Refuse it for infrastructure/command packages;
	// infrastructure removal has its own owner path (InfrastructureRelease with
	// spec.removing), not the service desired path. Runs before the leader-forward
	// because a cross-kind remove is invalid on every node.
	if err := rejectCrossKindDesiredWrite(req.GetServiceId()); err != nil {
		return nil, err
	}
	if !srv.isLeader() {
		resp := &cluster_controllerpb.DesiredState{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RemoveDesiredService", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := canonicalServiceName(req.GetServiceId())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "service_id is required")
	}

	// Delete the desired-state entry (stops new reconciliation from desired state).
	if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", name); err != nil {
		return nil, status.Errorf(codes.Internal, "remove desired service: %v", err)
	}

	// Set Removing flag on the ServiceRelease to trigger the removal workflow.
	releaseName := defaultPublisherID() + "/" + name
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", releaseName)
	if err == nil && obj != nil {
		if rel, ok := obj.(*cluster_controllerpb.ServiceRelease); ok && rel.Spec != nil {
			rel.Spec.Removing = true
			if _, err := srv.resources.Apply(ctx, "ServiceRelease", rel); err != nil {
				return nil, status.Errorf(codes.Internal, "mark release %s for removal: %v", releaseName, err)
			}
		}
	}
	// If no ServiceRelease exists, nothing to remove — just delete the SDV.

	return srv.listAllDesiredServices(ctx)
}

// importStats tracks the results of an import-from-installed operation.
type importStats struct {
	Imported       int      // new desired-state entries created
	AlreadyPresent int      // entries skipped (desired version already matches)
	Updated        int      // entries updated (desired version changed to match installed)
	Skipped        int      // installed entries intentionally not imported
	SkippedNames   []string // names of skipped entries
	Failed         int      // entries that failed to upsert
	FailedNames    []string // names of failed entries
}

// importInstalledToDesired is the core idempotent logic for importing
// installed services into the desired-state store. It:
//   - collects installed versions from all reporting nodes (union, first-seen wins)
//   - compares against existing desired-state entries
//   - creates/updates only what is missing or different
//   - returns import statistics
//
// This is safe to call repeatedly — already-present entries are skipped.
func (srv *server) importInstalledToDesired(ctx context.Context) (importStats, error) {
	var stats importStats

	if srv.resources == nil {
		return stats, fmt.Errorf("resource store unavailable")
	}

	// Step 1: Collect installed versions from canonical installed-state registry (etcd).
	// Union across all nodes, first-seen wins.
	allPkgs, err := installed_state.ListAllNodes(ctx, "SERVICE", "")
	if err != nil {
		// Fallback to in-memory node state if registry is unavailable.
		logger.Warn("importInstalledToDesired: installed-state registry unavailable, falling back to in-memory state", "error", err)
		allPkgs = nil
	}

	type installedInfo struct {
		version     string
		buildNumber int64
	}
	installed := make(map[string]installedInfo) // canonical name → version+build
	if len(allPkgs) > 0 {
		for _, pkg := range allPkgs {
			canon := canonicalServiceName(pkg.GetName())
			if canon == "" || pkg.GetVersion() == "" {
				continue
			}
			// Skip command-line tools — they are not services and should
			// never be imported into the desired-state model.
			if strings.HasSuffix(canon, "-cmd") {
				continue
			}
			// Reject non-authoritative entries:
			// - fallback/placeholder versions (defense-in-depth string check)
			// - partial_apply status (binary replaced without state update)
			// Only ManagedInstalled observations may contribute to desired state.
			ver := pkg.GetVersion()
			if ver == "unknown" || ver == "" {
				logger.Warn("importInstalledToDesired: skipping package with fallback version",
					"name", canon, "version", ver)
				continue
			}
			if pkg.GetStatus() == "partial_apply" {
				logger.Warn("importInstalledToDesired: skipping partial_apply package",
					"name", canon, "version", ver)
				continue
			}
			if _, exists := installed[canon]; !exists {
				if cv, err := versionutil.Canonical(ver); err == nil {
					ver = cv
				}
				installed[canon] = installedInfo{version: ver, buildNumber: pkg.GetBuildNumber()}
			}
		}
	} else {
		// Fallback: read from in-memory node state (legacy path, no build number).
		srv.lock("importInstalledToDesired")
		for _, node := range srv.state.Nodes {
			for svcID, ver := range node.InstalledVersions {
				canon := canonicalServiceName(svcID)
				if canon == "" || ver == "" || strings.HasSuffix(canon, "-cmd") {
					continue
				}
				// Reject fallback/placeholder versions.
				if ver == "unknown" || ver == "" {
					continue
				}
				if _, exists := installed[canon]; !exists {
					if cv, err := versionutil.Canonical(ver); err == nil {
						ver = cv
					}
					installed[canon] = installedInfo{version: ver}
				}
			}
		}
		srv.unlock()
	}

	srv.lock("importInstalledToDesired:nodeCount")
	nodeCount := len(srv.state.Nodes)
	srv.unlock()

	if nodeCount == 0 && len(allPkgs) == 0 {
		return stats, fmt.Errorf("no nodes have reported status yet; " +
			"wait for node-agent to start and report installed services")
	}
	if len(installed) == 0 {
		return stats, fmt.Errorf("nodes have reported but no installed services found")
	}

	// Step 2: Load existing desired state to compare.
	type existingInfo struct {
		version     string
		buildNumber int64
	}
	existingMap := make(map[string]existingInfo) // canonical name → version+build
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return stats, fmt.Errorf("list existing desired services: %w", err)
	}
	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" {
			canon = sdv.Spec.ServiceName
		}
		existingMap[canon] = existingInfo{version: sdv.Spec.Version, buildNumber: sdv.Spec.BuildNumber}
	}

	// Step 2.5: Remove command-type entries from desired state only.
	// INVARIANT: Never delete desired entries because a service is not currently
	// installed. Desired state (Layer 2) is authoritative — it must only change
	// via explicit operator action (deploy/seed/desired-remove). Deleting based on
	// Layer 3 (installed) observations is an authority inversion: a timing race
	// (e.g. node-agent restart during a join) can make ListAllNodes return an
	// incomplete snapshot, causing seed to wipe all workload desired state.
	for canon := range existingMap {
		if strings.HasSuffix(canon, "-cmd") {
			if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", canon); err == nil {
				logger.Info("importInstalledToDesired: removed command entry from desired state", "name", canon)
			}
			delete(existingMap, canon)
		}
	}

	// Step 2.7: Build set of infrastructure-managed packages so we don't
	// import them as services. MCP, gateway, xds, etc. are INFRASTRUCTURE
	// and already have InfrastructureRelease objects — creating a duplicate
	// ServiceDesiredVersion/ServiceRelease causes ghost entries in the admin UI.
	infraManaged := make(map[string]bool)
	if infraItems, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		for _, obj := range infraItems {
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
				infraManaged[canonicalServiceName(rel.Spec.Component)] = true
			}
		}
	}

	// Step 3: Upsert only what is missing or different.
	for name, inst := range installed {
		if infraManaged[name] {
			continue // managed by InfrastructureRelease, not ServiceRelease
		}
		ex, found := existingMap[name]
		if found && versionutil.EqualFull(ex.version, ex.buildNumber, inst.version, inst.buildNumber) {
			// Already present with matching version+build — skip.
			stats.AlreadyPresent++
			continue
		}

		// allowRegression=false: seeding desired from observed install must never
		// pull an existing desired record backward
		// (forbidden_fix:materialize_desired_from_unverified_local_install).
		if err := srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{
			ServiceId:   name,
			Version:     inst.version,
			BuildNumber: inst.buildNumber,
		}, false); err != nil {
			if status.Code(err) == codes.NotFound {
				stats.Skipped++
				stats.SkippedNames = append(stats.SkippedNames, name)
				logger.Warn("importInstalledToDesired: skipping installed service without repository artifact",
					slog.String("service", name),
					slog.String("version", inst.version),
					slog.Any("error", err))
				continue
			}
			stats.Failed++
			stats.FailedNames = append(stats.FailedNames, name)
			logger.Warn("importInstalledToDesired: failed to upsert",
				slog.String("service", name),
				slog.String("version", inst.version),
				slog.Any("error", err))
			continue
		}

		if found {
			stats.Updated++
			logger.Info("importInstalledToDesired: updated desired version",
				slog.String("service", name),
				slog.String("from", ex.version),
				slog.String("to", inst.version))
		} else {
			stats.Imported++
			logger.Info("importInstalledToDesired: imported new desired service",
				slog.String("service", name),
				slog.String("version", inst.version))
		}
	}

	// Step 4: Import APPLICATION packages as ApplicationRelease desired-state.
	appStats := srv.importInstalledAppsToDesired(ctx)
	stats.Imported += appStats.Imported
	stats.AlreadyPresent += appStats.AlreadyPresent
	stats.Updated += appStats.Updated
	stats.Skipped += appStats.Skipped
	stats.SkippedNames = append(stats.SkippedNames, appStats.SkippedNames...)
	stats.Failed += appStats.Failed
	stats.FailedNames = append(stats.FailedNames, appStats.FailedNames...)

	// Step 5: Import INFRASTRUCTURE packages as InfrastructureRelease desired-state.
	infraStats := srv.importInstalledInfraToDesired(ctx)
	stats.Imported += infraStats.Imported
	stats.AlreadyPresent += infraStats.AlreadyPresent
	stats.Updated += infraStats.Updated
	stats.Skipped += infraStats.Skipped
	stats.SkippedNames = append(stats.SkippedNames, infraStats.SkippedNames...)
	stats.Failed += infraStats.Failed
	stats.FailedNames = append(stats.FailedNames, infraStats.FailedNames...)

	return stats, nil
}

// installedPkgInfo holds the common fields extracted from an InstalledPackage
// for import into desired state.
type installedPkgInfo struct {
	name        string
	version     string
	buildNumber int64
	publisherID string
	platform    string
}

// releaseImportConfig parameterises importInstalledReleasesToDesired for
// different release types (APPLICATION vs INFRASTRUCTURE).
type releaseImportConfig struct {
	// installedKind is the installed-state kind filter (e.g. "APPLICATION").
	installedKind string
	// resourceType is the resource-store type name (e.g. "ApplicationRelease").
	resourceType string
	// logPrefix is used in log messages (e.g. "app" or "infra").
	logPrefix string
	// existingName extracts the name from a resource-store object.
	// Returns "" if the object is not of the expected type.
	existingName func(obj interface{}) string
	// buildRelease constructs the typed release object for Apply.
	buildRelease func(info installedPkgInfo) interface{}
}

// importInstalledReleasesToDesired is the generic implementation behind
// importInstalledAppsToDesired and importInstalledInfraToDesired.
func (srv *server) importInstalledReleasesToDesired(ctx context.Context, cfg releaseImportConfig) importStats {
	var stats importStats
	if srv.resources == nil {
		return stats
	}

	allPkgs, err := installed_state.ListAllNodes(ctx, cfg.installedKind, "")
	if err != nil || len(allPkgs) == 0 {
		return stats
	}

	// Collect unique entries (first-seen version wins).
	entries := make(map[string]installedPkgInfo)
	for _, pkg := range allPkgs {
		name := strings.TrimSpace(pkg.GetName())
		if name == "" || pkg.GetVersion() == "" {
			continue
		}
		if _, exists := entries[name]; !exists {
			pubID := strings.TrimSpace(pkg.GetPublisherId())
			if pubID == "" || pubID == "unknown" {
				pubID = defaultPublisherID()
			}
			entries[name] = installedPkgInfo{
				name:        name,
				version:     pkg.GetVersion(),
				buildNumber: pkg.GetBuildNumber(),
				publisherID: pubID,
				platform:    pkg.GetPlatform(),
			}
		}
	}

	// Check which releases already exist.
	existing := make(map[string]bool)
	if items, _, err := srv.resources.List(ctx, cfg.resourceType, ""); err == nil {
		for _, obj := range items {
			if n := cfg.existingName(obj); n != "" {
				existing[n] = true
			}
		}
	}

	// Create missing release objects.
	for _, info := range entries {
		if existing[info.name] {
			stats.AlreadyPresent++
			continue
		}
		rel := cfg.buildRelease(info)
		if _, err := srv.resources.Apply(ctx, cfg.resourceType, rel); err != nil {
			stats.Failed++
			stats.FailedNames = append(stats.FailedNames, cfg.logPrefix+":"+info.name)
			logger.Warn("import"+cfg.logPrefix+"ToDesired: failed to create",
				slog.String(cfg.logPrefix, info.name), slog.Any("error", err))
			continue
		}
		stats.Imported++
		logger.Info("import"+cfg.logPrefix+"ToDesired: imported",
			slog.String(cfg.logPrefix, info.name), slog.String("version", info.version))
	}

	return stats
}

// importInstalledAppsToDesired creates ApplicationRelease desired-state objects
// for APPLICATION packages found in the installed-state registry that don't
// already have a corresponding ApplicationRelease.
func (srv *server) importInstalledAppsToDesired(ctx context.Context) importStats {
	return srv.importInstalledReleasesToDesired(ctx, releaseImportConfig{
		installedKind: "APPLICATION",
		resourceType:  "ApplicationRelease",
		logPrefix:     "app",
		existingName: func(obj interface{}) string {
			if rel, ok := obj.(*cluster_controllerpb.ApplicationRelease); ok && rel.Meta != nil {
				return rel.Meta.Name
			}
			return ""
		},
		buildRelease: func(info installedPkgInfo) interface{} {
			return &cluster_controllerpb.ApplicationRelease{
				Meta: &cluster_controllerpb.ObjectMeta{Name: info.name},
				Spec: &cluster_controllerpb.ApplicationReleaseSpec{
					PublisherID: info.publisherID,
					AppName:     info.name,
					Version:     info.version,
					BuildNumber: info.buildNumber,
				},
				Status: &cluster_controllerpb.ApplicationReleaseStatus{},
			}
		},
	})
}

// importInstalledInfraToDesired creates InfrastructureRelease desired-state
// objects for INFRASTRUCTURE packages found in the installed-state registry
// that don't already have a corresponding InfrastructureRelease.
func (srv *server) importInstalledInfraToDesired(ctx context.Context) importStats {
	return srv.importInstalledReleasesToDesired(ctx, releaseImportConfig{
		installedKind: "INFRASTRUCTURE",
		resourceType:  "InfrastructureRelease",
		logPrefix:     "infra",
		existingName: func(obj interface{}) string {
			// Return the bare component name for dedup (existing[info.name] uses bare).
			// Meta.Name may be bare (legacy) or "<publisher>/<component>" (new). Prefer
			// Spec.Component which is always the bare component.
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok {
				if rel.Spec != nil && rel.Spec.Component != "" {
					return rel.Spec.Component
				}
				if rel.Meta != nil {
					// Fall back to Meta.Name, stripping "<publisher>/" prefix if present.
					n := rel.Meta.Name
					if i := strings.LastIndex(n, "/"); i >= 0 {
						return n[i+1:]
					}
					return n
				}
			}
			return ""
		},
		buildRelease: func(info installedPkgInfo) interface{} {
			// Canonical key: "<publisher>/<component>" — matches materializeInfraDesired
			// and ApplyInfrastructureRelease. Using a bare name here caused split-brain
			// duplicates in etcd (one entry per naming convention).
			relName := info.publisherID + "/" + info.name
			return &cluster_controllerpb.InfrastructureRelease{
				Meta: &cluster_controllerpb.ObjectMeta{Name: relName},
				Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
					PublisherID: info.publisherID,
					Component:   info.name,
					Version:     info.version,
					BuildNumber: info.buildNumber,
					Platform:    info.platform,
				},
				Status: &cluster_controllerpb.InfrastructureReleaseStatus{},
			}
		},
	})
}

// SeedDesiredState bulk-populates desired state.
//
// IMPORT_FROM_INSTALLED: reads installed_versions from all nodes (union),
// creates one DesiredService per unique service (first-seen version wins).
// Idempotent — safe to call repeatedly. Existing entries are preserved unless
// the installed version differs.
//
// DEFAULT_CORE_PROFILE: not yet defined; returns an error until a core
// profile catalogue is available.
func (srv *server) SeedDesiredState(ctx context.Context, req *cluster_controllerpb.SeedDesiredStateRequest) (*cluster_controllerpb.DesiredState, error) {
	if !reconcileVersionGate() {
		return nil, status.Errorf(codes.FailedPrecondition,
			"controller version %s is below minimum safe reconcile version %s — desired-state mutation is disabled",
			Version, minSafeReconcileVersion)
	}
	if !srv.isLeader() {
		resp := &cluster_controllerpb.DesiredState{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/SeedDesiredState", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	// Clean up any stale domain-prefixed keys from previous seeds.
	if n := srv.cleanupStaleDesiredKeys(ctx); n > 0 {
		logger.Info("SeedDesiredState: cleaned up stale entries",
			slog.Int("count", n))
	}

	switch req.GetMode() {
	case cluster_controllerpb.SeedDesiredStateRequest_IMPORT_FROM_INSTALLED:
		stats, err := srv.importInstalledToDesired(ctx)
		if err != nil {
			return nil, status.Errorf(codes.FailedPrecondition,
				"import from installed: %v", err)
		}

		logger.Info("SeedDesiredState: import complete",
			slog.Int("imported", stats.Imported),
			slog.Int("updated", stats.Updated),
			slog.Int("already_present", stats.AlreadyPresent),
			slog.Int("skipped", stats.Skipped),
			slog.Any("skipped_names", stats.SkippedNames),
			slog.Int("failed", stats.Failed))

		if stats.Failed > 0 {
			return nil, status.Errorf(codes.Internal,
				"import partially failed: %d imported, %d failed (%v)",
				stats.Imported, stats.Failed, stats.FailedNames)
		}

	default:
		return nil, status.Errorf(codes.Unimplemented,
			"SeedDesiredState mode %v is not yet implemented", req.GetMode())
	}

	return srv.listAllDesiredServices(ctx)
}

// ValidateArtifact checks whether an artifact is fit to deploy by querying
// the repository service for the manifest.  If the repository is unreachable
// a WARNING is returned rather than a hard error, so callers can proceed with
// manual confirmation.
func (srv *server) ValidateArtifact(_ context.Context, req *cluster_controllerpb.ValidateArtifactRequest) (*cluster_controllerpb.ValidationReport, error) {
	serviceId := strings.TrimSpace(req.GetServiceId())
	version := strings.TrimSpace(req.GetVersion())
	if serviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "service_id is required")
	}
	if version == "" {
		return nil, status.Error(codes.InvalidArgument, "version is required")
	}

	// Resolve repository address: env var → default.
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")

	repoClient, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return &cluster_controllerpb.ValidationReport{
			ChecksumOk:      false,
			SignatureStatus: "unknown",
			PlatformOk:      true,
			Issues: []*cluster_controllerpb.ValidationIssue{{
				Severity: cluster_controllerpb.ValidationIssue_WARNING,
				Message:  fmt.Sprintf("repository unreachable (%s): %v", addr, err),
			}},
		}, nil
	}
	defer repoClient.Close()

	// Build a unified index from both artifact manifests and bundle summaries.
	pkgIndex, repoNames := buildPackageIndex(repoClient)

	if cv, err := versionutil.Canonical(version); err == nil {
		version = cv
	}
	pkg, pkgFound := pkgIndex[normalizeServiceName(serviceId)+"@"+version]
	if !pkgFound {
		return &cluster_controllerpb.ValidationReport{
			ChecksumOk:      false,
			SignatureStatus: "unknown",
			PlatformOk:      false,
			Issues: []*cluster_controllerpb.ValidationIssue{{
				Severity: cluster_controllerpb.ValidationIssue_ERROR,
				Message:  fmt.Sprintf("artifact %q@%q not found in repository", serviceId, version),
			}},
		}, nil
	}

	checksumOk := pkg.checksum != ""
	var issues []*cluster_controllerpb.ValidationIssue

	if !checksumOk {
		issues = append(issues, &cluster_controllerpb.ValidationIssue{
			Severity: cluster_controllerpb.ValidationIssue_WARNING,
			Message:  "artifact has no checksum",
		})
	}

	// Platform check: compare artifact platform against each target node.
	artifactPlatform := normalizeArtifactPlatform(pkg.platform)
	platformOk := true

	srv.lock("ValidateArtifact")
	nodeCopy := make(map[string]*nodeState, len(srv.state.Nodes))
	for id, n := range srv.state.Nodes {
		nodeCopy[id] = n
	}
	srv.unlock()

	targetNodeIds := req.GetTargetNodeIds()
	if len(targetNodeIds) == 0 {
		for id := range nodeCopy {
			targetNodeIds = append(targetNodeIds, id)
		}
	}

	if artifactPlatform == "" {
		issues = append(issues, &cluster_controllerpb.ValidationIssue{
			Severity: cluster_controllerpb.ValidationIssue_WARNING,
			Message:  "artifact has no platform information; cannot verify compatibility",
		})
	} else {
		for _, nodeId := range targetNodeIds {
			node, ok := nodeCopy[nodeId]
			if !ok {
				continue
			}
			nodePlatform := normalizeArtifactPlatform(node.Identity.Os + "_" + node.Identity.Arch)
			if nodePlatform == "" || nodePlatform == "_" {
				issues = append(issues, &cluster_controllerpb.ValidationIssue{
					Severity: cluster_controllerpb.ValidationIssue_WARNING,
					Message:  fmt.Sprintf("node %q has no platform information; cannot verify compatibility", nodeId),
				})
				continue
			}
			if artifactPlatform != nodePlatform {
				issues = append(issues, &cluster_controllerpb.ValidationIssue{
					Severity: cluster_controllerpb.ValidationIssue_ERROR,
					Message:  fmt.Sprintf("platform mismatch: artifact is %q but node %q is %q", artifactPlatform, nodeId, nodePlatform),
				})
				platformOk = false
			}
		}
	}

	// Dependency existence check (populated only for artifact manifests; empty for bundles).
	for _, dep := range pkg.requires {
		dep = strings.TrimSpace(dep)
		if dep == "" {
			continue
		}
		if !repoNames[dep] {
			issues = append(issues, &cluster_controllerpb.ValidationIssue{
				Severity: cluster_controllerpb.ValidationIssue_WARNING,
				Message:  fmt.Sprintf("dependency %q not found in repository", dep),
			})
		}
	}

	return &cluster_controllerpb.ValidationReport{
		ChecksumOk:      checksumOk,
		SignatureStatus: "unsigned",
		PlatformOk:      platformOk,
		Issues:          issues,
	}, nil
}

// normalizeServiceName strips the proto package prefix and removes all
// non-alphanumeric characters so that bundle names and proto FQNs compare
// equal.  Examples:
//
//	"node-agent"                        → "nodeagent"
//	"node_agent.NodeAgentService"        → "nodeagent"
//	"cluster-controller"                → "clustercontroller"
//	"cluster_controller.ClusterCtrlSvc"  → "clustercontroller"
func normalizeServiceName(name string) string {
	if idx := strings.Index(name, "."); idx > 0 {
		name = name[:idx]
	}
	var b strings.Builder
	for _, c := range strings.ToLower(name) {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// resolvedPkg holds the validation-relevant fields from either an
// ArtifactManifest (legacy) or a BundleSummary (current publish path).
type resolvedPkg struct {
	platform string
	checksum string
	requires []string // populated only for artifact manifests
}

// buildPackageIndex queries the repository service for all available packages
// (artifact manifests first, bundle summaries second) and returns:
//
//   - pkgIndex: map from normalizeServiceName(name)+"@"+version → resolvedPkg
//   - nameSet:  set of all raw names found (for dependency existence checks)
func buildPackageIndex(rc *repository_client.Repository_Service_Client) (
	pkgIndex map[string]resolvedPkg, nameSet map[string]bool,
) {
	pkgIndex = make(map[string]resolvedPkg)
	nameSet = make(map[string]bool)

	if arts, err := rc.ListArtifacts(); err == nil {
		for _, m := range arts {
			if m.GetRef() == nil {
				continue
			}
			nameSet[m.GetRef().GetName()] = true
			ver := m.GetRef().GetVersion()
			if cv, err := versionutil.Canonical(ver); err == nil {
				ver = cv
			}
			k := normalizeServiceName(m.GetRef().GetName()) + "@" + ver
			if _, exists := pkgIndex[k]; !exists {
				pkgIndex[k] = resolvedPkg{
					platform: m.GetRef().GetPlatform(),
					checksum: strings.TrimSpace(m.GetChecksum()),
					requires: m.GetRequires(),
				}
			}
		}
	}

	// TODO(migration): Remove legacy bundle fallback once all packages use artifacts.
	if bundles, err := rc.ListBundles(); err == nil {
		for _, b := range bundles {
			nameSet[b.GetName()] = true
			bver := b.GetVersion()
			if cv, err := versionutil.Canonical(bver); err == nil {
				bver = cv
			}
			k := normalizeServiceName(b.GetName()) + "@" + bver
			if _, exists := pkgIndex[k]; !exists {
				pkgIndex[k] = resolvedPkg{
					platform: b.GetPlatform(),
					checksum: strings.TrimSpace(b.GetSha256()),
					// bundles carry no dependency list
				}
			}
		}
	}

	return
}

// normalizeArtifactPlatform lower-cases a platform string and normalises
// "/" and "-" separators to "_" so "linux/amd64", "linux-amd64", and
// "linux_amd64" all compare equal.
func normalizeArtifactPlatform(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	p = strings.ReplaceAll(p, "/", "_")
	p = strings.ReplaceAll(p, "-", "_")
	return p
}

// PreviewDesiredServices simulates applying a delta and reports per-node
// changes without mutating state.  It queries the repository to validate each
// artifact (existence + platform) and produces per-node will_install lists
// that reflect only nodes that actually need the change.
func (srv *server) PreviewDesiredServices(_ context.Context, req *cluster_controllerpb.DesiredServicesDelta) (*cluster_controllerpb.ServiceChangePreview, error) {
	// Snapshot node state under lock.
	srv.lock("PreviewDesiredServices")
	nodeCopy := make(map[string]*nodeState, len(srv.state.Nodes))
	for id, n := range srv.state.Nodes {
		nodeCopy[id] = n
	}
	srv.unlock()

	preview := &cluster_controllerpb.ServiceChangePreview{}

	// Query repository for artifact validation (best-effort; degraded if unreachable).
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")

	// Build a unified package index from both artifact manifests and bundle summaries.
	pkgIndex := make(map[string]resolvedPkg)
	repoAvailable := false

	if rc, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository"); err == nil {
		pkgIndex, _ = buildPackageIndex(rc)
		repoAvailable = len(pkgIndex) > 0
		rc.Close()
	}

	// Validate each upsert against repository; collect blocking issues.
	for _, svc := range req.GetUpserts() {
		name := svc.GetServiceId()
		ver := svc.GetVersion()

		if !repoAvailable {
			preview.BlockingIssues = append(preview.BlockingIssues, &cluster_controllerpb.ValidationIssue{
				Severity: cluster_controllerpb.ValidationIssue_WARNING,
				Message:  fmt.Sprintf("repository unreachable; cannot validate %s@%s", name, ver),
			})
			continue
		}

		if cv, err := versionutil.Canonical(ver); err == nil {
			ver = cv
		}
		pkg, ok := pkgIndex[normalizeServiceName(name)+"@"+ver]
		if !ok {
			preview.BlockingIssues = append(preview.BlockingIssues, &cluster_controllerpb.ValidationIssue{
				Severity: cluster_controllerpb.ValidationIssue_ERROR,
				Message:  fmt.Sprintf("artifact %q@%q not found in repository", name, ver),
			})
			continue
		}

		// Platform check per node.
		artifactPlatform := normalizeArtifactPlatform(pkg.platform)
		if artifactPlatform != "" {
			for nodeId, node := range nodeCopy {
				nodePlatform := normalizeArtifactPlatform(node.Identity.Os + "_" + node.Identity.Arch)
				if nodePlatform == "" || nodePlatform == "_" {
					continue // unknown node platform — warn but don't block
				}
				if artifactPlatform != nodePlatform {
					preview.BlockingIssues = append(preview.BlockingIssues, &cluster_controllerpb.ValidationIssue{
						Severity: cluster_controllerpb.ValidationIssue_ERROR,
						Message:  fmt.Sprintf("%s@%s: platform mismatch — artifact %q, node %q is %q", name, ver, artifactPlatform, nodeId, nodePlatform),
					})
				}
			}
		}
	}

	// Build per-node change list: only include nodes that actually need an update.
	for nodeId, node := range nodeCopy {
		change := &cluster_controllerpb.NodeChange{NodeId: nodeId}
		for _, svc := range req.GetUpserts() {
			name := svc.GetServiceId()
			ver := svc.GetVersion()
			// Only install if this node doesn't already have this version.
			if !versionutil.Equal(node.InstalledVersions[name], ver) {
				change.WillInstall = append(change.WillInstall,
					fmt.Sprintf("%s@%s", name, ver))
			}
		}
		for _, id := range req.GetRemovals() {
			change.WillRemove = append(change.WillRemove, id)
		}
		if len(change.WillInstall) > 0 || len(change.WillRemove) > 0 {
			preview.NodeChanges = append(preview.NodeChanges, change)
		}
	}

	return preview, nil
}
