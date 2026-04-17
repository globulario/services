package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/repository/repository_client"
)

// RepairStateAlignment diagnoses and optionally repairs alignment between the
// 4 state layers: artifact (repository), desired release, installed observed,
// and runtime health. This is the controller-side handler for the repair command.
func (srv *server) RepairStateAlignment(ctx context.Context, req *cluster_controllerpb.RepairStateAlignmentRequest) (*cluster_controllerpb.StateAlignmentReport, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.StateAlignmentReport{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RepairStateAlignment", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}

	report := &cluster_controllerpb.StateAlignmentReport{}

	// Step 1: Collect installed packages across all nodes (all kinds).
	type pkgInfo struct {
		name        string
		version     string
		buildNumber int64
		kind        string
		publisherID string
	}
	installed := make(map[string]pkgInfo) // "KIND/name" -> info

	for _, kind := range []string{"SERVICE", "APPLICATION", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListAllNodes(ctx, kind, "")
		if err != nil {
			log.Printf("RepairStateAlignment: ListAllNodes(%s) failed: %v", kind, err)
			continue
		}
		for _, pkg := range pkgs {
			name := strings.TrimSpace(pkg.GetName())
			if name == "" {
				continue
			}
			key := kind + "/" + name
			if _, exists := installed[key]; !exists {
				ver := pkg.GetVersion()
				if cv, err := versionutil.Canonical(ver); err == nil {
					ver = cv
				}
				installed[key] = pkgInfo{
					name:        name,
					version:     ver,
					buildNumber: pkg.GetBuildNumber(),
					kind:        kind,
					publisherID: pkg.GetPublisherId(),
				}
			}
		}
	}

	// Step 2: Collect desired releases (all kinds).
	desired := srv.collectDesiredVersions(ctx)

	// Step 3: Query repository for available versions (best-effort).
	type repoInfo struct {
		version     string
		buildNumber int64
	}
	repoVersions := make(map[string]repoInfo) // "normalized_name" -> latest
	repo := resolveRepositoryInfo()
	report.RepositoryAddr = repo.Address

	rc, err := repository_client.NewRepositoryService_Client(repo.Address, "repository.PackageRepository")
	if err != nil {
		log.Printf("RepairStateAlignment: repository unreachable at %s: %v", repo.Address, err)
	} else {
		if arts, err := rc.ListArtifacts(); err == nil {
			for _, m := range arts {
				if m.GetRef() == nil {
					continue
				}
				name := m.GetRef().GetName()
				ver := m.GetRef().GetVersion()
				if cv, err := versionutil.Canonical(ver); err == nil {
					ver = cv
				}
				key := strings.ToLower(name)
				existing, ok := repoVersions[key]
				if !ok {
					repoVersions[key] = repoInfo{version: ver, buildNumber: m.GetBuildNumber()}
				} else if cmp, cerr := versionutil.CompareFull(existing.version, existing.buildNumber, ver, m.GetBuildNumber()); cerr == nil && cmp < 0 {
					repoVersions[key] = repoInfo{version: ver, buildNumber: m.GetBuildNumber()}
				}
			}
		}
		if bundles, err := rc.ListBundles(); err == nil {
			for _, b := range bundles {
				name := canonicalServiceName(b.GetName())
				ver := b.GetVersion()
				if cv, err := versionutil.Canonical(ver); err == nil {
					ver = cv
				}
				key := strings.ToLower(name)
				if _, exists := repoVersions[key]; !exists {
					repoVersions[key] = repoInfo{version: ver, buildNumber: b.GetBuildNumber()}
				}
			}
		}
		rc.Close()
	}

	// Step 4: If not dry-run, run import to repair missing desired-state.
	if !req.DryRun {
		if n := srv.cleanupStaleDesiredKeys(ctx); n > 0 {
			log.Printf("RepairStateAlignment: cleaned up %d stale desired keys", n)
		}
		stats, err := srv.importInstalledToDesired(ctx)
		if err != nil {
			log.Printf("RepairStateAlignment: import failed: %v", err)
		} else {
			report.Repaired = stats.Imported + stats.Updated
		}
		// Re-read desired state after repair.
		desired = srv.collectDesiredVersions(ctx)
	}

	// Step 5: Cross-reference all layers and produce per-package status.
	seen := make(map[string]bool)
	for key, pkg := range installed {
		seen[key] = true
		desiredDV := desired[key]
		repo := repoVersions[strings.ToLower(pkg.name)]

		entry := &cluster_controllerpb.PackageAlignmentStatus{
			Name:             pkg.name,
			Kind:             pkg.kind,
			InstalledVersion: pkg.version,
			InstalledBuildNum: pkg.buildNumber,
			DesiredVersion:   desiredDV.version,
			DesiredBuildNum:  desiredDV.buildNumber,
			RepoVersion:      repo.version,
			RepoBuildNum:     repo.buildNumber,
		}

		switch {
		case desiredDV.version == "" && repo.version == "":
			entry.Status = "unmanaged"
			entry.Message = "installed but no desired release and not in repository"
			report.Unmanaged++
		case desiredDV.version == "":
			entry.Status = "unmanaged"
			entry.Message = "installed but no desired release"
			report.Unmanaged++
		case repo.version == "":
			entry.Status = "missing_in_repo"
			entry.Message = "desired release exists but artifact not found in repository"
			report.MissingInRepo++
		case !versionutil.EqualFull(desiredDV.version, desiredDV.buildNumber, pkg.version, pkg.buildNumber):
			entry.Status = "drifted"
			entry.Message = fmt.Sprintf("installed %s (build %d) differs from desired %s (build %d)", pkg.version, pkg.buildNumber, desiredDV.version, desiredDV.buildNumber)
			report.Drifted++
		default:
			entry.Status = "installed"
			report.Aligned++
		}

		report.Packages = append(report.Packages, entry)
	}

	// Check desired releases without installed packages.
	for key, dv := range desired {
		if seen[key] {
			continue
		}
		parts := strings.SplitN(key, "/", 2)
		kind, name := parts[0], parts[1]
		repo := repoVersions[strings.ToLower(name)]

		entry := &cluster_controllerpb.PackageAlignmentStatus{
			Name:            name,
			Kind:            kind,
			DesiredVersion:  dv.version,
			DesiredBuildNum: dv.buildNumber,
			RepoVersion:     repo.version,
			RepoBuildNum:    repo.buildNumber,
			Status:          "planned",
			Message:         "desired release exists but not installed on any node",
		}
		report.Drifted++ // planned counts toward drift for repair purposes
		report.Packages = append(report.Packages, entry)
	}

	return report, nil
}

// desiredVersionInfo holds a version and build number from a desired release.
type desiredVersionInfo struct {
	version     string
	buildNumber int64
}

// collectDesiredVersions reads all desired release versions from the resource
// store, returning a map from "KIND/name" to version+build.
func (srv *server) collectDesiredVersions(ctx context.Context) map[string]desiredVersionInfo {
	desired := make(map[string]desiredVersionInfo)
	if srv.resources == nil {
		return desired
	}
	// Services (ServiceDesiredVersion + ServiceRelease)
	if items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", ""); err == nil {
		for _, obj := range items {
			if sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion); ok && sdv.Spec != nil {
				canon := canonicalServiceName(sdv.Spec.ServiceName)
				if canon == "" {
					canon = sdv.Spec.ServiceName
				}
				desired["SERVICE/"+canon] = desiredVersionInfo{version: sdv.Spec.Version, buildNumber: sdv.Spec.BuildNumber}
			}
		}
	}
	if items, _, err := srv.resources.List(ctx, "ServiceRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*cluster_controllerpb.ServiceRelease); ok && rel.Spec != nil {
				canon := canonicalServiceName(rel.Spec.ServiceName)
				key := "SERVICE/" + canon
				if _, exists := desired[key]; !exists {
					desired[key] = desiredVersionInfo{version: rel.Spec.Version, buildNumber: rel.Spec.BuildNumber}
				}
			}
		}
	}
	// Applications
	if items, _, err := srv.resources.List(ctx, "ApplicationRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*cluster_controllerpb.ApplicationRelease); ok && rel.Spec != nil {
				desired["APPLICATION/"+rel.Spec.AppName] = desiredVersionInfo{version: rel.Spec.Version, buildNumber: rel.Spec.BuildNumber}
			}
		}
	}
	// Infrastructure
	if items, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
		for _, obj := range items {
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
				desired["INFRASTRUCTURE/"+rel.Spec.Component] = desiredVersionInfo{version: rel.Spec.Version, buildNumber: rel.Spec.BuildNumber}
			}
		}
	}
	return desired
}
