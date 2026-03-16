// Package policy loads authorization policy from external JSON files,
// falling back to compiled defaults when files are missing or invalid.
//
// Loading precedence (highest to lowest):
//
//	/etc/globular/policy/          — admin overrides (survives upgrades)
//	/var/lib/globular/policy/      — package-shipped defaults
//	compiled fallback              — hardcoded Go values (safety net)
//
// Services call LoadPermissions(serviceName) at startup. The RBAC service
// calls LoadClusterRoles() at startup. Both return the merged effective
// policy and a boolean indicating whether an external file was used.
package policy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Default filesystem roots. Overridable for testing.
var (
	AdminRoot   = "/etc/globular/policy"
	PackageRoot = "/var/lib/globular/policy"
)

// ── Permissions (per-service action→resource mappings) ───────────────────────

// PermissionsFile is the JSON schema for permissions.json.
type PermissionsFile struct {
	Version     string       `json:"version"`
	Service     string       `json:"service"`
	Permissions []Permission `json:"permissions"`
}

// Permission maps a gRPC action to its resource access requirements.
type Permission struct {
	Action     string     `json:"action"`
	Permission string     `json:"permission"`
	Resources  []Resource `json:"resources"`
}

// Resource identifies a field in the gRPC request message that must be
// checked for access control.
type Resource struct {
	Index      int    `json:"index"`
	Field      string `json:"field"`
	Permission string `json:"permission"`
}

// LoadPermissions loads permissions.json for a service.
// Returns (permissions, fromFile, error). If no external file is found
// or the file is invalid, returns (nil, false, nil) — the caller should
// use its compiled fallback.
func LoadPermissions(serviceName string) ([]interface{}, bool, error) {
	paths := permissionsPaths(serviceName)
	data, path, err := readFirst(paths)
	if err != nil {
		return nil, false, nil // no file found
	}

	var pf PermissionsFile
	if err := json.Unmarshal(data, &pf); err != nil {
		slog.Warn("policy: invalid JSON in permissions file", "path", path, "error", err)
		return nil, false, nil
	}

	if errs := validatePermissions(&pf); len(errs) > 0 {
		for _, e := range errs {
			slog.Error("policy: permissions validation error", "path", path, "error", e)
		}
		return nil, false, nil
	}

	result := permissionsToInterface(pf.Permissions)
	slog.Info("policy: loaded permissions from file", "service", serviceName, "path", path, "count", len(pf.Permissions))
	return result, true, nil
}

// ── Cluster Roles (RBAC role→methods mappings) ──────────────────────────────

// ClusterRolesFile is the JSON schema for cluster-roles.json.
type ClusterRolesFile struct {
	Version string              `json:"version"`
	Roles   map[string][]string `json:"roles"`
}

// LoadClusterRoles loads cluster-roles.json.
// Returns (roleMap, fromFile, error). If no external file is found or
// the file is invalid, returns (nil, false, nil) — the caller should
// use its compiled fallback.
func LoadClusterRoles() (map[string][]string, bool, error) {
	paths := clusterRolesPaths()
	data, path, err := readFirst(paths)
	if err != nil {
		return nil, false, nil
	}

	var crf ClusterRolesFile
	if err := json.Unmarshal(data, &crf); err != nil {
		slog.Warn("policy: invalid JSON in cluster-roles file", "path", path, "error", err)
		return nil, false, nil
	}

	if errs := validateClusterRoles(&crf); len(errs) > 0 {
		for _, e := range errs {
			slog.Error("policy: cluster-roles validation error", "path", path, "error", e)
		}
		return nil, false, nil
	}

	slog.Info("policy: loaded cluster roles from file", "path", path, "roles", len(crf.Roles))
	return crf.Roles, true, nil
}

// ── Path helpers ─────────────────────────────────────────────────────────────

func permissionsPaths(serviceName string) []string {
	return []string{
		filepath.Join(AdminRoot, "services", serviceName, "permissions.json"),
		filepath.Join(PackageRoot, "services", serviceName, "permissions.json"),
	}
}

func clusterRolesPaths() []string {
	return []string{
		filepath.Join(AdminRoot, "rbac", "cluster-roles.json"),
		filepath.Join(PackageRoot, "rbac", "cluster-roles.json"),
	}
}

// readFirst tries each path in order and returns the first readable file.
func readFirst(paths []string) ([]byte, string, error) {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return data, p, nil
		}
	}
	return nil, "", fmt.Errorf("no policy file found")
}

// ── Conversion helpers ──────────────────────────────────────────────────────

// permissionsToInterface converts typed Permission structs to the
// []interface{} format expected by the Service.SetPermissions() interface.
func permissionsToInterface(perms []Permission) []interface{} {
	result := make([]interface{}, 0, len(perms))
	for _, p := range perms {
		resources := make([]interface{}, 0, len(p.Resources))
		for _, r := range p.Resources {
			resources = append(resources, map[string]interface{}{
				"index":      r.Index,
				"field":      r.Field,
				"permission": r.Permission,
			})
		}
		result = append(result, map[string]interface{}{
			"action":     p.Action,
			"permission": p.Permission,
			"resources":  resources,
		})
	}
	return result
}

// ── Validation ──────────────────────────────────────────────────────────────

var validPermissionVerbs = map[string]bool{
	"read": true, "write": true, "delete": true, "admin": true,
}

func validatePermissions(pf *PermissionsFile) []string {
	var errs []string
	if pf.Version == "" {
		errs = append(errs, "missing version field")
	}
	if pf.Service == "" {
		errs = append(errs, "missing service field")
	}
	seen := make(map[string]bool)
	for i, p := range pf.Permissions {
		prefix := fmt.Sprintf("permissions[%d]", i)
		if p.Action == "" {
			errs = append(errs, prefix+": missing action")
		} else if !strings.HasPrefix(p.Action, "/") {
			errs = append(errs, prefix+": action must start with /")
		}
		if !validPermissionVerbs[p.Permission] {
			errs = append(errs, fmt.Sprintf("%s: invalid permission verb %q", prefix, p.Permission))
		}
		if seen[p.Action] {
			errs = append(errs, prefix+": duplicate action "+p.Action)
		}
		seen[p.Action] = true
		for j, r := range p.Resources {
			rprefix := fmt.Sprintf("%s.resources[%d]", prefix, j)
			if r.Field == "" {
				errs = append(errs, rprefix+": missing field")
			}
			if !validPermissionVerbs[r.Permission] {
				errs = append(errs, fmt.Sprintf("%s: invalid permission verb %q", rprefix, r.Permission))
			}
			if r.Index < 0 {
				errs = append(errs, rprefix+": index must be >= 0")
			}
		}
	}
	return errs
}

func validateClusterRoles(crf *ClusterRolesFile) []string {
	var errs []string
	if crf.Version == "" {
		errs = append(errs, "missing version field")
	}
	if len(crf.Roles) == 0 {
		errs = append(errs, "roles map is empty")
	}
	for role, methods := range crf.Roles {
		for i, m := range methods {
			if m == "/*" {
				continue // global wildcard is valid
			}
			if !strings.HasPrefix(m, "/") {
				errs = append(errs, fmt.Sprintf("roles[%s][%d]: method %q must start with /", role, i, m))
			}
		}
	}
	return errs
}
