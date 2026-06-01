package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/globulario/services/golang/config"
	persistencepb "github.com/globulario/services/golang/persistence/persistencepb"
	storagepb "github.com/globulario/services/golang/storage/storagepb"
)

// ── Endpoint helpers ─────────────────────────────────────────────────────────

func persistenceEndpoint() string {
	return config.ResolveServiceAddr("persistence.PersistenceService", "")
}

func storageEndpoint() string {
	return config.ResolveServiceAddr("storage.StorageService", "")
}

// ── Security: sensitive field redaction ──────────────────────────────────────

var sensitiveFields = map[string]bool{
	"password": true, "Password": true,
	"refreshToken": true, "RefreshToken": true, "refresh_token": true,
	"secret": true, "Secret": true, "secret_key": true,
	"token": true, "Token": true, "access_token": true,
	"private_key": true, "privateKey": true,
	"session_token": true, "sessionToken": true,
	"auth_token": true, "authToken": true,
	"api_key": true, "apiKey": true,
}

// buildRedactSet returns a merged set of sensitive fields from both the
// built-in defaults and any extra fields configured in the MCP config.
func (cfg *MCPConfig) buildRedactSet() map[string]bool {
	set := make(map[string]bool, len(sensitiveFields)+len(cfg.RedactFields))
	for k, v := range sensitiveFields {
		set[k] = v
	}
	for _, f := range cfg.RedactFields {
		set[f] = true
	}
	return set
}

func redactSensitive(data map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(data))
	for k, v := range data {
		if sensitiveFields[k] {
			out[k] = "***REDACTED***"
		} else if nested, ok := v.(map[string]interface{}); ok {
			out[k] = redactSensitive(nested)
		} else {
			out[k] = v
		}
	}
	return out
}

// ── Client helpers ───────────────────────────────────────────────────────────

func persistenceClient(ctx context.Context, pool *clientPool) (persistencepb.PersistenceServiceClient, error) {
	conn, err := pool.get(ctx, persistenceEndpoint())
	if err != nil {
		return nil, err
	}
	return persistencepb.NewPersistenceServiceClient(conn), nil
}

func storageClient(ctx context.Context, pool *clientPool) (storagepb.StorageServiceClient, error) {
	conn, err := pool.get(ctx, storageEndpoint())
	if err != nil {
		return nil, err
	}
	return storagepb.NewStorageServiceClient(conn), nil
}

// ── Tool registration ────────────────────────────────────────────────────────

