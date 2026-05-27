// rbac_deny_override_test.go: verify that explicit deny overrides allow for ALL
// subjects, including sa and owners (security.deny_overrides_allow invariant).

package main

import (
	"testing"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/storage/storage_store"
	"google.golang.org/protobuf/encoding/protojson"
)

// newDenyTestServer returns a minimal server with an in-memory cache.
// No ScyllaDB or resource service connections are needed — sa is
// always resolvable via accountExist's built-in fallback and
// permissions are pre-seeded into the BigCache.
func newDenyTestServer(t *testing.T) *server {
	t.Helper()
	cache := storage_store.NewBigCache_store()
	if err := cache.Open(""); err != nil {
		t.Fatalf("open bigcache: %v", err)
	}
	srv := &server{
		cache:  cache,
		Domain: "test.local",
	}
	return srv
}

// seedPermissions stores a Permissions proto in the cache so that
// getResourcePermissions can find it without hitting ScyllaDB.
func seedPermissions(t *testing.T, srv *server, path string, perms *rbacpb.Permissions) {
	t.Helper()
	perms.Path = path
	data, err := protojson.Marshal(perms)
	if err != nil {
		t.Fatalf("marshal permissions: %v", err)
	}
	if err := srv.cache.SetItem(path, data); err != nil {
		t.Fatalf("cache.SetItem: %v", err)
	}
}

// TestDenyOverridesSAAllow verifies that an explicit deny on a path
// blocks the sa account even though sa normally has full access.
func TestDenyOverridesSAAllow(t *testing.T) {
	srv := newDenyTestServer(t)

	seedPermissions(t, srv, "/secrets/nuclear-codes", &rbacpb.Permissions{
		Denied: []*rbacpb.Permission{
			{
				Name:     "read",
				Accounts: []string{"sa"},
			},
		},
	})

	hasAccess, accessDenied, err := srv.validateAccess(
		"sa", rbacpb.SubjectType_ACCOUNT, "read", "/secrets/nuclear-codes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("sa should NOT have access when explicitly denied")
	}
	if !accessDenied {
		t.Error("accessDenied should be true when sa is explicitly denied")
	}
}

// TestSAAllowedWhenNoDeny verifies that sa retains full access when
// there is no explicit deny rule on the path.
func TestSAAllowedWhenNoDeny(t *testing.T) {
	srv := newDenyTestServer(t)

	// Seed permissions with allowed entries but no deny for sa.
	seedPermissions(t, srv, "/cluster/config", &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{
			{
				Name:     "read",
				Accounts: []string{"operator"},
			},
		},
	})

	hasAccess, accessDenied, err := srv.validateAccess(
		"sa", rbacpb.SubjectType_ACCOUNT, "read", "/cluster/config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAccess {
		t.Error("sa should have access when no explicit deny exists")
	}
	if accessDenied {
		t.Error("accessDenied should be false when sa is not denied")
	}
}

// TestSAAllowedWhenNoPermissionsExist verifies that sa works even
// when no permissions record exists for the path at all (bootstrap).
func TestSAAllowedWhenNoPermissionsExist(t *testing.T) {
	srv := newDenyTestServer(t)

	// No permissions seeded — simulates a fresh path during bootstrap.
	hasAccess, accessDenied, err := srv.validateAccess(
		"sa", rbacpb.SubjectType_ACCOUNT, "write", "/bootstrap/new-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAccess {
		t.Error("sa should have access when no permissions record exists")
	}
	if accessDenied {
		t.Error("accessDenied should be false when no deny record exists")
	}
}

// TestDenyOverridesOwnerAllow verifies that an explicit deny on a path
// blocks an owner even though owners normally have full access.
// Since owner validation requires the resource service for non-sa accounts,
// we test this by setting up sa as an owner AND explicitly denied.
// The deny must win over ownership.
func TestDenyOverridesOwnerAllow(t *testing.T) {
	srv := newDenyTestServer(t)

	seedPermissions(t, srv, "/data/sensitive", &rbacpb.Permissions{
		Owners: &rbacpb.Permission{
			Accounts: []string{"sa"},
		},
		Denied: []*rbacpb.Permission{
			{
				Name:     "write",
				Accounts: []string{"sa"},
			},
		},
	})

	hasAccess, accessDenied, err := srv.validateAccess(
		"sa", rbacpb.SubjectType_ACCOUNT, "write", "/data/sensitive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("owner should NOT have access when explicitly denied")
	}
	if !accessDenied {
		t.Error("accessDenied should be true when owner is explicitly denied")
	}
}

// TestSADenyOnParentPath verifies that deny rules on ancestor paths
// propagate correctly and still block sa.
func TestSADenyOnParentPath(t *testing.T) {
	srv := newDenyTestServer(t)

	// Deny at a parent path — validateAccessDenied walks up the path tree.
	seedPermissions(t, srv, "/restricted", &rbacpb.Permissions{
		Denied: []*rbacpb.Permission{
			{
				Name:     "read",
				Accounts: []string{"sa"},
			},
		},
	})

	hasAccess, accessDenied, err := srv.validateAccess(
		"sa", rbacpb.SubjectType_ACCOUNT, "read", "/restricted/subdir/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("sa should be denied when ancestor path has explicit deny")
	}
	if !accessDenied {
		t.Error("accessDenied should be true when ancestor deny applies")
	}
}

// TestSALegacyDomainFormat verifies deny works for "sa@domain" format.
func TestSALegacyDomainFormat(t *testing.T) {
	srv := newDenyTestServer(t)

	seedPermissions(t, srv, "/legacy/path", &rbacpb.Permissions{
		Denied: []*rbacpb.Permission{
			{
				Name:     "delete",
				Accounts: []string{"sa"},
			},
		},
	})

	hasAccess, accessDenied, err := srv.validateAccess(
		"sa@test.local", rbacpb.SubjectType_ACCOUNT, "delete", "/legacy/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasAccess {
		t.Error("sa@domain should be denied when sa is explicitly denied")
	}
	if !accessDenied {
		t.Error("accessDenied should be true for sa@domain format")
	}
}

// TestSADifferentPermissionNotDenied verifies that a deny on one
// permission does not affect a different permission for sa.
func TestSADifferentPermissionNotDenied(t *testing.T) {
	srv := newDenyTestServer(t)

	seedPermissions(t, srv, "/selective/path", &rbacpb.Permissions{
		Denied: []*rbacpb.Permission{
			{
				Name:     "delete",
				Accounts: []string{"sa"},
			},
		},
	})

	// "read" is NOT denied, only "delete" is.
	hasAccess, accessDenied, err := srv.validateAccess(
		"sa", rbacpb.SubjectType_ACCOUNT, "read", "/selective/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasAccess {
		t.Error("sa should have read access when only delete is denied")
	}
	if accessDenied {
		t.Error("accessDenied should be false for non-denied permission")
	}
}
