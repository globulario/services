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
		"build_id":      m.GetBuildId(),
		"publish_state": publishStateStr(m.GetPublishState()),
		"channel":       m.GetChannel().String(),
		"description":   m.GetDescription(),
	}
	if ui := m.GetUpstreamImport(); ui != nil && ui.GetSourceName() != "" {
		result["upstream_source"] = ui.GetSourceName()
		result["origin_release"] = ui.GetOriginRelease()
		if ui.GetChangedInRelease() {
			result["changed_in_release"] = true
		} else if ui.GetPlatformRelease() != "" {
			result["changed_in_release"] = false
		}
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
		"build_id":             m.GetBuildId(),
		"entrypoint_checksum":  m.GetEntrypointChecksum(),
		"channel":              m.GetChannel().String(),
		"profiles":             m.GetProfiles(),
		"hard_deps":            formatDeps(m.GetHardDeps()),
	}
	// Upstream provenance.
	if ui := m.GetUpstreamImport(); ui != nil && ui.GetSourceName() != "" {
		result["upstream_import"] = map[string]interface{}{
			"source_name":             ui.GetSourceName(),
			"release_tag":             ui.GetReleaseTag(),
			"origin_release":          ui.GetOriginRelease(),
			"platform_release":        ui.GetPlatformRelease(),
			"changed_in_release":      ui.GetChangedInRelease(),
			"imported_at":             fmtTime(ui.GetImportedAt()),
			"package_contract_digest": ui.GetPackageContractDigest(),
		}
	}
	return result
}

// formatDeps converts ArtifactDependencyRef slice to string slice for display.
func formatDeps(deps []*repositorypb.ArtifactDependencyRef) []string {
	if len(deps) == 0 {
		return nil
	}
	out := make([]string, len(deps))
	for i, d := range deps {
		out[i] = d.GetName()
	}
	return out
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

	// ── Phase F lifecycle tools ────────────────────────────────────────────
	registerRepositoryLifecycleTools(s)
}

// kindEnum maps the operator-friendly string into the proto enum.
// Returns SERVICE for empty / unknown values to match the CLI default.
func kindEnum(s string) repositorypb.ArtifactKind {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "APPLICATION":
		return repositorypb.ArtifactKind_APPLICATION
	case "INFRASTRUCTURE":
		return repositorypb.ArtifactKind_INFRASTRUCTURE
	case "COMMAND":
		return repositorypb.ArtifactKind_COMMAND
	case "AGENT":
		return repositorypb.ArtifactKind_AGENT
	case "SUBSYSTEM":
		return repositorypb.ArtifactKind_SUBSYSTEM
	}
	return repositorypb.ArtifactKind_SERVICE
}

