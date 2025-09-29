package rbac_client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
)

// -----------------------------------------------------------------------------
// Clients & helpers
// -----------------------------------------------------------------------------

type clients struct {
	rbac     *Rbac_Client
	resource *resource_client.Resource_Client
	auth     *authentication_client.Authentication_Client
	domain   string
}

func mustNoErr(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

func mustClients(t *testing.T) clients {
	t.Helper()

	domain := "globular.io"
	address := "globule-ryzen.globular.io"

	rbacCli, err := NewRbacService_Client(address, "rbac.RbacService")
	if err != nil {
		t.Fatalf("create RBAC client (%s): %v", address, err)
	}
	resCli, err := resource_client.NewResourceService_Client(address, "resource.ResourceService")
	if err != nil {
		t.Fatalf("create Resource client (%s): %v", address, err)
	}
	authCli, err := authentication_client.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
	if err != nil {
		t.Fatalf("create Authentication client (%s): %v", address, err)
	}

	return clients{rbac: rbacCli, resource: resCli, auth: authCli, domain: domain}
}

func mustAuthSA(t *testing.T, c clients) string {
	t.Helper()
	token, err := c.auth.Authenticate("sa", "adminadmin")
	mustNoErr(t, err, "authenticate sa")
	return token
}

func makeTempFile(t *testing.T, contents string) (path string, uri string) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "toto.txt")
	mustNoErr(t, os.WriteFile(path, []byte(contents), 0o666), "write temp file")
	return path, "file:" + path
}

func ignoreExists(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "exist") || strings.Contains(s, "already")
}

func ensureAccount(t *testing.T, c clients, id string) {
	t.Helper()
	if err := c.resource.RegisterAccount(c.domain, id, id+" name", id+"@test.com", "1234", "1234"); err != nil && !ignoreExists(err) {
		t.Fatalf("ensureAccount(%s): %v", id, err)
	}
}

func ensureGroup(t *testing.T, c clients, token, id, name string) {
	t.Helper()
	if err := c.resource.CreateGroup(token, id, name, "test"); err != nil && !ignoreExists(err) {
		t.Fatalf("ensureGroup(%s): %v", id, err)
	}
}

func ensureOrganization(t *testing.T, c clients, token, id, name, email string) {
	t.Helper()
	if err := c.resource.CreateOrganization(token, id, name, email, "test", "test"); err != nil && !ignoreExists(err) {
		t.Fatalf("ensureOrganization(%s): %v", id, err)
	}
}

func ensureGroupMemberAccount(t *testing.T, c clients, token, groupID, accountID string) {
	t.Helper()
	if err := c.resource.AddGroupMemberAccount(token, groupID, accountID); err != nil && !ignoreExists(err) {
		t.Fatalf("ensureGroupMemberAccount(%s,%s): %v", groupID, accountID, err)
	}
}

func ensureOrganizationGroup(t *testing.T, c clients, token, orgID, groupID string) {
	t.Helper()
	if err := c.resource.AddOrganizationGroup(token, orgID, groupID); err != nil && !ignoreExists(err) {
		t.Fatalf("ensureOrganizationGroup(%s,%s): %v", orgID, groupID, err)
	}
}

func fqAccount(c clients, id string) string { return id + "@" + c.domain }

// -----------------------------------------------------------------------------
// Suite
// -----------------------------------------------------------------------------

