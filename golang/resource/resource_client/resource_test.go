package resource_client

import (
	//"encoding/json"
	"log"
	"testing"
)

var (
	// Connect to the plc client.
	client, _ = NewResourceService_Client("localhost", "resource.ResourceService")
	token     string
)

func TestAuthenticate(t *testing.T) {
	log.Println("---> test authenticate account.")
	var err error
	token, err = client.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("---> ", err)
	} else {
		log.Println("---> ", token)
	}
}

/** Test create account **/
func TestRegisterAccount(t *testing.T) {
	err := client.RegisterAccount("dave", "dave@globular.io", "1234", "1234")
	if err != nil {
		log.Println("---> create account fail! ", err)
	} else {
		log.Println("---> dave account was created!")
	}
}

/** Test create group **/
func TestRegisterGroup(t *testing.T) {
	err := client.CreateGroup("group_0", "group_0")
	if err != nil {
		log.Println("---> create group group_0 fail! ", err)
	} else {
		log.Println("---> create group_0 succed!")
	}
}

func TestAddGroupMemberAccount(t *testing.T) {
	err := client.AddGroupMemberAccount("group_0", "dave")
	if err != nil {
		log.Println("---> create group group_0 fail! ", err)
	} else {
		log.Println("---> create group_0 succed!")
	}
}

func TestRemoveMemberAccount(t *testing.T) {
	err := client.RemoveGroupMemberAccount("group_0", "dave")
	if err != nil {
		log.Println("---> remove group group_0 fail! ", err)
	} else {
		log.Println("---> remove group_0 succed!")
	}
}

func TestGetGroups(t *testing.T) {
	err := client.GetGroups("group_0")
	if err != nil {
		log.Println("---> get group group_0 fail! ", err)
	} else {
		log.Println("---> remove group_0 succed!")
	}
}

/*
func TestCreateRole(t *testing.T) {
	log.Println("---> create role ")
	err := client.CreateRole("db_user", []string{"/persistence.PersistenceService/InsertOne", "/persistence.PersistenceService/InsertMany", "/persistence.PersistenceService/Find", "/persistence.PersistenceService/FindOne", "/persistence.PersistenceService/ReplaceOne", "/persistence.PersistenceService/DeleteOne", "/persistence.PersistenceService/Delete", "/persistence.PersistenceService/Count", "/persistence.PersistenceService/Update", "/persistence.PersistenceService/UpdateOne"})
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestAddRoleAction(t *testing.T) {
	log.Println("---> Add Role action ")
	err := client.AddRoleAction("db_user", "/toto")
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestRemoveRoleAction(t *testing.T) {
	log.Println("---> Remove Role action ")
	err := client.RemoveRoleAction("db_user", "/toto")
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestAddAccountRole(t *testing.T) {
	log.Println("---> Add account Role ")
	err := client.AddAccountRole("davecourtois", "db_user")
	if err != nil {
		log.Println("---> ", err)
	}
}

func TestRemoveAccountRole(t *testing.T) {
	log.Println("---> Remove account Role ")
	err := client.RemoveAccountRole("davecourtois", "db_user")
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
*/
// Remove an account.
/*
func TestDeleteAccount(t *testing.T) {

	log.Println("---> test remove existing account.")
	err := client.DeleteAccount("dave")
	if err != nil {

		log.Println("---> ", err)
	}
}
*/
/** Get the root file informations **/
/*func TestGetAllFilesInfo(t *testing.T) {
	log.Println("---> Get All File Info")
	infos, err := client.GetAllFilesInfo()
	if err != nil {
		log.Println("---> ", err)
		return
	}
	log.Println(infos)
}*/
