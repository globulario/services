package rbac_client

import (
	"log"
	"testing"

	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
)

var (
	// Connect to the services client.
	rbac_client_, _ = NewRbacService_Client("localhost", "resource.RbacService")

	resource_client_, _ = resource_client.NewResourceService_Client("localhost", "resource.ResourceService")
)

/** Create All ressource to be use to test permission **/
func TestSetResources(t *testing.T) {
	log.Println("----> Create resources...")
	/** Create organization **/
	err := resource_client_.CreateOrganization("organization_0", "Organization 0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.CreateOrganization("organization_1", "Organization 1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	/** Create group */
	err = resource_client_.CreateGroup("group_0", "Group 0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.CreateGroup("group_1", "Group 1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	/** Create an account */
	err = resource_client_.RegisterAccount("account_0", "account_0@test.com", "1234", "1234")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.RegisterAccount("account_1", "account_1@test.com", "1234", "1234")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.RegisterAccount("account_2", "account_2@test.com", "1234", "1234")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Now set the account to the group
	err = resource_client_.AddGroupMemberAccount("group_0", "account_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.AddGroupMemberAccount("group_1", "account_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Add the group to the organization.
	err = resource_client_.AddOrganizationGroup("organization_0", "group_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.AddOrganizationGroup("organization_1", "group_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Now create a peer
	err = resource_client_.RegisterPeer("p0.test.com")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Now create a peer
	err = resource_client_.RegisterPeer("p1.test.com")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

}

func TestSetResourcePermissions(t *testing.T) {

	// A fictive file path...
	filePath := "C:/temp/toto.txt"

	permissions := &resourcepb.Permissions{
		Allowed: []*resourcepb.Permission{
			&resourcepb.Permission{
				Name:          "read",
				Applications:  []string{},
				Accounts:      []string{"account_0", "account_1"},
				Groups:        []string{"group_0", "group_1"},
				Peers:         []string{"p0.test.com", "p1.test.com"},
				Organizations: []string{"organization_0", "organization_1"},
			},
			&resourcepb.Permission{
				Name:          "write",
				Applications:  []string{},
				Accounts:      []string{"account_0"},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
			&resourcepb.Permission{
				Name:          "execute",
				Applications:  []string{},
				Accounts:      []string{"account_1"},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
		},
		Denied: []*resourcepb.Permission{
			&resourcepb.Permission{
				Name:          "read",
				Applications:  []string{},
				Accounts:      []string{"account_2"},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
		},
		Owners: &resourcepb.Permission{
			Name:          "read",
			Applications:  []string{},
			Accounts:      []string{"account_0"},
			Groups:        []string{},
			Peers:         []string{},
			Organizations: []string{},
		},
	}

	err := rbac_client_.SetResourcePermissions(filePath, permissions)
	if err != nil {
		log.Println(err)
	}

}

// Test read a given permission to determine if suject can do given action...
func TestGetResourcePermission(t *testing.T) {

	filePath := "C:/temp/toto.txt"
	_, err := rbac_client_.GetResourcePermission(filePath, "read", resourcepb.PermissionType_ALLOWED)
	if err != nil {
		log.Println(err)
	}

}

func TestSetResourcePermission(t *testing.T) {
	filePath := "C:/temp/toto.txt"
	err := rbac_client_.DeleteResourcePermission(filePath, "execute", resourcepb.PermissionType_ALLOWED)
	if err != nil {
		log.Println(err)
	}
}

func TestValidateAccess(t *testing.T) {
	filePath := "C:/temp/toto.txt"

	// Test if account owner can do anything.
	hasPermission_0, err := rbac_client_.ValidateAccess("account_0", resourcepb.SubjectType_ACCOUNT, "read", filePath)
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

	// Test if the owner has permission event the permission is explicitly specify!
	hasPermission_3, err := rbac_client_.ValidateAccess("account_0", resourcepb.SubjectType_ACCOUNT, "execute", filePath)
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
	hasPermission_1, err := rbac_client_.ValidateAccess("account_1", resourcepb.SubjectType_ACCOUNT, "read", filePath)
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
	hasPermission_2, err := rbac_client_.ValidateAccess("account_1", resourcepb.SubjectType_ACCOUNT, "write", filePath)
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
	hasPermission_4, err := rbac_client_.ValidateAccess("account_2", resourcepb.SubjectType_ACCOUNT, "read", filePath)
	if err != nil {
		log.Println(err)
	}
	if !hasPermission_4 {
		log.Println("account_2 has permission denied to read " + filePath)
	} else {
		log.Println("account_2 can read  " + filePath)
		t.Fail()
	}
}

// Test delete a specific ressource permission...
func TestDeleteResourcePermissions(t *testing.T) {
	t.FailNow()
}

func TestDeleteResourcePermission(t *testing.T) {
	t.FailNow()
}

func TestAddResourceOwner(t *testing.T) {
	t.FailNow()
}

func TestRemoveResourceOwner(t *testing.T) {
	t.FailNow()
}

func TestDeleteAllAccess(t *testing.T) {
	t.FailNow()
}

func TestResetResources(t *testing.T) {

	err := resource_client_.DeleteAccount("account_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the group
	err = resource_client_.DeleteGroup("group_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the organization.
	err = resource_client_.DeleteOrganization("organization_0")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeletePeer("p0.test.com")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeleteAccount("account_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeleteAccount("account_2")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the group
	err = resource_client_.DeleteGroup("group_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	// Delete the organization.
	err = resource_client_.DeleteOrganization("organization_1")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

	err = resource_client_.DeletePeer("p1.test.com")
	if err != nil {
		log.Println(err)
		t.Fail()
	}

}
