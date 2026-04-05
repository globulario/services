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
	"log/slog"
	"os"
	"strings"
	"sync"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/repository/repository_client"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ── helpers ──────────────────────────────────────────────────────────────────

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
	type listResult struct {
		items []interface{}
		rv    string
	}
	var (
		sdvRes, svcRelRes, infraRelRes, appRelRes listResult
		wg                                         sync.WaitGroup
	)
	wg.Add(4)
	go func() { defer wg.Done(); items, rv, _ := srv.resources.List(ctx, "ServiceDesiredVersion", ""); sdvRes = listResult{items, rv} }()
	go func() { defer wg.Done(); items, _, _ := srv.resources.List(ctx, "ServiceRelease", ""); svcRelRes = listResult{items: items} }()
	go func() { defer wg.Done(); items, _, _ := srv.resources.List(ctx, "InfrastructureRelease", ""); infraRelRes = listResult{items: items} }()
	go func() { defer wg.Done(); items, _, _ := srv.resources.List(ctx, "ApplicationRelease", ""); appRelRes = listResult{items: items} }()
	wg.Wait()

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
			ServiceId:   canon,
			Version:     sdv.Spec.Version,
			BuildNumber: sdv.Spec.BuildNumber,
			Status:      releasePhases[canon],
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
			_ = srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{
				ServiceId: s.canon,
				Version:   s.version,
			})
			canonExists[s.canon] = true
		}
		if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", s.storedKey); err == nil {
			removed++
		}
	}
	return removed
}

// upsertOne applies a single DesiredService to the resource store.
func (srv *server) upsertOne(ctx context.Context, svc *cluster_controllerpb.DesiredService) error {
	if svc == nil {
		return fmt.Errorf("nil service")
	}
	canon := canonicalServiceName(svc.ServiceId)
	if canon == "" {
		return fmt.Errorf("invalid service_id %q", svc.ServiceId)
	}
	version := strings.TrimSpace(svc.Version)
	if version == "" {
		return fmt.Errorf("version is required for %q", svc.ServiceId)
	}
	if cv, err := versionutil.Canonical(version); err != nil {
		return fmt.Errorf("invalid version %q for %q: %w", svc.Version, svc.ServiceId, err)
	} else {
		version = cv
	}
	obj := &cluster_controllerpb.ServiceDesiredVersion{
		Meta: &cluster_controllerpb.ObjectMeta{Name: canon},
		Spec: &cluster_controllerpb.ServiceDesiredVersionSpec{
			ServiceName: canon,
			Version:     version,
			BuildNumber: svc.BuildNumber,
		},
	}
	_, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", obj)
	if err != nil {
		return err
	}

	// Bridge: ensure a corresponding ServiceRelease exists so the release
	// reconciler can track per-service lifecycle phases.
	srv.ensureServiceRelease(ctx, canon, version, svc.BuildNumber)

	return nil
}

// ── Handlers ─────────────────────────────────────────────────────────────────

// GetDesiredState returns the current desired-service plan.
func (srv *server) GetDesiredState(ctx context.Context, _ *emptypb.Empty) (*cluster_controllerpb.DesiredState, error) {
	return srv.listAllDesiredServices(ctx)
}

// UpsertDesiredService creates or updates a single desired-service entry.
func (srv *server) UpsertDesiredService(ctx context.Context, req *cluster_controllerpb.UpsertDesiredServiceRequest) (*cluster_controllerpb.DesiredState, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req.GetService() == nil {
		return nil, status.Error(codes.InvalidArgument, "service is required")
	}
	if err := srv.upsertOne(ctx, req.Service); err != nil {
		return nil, status.Errorf(codes.Internal, "upsert desired service: %v", err)
	}
	return srv.listAllDesiredServices(ctx)
}

// RemoveDesiredService deletes the ServiceDesiredVersion and sets the Removing
// flag on the corresponding ServiceRelease, triggering a lifecycle-tracked
// removal workflow (REMOVING → REMOVED).
func (srv *server) RemoveDesiredService(ctx context.Context, req *cluster_controllerpb.RemoveDesiredServiceRequest) (*cluster_controllerpb.DesiredState, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
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
	Imported      int      // new desired-state entries created
	AlreadyPresent int     // entries skipped (desired version already matches)
	Updated       int      // entries updated (desired version changed to match installed)
	Failed        int      // entries that failed to upsert
	FailedNames   []string // names of failed entries
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
			if _, exists := installed[canon]; !exists {
				ver := pkg.GetVersion()
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

	// Step 2.5: Clean up stale desired-state entries:
	//   - Command-line tools (names ending in -cmd) should never be in desired state
	//   - Services that are no longer in the installed-state registry were either
	//     never truly installed or have been removed — remove from desired state
	for canon := range existingMap {
		shouldRemove := false
		if strings.HasSuffix(canon, "-cmd") {
			shouldRemove = true
		} else if _, stillInstalled := installed[canon]; !stillInstalled {
			shouldRemove = true
		}
		if shouldRemove {
			if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", canon); err == nil {
				logger.Info("importInstalledToDesired: removed stale desired entry", "name", canon)
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

		if err := srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{
			ServiceId:   name,
			Version:     inst.version,
			BuildNumber: inst.buildNumber,
		}); err != nil {
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
	stats.Failed += appStats.Failed
	stats.FailedNames = append(stats.FailedNames, appStats.FailedNames...)

	// Step 5: Import INFRASTRUCTURE packages as InfrastructureRelease desired-state.
	infraStats := srv.importInstalledInfraToDesired(ctx)
	stats.Imported += infraStats.Imported
	stats.AlreadyPresent += infraStats.AlreadyPresent
	stats.Updated += infraStats.Updated
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
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
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
	addr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if addr == "" {
		addr = "localhost:10101"
	}

	repoClient, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return &cluster_controllerpb.ValidationReport{
			ChecksumOk:     false,
			SignatureStatus: "unknown",
			PlatformOk:     true,
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
			ChecksumOk:     false,
			SignatureStatus: "unknown",
			PlatformOk:     false,
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
		SignatureStatus:  "unsigned",
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
	addr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if addr == "" {
		addr = "localhost:10101"
	}

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
