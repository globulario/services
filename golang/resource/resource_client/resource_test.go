package resource_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
)

var (
	// Connect to the plc client.
	domain                    = "globule-dell.globular.cloud"
	client, _                 = NewResourceService_Client(domain, "resource.ResourceService")
	authentication_client_, _ = authentication_client.NewAuthenticationService_Client(domain, "authentication.AuthenticationService")
	//token, _                  = authentication_client_.Authenticate("sa", "adminadmin")
)

/** Test create account **/
/** Test create Organization */
func TestCreateOrganization(t *testing.T) {
	authentication_client_.Authenticate("sa", "adminadmin")
	log.Println("---> test create organization.")
	err := client.CreateOrganization("globulario", "globulario", "globulario@globuar.io", "description", "")
	if err != nil {
		log.Println("---> create organization fail! ", err)
	} else {
		log.Println("successed create organization globulario")
	}
}

/** Test create account **/
func TestCreateAccount(t *testing.T) {
	err := client.RegisterAccount(domain, "dave", "dave", "dave@globular.io", "1234", "1234")
	if err != nil {
		log.Println("---> create account fail! ", err)
	} else {
		log.Println("---> dave account was created!")
	}
}

/** Test create group **/
func TestCreateGroup(t *testing.T) {
	err := client.CreateGroup("group_0", "group_0", "test")
	if err != nil {
		log.Println("---> create group group_0 fail! ", err)
	} else {
		log.Println("---> create group_0 succeed!")
	}
}

/** Test Add account, group and role to the organization **/
func TestAddToOrganization(t *testing.T) {
	client.AddOrganizationAccount("globulario", "dave")
	client.AddOrganizationRole("globulario", "db_user")
	client.AddOrganizationGroup("globulario", "group_0")
}

func TestRemoveFromOrganization(t *testing.T) {
	client.RemoveOrganizationAccount("globulario", "dave")
	client.RemoveOrganizationRole("globulario", "db_user")
	client.RemoveOrganizationGroup("globulario", "group_0")
}

func TestAddGroupMemberAccount(t *testing.T) {
	err := client.AddGroupMemberAccount("group_0", "dave")
	if err != nil {
		log.Println("---> add group member group_0 fail! ", err)
	} else {
		log.Println("---> add group memeber dave to  group_0 succssed!")
	}
}

func TestGetGroups(t *testing.T) {
	groups, err := client.GetGroups(`{"_id":"group_0"}`)
	if err != nil {
		log.Println("---> get group group_0 fail! ", err)
	} else {
		log.Println("---> get group_0 succeed! ", groups)
	}
}

func TestCreateRole(t *testing.T) {
	log.Println("---> create role ")
	err := client.CreateRole("db_user", "db_user", []string{
		"/persistence.PersistenceService/InsertOne",
		"/persistence.PersistenceService/InsertMany",
		"/persistence.PersistenceService/Find",
		"/persistence.PersistenceService/FindOne",
		"/persistence.PersistenceService/ReplaceOne",
		"/persistence.PersistenceService/DeleteOne",
		"/persistence.PersistenceService/Delete",
		"/persistence.PersistenceService/Count",
		"/persistence.PersistenceService/Update",
		"/persistence.PersistenceService/UpdateOne"})
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestRemoveMemberAccount(t *testing.T) {
	err := client.RemoveGroupMemberAccount("group_0", "dave")
	if err != nil {
		log.Println("---> remove group group_0 fail! ", err)
	} else {
		log.Println("---> remove group_0 succeed!")
	}
}

func TestDeleteGroup(t *testing.T) {
	err := client.DeleteGroup("group_0")
	if err != nil {
		log.Println("---> delete group group_0 fail! ", err)
	} else {
		log.Println("---> delete group_0 succeed!")
	}
}

func TestAddRoleActions(t *testing.T) {
	log.Println("---> Add Role action ")
	err := client.AddRoleActions("db_user", []string{"/file.FileService/ReadDir"})
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestAddAccountRole(t *testing.T) {
	log.Println("---> Add account Role ")
	err := client.AddAccountRole("dave", "db_user")
	if err != nil {
		log.Println("---> ", err)
	}

	err = client.AddAccountRole("dave", "globular_user")
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestValidateAction(t *testing.T) {
	/*
		infos, err := client.GetActionResourceInfos("/file.FileService/ReadDir")
		if err != nil {
			log.Println("---> ", err)
			return
		}

		// Set the path to read...
		path := "/tmp/test.txt"
		infos[0].Path = path

		// Give permission.
		permissions := &resourcepb.Permissions{
			Allowed: []*resourcepb.Permission{
				&resourcepb.Permission{
					Name:          "read",
					Applications:  []string{},
					Accounts:      []string{"dave"},
					Groups:        []string{},
					Peers:         []string{},
					Organizations: []string{},
				},
			},
			Denied: []*resourcepb.Permission{},
			Owners: &resourcepb.Permission{},
		}

		rbac_client_.SetResourcePermissions(path, permissions)

		hasAccess, err := client.ValidateAction("/file.FileService/ReadDir", "dave", resourcepb.SubjectType_ACCOUNT, infos)
		if err != nil {
			log.Println(err)
		}

		if hasAccess {
			log.Println("----------> dave has access to read file " + path)
		} else {
			log.Println("----------> dave has not access to read file " + path)
		}
	*/
}

func TestRemoveAccountRole(t *testing.T) {
	err := client.RemoveAccountRole("dave", "db_user")
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestRemoveRoleAction(t *testing.T) {
	log.Println("---> Remove Role action ")
	err := client.RemoveRoleAction("db_user", "/file.FileService/ReadDir")
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestDeleteRole(t *testing.T) {
	log.Println("---> Delete role ")
	err := client.DeleteRole("db_user")
	if err != nil {
		log.Println("---> ", err)
	}
}

/*
// Remove an account.
func TestDeleteAccount(t *testing.T) {
	log.Println("---> test remove existing account.")
	err := client.DeleteAccount("dave")
	if err != nil {
		log.Println("---> ", err)
	}
}


func TestDeleteOrganization(t *testing.T) {
	log.Println("---> test delete organization")
	err := client.DeleteOrganization("globulario")
	if err != nil {
		log.Println("---> ", err)
	}
}
*/
