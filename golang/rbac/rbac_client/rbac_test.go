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

func TestSetActionPermission(t *testing.T) {
	action := ""
	resources := make([]*resourcepb.ActionResourceParameterPermission, 0)

	err := client.SetActionPermission(action, resources)
	if err != nil {
		log.Panicln("error ", err)
	}
}

func TestGetActionPermission(t *testing.T) {
	t.FailNow()
}

func TestSetResourcePermissions(t *testing.T) {
	t.FailNow()
}

func TestDeleteResourcePermissions(t *testing.T) {
	t.FailNow()
}

func TestDeleteResourcePermission(t *testing.T) {
	t.FailNow()
}

func TestSetResourcePermission(t *testing.T) {
	t.FailNow()
}

func TestGetResourcePermission(t *testing.T) {
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

func TestValidateAccess(t *testing.T) {
	t.FailNow()
}

func TestGetAccesses(t *testing.T) {
	t.FailNow()
}
