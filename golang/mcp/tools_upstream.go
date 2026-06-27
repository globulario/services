package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

func registerUpstreamTools(s *server) {

	// ── repository_upstream_list ─────────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_upstream_list",
		Description: "List all registered upstream release sources. Shows provider type, " +
			"sync status, channel, platform, and trust policy. Credentials are redacted.",
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

		resp, err := client.ListUpstreams(callCtx, &repositorypb.ListUpstreamsRequest{})
		if err != nil {
			return nil, fmt.Errorf("ListUpstreams: %w", err)
		}

		sources := make([]map[string]interface{}, 0, len(resp.GetSources()))
		for _, src := range resp.GetSources() {
			entry := map[string]interface{}{
				"name":             src.GetName(),
				"type":             src.GetType().String(),
				"enabled":          src.GetEnabled(),
				"channel":          src.GetChannel(),
				"platform":         src.GetPlatform(),
				"trust_policy":     orStr(src.GetTrustPolicy(), "import"),
				"require_checksum": src.GetRequireChecksum(),
				"last_synced_tag":  orStr(src.GetLastSyncedTag(), "-"),
				"last_sync_status": orStr(src.GetLastSyncStatus(), "-"),
				"credentials":      orStr(src.GetCredentialsRef(), "(not set)"),
			}
			if src.GetRepoUrl() != "" {
				entry["repo_url"] = src.GetRepoUrl()
			}
			if src.GetIndexUrl() != "" {
				entry["index_url"] = src.GetIndexUrl()
			}
			if src.GetLocalRoot() != "" {
				entry["local_root"] = src.GetLocalRoot()
			}
			if len(src.GetAllowedPublishers()) > 0 {
				entry["allowed_publishers"] = src.GetAllowedPublishers()
			}
			if len(src.GetAllowedKinds()) > 0 {
				entry["allowed_kinds"] = src.GetAllowedKinds()
			}
			if len(src.GetAllowedChannels()) > 0 {
				entry["allowed_channels"] = src.GetAllowedChannels()
			}
			if src.GetLastSyncUnix() > 0 {
				entry["last_sync_at"] = fmtTime(src.GetLastSyncUnix())
			}
			if src.GetLastSyncError() != "" {
				entry["last_sync_error"] = src.GetLastSyncError()
			}
			sources = append(sources, entry)
		}

		return map[string]interface{}{
			"count":   len(sources),
			"sources": sources,
		}, nil
	})

	// ── repository_upstream_sync ─────────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_upstream_sync",
		Description: "Sync packages from an upstream source. Fetches the release index for " +
			"the specified tag, validates each package against import policy, and imports " +
			"new/changed artifacts. Use dry_run=true to preview without importing. " +
			"Use resolve_latest=true to discover the latest release tag automatically " +
			"(requires repo_url on the source). Returns per-package results with action, " +
			"policy status, and version comparison.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"source_name": {
					Type:        "string",
					Description: "Name of a registered upstream source (required).",
				},
				"release_tag": {
					Type:        "string",
					Description: "Explicit release tag to sync (e.g. 'v1.0.84'). Required unless resolve_latest=true.",
				},
				"resolve_latest": {
					Type:        "boolean",
					Description: "Discover and sync the latest release from the source's repo_url. Mutually exclusive with release_tag.",
				},
				"dry_run": {
					Type:        "boolean",
					Description: "Preview only — no artifacts are written. Shows what would be imported/skipped/rejected.",
				},
				"only": {
					Type:        "string",
					Description: "Comma-separated list of package names to import (optional filter).",
				},
			},
			Required: []string{"source_name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		sourceName := getStr(args, "source_name")
		if sourceName == "" {
			return nil, fmt.Errorf("source_name is required")
		}

		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)

		// Longer timeout for sync — can be slow on first import.
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Minute)
		defer cancel()

		req := &repositorypb.SyncFromUpstreamRequest{
			SourceName:    sourceName,
			ReleaseTag:    getStr(args, "release_tag"),
			DryRun:        getBool(args, "dry_run", false),
			ResolveLatest: getBool(args, "resolve_latest", false),
		}
		if only := getStr(args, "only"); only != "" {
			for _, n := range strings.Split(only, ",") {
				n = strings.TrimSpace(n)
				if n != "" {
					req.Only = append(req.Only, n)
				}
			}
		}

		resp, err := client.SyncFromUpstream(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("SyncFromUpstream: %w", err)
		}

		results := make([]map[string]interface{}, 0, len(resp.GetResults()))
		for _, r := range resp.GetResults() {
			entry := map[string]interface{}{
				"name":    r.GetName(),
				"version": r.GetVersion(),
				"status":  r.GetStatus().String(),
				"action":  r.GetAction(),
				"detail":  r.GetDetail(),
			}
			if r.GetChannel() != "" {
				entry["channel"] = r.GetChannel()
			}
			if r.GetBuildNumber() > 0 {
				entry["build_number"] = r.GetBuildNumber()
			}
			if r.GetBlockedReason() != "" {
				entry["blocked_reason"] = r.GetBlockedReason()
			}
			if r.GetLocalVersion() != "" {
				entry["local_version"] = r.GetLocalVersion()
			}
			results = append(results, entry)
		}

		return map[string]interface{}{
			"source_name":  resp.GetSourceName(),
			"resolved_tag": resp.GetResolvedTag(),
			"dry_run":      resp.GetDryRun(),
			"imported":     resp.GetImported(),
			"skipped":      resp.GetSkipped(),
			"rejected":     resp.GetRejected(),
			"failed":       resp.GetFailed(),
			"results":      results,
		}, nil
	})

	// ── repository_upstream_register ─────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_upstream_register",
		Description: "Register or update an upstream release source. Supports all provider " +
			"types: GITHUB_RELEASE, HTTP_INDEX, LOCAL_DIR, GIT_INDEX. " +
			"Safe defaults: quarantine trust policy, require_checksum=true, " +
			"allowed_channels=stable for non-official sources.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name":                {Type: "string", Description: "Unique source name (required)."},
				"type":                {Type: "string", Description: "Provider type: GITHUB_RELEASE, HTTP_INDEX, LOCAL_DIR, GIT_INDEX.", Enum: []string{"GITHUB_RELEASE", "HTTP_INDEX", "LOCAL_DIR", "GIT_INDEX"}},
				"index_url":           {Type: "string", Description: "Index URL template with {tag} placeholder."},
				"repo_url":            {Type: "string", Description: "Git repo URL or GitHub owner/repo."},
				"owner":               {Type: "string", Description: "GitHub owner (GITHUB_RELEASE)."},
				"repo":                {Type: "string", Description: "GitHub repo name (GITHUB_RELEASE)."},
				"branch":              {Type: "string", Description: "Git branch (GIT_INDEX)."},
				"index_path_template": {Type: "string", Description: "Index path within repo/dir: releases/{tag}/release-index.json"},
				"artifact_base_url":   {Type: "string", Description: "Base URL for artifact downloads."},
				"local_root":          {Type: "string", Description: "Filesystem root for LOCAL_DIR sources."},
				"channel":             {Type: "string", Description: "Release channel (default: stable)."},
				"platform":            {Type: "string", Description: "Target platform (default: linux_amd64)."},
				"trust_policy":        {Type: "string", Description: "Trust policy: import or quarantine.", Enum: []string{"import", "quarantine"}},
				"require_checksum":    {Type: "boolean", Description: "Reject entries without sha256 digest."},
				"credentials_ref":     {Type: "string", Description: "etcd key under /globular/credentials/ for auth."},
				"allowed_publishers":  {Type: "string", Description: "Comma-separated allowed publishers."},
				"allowed_kinds":       {Type: "string", Description: "Comma-separated allowed kinds."},
				"allowed_channels":    {Type: "string", Description: "Comma-separated allowed channels."},
				"enabled":             {Type: "boolean", Description: "Enable the source (default: true)."},
			},
			Required: []string{"name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}

		// Map type string to enum.
		srcType := repositorypb.UpstreamSourceType_GITHUB_RELEASE
		if t := strings.ToUpper(getStr(args, "type")); t != "" {
			if v, ok := repositorypb.UpstreamSourceType_value[t]; ok {
				srcType = repositorypb.UpstreamSourceType(v)
			}
		}

		src := &repositorypb.UpstreamSource{
			Name:              name,
			Type:              srcType,
			IndexUrl:          getStr(args, "index_url"),
			Channel:           orStr(getStr(args, "channel"), "stable"),
			Platform:          orStr(getStr(args, "platform"), "linux_amd64"),
			Enabled:           getBool(args, "enabled", true),
			TrustPolicy:       orStr(getStr(args, "trust_policy"), "quarantine"),
			RequireChecksum:   getBool(args, "require_checksum", true),
			CredentialsRef:    getStr(args, "credentials_ref"),
			RepoUrl:           getStr(args, "repo_url"),
			Owner:             getStr(args, "owner"),
			Repo:              getStr(args, "repo"),
			Branch:            getStr(args, "branch"),
			IndexPathTemplate: getStr(args, "index_path_template"),
			ArtifactBaseUrl:   getStr(args, "artifact_base_url"),
			LocalRoot:         getStr(args, "local_root"),
		}

		// Parse list fields.
		if ap := getStr(args, "allowed_publishers"); ap != "" {
			for _, p := range strings.Split(ap, ",") {
				if p = strings.TrimSpace(p); p != "" {
					src.AllowedPublishers = append(src.AllowedPublishers, p)
				}
			}
		}
		if ak := getStr(args, "allowed_kinds"); ak != "" {
			for _, k := range strings.Split(ak, ",") {
				if k = strings.TrimSpace(k); k != "" {
					src.AllowedKinds = append(src.AllowedKinds, k)
				}
			}
		}
		if ac := getStr(args, "allowed_channels"); ac != "" {
			for _, c := range strings.Split(ac, ",") {
				if c = strings.TrimSpace(c); c != "" {
					src.AllowedChannels = append(src.AllowedChannels, c)
				}
			}
		}

		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.RegisterUpstream(callCtx, &repositorypb.RegisterUpstreamRequest{Source: src})
		if err != nil {
			return nil, fmt.Errorf("RegisterUpstream: %w", err)
		}

		return map[string]interface{}{
			"registered": true,
			"name":       resp.GetSource().GetName(),
			"type":       resp.GetSource().GetType().String(),
		}, nil
	})

	// ── repository_upstream_remove ───────────────────────────────────────────
	s.register(toolDef{
		Name:        "repository_upstream_remove",
		Description: "Remove a registered upstream source by name.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"name": {Type: "string", Description: "Upstream source name to remove (required)."},
			},
			Required: []string{"name"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		name := getStr(args, "name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}

		conn, err := s.clients.get(ctx, repositoryEndpoint())
		if err != nil {
			return nil, err
		}
		client := repositorypb.NewPackageRepositoryClient(conn)
		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		_, err = client.RemoveUpstream(callCtx, &repositorypb.RemoveUpstreamRequest{Name: name})
		if err != nil {
			return nil, fmt.Errorf("RemoveUpstream: %w", err)
		}

		return map[string]interface{}{
			"removed": true,
			"name":    name,
		}, nil
	})

	// ── repository_active_release ────────────────────────────────────────────
	s.register(toolDef{
		Name: "repository_active_release",
		Description: "Read the active platform release BOM from /var/lib/globular/release-index.json. " +
			"Shows the platform release tag, all package versions, change status, and origin releases. " +
			"This is the authoritative source of truth for what the cluster should be running. " +
			"If no BOM exists, returns a legacy-mode warning.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		const bomPath = "/var/lib/globular/release-index.json"

		// etcd is the authority for the active release; the file is a projection.
		anchorTag, anchorPlatform, anchorStatus := readActiveReleaseAnchorForTool(ctx)

		data, err := os.ReadFile(bomPath)
		if err != nil {
			out := map[string]interface{}{
				"bom_present": false,
				"warning":     "No active release BOM projection at " + bomPath + ".",
			}
			applyActiveReleaseAuthority(out, anchorTag, anchorPlatform, anchorStatus)
			return out, nil
		}

		var idx map[string]interface{}
		if jsonErr := json.Unmarshal(data, &idx); jsonErr != nil {
			return nil, fmt.Errorf("parse release-index.json: %w", jsonErr)
		}

		// Summarize packages.
		packages, _ := idx["packages"].([]interface{})
		var changed, unchanged int
		pkgSummary := make([]map[string]interface{}, 0, len(packages))
		for _, p := range packages {
			pkg, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			entry := map[string]interface{}{
				"name":    pkg["name"],
				"version": pkg["version"],
				"kind":    pkg["kind"],
			}
			if bn, ok := pkg["build_number"].(float64); ok && bn > 0 {
				entry["build_number"] = int64(bn)
			}
			if or, ok := pkg["origin_release"].(string); ok && or != "" {
				entry["origin_release"] = or
			}
			if ci, ok := pkg["changed_in_release"].(bool); ok {
				entry["changed_in_release"] = ci
				if ci {
					changed++
				} else {
					unchanged++
				}
			} else {
				changed++ // v1 default
			}
			pkgSummary = append(pkgSummary, entry)
		}

		result := map[string]interface{}{
			"bom_present":      true,
			"release_tag":      idx["release_tag"],
			"platform_release": idx["platform_release"],
			"publisher":        idx["publisher"],
			"total_packages":   len(pkgSummary),
			"changed":          changed,
			"unchanged":        unchanged,
			"packages":         pkgSummary,
		}
		if refs, ok := idx["referenced_releases"].([]interface{}); ok && len(refs) > 0 {
			result["referenced_releases"] = refs
		}
		applyActiveReleaseAuthority(result, anchorTag, anchorPlatform, anchorStatus)

		return result, nil
	})
}

