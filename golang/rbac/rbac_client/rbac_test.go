package rbac_client

import (
	"log"
	"os"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	Utility "github.com/globulario/utility"
)

var (
	// Connect to the services client.
	domain                  = "localhost"
	rbac_client_, _         = NewRbacService_Client(domain, "rbac.RbacService")
	resource_client_, _     = resource_client.NewResourceService_Client(domain, "resource.ResourceService")
	authencation_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
)

/** Create All resource to be use to test permission **/
func TestSetResources(t *testing.T) {

	// authenticate as admin
	token, err := authencation_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	/** Create organization **/
	err = resource_client_.CreateOrganization(token, "organization_0", "Organization 0", "organization_0@test.com", "test", "test")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("organization_0 was created successfully!")

	err = resource_client_.CreateOrganization(token, "organization_1", "Organization 1", "organization_1@test.com", "test", "test")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("organization_1 was created successfully!")

	/** Create group */
	err = resource_client_.CreateGroup(token, "group_0", "Group 0", "test")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("group_0 was created successfully!")

	err = resource_client_.CreateGroup(token, "group_1", "Group 1", "test")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("group_1 was created successfully!")

	/** Create an account */
	err = resource_client_.RegisterAccount(domain, "account_0", "account 0", "account_0@test.com", "1234", "1234")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("account_0 was created successfully!")

	err = resource_client_.RegisterAccount(domain, "account_1", "account 1", "account_1@test.com", "1234", "1234")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("account_1 was created successfully!")

	err = resource_client_.RegisterAccount(domain, "account_2", "account 2", "account_2@test.com", "1234", "1234")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("account_2 was created successfully!")

	// Now set the account to the group
	err = resource_client_.AddGroupMemberAccount(token, "group_0", "account_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("account_0 was added to group_0 successfully!")

	err = resource_client_.AddGroupMemberAccount(token, "group_1", "account_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("account_1 was added to group_1 successfully!")

	// Add the group to the organization.
	err = resource_client_.AddOrganizationGroup(token, "organization_0", "group_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("group_0 was added to organization_0 successfully!")

	err = resource_client_.AddOrganizationGroup(token, "organization_1", "group_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	log.Println("group_1 was added to organization_1 created successfully!")

	// Now create a peer
	/*
		_, _, err = resource_client_.RegisterPeer("", "p0.test.com")
		if err != nil {
			log.Println(err)
			t.Fail()
		}
		log.Println("p0.test.com was created successfully!")

		// Now create a peer
		err = resource_client_.RegisterPeer("", "p1.test.com")
		if err != nil {
			log.Println(err)
			t.Fail()
		}
		log.Println("p1.test.com was created successfully!")
	*/

}

func TestSetResourcePermissions(t *testing.T) {

	// Here I will create a file and set permission on it.
	// authenticate as admin
	token, err := authencation_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// A fictive file path...
	filePath := "/tmp/toto.txt"
	if Utility.Exists(filePath) {
		os.Remove(filePath)
	}
	err = os.WriteFile(filePath, []byte("La vie ne vaut rien, mais rien ne vaut la vie!"), 0777)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	permissions := &rbacpb.Permissions{
		Allowed: []*rbacpb.Permission{
			{
				Name:          "read",
				Applications:  []string{},
				Accounts:      []string{"account_0", "account_1"},
				Groups:        []string{"group_0", "group_1"},
				Peers:         []string{"p0.test.com", "p1.test.com"},
				Organizations: []string{"organization_0", "organization_1"},
			},
			{
				Name:          "write",
				Applications:  []string{},
				Accounts:      []string{"account_0"},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
			{
				Name:          "execute",
				Applications:  []string{},
				Accounts:      []string{"account_1"},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
			{
				Name:          "delete",
				Applications:  []string{},
				Accounts:      []string{"account_0", "account_1"}, // must not work because of organization_0 is in the list of denied...
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
		},
		Denied: []*rbacpb.Permission{
			{
				Name:          "read",
				Applications:  []string{},
				Accounts:      []string{"account_2"},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
			{
				Name:          "delete",
				Applications:  []string{},
				Accounts:      []string{},
				Groups:        []string{"group_1"},
				Peers:         []string{},
				Organizations: []string{"organization_1"},
			},
		},
		Owners: &rbacpb.Permission{
			Name:          "owner", // The name is informative in that particular case.
			Applications:  []string{},
			Accounts:      []string{"account_0"},
			Groups:        []string{},
			Peers:         []string{},
			Organizations: []string{},
		},
	}

	err = rbac_client_.SetResourcePermissions(token, filePath, "file", permissions)
	if err != nil {
		log.Println(err)
	}

}

// Test read a given permission to determine if suject can do given action...
func TestGetResourcePermission(t *testing.T) {

	filePath := "/tmp/toto.txt"
	_, err := rbac_client_.GetResourcePermission(filePath, "read", rbacpb.PermissionType_ALLOWED)
	if err != nil {
		log.Println(err)
	}

}

func TestSetResourcePermission(t *testing.T) {
	filePath := "file:/tmp/toto.txt"
	err := rbac_client_.DeleteResourcePermission(filePath, "execute", rbacpb.PermissionType_ALLOWED)
	if err != nil {
		log.Println(err)
	}
}

func TestValidateAccess(t *testing.T) {
	filePath := "file:/tmp/toto.txt"

	// Test if account owner can do anything.
	hasPermission_0, _, err := rbac_client_.ValidateAccess("account_0", rbacpb.SubjectType_ACCOUNT, "read", filePath)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	if hasPermission_0 {
		log.Println("account_0 has the permission to read " + filePath)
	} else {
		log.Println("account_0 has not the permission to read " + filePath)
		t.Fail()
	}

	// Now I will test remove the resource owner and play the same action again.
	err = rbac_client_.RemoveResourceOwner(filePath, "account_0", rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Test if the owner has permission event the permission is explicitly specify!
	hasPermission_3, _, err := rbac_client_.ValidateAccess("account_0", rbacpb.SubjectType_ACCOUNT, "execute", filePath)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	if hasPermission_3 {
		log.Println("account_0 has the permission to execute " + filePath)
		t.Fail()
	} else {
		log.Println("account_0 has not the permission to execute " + filePath)

	}

	// Put back account_0 in list of owners
	err = rbac_client_.AddResourceOwner(filePath, "account_0", "file", rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	hasPermission_3, _, err = rbac_client_.ValidateAccess("account_0", rbacpb.SubjectType_ACCOUNT, "execute", filePath)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	if hasPermission_3 {
		log.Println("account_0 has the permission to execute " + filePath)
	} else {
		log.Println("account_0 has not the permission to execute " + filePath)
		t.Fail()
	}

	// Test get permission without being the owner.
	hasPermission_1, _, err := rbac_client_.ValidateAccess("account_1", rbacpb.SubjectType_ACCOUNT, "read", filePath)
	if err != nil {
		log.Println(err)
	}
	if hasPermission_1 {
		log.Println("account_1 has the permission to read " + filePath)
	} else {
		log.Println("account_1 has not the permission to read " + filePath)
		t.Fail()
	}

	// Test not having permission whitout being the owner.
	hasPermission_2, _, err := rbac_client_.ValidateAccess("account_1", rbacpb.SubjectType_ACCOUNT, "write", filePath)
	if err != nil {
		log.Println(err)
	}
	if hasPermission_2 {
		log.Println("account_1 has the permission to write " + filePath)
	} else {
		log.Println("account_1 has not the permission to write " + filePath)
		t.Fail()
	}

	// Test having permission denied.
	hasPermission_4, _, err := rbac_client_.ValidateAccess("account_2", rbacpb.SubjectType_ACCOUNT, "read", filePath)
	if err != nil {
		log.Println(err)
	}

	if !hasPermission_4 {
		log.Println("account_2 has permission denied to read " + filePath)
	} else {
		log.Println("account_2 can read  " + filePath)
		t.Fail()
	}

	// Test permission denied for orgnization...
	hasPermission_5, _, err := rbac_client_.ValidateAccess("account_0", rbacpb.SubjectType_ACCOUNT, "delete", filePath)
	if err != nil {
		log.Println(err)
	}

	// Here the owner write beat the denied permission.
	if !hasPermission_5 {
		log.Println("account_0 has permission denied to delete " + filePath)
		t.Fail()
	} else {
		log.Println("account_0 can delete  " + filePath)
	}

	// Test permission denied because of account is in denied organization.
	hasPermission_6, _, err := rbac_client_.ValidateAccess("account_1", rbacpb.SubjectType_ACCOUNT, "delete", filePath)
	if err != nil {
		log.Println(err)
	}

	// Here the owner write beat the denied permission.
	if !hasPermission_6 {
		log.Println("account_1 has permission denied to delete " + filePath)

	} else {
		log.Println("account_1 can delete  " + filePath)
		t.Fail()
	}

	// Now I will try to delete one permission...
	err = rbac_client_.DeleteResourcePermission(filePath, "execute", rbacpb.PermissionType_ALLOWED)
	if err != nil {
		log.Println(err)
	}
	hasPermission_3, _, err = rbac_client_.ValidateAccess("account_1", rbacpb.SubjectType_ACCOUNT, "execute", filePath)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	if hasPermission_3 {
		log.Println("account_1 has the permission to execute " + filePath)
		t.Fail()
	} else {
		log.Println("account_1 has not the permission to execute " + filePath)

	}

	// Here I will try to remove all access ...
	err = rbac_client_.DeleteAllAccess("account_0", rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		log.Println(err)
	}

	hasPermission_6, _, err = rbac_client_.ValidateAccess("account_0", rbacpb.SubjectType_ACCOUNT, "delete", filePath)
	if err != nil {
		log.Println(err)
	}

	// Here the owner write beat the denied permission.
	if !hasPermission_6 {
		log.Println("account_0 dosen't has the permission to delete " + filePath)
		t.Fail()
	} else {
		log.Println("account_0 can delete  " + filePath)
	}
}

// Test delete a specific resource permission...
func TestDeleteResourcePermissions(t *testing.T) {
	filePath := "/tmp/toto.txt"

	err := rbac_client_.DeleteResourcePermissions(filePath)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
}

func TestResetResources(t *testing.T) {
	// Here I will create a file and set permission on it.
	// authenticate as admin
	token, err := authencation_client_.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeleteAccount("account_0", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the group
	err = resource_client_.DeleteGroup("group_0", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the organization.
	err = resource_client_.DeleteOrganization("organization_0", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeleteAccount("account_1", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeleteAccount("account_2", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the group
	err = resource_client_.DeleteGroup("group_1", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the organization.
	err = resource_client_.DeleteOrganization("organization_1", token)
	if err != nil {
		log.Println(err)
		t.Fail()
	}
	/*
		err = resource_client_.DeletePeer("", "p1.test.com")
		if err != nil {
			log.Println(err)
			t.Fail()
		}


		err = resource_client_.DeletePeer("", "p0.test.com")
		if err != nil {
			log.Println(err)
			t.Fail()
		}
	*/

}
