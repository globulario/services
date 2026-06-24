package main

// release_authority_test.go — required tests for the RBAC-native release
// authority gate (docs/design/package-lifecycle.md §3.4 + the
// package.* invariants in docs/awareness/package_identity_invariants.yaml).
//
// These prove the two-step trust model: federation (resolveForgeIdentity)
// resolves a subject and grants nothing; authorization (authorizeRelease) is
// the ONLY thing that decides STABLE, and it is RBAC-only. Forge identity, CI,
// and git tags carry no implicit privilege; failures fail closed.

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// withReleaseAccess swaps the injectable RBAC permission check for the duration
// of a test and restores it afterward.
func withReleaseAccess(t *testing.T, allow bool, err error) {
	t.Helper()
	prev := releaseAccessCheck
	releaseAccessCheck = func(_ *server, _ *security.AuthContext, _ string) (bool, error) {
		return allow, err
	}
	t.Cleanup(func() { releaseAccessCheck = prev })
}

func authCtx(subject, principalType string) context.Context {
	return (&security.AuthContext{Subject: subject, PrincipalType: principalType}).
		ToContext(context.Background())
}

// 1. Federation must resolve to a subject before authorization can run; an
//    authenticated caller with no subject cannot be authorized.
func TestForgeIdentityMustResolveToSubjectBeforeAuthorization(t *testing.T) {
	srv := &server{}

	id := srv.resolveForgeIdentity(authCtx("davecourtois", "user"))
	if id.Internal {
		t.Fatal("authenticated caller must not be treated as internal")
	}
	if id.Subject != "davecourtois" {
		t.Fatalf("federation must resolve the subject; got %q", id.Subject)
	}

	// No subject resolved → authorization must refuse to bind authority.
	empty := srv.resolveForgeIdentity(authCtx("", "user"))
	allow, err := srv.authorizeRelease(context.Background(), empty, "globulario")
	if allow {
		t.Fatal("must not authorize a release without a resolved subject")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

// 2. Release allocation requires the RBAC permission on the namespace.
func TestReleaseAllocationRequiresNamespacePermission(t *testing.T) {
	srv := &server{}
	withReleaseAccess(t, false, nil) // subject holds no permission

	id := srv.resolveForgeIdentity(authCtx("davecourtois", "user"))
	allow, err := srv.authorizeRelease(context.Background(), id, "globulario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow {
		t.Fatal("release must be denied when the namespace permission is absent")
	}
}

// 3. A GitHub org-shaped identity does not bypass RBAC: without the permission,
//    it is denied even though the subject string looks authoritative.
func TestGitHubOrgBindingDoesNotBypassRBAC(t *testing.T) {
	srv := &server{}
	withReleaseAccess(t, false, nil)

	// Subject resolved from a github-org binding, but RBAC says no.
	id := srv.resolveForgeIdentity(authCtx("globulario", "application"))
	allow, err := srv.authorizeRelease(context.Background(), id, "globulario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow {
		t.Fatal("forge/org identity must not grant release authority without RBAC")
	}
}

// 4. An Account subject without release permission gets DEV only.
func TestAccountWithoutReleasePermissionGetsDevOnly(t *testing.T) {
	srv := &server{}
	withReleaseAccess(t, false, nil)

	id := srv.resolveForgeIdentity(authCtx("davecourtois", "user"))
	allow, err := srv.authorizeRelease(context.Background(), id, "globulario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allow {
		t.Fatal("an account without release permission must be DEV-only (allow=false)")
	}
}

// 5. An Organization owner with the permission may allocate a STABLE release.
func TestOrganizationOwnerMayAllocateStableWhenPermissionExists(t *testing.T) {
	srv := &server{}
	withReleaseAccess(t, true, nil) // subject holds release.allocate on the namespace

	id := srv.resolveForgeIdentity(authCtx("globulario", "application"))
	allow, err := srv.authorizeRelease(context.Background(), id, "globulario")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allow {
		t.Fatal("organization owner with the permission must be allowed STABLE")
	}
}

// 6. CI without a resolved subject cannot allocate a release (no implicit
//    privilege); it fails closed.
func TestCIWithoutResolvedSubjectCannotAllocateRelease(t *testing.T) {
	srv := &server{}
	// CI presented a token that authenticated but did not resolve to a subject.
	id := srv.resolveForgeIdentity(authCtx("", "application"))
	allow, err := srv.authorizeRelease(context.Background(), id, "core@globular.io")
	if allow {
		t.Fatal("CI without a resolved subject must not allocate a release")
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
}

// 7. Federation and authorization are separate steps: the SAME federated
//    identity yields opposite results depending only on the RBAC permission —
//    proving the forge identity never decides.
func TestForgeTrustAndRBACAuthorizationAreSeparateSteps(t *testing.T) {
	srv := &server{}
	ctx := authCtx("globulario", "application")

	// Same federation result both times.
	id := srv.resolveForgeIdentity(ctx)
	if id.Subject != "globulario" {
		t.Fatalf("federation should resolve subject; got %q", id.Subject)
	}

	withReleaseAccess(t, false, nil)
	if allow, _ := srv.authorizeRelease(context.Background(), id, "globulario"); allow {
		t.Fatal("denied permission must deny the release for the same identity")
	}

	withReleaseAccess(t, true, nil)
	if allow, _ := srv.authorizeRelease(context.Background(), id, "globulario"); !allow {
		t.Fatal("granted permission must allow the release for the same identity")
	}
}

// withRolePermissions swaps the package-level role→actions map (loaded from
// cluster-roles.json at init) for a controlled one, so the decision policy is
// tested hermetically regardless of any cluster-roles.json present on disk.
func withRolePermissions(t *testing.T, m map[string][]string) {
	t.Helper()
	prev := security.RolePermissions
	security.RolePermissions = m
	t.Cleanup(func() { security.RolePermissions = prev })
}

// 8. P3: explicit resource grant (or namespace ownership) on the namespace is
//    sufficient release authority on its own.
func TestReleaseAuthority_ExplicitResourceGrantIsSufficient(t *testing.T) {
	if !releaseAuthorityDecision(true, nil, false) {
		t.Fatal("an explicit release.allocate resource grant must authorize STABLE")
	}
}

// 9. P3: a bound role granting release.allocate is authority ONLY when the
//    subject is associated with the namespace — capability without association
//    is not blanket authority and must be forced to DEV. This is the core
//    namespace-scoping property of the RBAC-native grant.
func TestReleaseAuthority_RoleCapabilityRequiresNamespaceAssociation(t *testing.T) {
	withRolePermissions(t, map[string][]string{"releaser": {"release.allocate"}})

	if !releaseAuthorityDecision(false, []string{"releaser"}, true) {
		t.Fatal("role granting release.allocate + namespace association must authorize STABLE")
	}
	if releaseAuthorityDecision(false, []string{"releaser"}, false) {
		t.Fatal("release.allocate capability WITHOUT namespace association must be DEV-only")
	}
}

// 10. P3: a bound role that does NOT grant release.allocate confers no release
//     authority, even with namespace association (association alone ≠ release).
func TestReleaseAuthority_RoleWithoutCapabilityIsDevOnly(t *testing.T) {
	withRolePermissions(t, map[string][]string{"publisher-only": {"repository.artifact.write"}})

	if releaseAuthorityDecision(false, []string{"publisher-only"}, true) {
		t.Fatal("a role without release.allocate must not grant release authority")
	}
	if releaseAuthorityDecision(false, nil, true) {
		t.Fatal("no roles must not grant release authority")
	}
}

// 11. P3: the cluster-roles.json grant is actually wired — the publisher service
//     account role carries release.allocate in the embedded policy, so a subject
//     bound to it resolves the capability via HasRolePermission. This is what
//     makes Slice 1's gate operable for a real, non-superuser CI release authority.
func TestPublisherSARoleGrantsReleaseAllocate(t *testing.T) {
	roles, err := policy.LoadEmbeddedClusterRoles()
	if err != nil {
		t.Fatalf("load embedded cluster roles: %v", err)
	}
	actions := roles[security.RoleRepositoryPublisherSA]
	found := false
	for _, a := range actions {
		if a == releaseAllocateAction {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("role %q must grant %q in cluster-roles.json; actions=%v",
			security.RoleRepositoryPublisherSA, releaseAllocateAction, actions)
	}

	// And the resolver agrees: a subject bound to that role holds the capability.
	// Pin RolePermissions to the embedded map so this is independent of any
	// cluster-roles.json present on the test host.
	withRolePermissions(t, roles)
	if !security.HasRolePermission([]string{security.RoleRepositoryPublisherSA}, releaseAllocateAction) {
		t.Fatalf("HasRolePermission must resolve %q for the publisher SA role", releaseAllocateAction)
	}
}