func registerPersistenceTools(s *server) {

	// ── 1. db_ping_connection ────────────────────────────────────────────
	s.register(toolDef{
		Name:        "db_ping_connection",
		Description: "Ping a persistence database connection to check reachability.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {
					Type:        "string",
					Description: "The persistence connection ID to ping.",
				},
			},
			Required: []string{"connection_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		if connID == "" {
			return nil, fmt.Errorf("missing required argument: connection_id")
		}
		if err := s.cfg.validatePersistenceAccess(connID, "", ""); err != nil {
			return nil, err
		}

		client, err := persistenceClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		_, err = client.Ping(callCtx, &persistencepb.PingConnectionRqst{Id: connID})
		if err != nil {
			return map[string]interface{}{
				"connection_id": connID,
				"reachable":     false,
				"message":       err.Error(),
				"summary":       fmt.Sprintf("Connection '%s' is not reachable: %v", connID, err),
			}, nil
		}

		return map[string]interface{}{
			"connection_id": connID,
			"reachable":     true,
			"message":       "pong",
			"summary":       fmt.Sprintf("Connection '%s' is reachable", connID),
		}, nil
	})

	// ── 2. db_find_one ───────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "db_find_one",
		Description: "Find a single document matching a query in a database collection. Sensitive fields are redacted.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {
					Type:        "string",
					Description: "The persistence connection ID.",
				},
				"database": {
					Type:        "string",
					Description: "Database name.",
				},
				"collection": {
					Type:        "string",
					Description: "Collection or table name.",
				},
				"query": {
					Type:        "string",
					Description: "JSON query string (e.g. {\"_id\": \"abc\"}).",
				},
				"options": {
					Type:        "string",
					Description: "Optional JSON options string.",
				},
			},
			Required: []string{"connection_id", "database", "collection", "query"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		database := getStr(args, "database")
		collection := getStr(args, "collection")
		query := getStr(args, "query")
		options := getStr(args, "options")

		if connID == "" {
			return nil, fmt.Errorf("missing required argument: connection_id")
		}
		if database == "" {
			return nil, fmt.Errorf("missing required argument: database")
		}
		if collection == "" {
			return nil, fmt.Errorf("missing required argument: collection")
		}
		if query == "" {
			return nil, fmt.Errorf("missing required argument: query")
		}
		if err := s.cfg.validatePersistenceAccess(connID, database, collection); err != nil {
			return nil, err
		}

		client, err := persistenceClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.FindOne(callCtx, &persistencepb.FindOneRqst{
			Id:         connID,
			Database:   database,
			Collection: collection,
			Query:      query,
			Options:    options,
		})
		if err != nil {
			return map[string]interface{}{
				"connection_id": connID,
				"database":      database,
				"collection":    collection,
				"found":         false,
				"record":        nil,
			}, nil
		}

		result := resp.GetResult()
		if result == nil {
			return map[string]interface{}{
				"connection_id": connID,
				"database":      database,
				"collection":    collection,
				"found":         false,
				"record":        nil,
			}, nil
		}

		record := redactSensitive(result.AsMap())
		return map[string]interface{}{
			"connection_id": connID,
			"database":      database,
			"collection":    collection,
			"found":         true,
			"record":        record,
		}, nil
	})

	// ── 3. db_find_many ──────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "db_find_many",
		Description: "Find multiple documents matching a query in a database collection. Sensitive fields are redacted.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {
					Type:        "string",
					Description: "The persistence connection ID.",
				},
				"database": {
					Type:        "string",
					Description: "Database name.",
				},
				"collection": {
					Type:        "string",
					Description: "Collection or table name.",
				},
				"query": {
					Type:        "string",
					Description: "JSON query string.",
				},
				"options": {
					Type:        "string",
					Description: "Optional JSON options string.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of records to return (default 20, max 100).",
				},
			},
			Required: []string{"connection_id", "database", "collection", "query"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		database := getStr(args, "database")
		collection := getStr(args, "collection")
		query := getStr(args, "query")
		options := getStr(args, "options")
		limit := getInt(args, "limit", 20)

		if connID == "" {
			return nil, fmt.Errorf("missing required argument: connection_id")
		}
		if database == "" {
			return nil, fmt.Errorf("missing required argument: database")
		}
		if collection == "" {
			return nil, fmt.Errorf("missing required argument: collection")
		}
		if query == "" {
			return nil, fmt.Errorf("missing required argument: query")
		}
		if limit > 100 {
			limit = 100
		}
		if err := s.cfg.validatePersistenceAccess(connID, database, collection); err != nil {
			return nil, err
		}

		client, err := persistenceClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		stream, err := client.Find(callCtx, &persistencepb.FindRqst{
			Id:         connID,
			Database:   database,
			Collection: collection,
			Query:      query,
			Options:    options,
		})
		if err != nil {
			return nil, fmt.Errorf("Find: %w", err)
		}

		var records []map[string]interface{}
		truncated := false
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("Find stream: %w", err)
			}
			var doc map[string]interface{}
			if json.Unmarshal(resp.GetData(), &doc) == nil {
				records = append(records, redactSensitive(doc))
			}
			if len(records) >= limit {
				truncated = true
				break
			}
		}

		return map[string]interface{}{
			"connection_id": connID,
			"database":      database,
			"collection":    collection,
			"count":         len(records),
			"records":       records,
			"truncated":     truncated,
		}, nil
	})

	// ── 4. db_count ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "db_count",
		Description: "Count documents in a database collection matching an optional query.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {
					Type:        "string",
					Description: "The persistence connection ID.",
				},
				"database": {
					Type:        "string",
					Description: "Database name.",
				},
				"collection": {
					Type:        "string",
					Description: "Collection or table name.",
				},
				"query": {
					Type:        "string",
					Description: "JSON query string (default \"{}\").",
				},
				"options": {
					Type:        "string",
					Description: "Optional JSON options string.",
				},
			},
			Required: []string{"connection_id", "database", "collection"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		database := getStr(args, "database")
		collection := getStr(args, "collection")
		query := getStr(args, "query")
		options := getStr(args, "options")

		if connID == "" {
			return nil, fmt.Errorf("missing required argument: connection_id")
		}
		if database == "" {
			return nil, fmt.Errorf("missing required argument: database")
		}
		if collection == "" {
			return nil, fmt.Errorf("missing required argument: collection")
		}
		if query == "" {
			query = "{}"
		}
		if err := s.cfg.validatePersistenceAccess(connID, database, collection); err != nil {
			return nil, err
		}

		client, err := persistenceClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.Count(callCtx, &persistencepb.CountRqst{
			Id:         connID,
			Database:   database,
			Collection: collection,
			Query:      query,
			Options:    options,
		})
		if err != nil {
			return nil, fmt.Errorf("Count: %w", err)
		}

		count := resp.GetResult()
		return map[string]interface{}{
			"connection_id": connID,
			"database":      database,
			"collection":    collection,
			"count":         count,
			"summary":       fmt.Sprintf("Collection '%s.%s' has %d documents matching the query", database, collection, count),
		}, nil
	})

	// ── 5. db_aggregate_safe ─────────────────────────────────────────────
	s.register(toolDef{
		Name:        "db_aggregate_safe",
		Description: "Run a read-only aggregation pipeline on a database collection. Sensitive fields are redacted.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {
					Type:        "string",
					Description: "The persistence connection ID.",
				},
				"database": {
					Type:        "string",
					Description: "Database name.",
				},
				"collection": {
					Type:        "string",
					Description: "Collection or table name.",
				},
				"pipeline": {
					Type:        "string",
					Description: "JSON array string representing the aggregation pipeline.",
				},
				"options": {
					Type:        "string",
					Description: "Optional JSON options string.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of results to return (default 20, max 100).",
				},
			},
			Required: []string{"connection_id", "database", "collection", "pipeline"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		database := getStr(args, "database")
		collection := getStr(args, "collection")
		pipeline := getStr(args, "pipeline")
		options := getStr(args, "options")
		limit := getInt(args, "limit", 20)

		if connID == "" {
			return nil, fmt.Errorf("missing required argument: connection_id")
		}
		if database == "" {
			return nil, fmt.Errorf("missing required argument: database")
		}
		if collection == "" {
			return nil, fmt.Errorf("missing required argument: collection")
		}
		if pipeline == "" {
			return nil, fmt.Errorf("missing required argument: pipeline")
		}
		if limit > 100 {
			limit = 100
		}
		if err := s.cfg.validatePersistenceAccess(connID, database, collection); err != nil {
			return nil, err
		}

		client, err := persistenceClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		stream, err := client.Aggregate(callCtx, &persistencepb.AggregateRqst{
			Id:         connID,
			Database:   database,
			Collection: collection,
			Pipeline:   pipeline,
			Options:    options,
		})
		if err != nil {
			return nil, fmt.Errorf("Aggregate: %w", err)
		}

		var results []map[string]interface{}
		truncated := false
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("Aggregate stream: %w", err)
			}
			var doc map[string]interface{}
			if json.Unmarshal(resp.GetData(), &doc) == nil {
				results = append(results, redactSensitive(doc))
			}
			if len(results) >= limit {
				truncated = true
				break
			}
		}

		return map[string]interface{}{
			"connection_id": connID,
			"database":      database,
			"collection":    collection,
			"count":         len(results),
			"results":       results,
			"truncated":     truncated,
		}, nil
	})

}

