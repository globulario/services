package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ── Path safety ──────────────────────────────────────────────────────────────

// validateFilePath checks if a path is within the configured allowed roots.
// Returns a cleaned absolute path or an error.
func (cfg *MCPConfig) validateFilePath(rawPath string) (string, error) {
	if len(cfg.FileAllowedRoots) == 0 {
		return "", fmt.Errorf("path_not_allowed: no file roots configured in MCP config")
	}

	// Clean and resolve the path to prevent traversal.
	cleaned := filepath.Clean(rawPath)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path_not_allowed: path must be absolute: %s", rawPath)
	}

	// Check against allowed roots.
	for _, root := range cfg.FileAllowedRoots {
		root = filepath.Clean(root)
		if cleaned == root || strings.HasPrefix(cleaned, root+string(filepath.Separator)) {
			return cleaned, nil
		}
	}

	return "", fmt.Errorf("path_not_allowed: %s is not within any configured file root", rawPath)
}

// ── Persistence safety ───────────────────────────────────────────────────────

func (cfg *MCPConfig) validatePersistenceAccess(connID, database, collection string) error {
	// If enabled but no allowlists configured at all, deny everything.
	if len(cfg.PersistenceAllowedConns) == 0 && len(cfg.PersistenceAllowedDBs) == 0 && len(cfg.PersistenceAllowedColls) == 0 {
		return fmt.Errorf("collection_not_allowed: persistence tools enabled but no allowlists configured — add persistence_allowed_connections/databases/collections to MCP config")
	}
	if len(cfg.PersistenceAllowedConns) > 0 && !contains(cfg.PersistenceAllowedConns, connID) {
		return fmt.Errorf("collection_not_allowed: connection %q not in allowlist", connID)
	}
	if database != "" && len(cfg.PersistenceAllowedDBs) > 0 && !contains(cfg.PersistenceAllowedDBs, database) {
		return fmt.Errorf("collection_not_allowed: database %q not in allowlist", database)
	}
	if collection != "" && len(cfg.PersistenceAllowedColls) > 0 && !contains(cfg.PersistenceAllowedColls, collection) {
		return fmt.Errorf("collection_not_allowed: collection %q not in allowlist", collection)
	}
	return nil
}

// ── Storage safety ───────────────────────────────────────────────────────────

func (cfg *MCPConfig) validateStorageAccess(connID, key string) error {
	// If enabled but no allowlists configured at all, deny everything.
	if len(cfg.StorageAllowedConns) == 0 && len(cfg.StorageAllowedKeyPrefixes) == 0 {
		return fmt.Errorf("key_prefix_not_allowed: storage tools enabled but no allowlists configured — add storage_allowed_connections/key_prefixes to MCP config")
	}
	if len(cfg.StorageAllowedConns) > 0 && !contains(cfg.StorageAllowedConns, connID) {
		return fmt.Errorf("key_prefix_not_allowed: connection %q not in allowlist", connID)
	}
	if key != "" && len(cfg.StorageAllowedKeyPrefixes) > 0 {
		allowed := false
		for _, prefix := range cfg.StorageAllowedKeyPrefixes {
			if strings.HasPrefix(key, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("key_prefix_not_allowed: key %q does not match any allowed prefix", key)
		}
	}
	return nil
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
