// @awareness namespace=globular.platform
// @awareness component=platform_mcp.tools_cluster_config
// @awareness file_role=mcp_bridge_for_cluster_config_minio_bucket_get_put_with_no_local_disk_fallback
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness risk=critical
package main

// tools_cluster_config.go — get/put cluster configuration via
// the MinIO cluster config bucket (cluster-wide shared
// configuration). cluster_config_put is the most
// cluster-affecting tool in this group: an arbitrary file
// pushed here changes runtime behavior across nodes.
//
// MUST route through the existing config service / MinIO API —
// NO local-disk fallback to `/var/lib/globular/config/` (or any
// other host path) on the MCP node. A local-disk write here
// would shadow the cluster-shared truth and produce silent
// drift between nodes.

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/config"
)

func registerClusterConfigTools(s *server) {

	// ── cluster_config_put ────────────────────────────────────────────
	s.register(toolDef{
		Name: "cluster_config_put",
		Description: "Upload a configuration file to the shared MinIO cluster config bucket. " +
			"All nodes can read it. Use for AI rules (ai/CLAUDE.md), PKI certs, RBAC policies, etc. " +
			"Requires read_only=false.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"key":     {Type: "string", Description: "Object key path (e.g. 'ai/CLAUDE.md', 'pki/ca.pem')"},
				"content": {Type: "string", Description: "File content to upload"},
			},
			Required: []string{"key", "content"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if s.cfg.ReadOnly {
			return nil, fmt.Errorf("read-only mode — set read_only=false to write cluster config")
		}
		key := strArg(args, "key")
		content := strArg(args, "content")
		if key == "" || content == "" {
			return nil, fmt.Errorf("key and content are required")
		}

		if err := config.EnsureClusterConfigBucket(); err != nil {
			return nil, fmt.Errorf("ensure bucket: %w", err)
		}

		if err := config.PutClusterConfig(key, []byte(content)); err != nil {
			return nil, fmt.Errorf("put %s: %w", key, err)
		}

		return map[string]interface{}{
			"bucket": config.ClusterConfigBucket,
			"key":    key,
			"size":   len(content),
			"status": "uploaded",
		}, nil
	})

	// ── cluster_config_get ────────────────────────────────────────────
	s.register(toolDef{
		Name: "cluster_config_get",
		Description: "Download a configuration file from the shared MinIO cluster config bucket. " +
			"Well-known keys: ai/CLAUDE.md, pki/ca.pem, policy/rbac/cluster-roles.json",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"key": {Type: "string", Description: "Object key path (e.g. 'ai/CLAUDE.md')"},
			},
			Required: []string{"key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		key := strArg(args, "key")
		if key == "" {
			return nil, fmt.Errorf("key is required")
		}

		data, err := config.GetClusterConfig(key)
		if err != nil {
			return nil, fmt.Errorf("get %s: %w", key, err)
		}
		if data == nil {
			return map[string]interface{}{
				"key":    key,
				"exists": false,
			}, nil
		}

		return map[string]interface{}{
			"key":     key,
			"exists":  true,
			"size":    len(data),
			"content": string(data),
		}, nil
	})
}
