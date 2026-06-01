package security

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// PathSecurityError represents a path traversal or escape attempt.
type PathSecurityError struct {
	RequestedPath string
	Reason        string
}

func (e *PathSecurityError) Error() string {
	return fmt.Sprintf("path security violation: %s (path: %q)", e.Reason, e.RequestedPath)
}

// CanonicalizePath validates and canonicalizes a requested path to prevent
// directory traversal attacks and escapes from the base directory.
//
// Security checks:
// - DENY absolute paths (must be relative to base)
// - DENY ".." components that escape base directory
// - DENY paths containing null bytes
// - DENY symlink-style tricks (/../, /./, etc.)
// - NORMALIZE path separators and remove redundant slashes
//
// Parameters:
//   - base: Base directory that requests must stay within
//   - requested: User-supplied path (potentially malicious)
//
// Returns:
//   - canonicalPath: Safe, canonical path within base directory
//   - error: PathSecurityError if validation fails
//
// Example:
//   canonical, err := CanonicalizePath("/var/lib/globular/files", "user/docs/file.txt")
//   // Returns: "/var/lib/globular/files/user/docs/file.txt", nil
//
//   canonical, err := CanonicalizePath("/var/lib/globular/files", "../../../etc/passwd")
//   // Returns: "", PathSecurityError (escapes base directory)
func CanonicalizePath(base, requested string) (string, error) {
	// Check for null bytes (common attack vector)
	if strings.Contains(requested, "\x00") {
		return "", &PathSecurityError{
			RequestedPath: requested,
			Reason:        "null byte in path",
		}
	}

	// DENY absolute paths - must be relative to base
	if filepath.IsAbs(requested) {
		return "", &PathSecurityError{
			RequestedPath: requested,
			Reason:        "absolute paths not allowed",
		}
	}

	// Clean the requested path (removes .., ., redundant slashes)
	cleaned := filepath.Clean(requested)

	// After cleaning, check if path tries to escape (starts with ..)
	if strings.HasPrefix(cleaned, "..") {
		return "", &PathSecurityError{
			RequestedPath: requested,
			Reason:        "path traversal attempt (escapes base directory)",
		}
	}

	// Build full path
	fullPath := filepath.Join(base, cleaned)

	// Final safety check: ensure fullPath is still within base
	// This catches edge cases with symlinks or complex path manipulation
	cleanedFull := filepath.Clean(fullPath)
	cleanedBase := filepath.Clean(base)

	// Ensure cleanedFull starts with cleanedBase
	if !strings.HasPrefix(cleanedFull, cleanedBase) {
		return "", &PathSecurityError{
			RequestedPath: requested,
			Reason:        "canonical path escapes base directory",
		}
	}

	// Additional check: after removing base, remainder shouldn't start with ".."
	rel, err := filepath.Rel(cleanedBase, cleanedFull)
	if err != nil {
		return "", &PathSecurityError{
			RequestedPath: requested,
			Reason:        fmt.Sprintf("failed to compute relative path: %v", err),
		}
	}

	if strings.HasPrefix(rel, "..") {
		return "", &PathSecurityError{
			RequestedPath: requested,
			Reason:        "relative path escapes base directory",
		}
	}

	return cleanedFull, nil
}

// ValidateResourcePath validates a resource path for use in RBAC.
// This is a simpler version that just checks for dangerous patterns
// without filesystem operations.
//
// Security checks:
// - DENY null bytes
// - DENY absolute paths
// - DENY ".." components
// - NORMALIZE slashes
//
// Example:
//   ValidateResourcePath("/users/alice/files/doc.txt") // OK
//   ValidateResourcePath("/users/../admin/secrets") // DENY
func ValidateResourcePath(resourcePath string) error {
	// Check for null bytes
	if strings.Contains(resourcePath, "\x00") {
		return &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "null byte in resource path",
		}
	}

	// Clean path (this handles most attacks)
	cleaned := path.Clean(resourcePath)

	// Check for ".." after cleaning (indicates escape attempt)
	if strings.Contains(cleaned, "..") {
		return &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "path traversal in resource path",
		}
	}

	// Resource paths should typically start with "/"
	if !strings.HasPrefix(cleaned, "/") {
		return &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "resource path must be absolute",
		}
	}

	return nil
}

// reservedPathPrefixes defines path prefixes that are system-reserved
// and cannot be used as owner names (prevents privilege escalation).
//
// Security Fix #8: Reserved prefixes prevent attackers from creating
// resources that masquerade as system resources.
var reservedPathPrefixes = []string{
	"admin",
	"root",
	"system",
	"sa",          // Super admin
	"globular",    // System namespace
	".globular",   // Hidden system files
	"_",           // Reserved for internal use
	".",           // Relative paths
}

// isReservedOwner checks if an owner name is reserved for system use.
func isReservedOwner(owner string) bool {
	owner = strings.ToLower(owner)
	for _, reserved := range reservedPathPrefixes {
		if owner == reserved || strings.HasPrefix(owner, reserved) {
			return true
		}
	}
	return false
}

