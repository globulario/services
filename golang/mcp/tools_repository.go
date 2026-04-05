package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

// artifactKindStr converts an ArtifactKind enum to a lowercase human-readable string.
func artifactKindStr(k repositorypb.ArtifactKind) string {
	switch k {
	case repositorypb.ArtifactKind_SERVICE:
		return "service"
	case repositorypb.ArtifactKind_APPLICATION:
		return "application"
	case repositorypb.ArtifactKind_AGENT:
		return "agent"
	case repositorypb.ArtifactKind_SUBSYSTEM:
		return "subsystem"
	case repositorypb.ArtifactKind_INFRASTRUCTURE:
		return "infrastructure"
	case repositorypb.ArtifactKind_COMMAND:
		return "command"
	default:
		return "unknown"
	}
}

// publishStateStr converts a PublishState enum to a lowercase human-readable string.
func publishStateStr(s repositorypb.PublishState) string {
	switch s {
	case repositorypb.PublishState_STAGING:
		return "staging"
	case repositorypb.PublishState_VERIFIED:
		return "verified"
	case repositorypb.PublishState_PUBLISHED:
		return "published"
	case repositorypb.PublishState_FAILED:
		return "failed"
	case repositorypb.PublishState_ORPHANED:
		return "orphaned"
	case repositorypb.PublishState_DEPRECATED:
		return "deprecated"
	case repositorypb.PublishState_YANKED:
		return "yanked"
	case repositorypb.PublishState_QUARANTINED:
		return "quarantined"
	case repositorypb.PublishState_REVOKED:
		return "revoked"
	default:
		return "staging"
	}
}

// normalizeArtifactManifest converts a protobuf ArtifactManifest to a normalized map.
func normalizeArtifactManifest(m *repositorypb.ArtifactManifest) map[string]interface{} {
	ref := m.GetRef()
	result := map[string]interface{}{
		"publisher":     ref.GetPublisherId(),
		"name":          ref.GetName(),
		"version":       ref.GetVersion(),
		"platform":      ref.GetPlatform(),
		"kind":          artifactKindStr(ref.GetKind()),
		"size":          fmtBytes(uint64(m.GetSizeBytes())),
		"build_number":  m.GetBuildNumber(),
		"publish_state": publishStateStr(m.GetPublishState()),
		"description":   m.GetDescription(),
	}
	return result
}

// normalizeArtifactManifestFull converts a protobuf ArtifactManifest to a detailed map.
func normalizeArtifactManifestFull(m *repositorypb.ArtifactManifest) map[string]interface{} {
	ref := m.GetRef()
	result := map[string]interface{}{
		"publisher":            ref.GetPublisherId(),
		"name":                 ref.GetName(),
		"version":              ref.GetVersion(),
		"platform":             ref.GetPlatform(),
		"kind":                 artifactKindStr(ref.GetKind()),
		"checksum":             m.GetChecksum(),
		"size_bytes":           m.GetSizeBytes(),
		"size":                 fmtBytes(uint64(m.GetSizeBytes())),
		"build_number":         m.GetBuildNumber(),
		"publish_state":        publishStateStr(m.GetPublishState()),
		"description":          m.GetDescription(),
		"alias":                m.GetAlias(),
		"license":              m.GetLicense(),
		"keywords":             m.GetKeywords(),
		"provides":             m.GetProvides(),
		"requires":             m.GetRequires(),
		"entrypoints":          m.GetEntrypoints(),
		"min_globular_version": m.GetMinGlobularVersion(),
		"modified_at":          fmtTime(m.GetModifiedUnix()),
		"published_at":         fmtTime(m.GetPublishedUnix()),
		"build_commit":         m.GetBuildCommit(),
		"build_timestamp":      fmtTime(m.GetBuildTimestampUnix()),
		"build_source":         m.GetBuildSource(),
		"build_notes":          m.GetBuildNotes(),
	}
	return result
}

