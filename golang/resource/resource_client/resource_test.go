package resource_client

import (
	//"encoding/json"
	"testing"
	"log"

	//"github.com/globulario/services/golang/rbac/rbac_client"
	// "github.com/globulario/services/golang/resource/resourcepb"
)

var (
	// Connect to the plc client.
	client, _       = NewResourceService_Client("localhost", "resource.ResourceService")
	//rbac_client_, _ = rbac_client.NewRbacService_Client("localhost", "resource.RbacService")

	//token string // the token use by test.
)

/** Test create account **/
func TestCreateAccount(t *testing.T) {
	log.Println("run test create account...")
	err := client.RegisterAccount("dave", "dave@globular.io", "1234", "1234")
	if err != nil {
		log.Println("fail to run test with error ", err)
	} else {
		t.Log("account was created!")
	}
}
