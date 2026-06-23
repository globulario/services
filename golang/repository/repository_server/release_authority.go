// @awareness namespace=globular.platform
// @awareness component=platform_repository.release_authority
// @awareness file_role=rbac_native_release_authority_gate
// @awareness implements=globular.platform:intent.package.release_authority_is_rbac_native
// @awareness risk=high
package main

// release_authority.go — RBAC-native release authority (P1 Slice 1).
//
// Implements the two-step trust model from docs/design/package-lifecycle.md §3.4:
//
//	Step 1 FEDERATION  (resolveForgeIdentity): a forge token / authenticated
//	       principal resolves to an RBAC subject. Grants NOTHING.
//	Step 2 AUTHORIZATION (authorizeRelease): RBAC only — may the resolved subject
//	       allocate a STABLE release on the publisher namespace? The repository
//	       then allocates the release identity (AllocateUpload) on the gated channel.
//
// Invariants enforced (see docs/awareness/package_identity_invariants.yaml):
//   - package.release_allocation_requires_rbac_permission
//   - package.forge_binding_is_not_authorization
//   - package.ci_is_not_release_authority
//   - package.stable_channel_requires_release_permission
//
// Fail-closed: any inability to PROVE the permission denies STABLE and forces
// the caller to DEV; an authenticated-but-unidentified caller is denied entirely.

import (
	"context"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// releaseAllocateAction is the RBAC permission verb required on a publisher
// namespace to allocate a release-channel (STABLE) artifact identity. It is the
// resource-permission form of the contract permission repository.release.allocate.
// Release authority is THIS permission on the namespace — never a forge identity,
// git tag, CI environment, or hardcoded service account.
const releaseAllocateAction = "release.allocate"

// forgeIdentity is the result of the FEDERATION step: a caller resolved to an
// RBAC subject. It carries NO authority — authorization is a separate step.
type forgeIdentity struct {
	Subject   string // resolved RBAC subject ("" if authenticated but empty)
	Internal  bool   // true: in-process/direct call (no interceptor AuthContext)
	Superuser bool   // sa bypass

	authCtx *security.AuthContext
}

// resolveForgeIdentity performs STEP 1 (federation) ONLY. The auth interceptor
// has already federated the forge token / principal into the AuthContext; this
// reads it and grants nothing. (docs/design/package-lifecycle.md §3.4 step 1;
// invariant package.forge_binding_is_not_authorization.)
func (srv *server) resolveForgeIdentity(ctx context.Context) forgeIdentity {
	authCtx := security.FromContext(ctx)
	if authCtx == nil {
		// No interceptor context: in-process / direct call. Trusted system path,
		// consistent with validatePublisherAccess.
		return forgeIdentity{Internal: true}
	}
	return forgeIdentity{
		Subject:   authCtx.Subject,
		Superuser: authCtx.Subject == "sa",
		authCtx:   authCtx,
	}
}

// releaseAccessCheck is the injectable RBAC permission check for release
// allocation (overridden in tests). Default: ValidateAccess(release.allocate)
// on the publisher namespace. This is the ONLY thing that decides STABLE; the
// forge identity never does.
var releaseAccessCheck = func(srv *server, authCtx *security.AuthContext, publisherID string) (bool, error) {
	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return false, err
	}
	subjectType := principalToSubjectType(authCtx.PrincipalType)
	path := namespacePath(publisherID)
	hasAccess, denied, verr := rbacClient.ValidateAccess(authCtx.Subject, subjectType, releaseAllocateAction, path)
	if verr != nil {
		return false, verr
	}
	return hasAccess && !denied, nil
}

// authorizeRelease performs STEP 2 (authorization) ONLY: may the resolved
// subject allocate a STABLE release on the publisher namespace? RBAC only.
//
// Returns:
//   - allow=true  → subject holds release.allocate on the namespace (STABLE ok)
//   - allow=false → caller must be forced to DEV (no release authority proven)
//   - err != nil  → hard failure (authenticated caller with no subject) — deny entirely
//
// Fail-closed: a permission check that cannot be evaluated returns allow=false
// (force DEV), never allow=true.
func (srv *server) authorizeRelease(ctx context.Context, id forgeIdentity, publisherID string) (allow bool, err error) {
	// Internal/direct calls (no interceptor) are trusted system paths, consistent
	// with validatePublisherAccess. They may target STABLE.
	if id.Internal {
		return true, nil
	}
	// Authenticated path with no subject — cannot bind authority. Deny entirely;
	// even the DEV path requires a resolved subject (§3.4).
	if strings.TrimSpace(id.Subject) == "" {
		return false, status.Error(codes.Unauthenticated, "authentication required for release allocation")
	}
	// Superuser bypass (matches validatePublisherAccess).
	if id.Superuser {
		return true, nil
	}
	allowed, cerr := releaseAccessCheck(srv, id.authCtx, publisherID)
	if cerr != nil {
		// Cannot evaluate the permission → fail closed (no STABLE). Caller → DEV.
		slog.Warn("release-authority: permission check failed, forcing DEV",
			"publisher", publisherID, "subject", id.Subject, "err", cerr)
		return false, nil
	}
	return allowed, nil
}