// applyActiveReleaseAuthority overlays the etcd active-release authority onto a
// repository_active_release result. etcd is authoritative; the on-disk
// release-index.json is a projection. When etcd is unreachable the result is
// marked degraded rather than presented as authoritative truth.
func applyActiveReleaseAuthority(out map[string]interface{}, tag, platform, status string) {
	switch status {
	case "ok":
		if existing, ok := out["release_tag"].(string); ok && existing != "" && existing != tag {
			out["release_tag_file_projection"] = existing
		}
		out["active_release_authority"] = "etcd"
		out["release_tag"] = tag
		out["platform_release"] = platform
		out["release_index_file_role"] = "projection (etcd anchor is authoritative)"
	case "unset":
		out["active_release_authority"] = "none"
		out["note"] = "No etcd active_release anchor yet; the release-index.json value is the install-time projection, not an anchored active release."
	case "unavailable":
		out["active_release_authority"] = "unavailable"
		out["degraded"] = true
		out["warning"] = "active release authority unavailable (etcd unreachable); release-index.json shown as last-known local projection, NOT authoritative."
	}
}

func orStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// readActiveReleaseAnchorForTool reads the AUTHORITATIVE active-release pointer
// from etcd (/globular/platform/active_release). Per
// docs/design/platform-release-pointer-advance.md, etcd owns the active release
// pointer; /var/lib/globular/release-index.json is a node-local projection.
// Returns status "ok" with the tag/platform when the anchor is present, "unset"
// when the key is absent, or "unavailable" when etcd cannot be reached — in
// which case the on-disk file is the only signal and MUST be reported as
// degraded, never as authoritative.
func readActiveReleaseAnchorForTool(ctx context.Context) (tag, platform, status string) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return "", "", "unavailable"
	}
	defer cli.Close()
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := cli.Get(cctx, "/globular/platform/active_release")
	if err != nil {
		return "", "", "unavailable"
	}
	if len(resp.Kvs) == 0 {
		return "", "", "unset"
	}
	var a struct {
		ReleaseTag      string `json:"release_tag"`
		PlatformRelease string `json:"platform_release"`
	}
	if json.Unmarshal(resp.Kvs[0].Value, &a) != nil {
		return "", "", "unavailable"
	}
	return a.ReleaseTag, a.PlatformRelease, "ok"
}
