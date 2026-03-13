package main

// namespace.go — namespace and package ownership model using existing RBAC infrastructure.
//
// Namespaces map to RBAC paths `/namespaces/{publisherID}` with resource type "namespace".
// Packages map to RBAC paths `/packages/{publisherID}/{packageName}` with resource type "package".
// Existing AddResourceOwner / ValidateAccess RPCs handle ownership — zero new RBAC
// infrastructure needed.
//
// Canonical namespace rules:
//   - Lowercase ASCII only: a-z, 0-9, ., _, -, @
//   - Max length: 128 characters
//   - No leading/trailing separators
//   - No consecutive separators (.., --, __)
//   - Reserved prefixes blocked from user claims: globular, system, core, internal, admin

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"unicode"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	rbacpb "github.com/globulario/services/golang/rbac/rbacpb"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	namespaceResourceType = "namespace"
	packageResourceType   = "package"
	namespacePathPrefix   = "/namespaces/"
	packagePathPrefix     = "/packages/"
	namespaceMaxLength    = 128
)

// reservedPrefixes are namespace prefixes that cannot be claimed by regular users.
// Only the "sa" superuser can create namespaces under these prefixes.
var reservedPrefixes = []string{
	"globular",
	"system",
	"core",
	"internal",
	"admin",
}

