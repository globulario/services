package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

func registerPackageTools(s *server) {

	// ── package_build ───────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "package_build",
		Description: `Build a Globular package (.tgz) from a spec YAML and payload root.

Wraps 'globular pkg build'. Returns the built artifact path and manifest details.

NOTE: This runs as the MCP server user (globular). Ensure spec, root, and out paths are readable/writable by that user. For files under /home/dave/, use globular_cli.execute with force=true instead.

Common spec locations:
- /var/lib/globular/specs/ or /usr/lib/globular/specs/
- Or copy specs to a globular-accessible location first.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"spec":         {Type: "string", Description: "Path to the spec YAML file"},
				"root":         {Type: "string", Description: "Payload root directory containing bin/"},
				"out":          {Type: "string", Description: "Output directory for the .tgz (default: /var/lib/globular/packages/out/)"},
				"version":      {Type: "string", Description: "Package version (default: 0.0.1)"},
				"build_number": {Type: "number", Description: "Build iteration within version (default: 0). Bump when republishing same version."},
				"publisher":    {Type: "string", Description: "Publisher identifier (default: core@globular.io)"},
				"platform":     {Type: "string", Description: "Target platform (default: linux_amd64)"},
			},
			Required: []string{"spec", "root"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		spec := getStr(args, "spec")
		root := getStr(args, "root")
		if spec == "" || root == "" {
			return nil, fmt.Errorf("spec and root are required")
		}

		out := getStr(args, "out")
		if out == "" {
			out = "/var/lib/globular/packages/out/"
		}
		version := getStr(args, "version")
		if version == "" {
			version = "0.0.1"
		}
		publisher := getStr(args, "publisher")
		if publisher == "" {
			publisher = "core@globular.io"
		}
		platform := getStr(args, "platform")
		if platform == "" {
			platform = "linux_amd64"
		}
		buildNumber := getInt(args, "build_number", 0)

		cmdArgs := []string{
			"pkg", "build",
			"--spec", spec,
			"--root", root,
			"--out", out,
			"--version", version,
			"--build-number", fmt.Sprintf("%d", buildNumber),
			"--publisher", publisher,
			"--platform", platform,
		}

		return runGlobularCommand(ctx, cmdArgs)
	})

	// ── package_publish ─────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "package_publish",
		Description: `Publish a package (.tgz) to the Globular repository.

Wraps 'globular pkg publish'. Validates the package, uploads the bundle, and registers the descriptor.

If you get AlreadyExists error, rebuild with a higher build_number using package_build first.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"file":       {Type: "string", Description: "Path to the .tgz package file"},
				"dir":        {Type: "string", Description: "Directory containing .tgz files to publish (alternative to file)"},
				"repository": {Type: "string", Description: "Repository address (default: localhost:443)"},
				"force":      {Type: "boolean", Description: "Overwrite existing artifact even if checksum differs (default: false)"},
				"dry_run":    {Type: "boolean", Description: "Validate without uploading (default: false)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		file := getStr(args, "file")
		dir := getStr(args, "dir")
		if file == "" && dir == "" {
			return nil, fmt.Errorf("either file or dir is required")
		}

		repository := getStr(args, "repository")
		if repository == "" {
			if mesh, err := config.GetMeshAddress(); err == nil {
				repository = mesh
			}
		}

		cmdArgs := []string{"pkg", "publish", "--repository", repository}

		if file != "" {
			cmdArgs = append(cmdArgs, "--file", file)
		} else {
			cmdArgs = append(cmdArgs, "--dir", dir)
		}

		if getBool(args, "force", false) {
			cmdArgs = append(cmdArgs, "--force")
		}
		if getBool(args, "dry_run", false) {
			cmdArgs = append(cmdArgs, "--dry-run")
		}

		return runGlobularCommand(ctx, cmdArgs)
	})

	// ── Phase F lifecycle tools ────────────────────────────────────────────
	registerPackageLifecycleTools(s)
}

