package rbac_client

import (
	"log"
	"testing"

	"github.com/globulario/services/golang/resource/resourcepb"
)

var (
	// Connect to the services client.
	client, _ = NewRbacService_Client("localhost", "resource.RbacService")
)

func TestSetActionResourcesPermission(t *testing.T) {
	action := ""
	resources := make([]*resourcepb.ActionResourceParameterPermission, 0)

	err := client.SetActionResourcesPermission(action, resources)
	if err != nil {
		log.Panicln("error ", err)
	}
}

func TestGetActionResourcesPermission(t *testing.T) {

}

func TestSetResourcePermissions(t *testing.T) {

}

func TestDeleteResourcePermissions(t *testing.T) {

}

func TestDeleteResourcePermission(t *testing.T) {

}

func TestSetResourcePermission(t *testing.T) {

}

func TestGetResourcePermission(t *testing.T) {

}

func TestAddResourceOwner(t *testing.T) {

}

func TestRemoveResourceOwner(t *testing.T) {

}

func TestDeleteAllAccess(t *testing.T) {

}

func TestValidateAccess(t *testing.T) {

}

func TestGetAccesses(t *testing.T) {

}