// validNamespaceRe matches a canonical namespace ID: lowercase ASCII, digits, separators.
var validNamespaceRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._@-]*[a-z0-9]$|^[a-z0-9]$`)

// consecutiveSepRe matches consecutive separator characters.
var consecutiveSepRe = regexp.MustCompile(`[._-]{2,}`)

// ── Canonical namespace validation ──────────────────────────────────────────

// ValidateNamespaceID validates and normalizes a namespace identifier.
// Returns the canonical (lowercase) form or an error.
func ValidateNamespaceID(raw string) (string, error) {
	// Normalize to lowercase.
	ns := strings.ToLower(strings.TrimSpace(raw))

	if ns == "" {
		return "", fmt.Errorf("namespace ID cannot be empty")
	}

	// Reject Unicode characters (must be pure ASCII).
	for _, r := range ns {
		if r > unicode.MaxASCII {
			return "", fmt.Errorf("namespace %q contains non-ASCII characters — only a-z, 0-9, '.', '_', '-', '@' are allowed", raw)
		}
	}

	if len(ns) > namespaceMaxLength {
		return "", fmt.Errorf("namespace %q exceeds maximum length of %d characters", ns, namespaceMaxLength)
	}

	if !validNamespaceRe.MatchString(ns) {
		return "", fmt.Errorf("namespace %q is not valid — must contain only a-z, 0-9, '.', '_', '-', '@' and not start/end with a separator", ns)
	}

	if consecutiveSepRe.MatchString(ns) {
		return "", fmt.Errorf("namespace %q contains consecutive separator characters", ns)
	}

	return ns, nil
}

// isReservedNamespace returns true if the namespace is under a reserved prefix.
func isReservedNamespace(ns string) bool {
	for _, prefix := range reservedPrefixes {
		if ns == prefix || strings.HasPrefix(ns, prefix+".") || strings.HasPrefix(ns, prefix+"-") || strings.HasPrefix(ns, prefix+"_") || strings.HasPrefix(ns, prefix+"@") {
			return true
		}
	}
	return false
}

// ── RBAC path helpers ──────────────────────────────────────────────────────

// namespacePath returns the RBAC resource path for a publisher namespace.
func namespacePath(publisherID string) string {
	return namespacePathPrefix + publisherID
}

// packagePath returns the RBAC resource path for a specific package.
func packagePath(publisherID, packageName string) string {
	return packagePathPrefix + publisherID + "/" + packageName
}

// ── RBAC client ──────────────────────────────────────────────────────────────

// getRbacClient returns a connected RBAC service client.
func (srv *server) getRbacClient() (*rbac_client.Rbac_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, fmt.Errorf("connect to RBAC service: %w", err)
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// ── Namespace operations ────────────────────────────────────────────────────

// ensureNamespaceExists creates an RBAC namespace resource if it doesn't already exist.
// The caller (ownerSubject) becomes the namespace owner.
func (srv *server) ensureNamespaceExists(ctx context.Context, publisherID, ownerSubject, token string) error {
	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	path := namespacePath(publisherID)

	// Check if namespace already exists by trying to get permissions.
	perms, err := rbacClient.GetResourcePermissions(path)
	if err == nil && perms != nil && perms.Owners != nil && len(perms.Owners.Accounts) > 0 {
		// Namespace exists, nothing to do.
		return nil
	}

	// Determine subject type from AuthContext.
	subjectType := rbacpb.SubjectType_ACCOUNT
	if authCtx := security.FromContext(ctx); authCtx != nil && authCtx.PrincipalType == "application" {
		subjectType = rbacpb.SubjectType_APPLICATION
	}

	// Create namespace by adding the caller as owner.
	if err := rbacClient.AddResourceOwner(token, path, ownerSubject, namespaceResourceType, subjectType); err != nil {
		return fmt.Errorf("create namespace %q with owner %q: %w", publisherID, ownerSubject, err)
	}

	slog.Info("namespace created", "namespace", publisherID, "owner", ownerSubject)

	// If a real user (not "sa" / not migration) claims a namespace, remove it from
	// the unclaimed list so trust labels reflect the ownership change.
	if ownerSubject != "sa" {
		srv.removeUnclaimedNamespace(ctx, publisherID)
	}

	// Audit event.
	srv.publishAuditEvent(ctx, "namespace.claimed", map[string]any{
		"namespace": publisherID,
		"owner":     ownerSubject,
	})

	return nil
}

// ensurePackageOwnership creates a package-level RBAC resource on first publish.
// Package ownership initially inherits from namespace ownership but can later diverge.
func (srv *server) ensurePackageOwnership(ctx context.Context, publisherID, packageName, ownerSubject, token string) {
	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return
	}

	path := packagePath(publisherID, packageName)

	// Check if package resource already exists.
	perms, err := rbacClient.GetResourcePermissions(path)
	if err == nil && perms != nil && perms.Owners != nil && len(perms.Owners.Accounts) > 0 {
		return // already exists
	}

	// Determine subject type.
	subjectType := rbacpb.SubjectType_ACCOUNT
	if authCtx := security.FromContext(ctx); authCtx != nil && authCtx.PrincipalType == "application" {
		subjectType = rbacpb.SubjectType_APPLICATION
	}

	if err := rbacClient.AddResourceOwner(token, path, ownerSubject, packageResourceType, subjectType); err != nil {
		slog.Debug("package ownership creation failed (non-fatal)", "package", path, "err", err)
	}
}

// ── Publisher access validation ──────────────────────────────────────────────

// validatePublisherAccess checks that the caller has write access to the publisher namespace.
// Uses the authenticated principal type (not hardcoded ACCOUNT) for authorization.
// Returns nil if access is granted, or a gRPC error if denied.
func (srv *server) validatePublisherAccess(ctx context.Context, publisherID string) error {
	if strings.TrimSpace(publisherID) == "" {
		return status.Error(codes.InvalidArgument, "publisher_id is required")
	}

	authCtx := security.FromContext(ctx)
	if authCtx == nil {
		// No AuthContext means this is an internal/direct call (not through interceptors).
		// In production, interceptors always inject AuthContext.
		// Allow internal calls to proceed without namespace validation.
		return nil
	}
	if authCtx.Subject == "" {
		return status.Error(codes.Unauthenticated, "authentication required for publish operations")
	}

	// Superuser bypass.
	if authCtx.Subject == "sa" {
		return nil
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return status.Errorf(codes.Internal, "RBAC service unavailable: %v", err)
	}

	path := namespacePath(publisherID)

	// Use the authenticated principal type for authorization (recommendation #3).
	subjectType := principalToSubjectType(authCtx.PrincipalType)

	hasAccess, accessDenied, err := rbacClient.ValidateAccess(authCtx.Subject, subjectType, "write", path)
	if err != nil {
		slog.Debug("namespace access check failed", "namespace", publisherID, "subject", authCtx.Subject, "err", err)
		return status.Errorf(codes.PermissionDenied,
			"namespace %q not found or access denied for %q — claim the namespace first with 'globular namespace claim %s'",
			publisherID, authCtx.Subject, publisherID)
	}

	if accessDenied || !hasAccess {
		return status.Errorf(codes.PermissionDenied,
			"subject %q does not have write access to namespace %q",
			authCtx.Subject, publisherID)
	}

	return nil
}

// principalToSubjectType maps an AuthContext.PrincipalType to the RBAC SubjectType.
func principalToSubjectType(principalType string) rbacpb.SubjectType {
	switch principalType {
	case "application":
		return rbacpb.SubjectType_APPLICATION
	case "node":
		return rbacpb.SubjectType_NODE_IDENTITY
	default:
		return rbacpb.SubjectType_ACCOUNT
	}
}

// isNamespaceOwner checks if the subject is an owner of the given namespace.
func (srv *server) isNamespaceOwner(ctx context.Context, publisherID, subject string) bool {
	if subject == "sa" {
		return true
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return false
	}

	path := namespacePath(publisherID)
	perms, err := rbacClient.GetResourcePermissions(path)
	if err != nil || perms == nil || perms.Owners == nil {
		return false
	}

	for _, acc := range perms.Owners.Accounts {
		if acc == subject || strings.HasPrefix(acc, subject+"@") {
			return true
		}
	}
	for _, app := range perms.Owners.Applications {
		if app == subject {
			return true
		}
	}
	return false
}

// ── Package-level access validation ──────────────────────────────────────────

// validatePackageAccess checks if the caller has access to a specific package.
// If the package resource exists in RBAC, validates against it.
// Falls back to namespace-level access for new packages or if package resource doesn't exist.
func (srv *server) validatePackageAccess(ctx context.Context, publisherID, packageName string) error {
	// First validate namespace access (base requirement).
	if err := srv.validatePublisherAccess(ctx, publisherID); err != nil {
		return err
	}

	// If package resource exists, also validate package-level access.
	authCtx := security.FromContext(ctx)
	if authCtx == nil || authCtx.Subject == "sa" {
		return nil // internal/superuser bypass
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return nil // RBAC unavailable, namespace check was sufficient
	}

	path := packagePath(publisherID, packageName)
	perms, err := rbacClient.GetResourcePermissions(path)
	if err != nil || perms == nil || perms.Owners == nil || len(perms.Owners.Accounts) == 0 {
		// Package resource doesn't exist yet - namespace access is sufficient for first publish.
		return nil
	}

	// Package resource exists — validate against it.
	subjectType := principalToSubjectType(authCtx.PrincipalType)
	hasAccess, accessDenied, err := rbacClient.ValidateAccess(authCtx.Subject, subjectType, "write", path)
	if err != nil || accessDenied || !hasAccess {
		// Check if namespace owner (namespace owners always have package access).
		if srv.isNamespaceOwner(ctx, publisherID, authCtx.Subject) {
			return nil
		}
		return status.Errorf(codes.PermissionDenied,
			"subject %q does not have write access to package %s/%s",
			authCtx.Subject, publisherID, packageName)
	}

	return nil
}

// ── Publish mode classification ──────────────────────────────────────────────

// classifyPublishMode determines the trust level of the current publish operation.
// Returns:
//   - "internal" for calls without an AuthContext (internal/direct)
//   - "human" for non-APPLICATION principals (interactive user publish)
//   - "trusted_publisher" for APPLICATION principals with a matching trusted publisher relationship
//   - "machine_publisher" for APPLICATION principals without a matching relationship
func (srv *server) classifyPublishMode(ctx context.Context, publisherID, packageName string) string {
	authCtx := security.FromContext(ctx)
	if authCtx == nil {
		return "internal"
	}
	if authCtx.PrincipalType != "application" {
		return "human"
	}
	// APPLICATION principal — check for trusted publisher relationship.
	if srv.matchesTrustedPublisherBySubject(ctx, publisherID, packageName, authCtx.Subject) {
		return "trusted_publisher"
	}
	return "machine_publisher"
}

// ── GetNamespace RPC ─────────────────────────────────────────────────────────

// GetNamespace implements the GetNamespace RPC — returns ownership and permission info for a namespace.
func (srv *server) GetNamespace(ctx context.Context, req *repopb.GetNamespaceRequest) (*repopb.GetNamespaceResponse, error) {
	nsID := strings.TrimSpace(req.GetNamespaceId())
	if nsID == "" {
		return nil, status.Error(codes.InvalidArgument, "namespace_id is required")
	}

	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "RBAC service unavailable: %v", err)
	}

	path := namespacePath(nsID)
	perms, err := rbacClient.GetResourcePermissions(path)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "namespace %q not found", nsID)
	}

	info := &repopb.NamespaceInfo{
		NamespaceId: nsID,
	}

	if perms.Owners != nil {
		info.Owners = append(info.Owners, perms.Owners.Accounts...)
		info.Owners = append(info.Owners, perms.Owners.Applications...)
	}

	// Collect permitted subjects from allowed permissions.
	for _, perm := range perms.Allowed {
		info.Permitted = append(info.Permitted, perm.Accounts...)
		info.Permitted = append(info.Permitted, perm.Applications...)
	}

	return &repopb.GetNamespaceResponse{Namespace: info}, nil
}
