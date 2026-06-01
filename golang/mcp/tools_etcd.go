package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func registerEtcdTools(s *server) {

	// ── etcd_get ────────────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "etcd_get",
		Description: `Read one or more keys from etcd. Use prefix=true to list all keys under a path.

Returns key-value pairs. With keys_only=true, returns just the keys (faster for browsing).
Common prefixes: /globular/nodes/, /globular/resources/, /globular/system/`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"key":       {Type: "string", Description: "The etcd key or prefix to read (e.g. '/globular/nodes/')"},
				"prefix":    {Type: "boolean", Description: "If true, get all keys with this prefix (default: false)"},
				"keys_only": {Type: "boolean", Description: "If true, return only key names without values (default: false)"},
				"limit":     {Type: "number", Description: "Max number of keys to return (default: 100)"},
			},
			Required: []string{"key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		key := getStr(args, "key")
		if key == "" {
			return nil, fmt.Errorf("key is required")
		}

		prefix := getBool(args, "prefix", false)
		keysOnly := getBool(args, "keys_only", false)
		limit := getInt(args, "limit", 100)

		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		var opts []clientv3.OpOption
		if prefix {
			opts = append(opts, clientv3.WithPrefix())
		}
		if keysOnly {
			opts = append(opts, clientv3.WithKeysOnly())
		}
		if limit > 0 {
			opts = append(opts, clientv3.WithLimit(int64(limit)))
		}

		resp, err := cli.Get(callCtx, key, opts...)
		if err != nil {
			return nil, fmt.Errorf("etcd get %q: %w", key, err)
		}

		if !prefix && len(resp.Kvs) == 0 {
			return map[string]interface{}{
				"count": 0,
				"error": fmt.Sprintf("key %q not found", key),
			}, nil
		}

		kvs := make([]map[string]interface{}, 0, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			entry := map[string]interface{}{
				"key": string(kv.Key),
			}
			if !keysOnly {
				val := string(kv.Value)
				// Truncate very large values
				if len(val) > 8192 {
					val = val[:8192] + "... (truncated)"
				}
				entry["value"] = val
			}
			entry["version"] = kv.Version
			entry["mod_revision"] = kv.ModRevision
			kvs = append(kvs, entry)
		}

		result := map[string]interface{}{
			"count": len(kvs),
			"kvs":   kvs,
		}
		if resp.More {
			result["has_more"] = true
		}
		return result, nil
	})

	// ── etcd_put ────────────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "etcd_put",
		Description: `Write a key-value pair to etcd. Use this to fix state, seed configuration, or repair missing entries.

Returns the previous value if the key existed. Requires read_only=false in MCP config.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"key":   {Type: "string", Description: "The etcd key to write (e.g. '/globular/nodes/.../packages/INFRASTRUCTURE/mcp')"},
				"value": {Type: "string", Description: "The value to write (typically JSON)"},
			},
			Required: []string{"key", "value"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if s.cfg.ReadOnly {
			return map[string]interface{}{
				"ok":    false,
				"error": "etcd_put blocked: MCP server is in read-only mode",
			}, nil
		}

		key := getStr(args, "key")
		value := getStr(args, "value")
		if key == "" || value == "" {
			return nil, fmt.Errorf("key and value are required")
		}

		// Safety: block writes outside /globular/ namespace
		if !strings.HasPrefix(key, "/globular/") {
			return nil, fmt.Errorf("etcd_put only allows writes under /globular/ prefix")
		}

		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Get previous value for the response
		resp, err := cli.Put(callCtx, key, value, clientv3.WithPrevKV())
		if err != nil {
			return nil, fmt.Errorf("etcd put %q: %w", key, err)
		}

		result := map[string]interface{}{
			"ok":  true,
			"key": key,
		}
		if resp.PrevKv != nil {
			result["previous_value"] = string(resp.PrevKv.Value)
			result["was_update"] = true
		} else {
			result["was_update"] = false
		}
		return result, nil
	})

	// ── etcd_delete ─────────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "etcd_delete",
		Description: `Delete one or more keys from etcd. Use prefix=true to delete all keys under a path.

Returns the number of keys deleted. Requires read_only=false in MCP config.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"key":    {Type: "string", Description: "The etcd key or prefix to delete"},
				"prefix": {Type: "boolean", Description: "If true, delete all keys with this prefix (default: false)"},
			},
			Required: []string{"key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		if s.cfg.ReadOnly {
			return map[string]interface{}{
				"ok":    false,
				"error": "etcd_delete blocked: MCP server is in read-only mode",
			}, nil
		}

		key := getStr(args, "key")
		if key == "" {
			return nil, fmt.Errorf("key is required")
		}

		// Safety: block deletes outside /globular/ namespace
		if !strings.HasPrefix(key, "/globular/") {
			return nil, fmt.Errorf("etcd_delete only allows deletes under /globular/ prefix")
		}

		prefix := getBool(args, "prefix", false)

		cli, err := config.GetEtcdClient()
		if err != nil {
			return nil, fmt.Errorf("etcd client: %w", err)
		}

		callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		var opts []clientv3.OpOption
		if prefix {
			opts = append(opts, clientv3.WithPrefix())
		}

		resp, err := cli.Delete(callCtx, key, opts...)
		if err != nil {
			return nil, fmt.Errorf("etcd delete %q: %w", key, err)
		}

		return map[string]interface{}{
			"ok":      true,
			"deleted": resp.Deleted,
			"key":     key,
			"prefix":  prefix,
		}, nil
	})
}
