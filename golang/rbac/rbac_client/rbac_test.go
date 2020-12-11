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

func TestSetResourcePermissions(t *testing.T) {

	// A fictive file path...
	filePath := "C:/temp/toto.txt"

	permissions := &resourcepb.Permissions{
		Allowed: []*resourcepb.Permission{
			&resourcepb.Permission{
				Name:          "read",
				Applications:  []string{},
				Accounts:      []string{"dave"},
				Groups:        []string{"group_0"},
				Peers:         []string{"peer_0"},
				Organizations: []string{"peer_0"},
			},
		},
		Denied: []*resourcepb.Permission{
			&resourcepb.Permission{
				Name:          "read",
				Applications:  []string{},
				Accounts:      []string{"dave"},
				Groups:        []string{"group_0"},
				Peers:         []string{"peer_0"},
				Organizations: []string{"peer_0"},
			},
		},
		Owners: &resourcepb.Permission{
			Name:          "read",
			Applications:  []string{},
			Accounts:      []string{"dave"},
			Groups:        []string{"group_0"},
			Peers:         []string{"peer_0"},
			Organizations: []string{"peer_0"},
		},
	}

	err := client.SetResourcePermissions(filePath, permissions)
	if err != nil {
		log.Println(err)
	}

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