func registerRepositoryTools(s *server) {

	// ── repository_list_artifacts ───────────────────────────────────────────
	s.register(toolDef{
		Name:        "repository_list_artifacts",
		Description: "List all artifacts in the package repository catalog.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ListArtifacts(callCtx, &repositorypb.ListArtifactsRequest{})
		if err != nil {
			return nil, fmt.Errorf("ListArtifacts: %w", err)
		}

		artifacts := make([]map[string]interface{}, 0, len(resp.GetArtifacts()))
		for _, a := range resp.GetArtifacts() {
			artifacts = append(artifacts, normalizeArtifactManifest(a))
		}

		return map[string]interface{}{
			"count":     len(artifacts),
			"artifacts": artifacts,
		}, nil
	})

	// ── repository_search_artifacts ─────────────────────────────────────────
	s.register(toolDef{
		Name:        "repository_search_artifacts",
		Description: "Search for artifacts in the repository by query text, kind, or publisher.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query": {
					Type:        "string",
					Description: "Free-text search across name, description, and keywords.",
				},
				"kind": {
					Type:        "string",
					Description: "Filter by artifact kind.",
					Enum:        []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"},
				},
				"publisher_id": {
					Type:        "string",
					Description: "Filter by publisher ID.",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &repositorypb.SearchArtifactsRequest{
			Query:       getStr(args, "query"),
			PublisherId: getStr(args, "publisher_id"),
		}

		// Map kind string to enum.
		if kindStr := strings.ToUpper(getStr(args, "kind")); kindStr != "" {
			if v, ok := repositorypb.ArtifactKind_value[kindStr]; ok {
				req.Kind = repositorypb.ArtifactKind(v)
			}
		}

		resp, err := client.SearchArtifacts(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("SearchArtifacts: %w", err)
		}

		artifacts := make([]map[string]interface{}, 0, len(resp.GetArtifacts()))
		for _, a := range resp.GetArtifacts() {
			artifacts = append(artifacts, normalizeArtifactManifest(a))
		}

		return map[string]interface{}{
			"count":     len(artifacts),
			"artifacts": artifacts,
		}, nil
	})

	// ── repository_get_artifact_manifest ────────────────────────────────────
	s.register(toolDef{
		Name:        "repository_get_artifact_manifest",
		Description: "Get the full manifest of a specific artifact version from the repository.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {
					Type:        "string",
					Description: "Publisher ID (required).",
				},
				"name": {
					Type:        "string",
					Description: "Artifact name (required).",
				},
				"version": {
					Type:        "string",
					Description: "Artifact version (optional; returns latest if omitted).",
				},
				"platform": {
					Type:        "string",
					Description: "Target platform (optional, e.g. linux/amd64).",
				},
			},
			Required: []string{"publisher_id", "name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		publisherID := getStr(args, "publisher_id")
		name := getStr(args, "name")
		if publisherID == "" || name == "" {
			return nil, fmt.Errorf("missing required arguments: publisher_id and name")
		}

		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &repositorypb.GetArtifactManifestRequest{
			Ref: &repositorypb.ArtifactRef{
				PublisherId: publisherID,
				Name:        name,
				Version:     getStr(args, "version"),
				Platform:    getStr(args, "platform"),
			},
		}

		resp, err := client.GetArtifactManifest(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("GetArtifactManifest: %w", err)
		}

		manifest := resp.GetManifest()
		if manifest == nil {
			return map[string]interface{}{"error": "manifest not found"}, nil
		}

		return normalizeArtifactManifestFull(manifest), nil
	})

	// ── repository_get_artifact_versions ────────────────────────────────────
	s.register(toolDef{
		Name:        "repository_get_artifact_versions",
		Description: "List all available versions of a specific artifact in the repository.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {
					Type:        "string",
					Description: "Publisher ID (required).",
				},
				"name": {
					Type:        "string",
					Description: "Artifact name (required).",
				},
			},
			Required: []string{"publisher_id", "name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		publisherID := getStr(args, "publisher_id")
		name := getStr(args, "name")
		if publisherID == "" || name == "" {
			return nil, fmt.Errorf("missing required arguments: publisher_id and name")
		}

		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &repositorypb.GetArtifactVersionsRequest{
			PublisherId: publisherID,
			Name:        name,
		}

		resp, err := client.GetArtifactVersions(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("GetArtifactVersions: %w", err)
		}

		versions := make([]map[string]interface{}, 0, len(resp.GetVersions()))
		for _, v := range resp.GetVersions() {
			versions = append(versions, map[string]interface{}{
				"version":       v.GetRef().GetVersion(),
				"build_number":  v.GetBuildNumber(),
				"publish_state": publishStateStr(v.GetPublishState()),
				"published_at":  fmtTime(v.GetPublishedUnix()),
				"size":          fmtBytes(uint64(v.GetSizeBytes())),
			})
		}

		return map[string]interface{}{
			"name":      name,
			"publisher": publisherID,
			"versions":  versions,
		}, nil
	})

	// ── pkg_info ────────────────────────────────────────────────────────────
	// Phase 2 aggregator: "what is this package, what's desired, where is
	// it installed, is anything failing?" Single-package scoped per
	// Clause 5; flat output per Clause 11; chains with node_resolve for
	// hostname display.
	s.register(toolDef{
		Name: "pkg_info",
		Description: "Describes a single package: its kind, all published versions, the cluster-wide " +
			"desired version (if any), per-node install status, and which nodes are failing. " +
			"Chain node_id fields into node_resolve to get hostnames. Use this instead of " +
			"running 3 separate queries across repo/etcd/nodes.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name": {
					Type:        "string",
					Description: "Package name (e.g. 'cluster-controller', 'claude'). Both '-' and '_' separators accepted.",
				},
				"publisher_id": {
					Type:        "string",
					Description: "Optional: filter by publisher (e.g. 'core@globular.io').",
				},
			},
			Required: []string{"name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name, _ := args["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		publisher, _ := args["publisher_id"].(string)

		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		rsp, err := client.DescribePackage(callCtx, &repositorypb.DescribePackageRequest{
			Name:        name,
			PublisherId: publisher,
		})
		if err != nil {
			return nil, fmt.Errorf("DescribePackage: %w", err)
		}
		info := rsp.GetInfo()
		if info == nil {
			return nil, fmt.Errorf("no info returned for %q", name)
		}
		return packageInfoToMap(info), nil
	})
}