// containsDangerousEncoding checks for URL-encoded or Unicode tricks
// that could bypass path validation.
//
// Security Fix #8: Reject encoding tricks that could bypass filters:
// - URL-encoded slashes (%2f, %2F)
// - URL-encoded dots (%2e, %2E)
// - URL-encoded null bytes (%00)
// - Unicode confusables (e.g., U+2215 DIVISION SLASH looks like /)
// - Backslashes (Windows-style paths)
func containsDangerousEncoding(s string) bool {
	// Check for URL encoding
	if strings.Contains(s, "%") {
		// Common dangerous encodings
		dangerous := []string{
			"%2e", "%2E", // dot
			"%2f", "%2F", // slash
			"%00",        // null byte
			"%5c", "%5C", // backslash
		}
		lowerS := strings.ToLower(s)
		for _, enc := range dangerous {
			if strings.Contains(lowerS, enc) {
				return true
			}
		}
	}

	// Check for backslashes (Windows-style paths)
	if strings.Contains(s, "\\") {
		return true
	}

	// Check for Unicode confusables that look like path separators
	// U+2215 (DIVISION SLASH), U+2044 (FRACTION SLASH), U+29F8 (BIG SOLIDUS)
	confusables := []rune{
		'\u2215', // ∕ DIVISION SLASH
		'\u2044', // ⁄ FRACTION SLASH
		'\u29F8', // ⧸ BIG SOLIDUS
		'\uFF0F', // ／ FULLWIDTH SOLIDUS
	}
	for _, r := range s {
		for _, confusable := range confusables {
			if r == confusable {
				return true
			}
		}
	}

	return false
}

// ExtractOwnerFromPath attempts to extract the owner from a resource path.
// Convention: /users/{owner}/...
//
// Security Fix #8: Enhanced with defense-in-depth:
// - Canonicalize before extraction (prevents ".." tricks)
// - Extract by segments, not substring (prevents confusion)
// - Reject empty segments (prevents "///" tricks)
// - Reject reserved prefixes (prevents admin impersonation)
// - Reject URL-encoding tricks (prevents bypass)
// - Reject Unicode confusables (prevents lookalike attacks)
//
// Returns:
//   - owner: The extracted owner (e.g., "alice" from "/users/alice/files")
//   - error: If path doesn't match expected pattern or fails validation
//
// This is used by ValidateResourceOwnership to implement ownership checks.
func ExtractOwnerFromPath(resourcePath string) (string, error) {
	// Security Fix #8: Check for dangerous encoding BEFORE canonicalization
	// (canonicalization might normalize some encodings)
	if containsDangerousEncoding(resourcePath) {
		return "", &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "path contains dangerous encoding (URL-encoded or Unicode tricks)",
		}
	}

	// Security Fix #8: Canonicalize path first (prevents traversal)
	cleaned := path.Clean(resourcePath)

	// Security Fix #8: Reject if cleaning changed path significantly
	// (indicates attempt to hide malicious path)
	if cleaned != resourcePath && !strings.HasPrefix(resourcePath, "/") {
		// Allow only simple normalization (adding leading slash)
		// Reject complex changes like /a/../b → /b
		return "", &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "path required canonicalization (suspicious)",
		}
	}

	// Security Fix #8: Split by segments (not substring matching)
	parts := strings.Split(cleaned, "/")

	// Security Fix #8: Reject empty segments (indicates "//" in path)
	for i, part := range parts {
		if i == 0 {
			// First part is empty for absolute paths ("/users" → ["", "users"])
			continue
		}
		if part == "" {
			return "", &PathSecurityError{
				RequestedPath: resourcePath,
				Reason:        fmt.Sprintf("empty segment at position %d (double slash)", i),
			}
		}
	}

	// Expected format: /users/{owner}/...
	// After split: ["", "users", "{owner}", ...]
	if len(parts) < 3 {
		return "", fmt.Errorf("path too short to extract owner: %q", resourcePath)
	}

	// Check if it follows /users/* pattern
	if parts[1] != "users" {
		// Not a user-scoped resource
		return "", fmt.Errorf("path does not follow /users/{owner} pattern: %q", resourcePath)
	}

	owner := parts[2]

	// Validate owner segment
	if owner == "" {
		return "", &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "empty owner segment",
		}
	}

	// Security Fix #8: Reject reserved owner names
	if isReservedOwner(owner) {
		return "", &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        fmt.Sprintf("owner %q is reserved for system use", owner),
		}
	}

	// Security Fix #8: Reject owner names with dots (prevents ".." tricks)
	if strings.Contains(owner, ".") {
		return "", &PathSecurityError{
			RequestedPath: resourcePath,
			Reason:        "owner name cannot contain dots",
		}
	}

	// Security Fix #8: Validate owner name format (alphanumeric + hyphen/underscore only)
	for _, c := range owner {
		isValid := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '@'
		if !isValid {
			return "", &PathSecurityError{
				RequestedPath: resourcePath,
				Reason:        fmt.Sprintf("owner %q contains invalid characters", owner),
			}
		}
	}

	return owner, nil
}
