package resource_client

import (
	"fmt"
	"log"
	"testing"
	"time"

	authn "github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/testutil"
)

// ---------- Test harness ----------

// testEnv centralizes shared test state and helper methods.
type testEnv struct {
	Domain string
	Client *Resource_Client
	Auth   *authn.Authentication_Client
	Token  string

	Suffix     string
	Org        string
	Account    string
	Group      string
	Role       string
	RoleAction string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	testutil.SkipIfNoExternalServices(t)

	domain := testutil.GetDomain()
	address := testutil.GetAddress()

	client, err := NewResourceService_Client(address, "resource.ResourceService")
	if err != nil {
		t.Fatalf("resource client: %v", err)
	}

	auth, err := authn.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
	if err != nil {
		t.Fatalf("auth client: %v", err)
	}

	// Credentials from environment or defaults.
	saUser, saPass := testutil.GetSACredentials()
	token, err := auth.Authenticate(saUser, saPass)
	if err != nil {
		t.Fatalf("authenticate sa: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().Unix())

	return &testEnv{
		Domain:     domain,
		Client:     client,
		Auth:       auth,
		Token:      token,
		Suffix:     suffix,
		Org:        "org_" + suffix,
		Account:    "acct_" + suffix,
		Group:      "grp_" + suffix,
		Role:       "db_user_" + suffix,
		RoleAction: "/file.FileService/ReadDir",
	}
}

func mustNoErr(t *testing.T, step string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", step, err)
	}
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// waitForRoleAction polls GetRoles a few times to confirm an action is visible.
func waitForRoleAction(c *Resource_Client, roleID, action string, tries int, sleep time.Duration) (bool, error) {
	for range tries {
		roles, err := c.GetRoles(fmt.Sprintf(`{"_id":"%s"}`, roleID))
		if err != nil {
			return false, err
		}
		if len(roles) == 1 && contains(roles[0].Actions, action) {
			return true, nil
		}
		time.Sleep(sleep)
	}
	return false, nil
}

// ---------- End-to-end happy path ----------

func TestResourceServiceLifecycle(t *testing.T) {

	env := newTestEnv(t)
	c := env.Client

	
	// Create the account
	t.Run("RegisterAccount", func(t *testing.T) {
		// Create account first – many ops depend on it.
		err := c.RegisterAccount(env.Domain, env.Account, env.Account, env.Account+"@example.com", "1234", "1234")
		mustNoErr(t, "RegisterAccount", err)
		log.Println("account created:", env.Account)
	})

	t.Run("CreateOrganizationAndGroup", func(t *testing.T) {
		mustNoErr(t, "CreateOrganization",
			c.CreateOrganization(env.Token, env.Org, env.Org, env.Org+"@"+env.Domain, "test org", ""),
		)
		mustNoErr(t, "CreateGroup",
			c.CreateGroup(env.Token, env.Group, env.Group, "test group"),
		)
	})

	t.Run("CreateRole", func(t *testing.T) {
		// Do not assume the server keeps initial actions from CreateRole.
		mustNoErr(t, "CreateRole",
			c.CreateRole(env.Token, env.Role, env.Role, []string{}),
		)
	})

	t.Run("AddRoleActionsAndAssignToAccount", func(t *testing.T) {
		// Explicitly add the action we intend to remove later.
		mustNoErr(t, "AddRoleActions", c.AddRoleActions(env.Token, env.Role, []string{env.RoleAction}))

		// Assign the role to the account.
		mustNoErr(t, "AddAccountRole", c.AddAccountRole(env.Token, env.Account, env.Role))
	})

	// sanity check: role actually has the action we’ll remove (poll briefly to absorb store lag)
	t.Run("VerifyRoleActionPresent", func(t *testing.T) {
		ok, err := waitForRoleAction(c, env.Role, env.RoleAction, 5, 120*time.Millisecond)
		mustNoErr(t, "GetRoles", err)
		if !ok {
			t.Fatalf("role %s missing expected action %s", env.Role, env.RoleAction)
		}
	})

	t.Run("WireUpOrganization", func(t *testing.T) {
		mustNoErr(t, "AddOrganizationAccount", c.AddOrganizationAccount(env.Token, env.Org, env.Account))
		mustNoErr(t, "AddOrganizationGroup", c.AddOrganizationGroup(env.Token, env.Org, env.Group))
		mustNoErr(t, "AddOrganizationRole", c.AddOrganizationRole(env.Token, env.Org, env.Role))
	})

	t.Run("GroupMembership", func(t *testing.T) {
		mustNoErr(t, "AddGroupMemberAccount", c.AddGroupMemberAccount(env.Token, env.Group, env.Account))
		// idempotency: add twice should not hard-fail (server may ignore/allow)
		_ = c.AddGroupMemberAccount(env.Token, env.Group, env.Account)
	})

	t.Run("RemoveRoleAction", func(t *testing.T) {
		// Double-check presence right before removal to avoid noisy false negatives.
		ok, err := waitForRoleAction(c, env.Role, env.RoleAction, 3, 100*time.Millisecond)
		mustNoErr(t, "GetRoles(recheck)", err)
		if !ok {
			t.Skipf("skipping RemoveRoleAction: role %s still missing action %s", env.Role, env.RoleAction)
			return
		}
		mustNoErr(t, "RemoveRoleAction", c.RemoveRoleAction(env.Token, env.Role, env.RoleAction))
	})

	// ---------- Cleanup in reverse order ----------
	t.Run("Cleanup", func(t *testing.T) {
		// best-effort cleanups – ignore errors where safe
		_ = c.RemoveGroupMemberAccount(env.Token, env.Group, env.Account)
		_ = c.RemoveOrganizationAccount(env.Token, env.Org, env.Account)
		_ = c.RemoveOrganizationRole(env.Token, env.Org, env.Role)
		_ = c.RemoveOrganizationGroup(env.Token, env.Org, env.Group)
		_ = c.RemoveAccountRole(env.Token, env.Account, env.Role)

		_ = c.DeleteGroup(env.Token, env.Group)
		_ = c.DeleteRole(env.Token, env.Role)
		_ = c.DeleteAccount(env.Token, env.Account)
		_ = c.DeleteOrganization(env.Token, env.Org)
	})
}

// ---------- Focused unit-style checks (extra coverage) ----------

func TestRoleCRUD(t *testing.T) {
	env := newTestEnv(t)
	c := env.Client
	role := env.Role

	mustNoErr(t, "CreateRole", c.CreateRole(env.Token, role, role, nil))

	// Add action twice (should be idempotent or return a clear error).
	mustNoErr(t, "AddRoleActions:first", c.AddRoleActions(env.Token, role, []string{env.RoleAction}))
	_ = c.AddRoleActions(env.Token, role, []string{env.RoleAction})

	// Remove, then removing again should be safe to call and return a clear error or no-op.
	// If the first remove fails because the action didn't stick, it's a server-side issue.
	mustNoErr(t, "RemoveRoleAction:first", c.RemoveRoleAction(env.Token, role, env.RoleAction))
	_ = c.RemoveRoleAction(env.Token, role, env.RoleAction)

	mustNoErr(t, "DeleteRole", c.DeleteRole(env.Token, role))
}

func TestGroupMembershipLifecycle(t *testing.T) {
	env := newTestEnv(t)
	c := env.Client

	mustNoErr(t, "RegisterAccount",
		c.RegisterAccount(env.Domain, env.Account, env.Account, env.Account+"@example.com", "1234", "1234"),
	)
	mustNoErr(t, "CreateGroup", c.CreateGroup(env.Token, env.Group, env.Group, "desc"))

	mustNoErr(t, "AddGroupMemberAccount", c.AddGroupMemberAccount(env.Token, env.Group, env.Account))
	_ = c.AddGroupMemberAccount(env.Token, env.Group, env.Account) // idempotency

	mustNoErr(t, "RemoveGroupMemberAccount", c.RemoveGroupMemberAccount(env.Token, env.Group, env.Account))
	_ = c.DeleteGroup(env.Token, env.Group)
	_ = c.DeleteAccount(env.Account, env.Token)
}

func TestOrganizationLifecycle(t *testing.T) {
	env := newTestEnv(t)
	c := env.Client

	mustNoErr(t, "RegisterAccount",
		c.RegisterAccount(env.Domain, env.Account, env.Account, env.Account+"@example.com", "1234", "1234"),
	)
	mustNoErr(t, "CreateOrganization",
		c.CreateOrganization(env.Token, env.Org, env.Org, env.Org+"@"+env.Domain, "desc", ""),
	)
	mustNoErr(t, "CreateGroup", c.CreateGroup(env.Token, env.Group, env.Group, "desc"))
	mustNoErr(t, "CreateRole", c.CreateRole(env.Token, env.Role, env.Role, nil))

	mustNoErr(t, "AddOrganizationAccount", c.AddOrganizationAccount(env.Token, env.Org, env.Account))
	mustNoErr(t, "AddOrganizationGroup", c.AddOrganizationGroup(env.Token, env.Org, env.Group))
	mustNoErr(t, "AddOrganizationRole", c.AddOrganizationRole(env.Token, env.Org, env.Role))

	mustNoErr(t, "RemoveOrganizationRole", c.RemoveOrganizationRole(env.Token, env.Org, env.Role))
	mustNoErr(t, "RemoveOrganizationGroup", c.RemoveOrganizationGroup(env.Token, env.Org, env.Group))
	mustNoErr(t, "RemoveOrganizationAccount", c.RemoveOrganizationAccount(env.Token, env.Org, env.Account))

	_ = c.DeleteRole(env.Token, env.Role)
	_ = c.DeleteGroup(env.Token, env.Group)
	_ = c.DeleteOrganization(env.Token, env.Org)
	_ = c.DeleteAccount(env.Account, env.Token)
}
