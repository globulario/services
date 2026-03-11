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
	"os"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/versionutil"
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
	items, rv, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list desired services: %v", err)
	}
	ds := &cluster_controllerpb.DesiredState{Revision: rv}
	seen := make(map[string]bool)
	for _, obj := range items {
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
			ServiceId: canon,
			Version:   sdv.Spec.Version,
		})
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
		},
	}
	_, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", obj)
	return err
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

// RemoveDesiredService deletes a single desired-service entry.
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
	if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", name); err != nil {
		return nil, status.Errorf(codes.Internal, "remove desired service: %v", err)
	}
	return srv.listAllDesiredServices(ctx)
}

// SeedDesiredState bulk-populates desired state.
//
// IMPORT_FROM_INSTALLED: reads installed_versions from all nodes (union),
// creates one DesiredService per unique service (first-seen version wins).
//
// DEFAULT_CORE_PROFILE: not yet defined; returns an error until a core
// profile catalogue is available.
func (srv *server) SeedDesiredState(ctx context.Context, req *cluster_controllerpb.SeedDesiredStateRequest) (*cluster_controllerpb.DesiredState, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}

	// Clean up any stale domain-prefixed keys from previous seeds.
	if n := srv.cleanupStaleDesiredKeys(ctx); n > 0 {
		fmt.Printf("SeedDesiredState: cleaned up %d stale domain-prefixed entries\n", n)
	}

	switch req.GetMode() {
	case cluster_controllerpb.SeedDesiredStateRequest_IMPORT_FROM_INSTALLED:
		// Collect installed versions from all nodes (union, first-seen wins).
		srv.lock("SeedDesiredState")
		installed := make(map[string]string) // canonical name → version
		for _, node := range srv.state.Nodes {
			for svcID, ver := range node.InstalledVersions {
				canon := canonicalServiceName(svcID)
				if canon == "" || ver == "" {
					continue
				}
				if _, exists := installed[canon]; !exists {
					if cv, err := versionutil.Canonical(ver); err == nil {
						ver = cv
					}
					installed[canon] = ver
				}
			}
		}
		srv.unlock()

		if len(installed) == 0 {
			return nil, status.Error(codes.FailedPrecondition,
				"no installed services found on any node; cannot seed from installed")
		}

		for name, ver := range installed {
			if err := srv.upsertOne(ctx, &cluster_controllerpb.DesiredService{
				ServiceId: name,
				Version:   ver,
			}); err != nil {
				return nil, status.Errorf(codes.Internal, "seed %q: %v", name, err)
			}
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

// artifactNameMatchesService returns true when an artifact's stored name corresponds
// to the given serviceId.  Handles dot-qualified names like "authentication.AuthenticationService"
// matching a stored artifact named "AuthenticationService".
func artifactNameMatchesService(artifactName, serviceId string) bool {
	if artifactName == serviceId {
		return true
	}
	parts := strings.Split(serviceId, ".")
	return parts[len(parts)-1] == artifactName
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

// bundleNameMatchesService returns true when a bundle's stored name corresponds
// to the given serviceId after normalisation.
func bundleNameMatchesService(bundleName, serviceId string) bool {
	return normalizeServiceName(bundleName) == normalizeServiceName(serviceId)
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
func (srv *server) PreviewDesiredServices(_ context.Context, req *cluster_controllerpb.DesiredServicesDelta) (*cluster_controllerpb.PlanPreview, error) {
	// Snapshot node state under lock.
	srv.lock("PreviewDesiredServices")
	nodeCopy := make(map[string]*nodeState, len(srv.state.Nodes))
	for id, n := range srv.state.Nodes {
		nodeCopy[id] = n
	}
	srv.unlock()

	preview := &cluster_controllerpb.PlanPreview{}

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
