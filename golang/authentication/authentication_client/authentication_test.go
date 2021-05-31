package authentication_client

import (

	"testing"
	"log"
)

var (
	// Connect to the plc client.
	client, _       = NewAuthenticationService_Client("localhost", "authentication.AuthenticationService")
	//rbac_client_, _ = rbac_client.NewRbacService_Client("localhost", "resource.RbacService")

	//token string // the token use by test.
)

// Test various function here.
func Test(t *testing.T) {

	t.Log("Test")
}



func TestAuthenticate(t *testing.T) {
	token, err := client.Authenticate("dave", "400zm89AaB")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Authenticate succeed", token)
	}
}

func TestSetPassword(t *testing.T) {
	token, err := client.SetPassword("dave", "400zm89AaB", "400zm89AaB")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Password is updated ", token)
	}
}