// registerRepositoryLifecycleTools — Phase F additions:
//   repository_verify_artifact, repository_repair_artifact,
//   repository_explain_artifact, repository_signature_verify,
//   repository_trusted_publishers, repository_list_findings,
//   repository_list_installed_revisions, repository_rollback_candidates.
//
// These wrap the repository service's read/repair/audit RPCs so an AI
// executor (or any MCP client) can answer "is this artifact safe to install?"
// without invoking the CLI subprocess.
func registerRepositoryLifecycleTools(s *server) {
	// ── repository_verify_artifact ─────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_verify_artifact",
		Description: "Verify a single repository artifact's integrity (read-only). Returns " +
			"OK or one of BROKEN_MISSING_BLOB, BROKEN_CHECKSUM_MISMATCH, " +
			"BROKEN_LEDGER_MISSING, BROKEN_MANIFEST_MISSING, QUARANTINED, " +
			"REVOKED, INCONCLUSIVE — plus installable=true|false and " +
			"recommended_action. Use this BEFORE recommending an install.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id":   {Type: "string", Description: "Publisher (e.g. core@globular.io)."},
				"name":           {Type: "string", Description: "Package name."},
				"version":        {Type: "string", Description: "Package version (e.g. 1.0.84)."},
				"platform":       {Type: "string", Description: "Target platform (default linux_amd64)."},
				"kind":           {Type: "string", Description: "service|application|infrastructure|command|agent (default service).", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"build_number":   {Type: "integer", Description: "Specific build (0 = latest PUBLISHED)."},
				"verify_digest":  {Type: "boolean", Description: "Recompute sha256 over the blob (slower, exhaustive)."},
				"verify_signature": {Type: "boolean", Description: "Reserved for signature gate; verify_signature is checked automatically when policy requires it."},
			},
			Required: []string{"publisher_id", "name", "version"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()
		platform := getStr(args, "platform")
		if platform == "" {
			platform = "linux_amd64"
		}
		req := &repositorypb.VerifyArtifactRequest{
			Ref: &repositorypb.ArtifactRef{
				PublisherId: getStr(args, "publisher_id"),
				Name:        getStr(args, "name"),
				Version:     getStr(args, "version"),
				Platform:    platform,
				Kind:        kindEnum(getStr(args, "kind")),
			},
			BuildNumber:     int64(getInt(args, "build_number", 0)),
			VerifyDigest:    getBool(args, "verify_digest", false),
			VerifySignature: getBool(args, "verify_signature", false),
			IncludeBlob:     true,
			IncludeLedger:   true,
			IncludeManifest: true,
		}
		resp, err := client.VerifyArtifact(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("VerifyArtifact: %w", err)
		}
		return verifyResponseToMap(resp), nil
	})

	// ── repository_repair_artifact ─────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_repair_artifact",
		Description: "Repair a broken artifact by re-importing from the upstream source recorded " +
			"in its manifest. Refuses REVOKED unconditionally; refuses QUARANTINED unless " +
			"allow_quarantine_override is set. Returns the action taken: repair_blob, " +
			"repair_checksum_mismatch, skipped_ok, blocked_revoked, blocked_quarantined, " +
			"or failed. Use --dry-run via the dry_run input to preview.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id":              {Type: "string", Description: "Publisher (e.g. core@globular.io)."},
				"name":                      {Type: "string", Description: "Package name."},
				"version":                   {Type: "string", Description: "Package version."},
				"platform":                  {Type: "string", Description: "Target platform (default linux_amd64)."},
				"kind":                      {Type: "string", Description: "service|application|infrastructure|command|agent (default service).", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"build_number":              {Type: "integer", Description: "Specific build (0 = latest)."},
				"dry_run":                   {Type: "boolean", Description: "Preview the action without mutating state."},
				"force":                     {Type: "boolean", Description: "Allow repair from any non-REVOKED state (skip the same-version optimization)."},
				"allow_quarantine_override": {Type: "boolean", Description: "Permit repair of QUARANTINED rows (admin-only)."},
			},
			Required: []string{"publisher_id", "name", "version"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 60*time.Second)
		defer cancel()
		platform := getStr(args, "platform")
		if platform == "" {
			platform = "linux_amd64"
		}
		resp, err := client.RepairArtifact(callCtx, &repositorypb.RepairArtifactRequest{
			Ref: &repositorypb.ArtifactRef{
				PublisherId: getStr(args, "publisher_id"),
				Name:        getStr(args, "name"),
				Version:     getStr(args, "version"),
				Platform:    platform,
				Kind:        kindEnum(getStr(args, "kind")),
			},
			BuildNumber:             int64(getInt(args, "build_number", 0)),
			DryRun:                  getBool(args, "dry_run", false),
			Force:                   getBool(args, "force", false),
			AllowQuarantineOverride: getBool(args, "allow_quarantine_override", false),
		})
		if err != nil {
			return nil, fmt.Errorf("RepairArtifact: %w", err)
		}
		return map[string]interface{}{
			"artifact_key":           resp.GetArtifactKey(),
			"action":                 resp.GetAction(),
			"detail":                 resp.GetDetail(),
			"artifact_state_before":  resp.GetArtifactStateBefore(),
			"artifact_state_after":   resp.GetArtifactStateAfter(),
			"workflow_run_id":        resp.GetWorkflowRunId(),
		}, nil
	})

	// ── repository_explain_artifact ────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_explain_artifact",
		Description: "Operator/AI cockpit — composes manifest, ledger, blob status, signature, and " +
			"pipeline state into a single read-only answer. Includes installable=true|false and " +
			"recommended_action. Read this when the user asks 'why is this artifact failing/blocked'.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string", Description: "Publisher."},
				"name":         {Type: "string", Description: "Package name."},
				"version":      {Type: "string", Description: "Package version."},
				"platform":     {Type: "string", Description: "Target platform (default linux_amd64)."},
				"kind":         {Type: "string", Description: "service|application|infrastructure|command|agent.", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"build_number": {Type: "integer", Description: "Specific build (0 = latest)."},
			},
			Required: []string{"publisher_id", "name", "version"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()
		platform := getStr(args, "platform")
		if platform == "" {
			platform = "linux_amd64"
		}
		resp, err := client.ExplainArtifact(callCtx, &repositorypb.ExplainArtifactRequest{
			Ref: &repositorypb.ArtifactRef{
				PublisherId: getStr(args, "publisher_id"),
				Name:        getStr(args, "name"),
				Version:     getStr(args, "version"),
				Platform:    platform,
				Kind:        kindEnum(getStr(args, "kind")),
			},
			BuildNumber: int64(getInt(args, "build_number", 0)),
		})
		if err != nil {
			return nil, fmt.Errorf("ExplainArtifact: %w", err)
		}
		return map[string]interface{}{
			"artifact_key":            resp.GetArtifactKey(),
			"artifact_state":          resp.GetArtifactState(),
			"publish_state":           resp.GetPublishState().String(),
			"blob_key":                resp.GetBlobKey(),
			"blob_present":            resp.GetBlobPresent(),
			"expected_size":           resp.GetExpectedSize(),
			"actual_size":             resp.GetActualSize(),
			"expected_digest":         resp.GetExpectedDigest(),
			"actual_digest":           resp.GetActualDigest(),
			"manifest_present":        resp.GetManifestPresent(),
			"ledger_present":          resp.GetLedgerPresent(),
			"signature_status":        resp.GetSignatureStatus(),
			"installable":             resp.GetInstallable(),
			"recommended_action":      resp.GetRecommendedAction(),
			"related_workflow_run_id": resp.GetRelatedWorkflowRunId(),
			"verify_status":           strings.TrimPrefix(resp.GetVerifyStatus().String(), "ARTIFACT_VERIFY_"),
			"detail":                  resp.GetDetail(),
		}, nil
	})

	// ── repository_signature_verify ────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_signature_verify",
		Description: "Verify the most recent signature on an artifact against the trusted-publishers " +
			"registry and the active signature policy. Returns SIGNATURE_OK / MISSING / INVALID / " +
			"UNTRUSTED_PUBLISHER / REVOKED_KEY / EXPIRED_KEY / DIGEST_MISMATCH / INCONCLUSIVE.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string"},
				"name":         {Type: "string"},
				"version":      {Type: "string"},
				"platform":     {Type: "string"},
				"kind":         {Type: "string", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"build_number": {Type: "integer"},
			},
			Required: []string{"publisher_id", "name", "version"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()
		platform := getStr(args, "platform")
		if platform == "" {
			platform = "linux_amd64"
		}
		resp, err := client.VerifyArtifactSignature(callCtx, &repositorypb.VerifyArtifactSignatureRequest{
			Ref: &repositorypb.ArtifactRef{
				PublisherId: getStr(args, "publisher_id"),
				Name:        getStr(args, "name"),
				Version:     getStr(args, "version"),
				Platform:    platform,
				Kind:        kindEnum(getStr(args, "kind")),
			},
			BuildNumber: int64(getInt(args, "build_number", 0)),
		})
		if err != nil {
			return nil, fmt.Errorf("VerifyArtifactSignature: %w", err)
		}
		out := map[string]interface{}{
			"status": strings.TrimPrefix(resp.GetStatus().String(), "SIGNATURE_"),
			"reason": resp.GetReason(),
		}
		if sig := resp.GetSignature(); sig != nil {
			out["signature"] = map[string]interface{}{
				"public_key_id":  sig.GetPublicKeyId(),
				"algorithm":      sig.GetAlgorithm(),
				"signed_by":      sig.GetSignedBy(),
				"signed_at_unix": sig.GetSignedAtUnix(),
			}
		}
		if pub := resp.GetPublisher(); pub != nil {
			out["publisher"] = map[string]interface{}{
				"publisher_id":     pub.GetPublisherId(),
				"public_key_id":    pub.GetPublicKeyId(),
				"trust_state":      strings.TrimPrefix(pub.GetTrustState().String(), "TRUST_"),
				"valid_until_unix": pub.GetValidUntilUnix(),
			}
		}
		return out, nil
	})

	// ── repository_trusted_publishers ──────────────────────────────────────
	s.register(toolDef{
		Name:        "repository_trusted_publishers",
		Description: "List trusted publisher keys with their trust state (TRUSTED / REVOKED / EXPIRED).",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string", Description: "Optional filter (default: all publishers)."},
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
		resp, err := client.ListTrustedPublishers(callCtx, &repositorypb.ListTrustedPublishersRequest{
			PublisherId: getStr(args, "publisher_id"),
		})
		if err != nil {
			return nil, fmt.Errorf("ListTrustedPublishers: %w", err)
		}
		rows := make([]map[string]interface{}, 0, len(resp.GetPublishers()))
		for _, p := range resp.GetPublishers() {
			rows = append(rows, map[string]interface{}{
				"publisher_id":     p.GetPublisherId(),
				"public_key_id":    p.GetPublicKeyId(),
				"algorithm":        p.GetAlgorithm(),
				"trust_state":      strings.TrimPrefix(p.GetTrustState().String(), "TRUST_"),
				"valid_from_unix":  p.GetValidFromUnix(),
				"valid_until_unix": p.GetValidUntilUnix(),
				"created_by":       p.GetCreatedBy(),
				"notes":            p.GetNotes(),
			})
		}
		return map[string]interface{}{
			"count":      len(rows),
			"publishers": rows,
		}, nil
	})

	// ── repository_list_findings ───────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_list_findings",
		Description: "List repository self-reported integrity findings: PUBLISHED rows with missing " +
			"blobs, checksum mismatches, missing required signatures, and lifecycle " +
			"incoherence (publish_state vs artifact_state). The cluster-doctor pulls from " +
			"this same surface; the AI executor uses it to triage 'what's broken right now'.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"kind_filter": {
					Type:        "string",
					Description: "Optional finding-kind filter.",
					Enum: []string{
						"PUBLISHED_MISSING_BLOB",
						"PUBLISHED_CHECKSUM_MISMATCH",
						"PUBLISHED_UNSIGNED_REQUIRED",
						"REVOKED_INSTALLABLE",
						"QUARANTINED_INSTALLABLE",
						"CONFIG_CONFLICT",
						"ROLLBACK_FAILED",
					},
				},
				"limit": {Type: "integer", Description: "Max findings (default 200)."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		var kf repositorypb.RepositoryFindingKind
		if v := strings.ToUpper(getStr(args, "kind_filter")); v != "" {
			full := "REPO_FIND_" + v
			if id, ok := repositorypb.RepositoryFindingKind_value[full]; ok {
				kf = repositorypb.RepositoryFindingKind(id)
			}
		}
		resp, err := client.ListRepositoryFindings(callCtx, &repositorypb.ListRepositoryFindingsRequest{
			KindFilter: kf,
			Limit:      int32(int64(getInt(args, "limit", 0))),
		})
		if err != nil {
			return nil, fmt.Errorf("ListRepositoryFindings: %w", err)
		}
		out := make([]map[string]interface{}, 0, len(resp.GetFindings()))
		for _, f := range resp.GetFindings() {
			out = append(out, map[string]interface{}{
				"kind":                strings.TrimPrefix(f.GetKind().String(), "REPO_FIND_"),
				"severity":            strings.TrimPrefix(f.GetSeverity().String(), "REPO_FIND_"),
				"artifact_key":        f.GetArtifactKey(),
				"node_id":             f.GetNodeId(),
				"current_state":       f.GetCurrentState(),
				"expected_state":      f.GetExpectedState(),
				"reason":              f.GetReason(),
				"recommended_command": f.GetRecommendedCommand(),
				"evidence":            f.GetEvidence(),
				"observed_at_unix":    f.GetObservedAtUnix(),
			})
		}
		return map[string]interface{}{
			"generated_at_unix": resp.GetGeneratedAtUnix(),
			"count":             len(out),
			"findings":          out,
		}, nil
	})

	// ── repository_list_installed_revisions ────────────────────────────────
	s.register(toolDef{
		Name: "repository_list_installed_revisions",
		Description: "List the installed-revision history for a package (newest first). One row per " +
			"successful install / upgrade / rollback. Use this BEFORE recommending a rollback " +
			"to see what versions are reachable.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string"},
				"name":         {Type: "string"},
				"platform":     {Type: "string"},
				"kind":         {Type: "string", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"node_id":      {Type: "string", Description: "Optional: limit to one node."},
				"limit":        {Type: "integer", Description: "Max rows (default 25)."},
			},
			Required: []string{"publisher_id", "name", "platform"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()
		resp, err := client.ListInstalledRevisions(callCtx, &repositorypb.ListInstalledRevisionsRequest{
			PublisherId: getStr(args, "publisher_id"),
			Name:        getStr(args, "name"),
			Platform:    getStr(args, "platform"),
			Kind:        kindEnum(getStr(args, "kind")),
			NodeId:      getStr(args, "node_id"),
			Limit:       int32(int64(getInt(args, "limit", 0))),
		})
		if err != nil {
			return nil, fmt.Errorf("ListInstalledRevisions: %w", err)
		}
		rows := make([]map[string]interface{}, 0, len(resp.GetRevisions()))
		for _, r := range resp.GetRevisions() {
			rows = append(rows, revisionToMap(r))
		}
		return map[string]interface{}{
			"count":     len(rows),
			"revisions": rows,
		}, nil
	})

	// ── repository_rollback_candidates ─────────────────────────────────────
	s.register(toolDef{
		Name: "repository_rollback_candidates",
		Description: "List previous installable revisions for a package, with eligibility flags " +
			"(target must be PUBLISHED, blob verified, signature policy passed, not REVOKED " +
			"or QUARANTINED). Use this to advise on `globular pkg rollback --to-version <ver>`.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string"},
				"name":         {Type: "string"},
				"platform":     {Type: "string"},
				"kind":         {Type: "string", Enum: []string{"SERVICE", "APPLICATION", "AGENT", "SUBSYSTEM", "INFRASTRUCTURE", "COMMAND"}},
				"node_id":      {Type: "string", Description: "Optional: limit to one node."},
				"limit":        {Type: "integer", Description: "Max candidates (default 5)."},
			},
			Required: []string{"publisher_id", "name", "platform"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()
		resp, err := client.ListRollbackCandidates(callCtx, &repositorypb.ListRollbackCandidatesRequest{
			PublisherId: getStr(args, "publisher_id"),
			Name:        getStr(args, "name"),
			Platform:    getStr(args, "platform"),
			Kind:        kindEnum(getStr(args, "kind")),
			NodeId:      getStr(args, "node_id"),
			Limit:       int32(int64(getInt(args, "limit", 0))),
		})
		if err != nil {
			return nil, fmt.Errorf("ListRollbackCandidates: %w", err)
		}
		current := map[string]interface{}{}
		if c := resp.GetCurrentRef(); c != nil {
			current = map[string]interface{}{
				"publisher_id": c.GetPublisherId(),
				"name":         c.GetName(),
				"version":      c.GetVersion(),
				"platform":     c.GetPlatform(),
				"kind":         artifactKindStr(c.GetKind()),
			}
		}
		cands := make([]map[string]interface{}, 0, len(resp.GetCandidates()))
		for _, cand := range resp.GetCandidates() {
			eli := cand.GetEligibility()
			cands = append(cands, map[string]interface{}{
				"target_ref": map[string]interface{}{
					"publisher_id": cand.GetTargetRef().GetPublisherId(),
					"name":         cand.GetTargetRef().GetName(),
					"version":      cand.GetTargetRef().GetVersion(),
					"platform":     cand.GetTargetRef().GetPlatform(),
				},
				"revision":         revisionToMap(cand.GetRevision()),
				"eligible":         eli.GetEligible(),
				"reason":           eli.GetReason(),
				"verify_status":    strings.TrimPrefix(eli.GetVerifyStatus().String(), "ARTIFACT_VERIFY_"),
				"signature_status": strings.TrimPrefix(eli.GetSignatureStatus().String(), "SIGNATURE_"),
			})
		}
		return map[string]interface{}{
			"current":    current,
			"candidates": cands,
		}, nil
	})
}

