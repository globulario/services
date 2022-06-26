package applications_manager_client

import (
	//"encoding/json"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"log"
	"testing"
)

// Test various function here.
var (
	client, _ = NewApplicationsManager_Client("globular.cloud", "applications_manager.ApplicationManagerService")
	authenticator, _ = authentication_client.NewAuthenticationService_Client("globular.cloud", "authentication.AuthenticationService")
)
func TestDeployApplication(t *testing.T) {
	token, err := authenticator.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}
	size, err := client.DeployApplication("sa", "home", "", "/home/dave/home/dist", token, "globular.cloud", false)
	if err != nil {
		log.Println(err)
	}
	log.Println("the application was deployed with size ", size)
}