// registerStorageTools registers key-value storage tools (kv_* prefix).
func registerStorageTools(s *server) {

	// ── 6. kv_get_item_text ──────────────────────────────────────────────
	s.register(toolDef{
		Name:        "kv_get_item_text",
		Description: "Get a key-value store item as UTF-8 text. Returns an error if the value is binary.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"store_id": {
					Type:        "string",
					Description: "The storage connection/store ID.",
				},
				"key": {
					Type:        "string",
					Description: "The key to retrieve.",
				},
				"max_bytes": {
					Type:        "number",
					Description: "Maximum bytes to read (default 32768).",
				},
			},
			Required: []string{"store_id", "key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		storeID := getStr(args, "store_id")
		key := getStr(args, "key")
		maxBytes := getInt(args, "max_bytes", 32768)

		if storeID == "" {
			return nil, fmt.Errorf("missing required argument: store_id")
		}
		if key == "" {
			return nil, fmt.Errorf("missing required argument: key")
		}
		if err := s.cfg.validateStorageAccess(storeID, key); err != nil {
			return nil, err
		}

		client, err := storageClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		stream, err := client.GetItem(callCtx, &storagepb.GetItemRequest{Id: storeID, Key: key})
		if err != nil {
			return nil, fmt.Errorf("GetItem: %w", err)
		}

		var buf bytes.Buffer
		truncated := false
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("GetItem stream: %w", err)
			}
			buf.Write(resp.GetResult())
			if maxBytes > 0 && buf.Len() > maxBytes {
				truncated = true
				break
			}
		}

		content := buf.Bytes()
		if maxBytes > 0 && len(content) > maxBytes {
			content = content[:maxBytes]
			truncated = true
		}

		if !utf8.Valid(content) {
			return nil, fmt.Errorf("value is binary, not valid UTF-8 text")
		}

		return map[string]interface{}{
			"store_id":  storeID,
			"key":       key,
			"size":      len(content),
			"content":   string(content),
			"truncated": truncated,
		}, nil
	})

	// ── 7. kv_get_item_json ──────────────────────────────────────────────
	s.register(toolDef{
		Name:        "kv_get_item_json",
		Description: "Get a key-value store item parsed as JSON. Sensitive fields are redacted.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"store_id": {
					Type:        "string",
					Description: "The storage connection/store ID.",
				},
				"key": {
					Type:        "string",
					Description: "The key to retrieve.",
				},
			},
			Required: []string{"store_id", "key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		storeID := getStr(args, "store_id")
		key := getStr(args, "key")

		if storeID == "" {
			return nil, fmt.Errorf("missing required argument: store_id")
		}
		if key == "" {
			return nil, fmt.Errorf("missing required argument: key")
		}
		if err := s.cfg.validateStorageAccess(storeID, key); err != nil {
			return nil, err
		}

		client, err := storageClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		stream, err := client.GetItem(callCtx, &storagepb.GetItemRequest{Id: storeID, Key: key})
		if err != nil {
			return nil, fmt.Errorf("GetItem: %w", err)
		}

		var buf bytes.Buffer
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("GetItem stream: %w", err)
			}
			buf.Write(resp.GetResult())
		}

		data := buf.Bytes()
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return map[string]interface{}{
				"store_id": storeID,
				"key":      key,
				"size":     len(data),
				"error":    "value is not valid JSON",
			}, nil
		}

		return map[string]interface{}{
			"store_id": storeID,
			"key":      key,
			"size":     len(data),
			"value":    redactSensitive(parsed),
		}, nil
	})

	// ── 8. kv_list_keys ──────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "kv_list_keys",
		Description: "List all keys in a key-value store, optionally filtered by prefix.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"store_id": {
					Type:        "string",
					Description: "The storage connection/store ID.",
				},
				"prefix": {
					Type:        "string",
					Description: "Optional prefix to filter keys.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of keys to return (default 100).",
				},
			},
			Required: []string{"store_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		storeID := getStr(args, "store_id")
		prefix := getStr(args, "prefix")
		limit := getInt(args, "limit", 100)

		if storeID == "" {
			return nil, fmt.Errorf("missing required argument: store_id")
		}
		if err := s.cfg.validateStorageAccess(storeID, prefix); err != nil {
			return nil, err
		}

		client, err := storageClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		stream, err := client.GetAllKeys(callCtx, &storagepb.GetAllKeysRequest{Id: storeID})
		if err != nil {
			return nil, fmt.Errorf("GetAllKeys: %w", err)
		}

		var keys []string
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("GetAllKeys stream: %w", err)
			}
			for _, k := range resp.GetKeys() {
				if prefix == "" || strings.HasPrefix(k, prefix) {
					keys = append(keys, k)
				}
			}
		}

		truncated := false
		if limit > 0 && len(keys) > limit {
			keys = keys[:limit]
			truncated = true
		}

		return map[string]interface{}{
			"store_id":  storeID,
			"count":     len(keys),
			"keys":      keys,
			"truncated": truncated,
		}, nil
	})

	// ── 9. kv_get_prefix_snapshot ────────────────────────────────────────
	s.register(toolDef{
		Name:        "kv_get_prefix_snapshot",
		Description: "Get all keys matching a prefix with their values parsed as JSON. Sensitive fields are redacted.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"store_id": {
					Type:        "string",
					Description: "The storage connection/store ID.",
				},
				"prefix": {
					Type:        "string",
					Description: "Key prefix to match.",
				},
				"limit": {
					Type:        "number",
					Description: "Maximum number of entries to return (default 20).",
				},
			},
			Required: []string{"store_id", "prefix"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		storeID := getStr(args, "store_id")
		prefix := getStr(args, "prefix")
		limit := getInt(args, "limit", 20)

		if storeID == "" {
			return nil, fmt.Errorf("missing required argument: store_id")
		}
		if prefix == "" {
			return nil, fmt.Errorf("missing required argument: prefix")
		}
		if err := s.cfg.validateStorageAccess(storeID, prefix); err != nil {
			return nil, err
		}

		client, err := storageClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 30*time.Second)
		defer cancel()

		// Step 1: Get all keys, filter by prefix.
		keyStream, err := client.GetAllKeys(callCtx, &storagepb.GetAllKeysRequest{Id: storeID})
		if err != nil {
			return nil, fmt.Errorf("GetAllKeys: %w", err)
		}

		var matchedKeys []string
		for {
			resp, err := keyStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("GetAllKeys stream: %w", err)
			}
			for _, k := range resp.GetKeys() {
				if strings.HasPrefix(k, prefix) {
					matchedKeys = append(matchedKeys, k)
				}
			}
		}

		truncated := false
		if limit > 0 && len(matchedKeys) > limit {
			matchedKeys = matchedKeys[:limit]
			truncated = true
		}

		// Step 2: For each key, get the value.
		var entries []map[string]interface{}
		for _, key := range matchedKeys {
			entry := map[string]interface{}{"key": key}

			itemStream, err := client.GetItem(callCtx, &storagepb.GetItemRequest{Id: storeID, Key: key})
			if err != nil {
				entry["error"] = err.Error()
				entries = append(entries, entry)
				continue
			}

			var buf bytes.Buffer
			for {
				resp, err := itemStream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					entry["error"] = err.Error()
					break
				}
				buf.Write(resp.GetResult())
			}

			data := buf.Bytes()
			entry["size"] = len(data)

			var parsed map[string]interface{}
			if json.Unmarshal(data, &parsed) == nil {
				entry["value"] = redactSensitive(parsed)
			} else {
				// Try as a JSON array or scalar.
				var raw interface{}
				if json.Unmarshal(data, &raw) == nil {
					entry["value"] = raw
				} else if utf8.Valid(data) {
					entry["value"] = string(data)
				} else {
					entry["value"] = fmt.Sprintf("<binary %d bytes>", len(data))
				}
			}

			entries = append(entries, entry)
		}

		return map[string]interface{}{
			"store_id":  storeID,
			"prefix":    prefix,
			"count":     len(entries),
			"entries":   entries,
			"truncated": truncated,
		}, nil
	})

	// ── 10. kv_get_item_metadata ─────────────────────────────────────────
	s.register(toolDef{
		Name:        "kv_get_item_metadata",
		Description: "Get key existence and size without decoding the full content.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"store_id": {
					Type:        "string",
					Description: "The storage connection/store ID.",
				},
				"key": {
					Type:        "string",
					Description: "The key to check.",
				},
			},
			Required: []string{"store_id", "key"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		storeID := getStr(args, "store_id")
		key := getStr(args, "key")

		if storeID == "" {
			return nil, fmt.Errorf("missing required argument: store_id")
		}
		if key == "" {
			return nil, fmt.Errorf("missing required argument: key")
		}
		if err := s.cfg.validateStorageAccess(storeID, key); err != nil {
			return nil, err
		}

		client, err := storageClient(ctx, s.clients)
		if err != nil {
			return nil, err
		}

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		stream, err := client.GetItem(callCtx, &storagepb.GetItemRequest{Id: storeID, Key: key})
		if err != nil {
			return map[string]interface{}{
				"store_id":   storeID,
				"key":        key,
				"exists":     false,
				"size":       0,
				"size_human": "0 B",
			}, nil
		}

		var totalSize int
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return map[string]interface{}{
					"store_id":   storeID,
					"key":        key,
					"exists":     false,
					"size":       0,
					"size_human": "0 B",
				}, nil
			}
			totalSize += len(resp.GetResult())
		}

		return map[string]interface{}{
			"store_id":   storeID,
			"key":        key,
			"exists":     true,
			"size":       totalSize,
			"size_human": fmtBytes(uint64(totalSize)),
		}, nil
	})
}
