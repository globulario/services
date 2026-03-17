// Package policy loads authorization policy from external JSON files,
// falling back to compiled defaults when files are missing or invalid.
//
// Loading precedence (highest to lowest):
//
//	/etc/globular/policy/          — admin overrides (survives upgrades)
//	/var/lib/globular/policy/      — package-shipped defaults
//	compiled fallback              — hardcoded Go values (safety net)
//
// The policy model separates transport (gRPC method paths) from authorization
// (stable action keys like "file.read", "file.write"). Services define
// method→action mappings in permissions.json; RBAC grants action keys in
// cluster-roles.json.
package policy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Default filesystem roots. Overridable for testing.
var (
	AdminRoot   = "/etc/globular/policy"
	PackageRoot = "/var/lib/globular/policy"
)

// ── Permissions (per-service method→action→resource mappings) ────────────────

// PermissionsFile is the JSON schema for permissions.json.
type PermissionsFile struct {
	Version     string       `json:"version"`
	Service     string       `json:"service"`
	Permissions []Permission `json:"permissions"`
}

// Permission maps a gRPC method to a stable action key and its resource
// access requirements.
type Permission struct {
	// Method is the gRPC full method path (e.g., "/file.FileService/ReadDir").
	// Used by the interceptor to resolve the action key at runtime.
	Method string `json:"method"`

	// Action is the stable RBAC action key (e.g., "file.list").
	// This is the canonical permission identifier used in role grants.
	// For backward compatibility, if Method is empty, Action may contain
	// a gRPC method path (v1 format).
	Action string `json:"action"`

	// Permission is the resource-level verb (read/write/delete/admin).
	Permission string `json:"permission,omitempty"`

	// Resources defines which request fields to check for access control.
	Resources []Resource `json:"resources"`
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

// ── Action Resolver ─────────────────────────────────────────────────────────

// ActionResolver maps gRPC method paths to stable action keys.
// Built from permissions data at service startup; used by interceptors
// to translate transport-level identifiers to RBAC-level identifiers.
type ActionResolver struct {
	mu          sync.RWMutex
	methodToAct map[string]string // "/file.FileService/ReadDir" → "file.list"
	actionToRes map[string][]Resource // "file.list" → resources
}

// NewResolver creates an empty ActionResolver.
func NewResolver() *ActionResolver {
	return &ActionResolver{
		methodToAct: make(map[string]string),
		actionToRes: make(map[string][]Resource),
	}
}

// globalResolver is the singleton used by the interceptor.
var globalResolver = NewResolver()

// GlobalResolver returns the singleton ActionResolver.
func GlobalResolver() *ActionResolver {
	return globalResolver
}

// Register adds method→action mappings from a Permission slice.
// Called by each service at startup after loading permissions.
func (r *ActionResolver) Register(perms []Permission) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range perms {
		method := p.Method
		action := p.Action
		if method == "" {
			// v1 format: action field contains the method path
			if strings.HasPrefix(action, "/") {
				method = action
				// No stable action key available in v1 format
			}
			continue
		}
		if action == "" || strings.HasPrefix(action, "/") {
			continue // no valid action key
		}
		r.methodToAct[method] = action
		if _, exists := r.actionToRes[action]; !exists {
			r.actionToRes[action] = p.Resources
		}
	}
}

// RegisterFromInterface registers method→action mappings from the
// []interface{} format used by Service.GetPermissions().
func (r *ActionResolver) RegisterFromInterface(perms []interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, raw := range perms {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		method, _ := m["method"].(string)
		action, _ := m["action"].(string)
		if method == "" || action == "" || strings.HasPrefix(action, "/") {
			continue
		}
		r.methodToAct[method] = action
	}
}

// Resolve returns the stable action key for a gRPC method path.
// If no mapping exists, returns the method path unchanged (backward compat).
func (r *ActionResolver) Resolve(method string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if action, ok := r.methodToAct[method]; ok {
		return action
	}
	return method // fallback: use method path as-is
}

// HasMapping returns true if a method→action mapping exists.
func (r *ActionResolver) HasMapping(method string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.methodToAct[method]
	return ok
}

// ── Cluster Roles (RBAC role→action-key grants) ─────────────────────────────

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
//
// In v2 format (method + action), the output map uses "action" for the stable
// action key and "method" for the gRPC method path. RBAC storage is keyed by
// the action key. For backward compatibility, v1 entries (no method field)
// use "action" as the method path directly.
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
		entry := map[string]interface{}{
			"resources": resources,
		}
		if p.Method != "" {
			// v2 format: action = stable key, method = transport path
			entry["method"] = p.Method
			entry["action"] = p.Action
		} else {
			// v1 format: action is the method path
			entry["action"] = p.Action
		}
		if p.Permission != "" {
			entry["permission"] = p.Permission
		}
		result = append(result, entry)
	}
	return result
}

// ── Validation ──────────────────────────────────────────────────────────────

var validPermissionVerbs = map[string]bool{
	"read": true, "write": true, "delete": true, "admin": true,
}

// actionKeyRe matches stable action keys: lowercase dotted identifiers.
var actionKeyRe = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)

// IsActionKey returns true if s is a stable action key (not a method path).
func IsActionKey(s string) bool {
	return s != "" && !strings.HasPrefix(s, "/") && actionKeyRe.MatchString(s)
}

// IsMethodPath returns true if s looks like a gRPC method path.
func IsMethodPath(s string) bool {
	return strings.HasPrefix(s, "/")
}

func validatePermissions(pf *PermissionsFile) []string {
	var errs []string
	if pf.Version == "" {
		errs = append(errs, "missing version field")
	}
	if pf.Service == "" {
		errs = append(errs, "missing service field")
	}
	seenMethod := make(map[string]bool)
	for i, p := range pf.Permissions {
		prefix := fmt.Sprintf("permissions[%d]", i)

		// v2 format: both method and action required
		if p.Method != "" {
			if !strings.HasPrefix(p.Method, "/") {
				errs = append(errs, prefix+": method must start with /")
			}
			if seenMethod[p.Method] {
				errs = append(errs, prefix+": duplicate method "+p.Method)
			}
			seenMethod[p.Method] = true

			if p.Action == "" {
				errs = append(errs, prefix+": missing action key")
			} else if !IsActionKey(p.Action) {
				errs = append(errs, fmt.Sprintf("%s: invalid action key %q (must be lowercase dotted identifier)", prefix, p.Action))
			}
		} else {
			// v1 format: action contains method path
			if p.Action == "" {
				errs = append(errs, prefix+": missing action")
			} else if !strings.HasPrefix(p.Action, "/") {
				errs = append(errs, prefix+": v1 action must start with / (or use method+action for v2)")
			}
		}

		// Permission verb is optional at top level (some entries only have resources)
		if p.Permission != "" && !validPermissionVerbs[p.Permission] {
			errs = append(errs, fmt.Sprintf("%s: invalid permission verb %q", prefix, p.Permission))
		}

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
	for role, grants := range crf.Roles {
		for i, g := range grants {
			if g == "*" || g == "/*" {
				continue // global wildcard is valid (both old and new syntax)
			}
			// Accept both action keys and method paths during migration
			if IsActionKey(g) || IsMethodPath(g) {
				continue
			}
			errs = append(errs, fmt.Sprintf("roles[%s][%d]: %q is neither a valid action key nor method path", role, i, g))
		}
	}
	return errs
}