// packageInfoToMap flattens PackageInfo into a plain map for MCP JSON output.
// Keeps the shape identical to CLI --json so callers can predict both.
func packageInfoToMap(info *repositorypb.PackageInfo) map[string]interface{} {
	desired := map[string]interface{}{"present": false}
	if d := info.GetDesired(); d != nil && d.GetPresent() {
		desired = map[string]interface{}{
			"present":    true,
			"version":    d.GetVersion(),
			"generation": d.GetGeneration(),
			"publisher":  d.GetPublisher(),
		}
	}
	installed := make([]map[string]interface{}, 0, len(info.GetInstalledOn()))
	for _, n := range info.GetInstalledOn() {
		installed = append(installed, map[string]interface{}{
			"node_id":      n.GetNodeId(),
			"version":      n.GetVersion(),
			"status":       n.GetStatus(),
			"installed_at": n.GetInstalledAt(),
		})
	}
	failing := make([]map[string]interface{}, 0, len(info.GetFailingOn()))
	for _, n := range info.GetFailingOn() {
		failing = append(failing, map[string]interface{}{
			"node_id": n.GetNodeId(),
			"version": n.GetVersion(),
			"status":  n.GetStatus(),
		})
	}
	return map[string]interface{}{
		"name":           info.GetName(),
		"kind":           info.GetKind().String(),
		"publisher":      info.GetPublisher(),
		"versions":       info.GetVersions(),
		"latest_version": info.GetLatestVersion(),
		"desired":        desired,
		"installed_on":   installed,
		"failing_on":     failing,
		"source":         info.GetSource(),
		"observed_at":    info.GetObservedAt(),
	}
}