// verifyResponseToMap flattens VerifyArtifactResponse into a JSON-friendly shape.
func verifyResponseToMap(resp *repositorypb.VerifyArtifactResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}
	return map[string]interface{}{
		"artifact_key":       resp.GetArtifactKey(),
		"artifact_state":     resp.GetArtifactState(),
		"publish_state":      resp.GetPublishState().String(),
		"installable":        resp.GetInstallable(),
		"status":             strings.TrimPrefix(resp.GetStatus().String(), "ARTIFACT_VERIFY_"),
		"reason":             resp.GetReason(),
		"blob_key":           resp.GetBlobKey(),
		"expected_size":      resp.GetExpectedSize(),
		"actual_size":        resp.GetActualSize(),
		"expected_digest":    resp.GetExpectedDigest(),
		"actual_digest":      resp.GetActualDigest(),
		"signature_status":   resp.GetSignatureStatus(),
		"provenance_status":  resp.GetProvenanceStatus(),
		"repairable":         resp.GetRepairable(),
		"recommended_action": resp.GetRecommendedAction(),
	}
}

// revisionToMap normalizes an InstalledPackageRevision into a flat map.
func revisionToMap(r *repositorypb.InstalledPackageRevision) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"revision_id":              r.GetRevisionId(),
		"publisher_id":              r.GetPublisherId(),
		"name":                      r.GetName(),
		"kind":                      artifactKindStr(r.GetKind()),
		"version":                   r.GetVersion(),
		"build_id":                  r.GetBuildId(),
		"build_number":              r.GetBuildNumber(),
		"platform":                  r.GetPlatform(),
		"checksum":                  r.GetChecksum(),
		"installed_at_unix":         r.GetInstalledAtUnix(),
		"installed_by_workflow_run_id": r.GetInstalledByWorkflowRunId(),
		"node_id":                   r.GetNodeId(),
		"previous_revision_id":      r.GetPreviousRevisionId(),
		"action":                    r.GetAction(),
		"service_status_after":      r.GetServiceStatusAfter(),
	}
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
