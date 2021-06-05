package authentication_client

import (
	"log"
	"testing"
)

var (
	// Connect to the plc client.
	client, _ = NewAuthenticationService_Client("globular.cloud", "authentication.AuthenticationService")
)

// Test various function here.
func Test(t *testing.T) {

	t.Log("Test")
}

func TestAuthenticate(t *testing.T) {
	log.Println("Test authethicate")
	token, err := client.Authenticate("dave", "1234")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Authenticate succeed", token)
	}

	// Now I will test to authenticate the root...
	token, err = client.Authenticate("sa", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Authenticate succeed", token)
	}
}


func TestSetPassword(t *testing.T) {
	token, err := client.SetPassword("dave", "400zm89Aaa", "400zm89Aaa")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Password is updated ", token)
	}
}

func TestSetRootPassword(t *testing.T) {
	token, err := client.SetRootPassword("adminadmin", "adminadmin")
	if err != nil {
		log.Println("Fail to authenticate with error ", err)
	} else {
		log.Println("Password is updated ", token)
	}
}