func Test_RBAC_Suite(t *testing.T) {
	c := mustClients(t)
	token := mustAuthSA(t, c)

	t.Run("01_SetupResources", func(t *testing.T) {
		// Orgs
		ensureOrganization(t, c, token, "organization_0", "Organization 0", "organization_0@test.com")
		ensureOrganization(t, c, token, "organization_1", "Organization 1", "organization_1@test.com")
		// Groups
		ensureGroup(t, c, token, "group_0", "Group 0")
		ensureGroup(t, c, token, "group_1", "Group 1")
		// Accounts
		ensureAccount(t, c, "account_0")
		ensureAccount(t, c, "account_1")
		ensureAccount(t, c, "account_2")
		// Memberships
		ensureGroupMemberAccount(t, c, token, "group_0", "account_0")
		ensureGroupMemberAccount(t, c, token, "group_1", "account_1")
		// Group → Organization
		ensureOrganizationGroup(t, c, token, "organization_0", "group_0")
		ensureOrganizationGroup(t, c, token, "organization_1", "group_1")
	})

	t.Run("02_SetResourcePermissions", func(t *testing.T) {
		_, fileURI := makeTempFile(t, "La vie ne vaut rien, mais rien ne vaut la vie!")

		perms := &rbacpb.Permissions{
			Allowed: []*rbacpb.Permission{
				{
					Name:          "read",
					Accounts:      []string{fqAccount(c, "account_0"), fqAccount(c, "account_1")},
					Groups:        []string{"group_0", "group_1"},
					Peers:         []string{"p0.test.com", "p1.test.com"},
					Organizations: []string{"organization_0", "organization_1"},
				},
				{Name: "write", Accounts: []string{fqAccount(c, "account_0")}},
				{Name: "execute", Accounts: []string{fqAccount(c, "account_1")}},
				{Name: "delete", Accounts: []string{fqAccount(c, "account_0"), fqAccount(c, "account_1")}},
			},
			Denied: []*rbacpb.Permission{
				{Name: "read", Accounts: []string{fqAccount(c, "account_2")}},
				{Name: "delete", Groups: []string{"group_1"}, Organizations: []string{"organization_1"}},
			},
			Owners: &rbacpb.Permission{Name: "owner", Accounts: []string{fqAccount(c, "account_0")}},
		}

		mustNoErr(t, c.rbac.SetResourcePermissions(token, fileURI, "file", perms), "SetResourcePermissions")
	})

	t.Run("03_GetResourcePermission", func(t *testing.T) {
		ensureAccount(t, c, "account_0")

		_, fileURI := makeTempFile(t, "abc")
		mustNoErr(t, c.rbac.SetResourcePermissions(token, fileURI, "file", &rbacpb.Permissions{}), "ensure resource exists")

		mustNoErr(t,
			c.rbac.SetResourcePermission(
				token,
				fileURI,
				"file",
				&rbacpb.Permission{Name: "read", Accounts: []string{fqAccount(c, "account_0")}},
				rbacpb.PermissionType_ALLOWED,
			),
			"SetResourcePermission(read)",
		)

		_, err := c.rbac.GetResourcePermission(fileURI, "read", rbacpb.PermissionType_ALLOWED)
		mustNoErr(t, err, "GetResourcePermission(read)")
	})

	t.Run("04_ValidateAccess", func(t *testing.T) {
		// Ensure fixtures relevant to group/org-based rules
		ensureAccount(t, c, "account_0")
		ensureAccount(t, c, "account_1")
		ensureAccount(t, c, "account_2")
		ensureGroup(t, c, token, "group_1", "Group 1")
		ensureOrganization(t, c, token, "organization_1", "Organization 1", "organization_1@test.com")
		ensureGroupMemberAccount(t, c, token, "group_1", "account_1")
		ensureOrganizationGroup(t, c, token, "organization_1", "group_1")

		_, fileURI := makeTempFile(t, "xyz")

		perms := &rbacpb.Permissions{
			Allowed: []*rbacpb.Permission{
				{Name: "read", Accounts: []string{fqAccount(c, "account_0"), fqAccount(c, "account_1")}},
				{Name: "write", Accounts: []string{fqAccount(c, "account_0")}},
				{Name: "execute", Accounts: []string{fqAccount(c, "account_1")}},
				{Name: "delete", Accounts: []string{fqAccount(c, "account_0"), fqAccount(c, "account_1")}},
			},
			Denied: []*rbacpb.Permission{
				{Name: "read", Accounts: []string{fqAccount(c, "account_2")}},
				{Name: "delete", Groups: []string{"group_1"}, Organizations: []string{"organization_1"}},
			},
			Owners: &rbacpb.Permission{Name: "owner", Accounts: []string{fqAccount(c, "account_0")}},
		}
		mustNoErr(t, c.rbac.SetResourcePermissions(token, fileURI, "file", perms), "seed perms")

		type check struct {
			subject string
			action  string
			want    bool
		}
		cases := []check{
			{fqAccount(c, "account_0"), "read", true},
			{fqAccount(c, "account_1"), "read", true},
			{fqAccount(c, "account_1"), "write", false},
			{fqAccount(c, "account_2"), "read", false},
			{fqAccount(c, "account_0"), "delete", true},
			{fqAccount(c, "account_1"), "delete", false}, // denied by group_1/organization_1
		}

		validate := func(subj, action string) bool {
			got, _, err := c.rbac.ValidateAccess(subj, rbacpb.SubjectType_ACCOUNT, action, fileURI)
			mustNoErr(t, err, "ValidateAccess")
			return got
		}

		for _, tc := range cases {
			got := validate(tc.subject, tc.action)
			if got != tc.want {
				t.Fatalf("ValidateAccess(%s, %s) = %v; want %v", tc.subject, tc.action, got, tc.want)
			}
		}

		// Remove owner; owner-derived rights should drop
		mustNoErr(t, c.rbac.RemoveResourceOwner(token, fileURI, fqAccount(c, "account_0"), rbacpb.SubjectType_ACCOUNT), "RemoveResourceOwner(account_0)")
		if got := validate(fqAccount(c, "account_0"), "execute"); got {
			t.Fatalf("expected account_0 cannot execute after owner removal")
		}

		// Restore owner using correct argument order
		mustNoErr(t, c.rbac.AddResourceOwner(token, fileURI, fqAccount(c, "account_0"), "file", rbacpb.SubjectType_ACCOUNT), "AddResourceOwner(account_0)")
		if got := validate(fqAccount(c, "account_0"), "execute"); !got {
			t.Fatalf("expected account_0 can execute after owner restored")
		}
	})

	t.Run("05_DeleteSinglePermission_Effect", func(t *testing.T) {
		ensureAccount(t, c, "account_1")

		_, fileURI := makeTempFile(t, "exec")

		mustNoErr(t, c.rbac.SetResourcePermission(
			token,
			fileURI,
			"file",
			&rbacpb.Permission{Name: "execute", Accounts: []string{fqAccount(c, "account_1")}},
			rbacpb.PermissionType_ALLOWED,
		), "SetResourcePermission(execute)")

		got, _, err := c.rbac.ValidateAccess(fqAccount(c, "account_1"), rbacpb.SubjectType_ACCOUNT, "execute", fileURI)
		mustNoErr(t, err, "ValidateAccess execute before delete")
		if !got {
			t.Fatalf("expected account_1 can execute before delete")
		}

		mustNoErr(t, c.rbac.DeleteResourcePermission(token, fileURI, "execute", rbacpb.PermissionType_ALLOWED), "DeleteResourcePermission(execute)")

		got, _, err = c.rbac.ValidateAccess(fqAccount(c, "account_1"), rbacpb.SubjectType_ACCOUNT, "execute", fileURI)
		mustNoErr(t, err, "ValidateAccess execute after delete")
		if got {
			t.Fatalf("expected account_1 cannot execute after delete")
		}
	})

	t.Run("06_DeleteAllAccess", func(t *testing.T) {
		ensureAccount(t, c, "account_0")

		_, fileURI := makeTempFile(t, "delall")

		perms := &rbacpb.Permissions{
			Allowed: []*rbacpb.Permission{
				{Name: "delete", Accounts: []string{fqAccount(c, "account_0")}},
			},
			Owners: &rbacpb.Permission{Name: "owner", Accounts: []string{fqAccount(c, "account_0")}},
		}
		mustNoErr(t, c.rbac.SetResourcePermissions(token, fileURI, "file", perms), "seed perms")

		got, _, err := c.rbac.ValidateAccess(fqAccount(c, "account_0"), rbacpb.SubjectType_ACCOUNT, "delete", fileURI)
		mustNoErr(t, err, "ValidateAccess delete (before)")
		if !got {
			t.Fatalf("expected owner can delete before DeleteAllAccess")
		}

		mustNoErr(t, c.rbac.DeleteAllAccess(token, fqAccount(c, "account_0"), rbacpb.SubjectType_ACCOUNT), "DeleteAllAccess(account_0)")

		got, _, err = c.rbac.ValidateAccess(fqAccount(c, "account_0"), rbacpb.SubjectType_ACCOUNT, "delete", fileURI)
		mustNoErr(t, err, "ValidateAccess delete (after)")
		if !got {
			t.Fatalf("expected owner can still delete after DeleteAllAccess (implicit owner rights)")
		}
	})

	t.Run("07_DeleteResourcePermissions", func(t *testing.T) {
		ensureAccount(t, c, "account_0")

		_, fileURI := makeTempFile(t, "cleanup")

		mustNoErr(t, c.rbac.SetResourcePermission(
			token, fileURI,"file",
			&rbacpb.Permission{Name: "read", Accounts: []string{fqAccount(c, "account_0")}},
			rbacpb.PermissionType_ALLOWED,
		), "seed read perm")

		mustNoErr(t, c.rbac.DeleteResourcePermissions(token, fileURI), "DeleteResourcePermissions(resource)")
	})

	t.Run("08_ResetResources", func(t *testing.T) {
		// Best-effort cleanup (ignore "not found")
		for _, grp := range []string{"group_0", "group_1"} {
			_ = c.resource.DeleteGroup(token, grp)
		}
		for _, org := range []string{"organization_0", "organization_1"} {
			_ = c.resource.DeleteOrganization(token, org)
		}
		for _, acc := range []string{"account_0", "account_1", "account_2"} {
			_ = c.resource.DeleteAccount(token, acc)
		}
	})
		
}
