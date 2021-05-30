package resource_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resourcepb"
)

var (
	// Connect to the plc client.
	client, _       = NewResourceService_Client("localhost", "resource.ResourceService")
	rbac_client_, _ = rbac_client.NewRbacService_Client("localhost", "resource.RbacService")

	token string // the token use by test.
)