// registerPackageLifecycleTools — Phase F additions for the AI/MCP surface:
//   package_config_conflicts, package_config_list.
// Both call into the repository service (single source of truth) — no CLI
// subprocess.
func registerPackageLifecycleTools(s *server) {
	s.register(toolDef{
		Name: "package_config_conflicts",
		Description: "List unresolved config-file CONFLICT receipts (FAIL_ON_LOCAL_MODIFICATION " +
			"hits, three-way merge gaps, and similar). Each receipt names the node, the path " +
			"(redacted for SECRET configs), and the merge strategy that fired. Surface these " +
			"BEFORE recommending an upgrade or rollback — they will block the apply.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string"},
				"name":         {Type: "string"},
				"platform":     {Type: "string", Description: "Default linux_amd64."},
				"node_id":      {Type: "string", Description: "Optional: limit to one node."},
				"limit":        {Type: "integer", Description: "Max receipts (default 100)."},
			},
			Required: []string{"publisher_id", "name"},
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
		resp, err := client.ListConfigReceipts(callCtx, &repositorypb.ListConfigReceiptsRequest{
			PublisherId:  getStr(args, "publisher_id"),
			Name:         getStr(args, "name"),
			Platform:     platform,
			NodeId:       getStr(args, "node_id"),
			ActionFilter: repositorypb.ConfigReceiptAction_CONFIG_RECEIPT_CONFLICT,
			Limit:        int32(getInt(args, "limit", 100)),
		})
		if err != nil {
			return nil, fmt.Errorf("ListConfigReceipts: %w", err)
		}
		rows := make([]map[string]interface{}, 0, len(resp.GetReceipts()))
		for _, r := range resp.GetReceipts() {
			rows = append(rows, configReceiptToMap(r))
		}
		return map[string]interface{}{
			"count":    len(rows),
			"receipts": rows,
		}, nil
	})

	s.register(toolDef{
		Name: "package_config_list",
		Description: "List ALL config receipts for a package (every action, not just CONFLICT). " +
			"Useful for audit trails: 'what did the last install / upgrade / rollback do to " +
			"each config file?'.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"publisher_id": {Type: "string"},
				"name":         {Type: "string"},
				"platform":     {Type: "string"},
				"node_id":      {Type: "string"},
				"limit":        {Type: "integer", Description: "Max receipts (default 100)."},
			},
			Required: []string{"publisher_id", "name"},
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
		resp, err := client.ListConfigReceipts(callCtx, &repositorypb.ListConfigReceiptsRequest{
			PublisherId: getStr(args, "publisher_id"),
			Name:        getStr(args, "name"),
			Platform:    platform,
			NodeId:      getStr(args, "node_id"),
			Limit:       int32(getInt(args, "limit", 100)),
		})
		if err != nil {
			return nil, fmt.Errorf("ListConfigReceipts: %w", err)
		}
		rows := make([]map[string]interface{}, 0, len(resp.GetReceipts()))
		for _, r := range resp.GetReceipts() {
			rows = append(rows, configReceiptToMap(r))
		}
		return map[string]interface{}{
			"count":    len(rows),
			"receipts": rows,
		}, nil
	})
}

// configReceiptToMap normalizes a PackageConfigReceipt for MCP / JSON output.
// Path is already redacted by the repository for SECRET / sensitive entries.
func configReceiptToMap(r *repositorypb.PackageConfigReceipt) map[string]interface{} {
	if r == nil {
		return nil
	}
	return map[string]interface{}{
		"node_id":         r.GetNodeId(),
		"publisher_id":    r.GetPublisherId(),
		"name":            r.GetName(),
		"platform":        r.GetPlatform(),
		"build_number":    r.GetBuildNumber(),
		"path":            r.GetPath(),
		"config_kind":     strings.TrimPrefix(r.GetConfigKind().String(), "CONFIG_"),
		"merge_strategy":  strings.TrimPrefix(r.GetMergeStrategy().String(), "MERGE_"),
		"checksum_before": r.GetChecksumBefore(),
		"checksum_after":  r.GetChecksumAfter(),
		"action":          strings.TrimPrefix(r.GetAction().String(), "CONFIG_RECEIPT_"),
		"snapshot_id":     r.GetSnapshotId(),
		"workflow_run_id": r.GetWorkflowRunId(),
		"timestamp_unix":  r.GetTimestampUnix(),
		"reason":          r.GetReason(),
		"sensitive":       r.GetSensitive(),
	}
}

// runGlobularCommand executes 'globular <args>' and returns structured output.
func runGlobularCommand(ctx context.Context, cmdArgs []string) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "globular", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := map[string]interface{}{
		"command":     "globular " + strings.Join(cmdArgs, " "),
		"duration_ms": duration.Milliseconds(),
	}

	if err != nil {
		result["success"] = false
		// Extract meaningful error lines
		var errLines []string
		for _, line := range strings.Split(stderr.String(), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Skip slog info/warn lines, keep errors
			if strings.Contains(line, "Error:") || strings.Contains(line, "[FAIL]") ||
				strings.Contains(line, "FAILED") || strings.Contains(line, "AlreadyExists") ||
				strings.Contains(line, "permission denied") {
				errLines = append(errLines, line)
			}
		}
		if len(errLines) > 0 {
			result["error"] = strings.Join(errLines, "\n")
		} else {
			result["error"] = strings.TrimSpace(stderr.String())
		}
		return result, nil
	}

	result["success"] = true

	// Parse structured output
	outStr := stdout.String()
	for _, line := range strings.Split(outStr, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "─") {
			continue
		}
		// Build output: [OK] name -> path
		if strings.Contains(line, "[OK]") && strings.Contains(line, "->") {
			parts := strings.SplitN(line, "->", 2)
			if len(parts) == 2 {
				result["artifact"] = strings.TrimSpace(parts[1])
			}
		}
		// Build output: manifest: ...
		if strings.HasPrefix(line, "manifest:") {
			result["manifest"] = strings.TrimSpace(strings.TrimPrefix(line, "manifest:"))
		}
		// Publish output: Key : Value
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.ToLower(strings.TrimSpace(parts[0]))
			val := strings.TrimSpace(parts[1])
			switch key {
			case "status":
				result["status"] = val
			case "name":
				result["name"] = val
			case "version":
				result["version"] = val
			case "build":
				result["build"] = val
			case "digest":
				result["digest"] = val
			case "size":
				result["size"] = val
			case "bundleid":
				result["bundle_id"] = val
			}
		}
	}

	return result, nil
}
