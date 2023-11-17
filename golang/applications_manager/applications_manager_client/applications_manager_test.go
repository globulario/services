package applications_manager_client

import (
	//"encoding/json"
	"log"
	"testing"

	"github.com/globulario/services/golang/authentication/authentication_client"
)

// Test various function here.
var (
	client, _ = NewApplicationsManager_Client("globule-ryzen.globular.cloud:443", "applications_manager.ApplicationManagerService")
	authenticator, _ = authentication_client.NewAuthenticationService_Client("globule-ryzen.globular.cloud:443", "authentication.AuthenticationService")
)
func TestInstallApplication(t *testing.T) {
	token, err := authenticator.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println(err)
		return
	}


	err = client.InstallApplication(token, "globular.cloud", "sa", "globule-ryzen.globular.cloud:443", "sa@globular.cloud", "console", false)
	if err != nil {
		log.Println(err)
	}

}